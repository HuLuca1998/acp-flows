package logging_test

// 日志基础设施。规范见 docs/rules/logging.md。
//
// 日志是 AI 调试时的唯一观测面——人可以 attach 调试器、看界面、凭经验猜，
// AI 只能看日志。所以这里的断言比一般项目严：关联字段必须自动带、
// 落库失败不能影响业务、级别按域可调。

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/platform/logging"
)

// R1 · 五个级别齐备，且 TRACE 低于 DEBUG。
func TestLevels_R1_FiveLevels(t *testing.T) {
	got := []slog.Level{
		logging.LevelTrace, slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError,
	}
	for i := 1; i < len(got); i++ {
		if got[i-1] >= got[i] {
			t.Fatalf("级别顺序不对: %v >= %v", got[i-1], got[i])
		}
	}
	if logging.LevelTrace != -8 {
		t.Errorf("LevelTrace = %d, want -8（docs/rules/logging.md §2）", logging.LevelTrace)
	}
}

// R2 ★ 按域调级别：DUET_LOG 的格式解析。
//
// 一次调试通常只关心一个组件，全局调 debug 会淹没在噪音里。
func TestParseLevels_R2_PerComponent(t *testing.T) {
	tests := []struct {
		name      string
		spec      string
		wantGlob  slog.Level
		wantComp  map[string]slog.Level
		wantError bool
	}{
		{"空串用默认", "", slog.LevelInfo, map[string]slog.Level{}, false},
		{"只有全局", "debug", slog.LevelDebug, map[string]slog.Level{}, false},
		{"只有组件", "acp=trace", slog.LevelInfo,
			map[string]slog.Level{"acp": logging.LevelTrace}, false},
		{"全局 + 多组件", "info,acp=trace,store=debug", slog.LevelInfo,
			map[string]slog.Level{"acp": logging.LevelTrace, "store": slog.LevelDebug}, false},
		{"大小写不敏感", "WARN,ACP=Debug", slog.LevelWarn,
			map[string]slog.Level{"acp": slog.LevelDebug}, false},
		{"空白容忍", " debug , acp = trace ", slog.LevelDebug,
			map[string]slog.Level{"acp": logging.LevelTrace}, false},
		{"未知级别名要报错", "bogus", 0, nil, true},
		{"组件的未知级别名要报错", "acp=bogus", 0, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			glob, comp, err := logging.ParseLevels(tt.spec)
			if tt.wantError {
				if err == nil {
					t.Fatalf("期望报错，但解析成功了: %v %v", glob, comp)
				}
				// 错误信息要给出可用取值，否则用户不知道该写什么
				if !strings.Contains(err.Error(), "trace") {
					t.Errorf("错误信息未列出可用级别: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("解析失败: %v", err)
			}
			if glob != tt.wantGlob {
				t.Errorf("全局级别 = %v, want %v", glob, tt.wantGlob)
			}
			if len(comp) != len(tt.wantComp) {
				t.Fatalf("组件数 = %d, want %d (%v)", len(comp), len(tt.wantComp), comp)
			}
			for k, v := range tt.wantComp {
				if comp[k] != v {
					t.Errorf("组件 %s = %v, want %v", k, comp[k], v)
				}
			}
		})
	}
}

// R3 ★ 关联字段从 context 自动继承。
//
// 日志的价值在于能把一件事的全过程串起来。手动传字段必然会漏。
func TestContext_R3_FieldsInherited(t *testing.T) {
	var buf bytes.Buffer
	base := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: logging.LevelTrace}))

	ctx := logging.With(context.Background(), "work_id", "work-08")
	ctx = logging.With(ctx, "unit_id", "unit-012")

	logging.FromContextWith(ctx, base).Info("单元契约已冻结", "version", 3)

	out := buf.String()
	for _, want := range []string{"work-08", "unit-012", "单元契约已冻结", `"version":3`} {
		if !strings.Contains(out, want) {
			t.Errorf("日志里缺少 %q\n实际: %s", want, out)
		}
	}
}

