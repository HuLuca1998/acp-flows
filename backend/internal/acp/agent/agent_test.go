package agent_test

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/acp/agent"
	"github.com/HuLuca1998/acp-flows/backend/internal/acp/fake"
	"github.com/HuLuca1998/acp-flows/backend/internal/acp/protocol"
	"github.com/HuLuca1998/acp-flows/backend/internal/acp/runtime"
	"github.com/HuLuca1998/acp-flows/backend/internal/acp/session"
	"github.com/HuLuca1998/acp-flows/backend/tests/testutil"
)

// U2.4.1 · 拉起 Agent 并把它说的话变成工作事件（验收点 V5 的 R3）
//
// ★ 这一层是**翻译**：ACP 的 13 类 session/update 进来，
// 契约里的 13 类 Event 出去。两套判别值不是一一对应的
// （比如 tool_call 与 tool_call_update 都归到 tool_call），
// 所以映射必须显式，不能靠字符串相等碰运气。

type recordingSink struct {
	mu     sync.Mutex
	events []agent.WorkEvent
}

func (s *recordingSink) Emit(e agent.WorkEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, e)
}

func (s *recordingSink) snapshot() []agent.WorkEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]agent.WorkEvent(nil), s.events...)
}

func (s *recordingSink) types() []string {
	out := []string{}
	for _, e := range s.snapshot() {
		out = append(out, e.Type)
	}
	return out
}

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

func newFake(t *testing.T, script *fake.Script) *fake.Runtime {
	t.Helper()
	rt := fake.New(fake.Options{Script: script, Clock: testutil.FixedClock(testutil.T0)})
	t.Cleanup(func() { _ = rt.Close() })
	return rt
}

// ★★ ACP 的判别值要翻成**契约里的事件类型**。
//
// 两套不是一一对应的：`agent_message_chunk` → `message_chunk`、
// `tool_call` 与 `tool_call_update` 都归到 `tool_call`。
// 靠字符串相等碰运气的话，前端的注册表会收到一堆认不出的类型，
// 全部落到「暂时看不懂的记录」上。
func TestRun_TranslatesUpdateKinds(t *testing.T) {
	tests := []struct {
		acpKind  protocol.SessionUpdateKind
		wantType string
	}{
		{protocol.UpdateAgentMessageChunk, "message_chunk"},
		{protocol.UpdateAgentThoughtChunk, "thought_chunk"},
		{protocol.UpdateToolCall, "tool_call"},
		{protocol.UpdateToolCallUpdate, "tool_call"},
		{protocol.UpdatePlan, "plan_version"},
		{protocol.UpdatePlanUpdate, "plan_version"},
	}

	for _, tt := range tests {
		t.Run(string(tt.acpKind), func(t *testing.T) {
			rt := newFake(t, &fake.Script{
				Name: "translate",
				Turns: []fake.Turn{{
					Steps:      []fake.Step{{Emit: emit(t, tt.acpKind, nil)}},
					StopReason: protocol.StopReasonEndTurn,
				}},
			})
			sink := &recordingSink{}

			if err := agent.Run(context.Background(), agent.Spec{
				Transport: rt.Transport(),
				Cwd:       t.TempDir(),
				WorkID:    "work-01",
				Prompt:    "做点事",
				Sink:      sink,
			}); err != nil {
				t.Fatalf("跑一轮失败: %v", err)
			}

			types := sink.types()
			found := false
			for _, got := range types {
				if got == tt.wantType {
					found = true
				}
			}
			if !found {
				t.Errorf("%s 没有翻成 %q，收到的是 %v——"+
					"前端注册表会把它当成认不出的类型", tt.acpKind, tt.wantType, types)
			}
		})
	}
}

