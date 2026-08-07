package session_test

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/acp/fake"
	"github.com/HuLuca1998/acp-flows/backend/internal/acp/protocol"
	"github.com/HuLuca1998/acp-flows/backend/internal/acp/session"
	"github.com/HuLuca1998/acp-flows/backend/tests/testutil"
)

// U2.2.2 · 一轮完整会话（验收点 V5）
//
// ★ 对手方是 Fake ACP Runtime，不是 mock。Fake 按官方规范独立说话
// （它只许 import protocol），所以「我们理解错了协议」这件事测得出来——
// 用 mock 的话，mock 和被测代码会一起错、测试照样绿。

func newFake(t *testing.T, script *fake.Script) *fake.Runtime {
	t.Helper()
	rt := fake.New(fake.Options{Script: script, Clock: testutil.FixedClock(testutil.T0)})
	t.Cleanup(func() { _ = rt.Close() })
	return rt
}

// emit 造一条 SessionUpdate 载荷。
func emit(t *testing.T, kind protocol.SessionUpdateKind, extra map[string]any) json.RawMessage {
	t.Helper()
	payload := map[string]any{"sessionUpdate": string(kind)}
	for k, v := range extra {
		payload[k] = v
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func textChunk(t *testing.T, kind protocol.SessionUpdateKind, text string) json.RawMessage {
	t.Helper()
	return emit(t, kind, map[string]any{
		"content": map[string]any{"type": "text", "text": text},
	})
}

// R4：工作目录必须是**已存在的绝对路径**，且校验失败时不发出建会话请求。
//
// ★ 「未发出」这半条同样重要：先发请求再校验的话，Agent 那边已经开了一个
// 会话，而我们这边报了错——它会挂在那儿占着资源，没人再去关它。
func TestOpen_ValidatesCwdBeforeAnyRequest(t *testing.T) {
	tests := []struct {
		name string
		cwd  string
	}{
		{"相对路径", "work/my-app"},
		{"空路径", ""},
		{"不存在的目录", "/tmp/definitely-not-here-9c8f"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := newFake(t, &fake.Script{Name: "cwd", Turns: []fake.Turn{{
				StopReason: protocol.StopReasonEndTurn,
			}}})

			_, err := session.Open(context.Background(), session.Options{
				Transport: rt.Transport(),
				Cwd:       tt.cwd,
			})
			if err == nil {
				t.Fatal("非法工作目录却开成功了")
			}
			if n := rt.CountMethod("session/new"); n != 0 {
				t.Errorf("校验失败却已经发了 %d 次 session/new——"+
					"Agent 那边会挂着一个没人关的会话", n)
			}
		})
	}
}

// R1：**真流式**。第一个字要远早于整轮结束。
//
// 攒完再吐的话，用户盯着一个不动的界面等好几秒，而 AI 其实早就开口了。
// 这条是 V5 的用户可感部分，不是内部优化。
func TestPrompt_StreamsFirstChunkLongBeforeTurnEnds(t *testing.T) {
	rt := newFake(t, &fake.Script{
		Name: "stream",
		Turns: []fake.Turn{{
			Steps: []fake.Step{
				{Emit: textChunk(t, protocol.UpdateAgentMessageChunk, "我先说第一句")},
			},
			// 首块之后隔很久才结束这一轮
			StopReason: protocol.StopReasonEndTurn,
			StopDelay:  fake.Dur(2 * time.Second),
		}},
	})

	s, err := session.Open(context.Background(), session.Options{
		Transport: rt.Transport(),
		Cwd:       t.TempDir(),
	})
	if err != nil {
		t.Fatalf("开会话失败: %v", err)
	}

	start := time.Now()
	firstAt := make(chan time.Duration, 1)
	var once sync.Once

	_, err = s.Prompt(context.Background(), "帮我加个功能", func(session.Event) {
		once.Do(func() { firstAt <- time.Since(start) })
	})
	if err != nil {
		t.Fatalf("prompt 失败: %v", err)
	}

	select {
	case d := <-firstAt:
		if d > 500*time.Millisecond {
			t.Errorf("第一块用了 %v 才到——攒完再吐的话用户会盯着不动的界面等", d)
		}
	default:
		t.Fatal("一个事件都没收到")
	}
	if total := time.Since(start); total < 1500*time.Millisecond {
		t.Errorf("整轮只用了 %v，Fake 的 2 秒延迟没生效，这条测试什么也没证明", total)
	}
}

// R2：五种结束原因，**只有 end_turn 算正常**。
//
// max_tokens 意味着产出是半成品——当成成功的话，用户会拿到一个截断的改动
// 而界面显示「完成」。refusal 是模型拒绝执行，更不能算成功。
func TestPrompt_OnlyEndTurnIsSuccess(t *testing.T) {
	tests := []struct {
		reason  protocol.StopReason
		wantErr bool
	}{
		{protocol.StopReasonEndTurn, false},
		{protocol.StopReasonMaxTokens, true},
		{protocol.StopReasonMaxTurnRequests, true},
		{protocol.StopReasonRefusal, true},
		{protocol.StopReasonCancelled, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.reason), func(t *testing.T) {
			rt := newFake(t, &fake.Script{
				Name:  "stop",
				Turns: []fake.Turn{{StopReason: tt.reason}},
			})

			s, err := session.Open(context.Background(), session.Options{
				Transport: rt.Transport(), Cwd: t.TempDir(),
			})
			if err != nil {
				t.Fatalf("开会话失败: %v", err)
			}

			got, err := s.Prompt(context.Background(), "做点事", func(session.Event) {})

			if got != tt.reason {
				t.Errorf("StopReason = %q, 想要 %q", got, tt.reason)
			}
			if tt.wantErr && err == nil {
				t.Errorf("%s 被当成了成功——用户会看到「完成」而产出其实是半成品", tt.reason)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("end_turn 却报错了: %v", err)
			}
		})
	}
}

