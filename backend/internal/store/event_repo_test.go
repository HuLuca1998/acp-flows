package store_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/store"
	"github.com/HuLuca1998/acp-flows/backend/tests/testutil"
)

// U2.3.1 · 事件落库（验收点 V6 的 R1 R2）
//
// 用真 SQLite 临时文件。「序号跨重启连续」问的正是磁盘上的东西——
// 用 :memory: 的话这条根本测不了。

// ★ 用 store.Event 而不是 eventbus.Event：两者字段一致，但**具名结构体
// 之间不能互相赋值**——Go 的结构化类型只对 interface 生效。
// 这份重复是 depguard 的 infra 规则要的：基础设施之间不互相依赖。
// 接缝在 cmd 层（唯一做装配的地方），由它把两个类型接起来。
func newEvent(typ string) *store.Event {
	return &store.Event{
		ID: "evt_0001", WorkID: "work-01", Source: "acp", Type: typ,
		TS: testutil.T0, Payload: json.RawMessage(`{"text":"hi"}`),
	}
}

// ★ R1：序号由**数据库**发放，写回 e.Seq。
func TestEventRepo_AssignsSeq(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	repo := s.Events()

	for i := range 5 {
		e := newEvent("message_chunk")
		if err := repo.AppendEvent(ctx, e); err != nil {
			t.Fatalf("落第 %d 条: %v", i, err)
		}
		if e.Seq != int64(i+1) {
			t.Fatalf("第 %d 条的 seq = %d，想要 %d——序号没写回或有洞", i, e.Seq, i+1)
		}
	}
}

// ★★ R1 的关键：**序号跨重启连续**。
//
// 关掉 Store 再用同一个文件打开，序号要接着往下发。
// 从 1 重来的话，前端按 seq 去重会把新事件当成旧的丢掉——
// 用户重启应用后，AI 说的话不再显示了。
func TestEventRepo_SeqSurvivesRestart(t *testing.T) {
	paths := testutil.TempPaths(t)
	testutil.GuardPath(t, paths.DBPath())
	ctx := context.Background()

	first, err := store.Open(paths.DBPath(), testutil.FixedClock(testutil.T0))
	if err != nil {
		t.Fatalf("打开: %v", err)
	}
	for range 3 {
		if err := first.Events().AppendEvent(ctx, newEvent("message_chunk")); err != nil {
			t.Fatal(err)
		}
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second, err := store.Open(paths.DBPath(), testutil.FixedClock(testutil.T0))
	if err != nil {
		t.Fatalf("重新打开: %v", err)
	}
	defer func() { _ = second.Close() }()

	e := newEvent("message_chunk")
	if err := second.Events().AppendEvent(ctx, e); err != nil {
		t.Fatal(err)
	}
	if e.Seq != 4 {
		t.Errorf("重启后第一条的 seq = %d，想要 4——从头发的话前端会把新事件当旧的丢掉", e.Seq)
	}
}

// MaxSeq 供启动时接续；空库返回 0。
func TestEventRepo_MaxSeq(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	repo := s.Events()

	if n, err := repo.MaxSeq(ctx); err != nil || n != 0 {
		t.Fatalf("空库 MaxSeq = %d, err = %v，想要 0", n, err)
	}

	for range 7 {
		if err := repo.AppendEvent(ctx, newEvent("tool_call")); err != nil {
			t.Fatal(err)
		}
	}

	n, err := repo.MaxSeq(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 7 {
		t.Errorf("MaxSeq = %d, 想要 7", n)
	}
}

// ★ R2：断线重连**只补发没收到的**。
//
// 从头补的话，用户重连后会看到整条时间线又重放一遍；
// 补少了则中间有洞，而洞是看不出来的——用户不知道自己漏了什么。
func TestEventRepo_EventsAfter(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	repo := s.Events()

	for range 10 {
		if err := repo.AppendEvent(ctx, newEvent("message_chunk")); err != nil {
			t.Fatal(err)
		}
	}

	got, err := repo.EventsAfter(ctx, 6, 100)
	if err != nil {
		t.Fatalf("补发查询: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("从 6 之后补到 %d 条，想要 4 条", len(got))
	}
	if got[0].Seq != 7 {
		t.Errorf("首条 seq = %d，想要 7——从 6 恢复时不该把第 6 条再发一遍", got[0].Seq)
	}
	for i := 1; i < len(got); i++ {
		if got[i].Seq != got[i-1].Seq+1 {
			t.Errorf("补发的序号有洞：%d 之后是 %d", got[i-1].Seq, got[i].Seq)
		}
	}

	// 全新连接（从 0 开始）拿到全部
	all, err := repo.EventsAfter(ctx, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 10 {
		t.Errorf("从 0 补到 %d 条，想要 10 条", len(all))
	}
}

// 补发要有上限：一个断了一整天的客户端重连时，不该把几万条一次性灌给它。
func TestEventRepo_EventsAfterRespectsLimit(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	repo := s.Events()

	for range 20 {
		if err := repo.AppendEvent(ctx, newEvent("message_chunk")); err != nil {
			t.Fatal(err)
		}
	}

	got, err := repo.EventsAfter(ctx, 0, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 5 {
		t.Errorf("limit=5 却拿到 %d 条", len(got))
	}
}

// 载荷原样存回原样取出——它是界面上真正显示的内容。
func TestEventRepo_PayloadRoundTrips(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	repo := s.Events()

	e := newEvent("message_chunk")
	e.Payload = json.RawMessage(`{"text":"你好，世界","nested":{"n":1}}`)
	if err := repo.AppendEvent(ctx, e); err != nil {
		t.Fatal(err)
	}

	got, err := repo.EventsAfter(ctx, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("取回 %d 条", len(got))
	}

	var payload struct {
		Text   string `json:"text"`
		Nested struct {
			N int `json:"n"`
		} `json:"nested"`
	}
	if err := json.Unmarshal(got[0].Payload, &payload); err != nil {
		t.Fatalf("载荷解析失败: %v（原文 %s）", err, got[0].Payload)
	}
	if payload.Text != "你好，世界" || payload.Nested.N != 1 {
		t.Errorf("载荷变了：%+v", payload)
	}
}