// 每条事件都要带上 work_id，前端靠它过滤。
//
// 不带的话，用户开着 A 工作会看到 B 工作的 AI 在说话。
func TestRun_TagsEveryEventWithWorkID(t *testing.T) {
	rt := newFake(t, &fake.Script{
		Name: "tag",
		Turns: []fake.Turn{{
			Steps: []fake.Step{
				{Emit: textChunk(t, protocol.UpdateAgentMessageChunk, "一句话")},
				{Emit: emit(t, protocol.UpdateToolCall, nil)},
			},
			StopReason: protocol.StopReasonEndTurn,
		}},
	})
	sink := &recordingSink{}

	if err := agent.Run(context.Background(), agent.Spec{
		Transport: rt.Transport(), Cwd: t.TempDir(),
		WorkID: "work-42", Prompt: "做点事", Sink: sink,
	}); err != nil {
		t.Fatal(err)
	}

	for _, e := range sink.snapshot() {
		if e.WorkID != "work-42" {
			t.Errorf("事件 %q 的 work_id = %q——前端靠它过滤，不带的话会串台", e.Type, e.WorkID)
		}
		if e.Source != "acp" {
			t.Errorf("事件 %q 的 source = %q, 想要 acp", e.Type, e.Source)
		}
	}
}

// ★ 轮次结束要发一条 `turn_end`，**并带上真实的 stopReason**。
//
// 不发的话，界面上 AI 说完最后一个字就停在那儿，用户不知道它是说完了
// 还是卡住了。带上原因是因为 `max_tokens` 与 `end_turn` 对用户是两件事——
// 前者意味着产出是半截的。
func TestRun_EmitsTurnEndWithReason(t *testing.T) {
	for _, reason := range []protocol.StopReason{
		protocol.StopReasonEndTurn,
		protocol.StopReasonMaxTokens,
		protocol.StopReasonRefusal,
	} {
		t.Run(string(reason), func(t *testing.T) {
			rt := newFake(t, &fake.Script{
				Name:  "stop",
				Turns: []fake.Turn{{StopReason: reason}},
			})
			sink := &recordingSink{}

			// max_tokens / refusal 会返回错误，但事件照样要发出去
			_ = agent.Run(context.Background(), agent.Spec{
				Transport: rt.Transport(), Cwd: t.TempDir(),
				WorkID: "work-01", Prompt: "做点事", Sink: sink,
			})

			var turnEnd *agent.WorkEvent
			for _, e := range sink.snapshot() {
				if e.Type == "turn_end" {
					turnEnd = &e
				}
			}
			if turnEnd == nil {
				t.Fatalf("没有发 turn_end——界面上 AI 会停在那儿，"+
					"用户不知道它是说完了还是卡住了（收到的是 %v）", sink.types())
			}
			if got := turnEnd.Payload["reason"]; got != string(reason) {
				t.Errorf("turn_end 的 reason = %v, 想要 %q——"+
					"max_tokens 与 end_turn 对用户是两件事", got, reason)
			}
		})
	}
}

// ★ 文本内容要带出来，那是用户真正读的东西。
func TestRun_CarriesText(t *testing.T) {
	rt := newFake(t, &fake.Script{
		Name: "text",
		Turns: []fake.Turn{{
			Steps: []fake.Step{
				{Emit: textChunk(t, protocol.UpdateAgentMessageChunk, "我看一下这个文件")},
			},
			StopReason: protocol.StopReasonEndTurn,
		}},
	})
	sink := &recordingSink{}

	if err := agent.Run(context.Background(), agent.Spec{
		Transport: rt.Transport(), Cwd: t.TempDir(),
		WorkID: "work-01", Prompt: "做点事", Sink: sink,
	}); err != nil {
		t.Fatal(err)
	}

	for _, e := range sink.snapshot() {
		if e.Type != "message_chunk" {
			continue
		}
		if got := e.Payload["text"]; got != "我看一下这个文件" {
			t.Errorf("text = %v", got)
		}
		return
	}
	t.Error("一条 message_chunk 都没有")
}