// 没有 context 字段时也要能正常打，不能 panic。
func TestContext_EmptyIsSafe(t *testing.T) {
	var buf bytes.Buffer
	base := slog.New(slog.NewJSONHandler(&buf, nil))
	logging.FromContextWith(context.Background(), base).Info("没有关联字段")
	if !strings.Contains(buf.String(), "没有关联字段") {
		t.Error("空 context 下日志没打出来")
	}
}

// R4 ★ 落库失败不能影响业务。
//
// SQLite 忙、磁盘满、表被锁——都只降级为「只写 stderr」，绝不向上抛错。
// 日志系统挂掉不该让产品挂掉。
func TestSink_R4_FailureDoesNotPropagate(t *testing.T) {
	var stderr bytes.Buffer
	sink := &failingSink{}

	h := logging.NewHandler(logging.Options{
		Stderr:       &stderr,
		Sink:         sink,
		GlobalLevel:  slog.LevelInfo,
		SyncForTests: true,
	})
	log := slog.New(h)

	// 这一行如果 panic 或阻塞，测试就挂了——那正是我们要防的
	log.Error("业务出错了", "err", "boom")

	if sink.calls == 0 {
		t.Error("sink 根本没被调用")
	}
	if !strings.Contains(stderr.String(), "业务出错了") {
		t.Errorf("落库失败后 stderr 也没写: %q", stderr.String())
	}
}

// R5 · 组件级别覆盖全局级别。
func TestHandler_R5_ComponentLevelOverridesGlobal(t *testing.T) {
	var stderr bytes.Buffer
	sink := &recordingSink{}

	h := logging.NewHandler(logging.Options{
		Stderr:          &stderr,
		Sink:            sink,
		GlobalLevel:     slog.LevelWarn, // 全局只要 warn 以上
		ComponentLevels: map[string]slog.Level{"acp": logging.LevelTrace},
		SyncForTests:    true,
	})

	slog.New(h).With("component", "acp").Log(context.Background(), logging.LevelTrace, "协议报文")
	slog.New(h).With("component", "store").Debug("这条应该被全局级别挡掉")

	if len(sink.msgs) != 1 {
		t.Fatalf("落库条数 = %d, want 1\n实际: %v", len(sink.msgs), sink.msgs)
	}
	if sink.msgs[0] != "协议报文" {
		t.Errorf("落库的是 %q，组件级别没生效", sink.msgs[0])
	}
}

// R6 · TRACE 与 DEBUG 不进 stderr（除非显式调低），但都落库。
func TestHandler_R6_TraceNotOnStderr(t *testing.T) {
	var stderr bytes.Buffer
	sink := &recordingSink{}

	h := logging.NewHandler(logging.Options{
		Stderr:          &stderr,
		Sink:            sink,
		GlobalLevel:     slog.LevelInfo,
		ComponentLevels: map[string]slog.Level{"acp": logging.LevelTrace},
		SyncForTests:    true,
	})
	slog.New(h).With("component", "acp").Log(context.Background(), logging.LevelTrace, "报文全文")

	if strings.Contains(stderr.String(), "报文全文") {
		t.Error("TRACE 进了 stderr —— 它量很大，会把生命周期日志淹掉")
	}
	if len(sink.msgs) != 1 {
		t.Errorf("TRACE 没落库（落库应该收全部级别）")
	}
}

// ── 测试替身 ────────────────────────────────────────────────

type failingSink struct{ calls int }

func (s *failingSink) Write(logging.Entry) { s.calls++ } // 内部吞掉错误，签名上就不返回 error
func (s *failingSink) Close() error        { return nil }

type recordingSink struct{ msgs []string }

func (s *recordingSink) Write(e logging.Entry) { s.msgs = append(s.msgs, e.Message) }
func (s *recordingSink) Close() error          { return nil }