// R5：**13 类事件每一类都有去处**（显式丢弃也算）。
//
// 穷举 protocol 里登记的全部判别值：新增一类而 session 没接住时，
// 这条会红。没有它的话，新事件会静默消失——界面上表现为「AI 好像少说了点什么」，
// 而没有任何报错。
func TestPrompt_EveryUpdateKindIsHandled(t *testing.T) {
	kinds := protocol.AllSessionUpdateKinds()
	if len(kinds) != 13 {
		t.Fatalf("protocol 登记了 %d 类事件，本测试假设 13 类——先确认哪边对", len(kinds))
	}

	steps := make([]fake.Step, 0, len(kinds))
	for _, k := range kinds {
		steps = append(steps, fake.Step{Emit: emit(t, k, nil)})
	}

	rt := newFake(t, &fake.Script{
		Name:  "all-kinds",
		Turns: []fake.Turn{{Steps: steps, StopReason: protocol.StopReasonEndTurn}},
	})

	s, err := session.Open(context.Background(), session.Options{
		Transport: rt.Transport(), Cwd: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("开会话失败: %v", err)
	}

	var mu sync.Mutex
	seen := map[protocol.SessionUpdateKind]bool{}
	unknown := []string{}

	if _, err := s.Prompt(context.Background(), "全都来一遍", func(e session.Event) {
		mu.Lock()
		defer mu.Unlock()
		seen[e.Kind] = true
		if e.Unhandled {
			unknown = append(unknown, string(e.Kind))
		}
	}); err != nil {
		t.Fatalf("prompt 失败: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	for _, k := range kinds {
		if !seen[k] {
			t.Errorf("事件 %q 没有被交出来——它会静默消失，界面上像是 AI 少说了点什么", k)
		}
	}
	if len(unknown) > 0 {
		t.Errorf("这些事件没有对应处理：%s", strings.Join(unknown, ", "))
	}
}

// 文本块要能原样取到——这是界面上「边说边显示」的内容本身。
func TestPrompt_CarriesTextThrough(t *testing.T) {
	rt := newFake(t, &fake.Script{
		Name: "text",
		Turns: []fake.Turn{{
			Steps: []fake.Step{
				{Emit: textChunk(t, protocol.UpdateAgentMessageChunk, "这一句")},
				{Emit: textChunk(t, protocol.UpdateAgentThoughtChunk, "那一句")},
			},
			StopReason: protocol.StopReasonEndTurn,
		}},
	})

	s, err := session.Open(context.Background(), session.Options{
		Transport: rt.Transport(), Cwd: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("开会话失败: %v", err)
	}

	var mu sync.Mutex
	texts := map[protocol.SessionUpdateKind]string{}
	if _, err := s.Prompt(context.Background(), "说两句", func(e session.Event) {
		mu.Lock()
		defer mu.Unlock()
		texts[e.Kind] += e.Text
	}); err != nil {
		t.Fatalf("prompt 失败: %v", err)
	}

	// ★ 读也要加锁：onEvent 跑在 jsonrpc 的读 goroutine 上，
	// 只锁写那一侧的话 -race 会报（第一版就是这么写的）。
	mu.Lock()
	defer mu.Unlock()
	if got := texts[protocol.UpdateAgentMessageChunk]; got != "这一句" {
		t.Errorf("消息文本 = %q", got)
	}
	// 思考摘要与正式消息**不能混成一路**：界面上它们是两种显示
	if got := texts[protocol.UpdateAgentThoughtChunk]; got != "那一句" {
		t.Errorf("思考文本 = %q", got)
	}
}

// 会话开好之后要能拿到 sessionID——后续的取消、恢复都靠它。
func TestOpen_ReturnsSessionID(t *testing.T) {
	rt := newFake(t, &fake.Script{
		Name:       "id",
		NewSession: &fake.NewSessionBehavior{SessionID: "sess-abc"},
		Turns:      []fake.Turn{{StopReason: protocol.StopReasonEndTurn}},
	})

	s, err := session.Open(context.Background(), session.Options{
		Transport: rt.Transport(), Cwd: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("开会话失败: %v", err)
	}
	if s.ID() != "sess-abc" {
		t.Errorf("SessionID = %q, 想要 sess-abc", s.ID())
	}
}