// ★ 事件要**边说边发**，不是攒完一起发。
//
// 攒的话用户盯着不动的界面等，而 AI 早就开口了——这是 V5 的用户可感部分。
func TestRun_EmitsWhileRunning(t *testing.T) {
	rt := newFake(t, &fake.Script{
		Name: "stream",
		Turns: []fake.Turn{{
			Steps:      []fake.Step{{Emit: textChunk(t, protocol.UpdateAgentMessageChunk, "第一句")}},
			StopReason: protocol.StopReasonEndTurn,
			StopDelay:  fake.Dur(1500 * time.Millisecond),
		}},
	})

	first := make(chan time.Duration, 1)
	var once sync.Once
	start := time.Now()

	sink := &callbackSink{fn: func(agent.WorkEvent) {
		once.Do(func() { first <- time.Since(start) })
	}}

	if err := agent.Run(context.Background(), agent.Spec{
		Transport: rt.Transport(), Cwd: t.TempDir(),
		WorkID: "work-01", Prompt: "做点事", Sink: sink,
	}); err != nil {
		t.Fatal(err)
	}

	select {
	case d := <-first:
		if d > 500*time.Millisecond {
			t.Errorf("第一条事件用了 %v 才发出来——攒完再发的话用户会盯着不动的界面等", d)
		}
	default:
		t.Fatal("一条事件都没发")
	}
	if total := time.Since(start); total < 1200*time.Millisecond {
		t.Errorf("整轮只用了 %v，Fake 的延迟没生效，这条测试什么也没证明", total)
	}
}

type callbackSink struct{ fn func(agent.WorkEvent) }

func (s *callbackSink) Emit(e agent.WorkEvent) { s.fn(e) }

// ★★ 翻译表必须盖住 ACP 判别值的**全集**。
//
// 这条是本包真正的守卫：Agent 那边新增一类事件时，protocol 包会多一个常量，
// 而这条测试立刻红——逼人做一次决定（上时间线翻成哪类，还是明确不上）。
// 没有它的话，新事件会静静掉进 default 分支，
// 表现为「AI 好像少说了点什么」而没有任何报错。
func TestTimelineType_CoversEveryKind(t *testing.T) {
	for _, kind := range protocol.AllSessionUpdateKinds() {
		if _, ok := agent.TimelineTypeOf(kind); !ok {
			t.Errorf("翻译表里没有 %q——请在 timelineType 里加一行：\n"+
				"  翻成契约里的某类事件，或者用空串表示「认识它但不上时间线」。\n"+
				"  不加的话它会静静消失，界面上没有任何提示。", kind)
		}
	}
}

// 会话元数据不上时间线——用量、可用命令这些没有可读的形态，
// 塞进去只会淹掉真正的进展。
func TestRun_SkipsSessionMetadata(t *testing.T) {
	rt := newFake(t, &fake.Script{
		Name: "meta",
		Turns: []fake.Turn{{
			Steps: []fake.Step{
				{Emit: emit(t, protocol.UpdateUsageUpdate, nil)},
				{Emit: emit(t, protocol.UpdateAvailableCommandsUpdate, nil)},
			},
			StopReason: protocol.StopReasonEndTurn,
		}},
	})
	sink := &recordingSink{}

	if err := agent.Run(context.Background(), agent.Spec{
		Transport: rt.Transport(), Cwd: t.TempDir(),
		WorkID: "work-01", Prompt: "做点事", Sink: sink,
	}); err != nil {
		t.Fatal(err)
	}

	if got := sink.types(); len(got) != 1 || got[0] != "turn_end" {
		t.Errorf("时间线上出现了会话元数据：%v，只该有 turn_end", got)
	}
}

// ★ 系统提示词**只在第一轮**拼上去。
//
// 每轮都发的话，几十轮之后上下文里全是重复的同一段话，
// 既烧 token 又稀释用户真正说的内容（U2.2.2 的 R3）。
func TestRun_PrependsSystemPromptOnce(t *testing.T) {
	rt := newFake(t, &fake.Script{
		Name:  "sys",
		Turns: []fake.Turn{{StopReason: protocol.StopReasonEndTurn}},
	})

	if err := agent.Run(context.Background(), agent.Spec{
		Transport: rt.Transport(), Cwd: t.TempDir(),
		WorkID: "work-01", Prompt: "帮我加个功能",
		SystemPrompt: "你是 Duet 的执行者", Sink: &recordingSink{},
	}); err != nil {
		t.Fatal(err)
	}

	var prompts []string
	for _, r := range rt.Requests() {
		if r.Method != protocol.MethodSessionPrompt {
			continue
		}
		prompts = append(prompts, string(r.Params))
	}
	if len(prompts) != 1 {
		t.Fatalf("发了 %d 次 session/prompt, 想要 1 次", len(prompts))
	}
	if !strings.Contains(prompts[0], "你是 Duet 的执行者") {
		t.Errorf("第一轮没带上系统提示词: %s", prompts[0])
	}
	if !strings.Contains(prompts[0], "帮我加个功能") {
		t.Errorf("用户的需求丢了: %s", prompts[0])
	}
}

// 工作目录非法时，**一个事件都不该发**——那意味着这一轮根本没开始。
func TestRun_InvalidCwdEmitsNothing(t *testing.T) {
	rt := newFake(t, &fake.Script{
		Name:  "cwd",
		Turns: []fake.Turn{{StopReason: protocol.StopReasonEndTurn}},
	})
	sink := &recordingSink{}

	err := agent.Run(context.Background(), agent.Spec{
		Transport: rt.Transport(), Cwd: "relative/path",
		WorkID: "work-01", Prompt: "做点事", Sink: sink,
	})
	if err == nil {
		t.Fatal("相对路径却跑成功了")
	}
	if n := len(sink.snapshot()); n != 0 {
		t.Errorf("这一轮没开始却发了 %d 条事件", n)
	}
}

// ★★ 我们加的元信息**不许和 ACP 自己的字段撞名**。
//
// 真机上撞到的：翻译层往 payload 里塞了 `kind`（ACP 判别值），
// 而 ACP 的 tool_call 载荷**本来就有一个 kind**（工具种类：read/edit/execute）。
// 结果同一个键有时是 "tool_call_update"、有时是 "read"，前端没法可靠使用——
// 表现是四条工具调用卡片长得一模一样，看不出 AI 在读哪个文件。
func TestRun_DoesNotClobberAgentFields(t *testing.T) {
	rt := newFake(t, &fake.Script{
		Name: "clobber",
		Turns: []fake.Turn{{
			Steps: []fake.Step{{Emit: emit(t, protocol.UpdateToolCall, map[string]any{
				// ACP 的 tool_call 自带这些字段，一个都不能被我们覆盖
				"kind":       "read",
				"title":      "Read README.md",
				"toolCallId": "toolu_01",
				"status":     "in_progress",
			})}},
			StopReason: protocol.StopReasonEndTurn,
		}},
	})
	sink := &recordingSink{}

	if err := agent.Run(context.Background(), agent.Spec{
		Transport: rt.Transport(), Cwd: t.TempDir(),
		WorkID: "work-01", Prompt: "做点事", Sink: sink,
	}); err != nil {
		t.Fatal(err)
	}

	for _, e := range sink.snapshot() {
		if e.Type != "tool_call" {
			continue
		}
		for _, f := range []struct{ key, want string }{
			{"kind", "read"},
			{"title", "Read README.md"},
			{"toolCallId", "toolu_01"},
			{"status", "in_progress"},
		} {
			if got := e.Payload[f.key]; got != f.want {
				t.Errorf("payload[%q] = %v, 想要 %q——Agent 的字段被我们的元信息覆盖了，"+
					"前端拿不到真实内容", f.key, got, f.want)
			}
		}
		// 我们自己的判别值要在**独立的键**上，前端靠它区分 tool_call 与 tool_call_update
		if got := e.Payload["acp_kind"]; got != string(protocol.UpdateToolCall) {
			t.Errorf("acp_kind = %v, 想要 %q", got, protocol.UpdateToolCall)
		}
		return
	}
	t.Error("一条 tool_call 都没有")
}

// ★★ U3.1.4：权限配置要**真的传下去**。
//
// 不传的话，会话拿到零值——每次都问、而且没人接线，于是一律 cancelled。
// 表现是「自动允许只读」这个开关在真跑的时候完全不生效。
func TestRun_PassesPermissionConfigToSession(t *testing.T) {
	rt := newFake(t, &fake.Script{
		Name: "perm",
		Turns: []fake.Turn{{
			Steps: []fake.Step{{Ask: &fake.PermissionAsk{
				ToolCallID: "tool-1", Title: "读一下", Kind: protocol.ToolKindRead,
				Options: []protocol.PermissionOption{
					{OptionID: "opt-allow", Name: "允许一次", Kind: protocol.PermissionAllowOnce},
					{OptionID: "opt-deny", Name: "拒绝", Kind: protocol.PermissionRejectOnce},
				},
			}}},
			StopReason: protocol.StopReasonEndTurn,
		}},
	})

	asked := 0
	err := agent.Run(context.Background(), agent.Spec{
		Transport: rt.Transport(), Cwd: t.TempDir(),
		WorkID: "work-01", Prompt: "读一下", Sink: &recordingSink{},
		Permission: session.Permission{
			Policy: session.PolicyAutoAllowReadonly,
			AskUser: func(context.Context, session.PermissionAsk) (session.Answer, error) {
				asked++
				return session.Answer{}, nil
			},
		},
	})
	if err != nil {
		t.Fatalf("这一轮失败了: %v", err)
	}

	if asked != 0 {
		t.Errorf("问了用户 %d 次——策略是「自动允许只读」而这是读类工具，"+
			"说明 Permission 根本没传给会话", asked)
	}
	if got := rt.LastPermissionOptionID(); got != "opt-allow" {
		t.Errorf("回给 Agent 的 optionId = %q, 想要 opt-allow", got)
	}
}

// ★ 交给用户时，工作标识要**绑进去**。
//
// 不绑的话，Broker 不知道这条请求属于哪个工作——事件发不到对的时间线上，
// 而用户在 A 工作里看到 B 工作的权限卡片。
func TestProcessRunner_BindsWorkIDIntoAskUser(t *testing.T) {
	var gotWorkID string
	r := &agent.ProcessRunner{
		Specs: []runtime.Spec{{Name: "claude", Bin: "/nonexistent", VersionArgs: []string{"-v"}}},
		AskUser: func(_ context.Context, workID string, _ session.PermissionAsk) (session.Answer, error) {
			gotWorkID = workID
			return session.Answer{}, nil
		},
	}

	// 直接验闭包：拉真进程要一整套脚本，而这里要验的只是「workID 绑对了没」
	ask := r.PermissionFor("work-42")
	if ask.AskUser == nil {
		t.Fatal("配了 AskUser 却没接上——会话拿到 nil，一律回 cancelled")
	}
	if _, err := ask.AskUser(context.Background(), session.PermissionAsk{}); err != nil {
		t.Fatal(err)
	}
	if gotWorkID != "work-42" {
		t.Errorf("绑进去的 workID = %q, 想要 work-42——"+
			"用户会在 A 工作里看到 B 工作的权限卡片", gotWorkID)
	}
}

// 没配 AskUser 时，会话拿到的必须是 **nil**，不是一个「总是成功」的空函数。
//
// 空函数会返回一个空的 Answer，而 session 把空 optionID 当成「用户没选」→
// cancelled。绕了一圈结果一样，但中间多了一层假装有人在接的代码。
func TestProcessRunner_NoAskUserLeavesItNil(t *testing.T) {
	r := &agent.ProcessRunner{}
	if r.PermissionFor("work-01").AskUser != nil {
		t.Error("没配 AskUser 却塞了一个空函数进去——中间多一层假装有人在接的代码")
	}
}
