package fake_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/acp/fake"
	"github.com/HuLuca1998/acp-flows/backend/internal/acp/protocol"
	"github.com/HuLuca1998/acp-flows/backend/tests/testutil"
)

// U3.1.1 · Fake 能主动发权限请求（V8 的地基）
//
// ★ 这一组测的是 **Fake 自己**：它得先能像真 Agent 一样发反向请求、
// 阻塞等应答，上层（U3.1.2 的裁决策略、U3.1.3 的权限卡片）才有对手方可打。
//
// 客户端用同目录 client_test.go 里那个手写实现，**不用 internal/acp/jsonrpc**：
// Fake 是那个包的参照物，共用实现的话分帧或字段名写错时两边一起错。

const askTimeout = 3 * time.Second

// startTurn 走完握手并发出 session/prompt，返回等这一轮结束用的通道。
func startTurn(t *testing.T, script *fake.Script) (*fake.Runtime, *client, chan frame) {
	t.Helper()
	rt := fake.New(fake.Options{Script: script, Clock: testutil.FixedClock(testutil.T0)})
	t.Cleanup(func() { _ = rt.Close() })

	c := newClient(t, rt.Transport())
	if _, err := c.call(protocol.MethodInitialize,
		map[string]any{"protocolVersion": 1}, askTimeout); err != nil {
		t.Fatalf("initialize 失败: %v", err)
	}
	if _, err := c.call(protocol.MethodSessionNew,
		map[string]any{"cwd": "/tmp", "mcpServers": []any{}}, askTimeout); err != nil {
		t.Fatalf("session/new 失败: %v", err)
	}

	// ★ 异步发：同步等的话，我们等这一轮结束、Fake 等我们应答权限请求，直接死锁
	return rt, c, c.callAsync(protocol.MethodSessionPrompt, promptParams())
}

// ★★ R1：Fake 主动发权限请求，**并且在收到应答之前这一轮不结束**。
//
// 不阻塞的话，上层的裁决逻辑就没有真实的对手方——测试会以为「问过了」，
// 而实际上 Agent 根本没等答案就往下干了。那正是这个产品最不能出的错。
func TestAskPermission_R1_BlocksUntilAnswered(t *testing.T) {
	_, c, done := startTurn(t, permissionScript(protocol.StopReasonEndTurn))

	ask, err := c.nextRequest(protocol.MethodRequestPermission, askTimeout)
	if err != nil {
		t.Fatalf("没收到权限请求: %v", err)
	}

	// ★ 此刻还没应答：这一轮**不该**结束
	if f, ok := c.tryAwait(done, 700*time.Millisecond); ok {
		t.Fatalf("还没应答权限请求，这一轮就结束了（%s）——"+
			"上层会以为「问过了」，而 Agent 根本没等答案", f.Result)
	}

	c.respond(ask.ID, selected("opt-allow"))

	f, err := c.await(done, askTimeout)
	if err != nil {
		t.Fatalf("应答之后这一轮仍未结束: %v", err)
	}
	if got := stopReasonOf(t, f.Result); got != protocol.StopReasonEndTurn {
		t.Errorf("stopReason = %q, 想要 %q", got, protocol.StopReasonEndTurn)
	}
}

// ★★ R2：应答 `cancelled` 时，这一轮以 **cancelled** 收尾。
//
// 照脚本里写的 end_turn 收尾的话，用户按了「拒绝」而界面显示「完成」——
// 比不问更糟。
func TestAskPermission_R2_CancelledEndsTurnAsCancelled(t *testing.T) {
	_, c, done := startTurn(t, permissionScript(protocol.StopReasonEndTurn))

	ask, err := c.nextRequest(protocol.MethodRequestPermission, askTimeout)
	if err != nil {
		t.Fatalf("没收到权限请求: %v", err)
	}
	c.respond(ask.ID, map[string]any{"outcome": map[string]any{"outcome": "cancelled"}})

	f, err := c.await(done, askTimeout)
	if err != nil {
		t.Fatalf("应答 cancelled 之后这一轮没结束: %v", err)
	}
	if got := stopReasonOf(t, f.Result); got != protocol.StopReasonCancelled {
		t.Errorf("stopReason = %q, 想要 %q——"+
			"用户拒了却显示「完成」，比不问更糟", got, protocol.StopReasonCancelled)
	}
}

// ★★ R5：权限请求**没有超时**。
//
// 真 Agent 会一直等用户，Fake 也必须一直等。自作主张地超时的话，
// 上层「等用户裁决」的逻辑测不出来——测试里那一轮会自己结束。
func TestAskPermission_R5_NeverTimesOut(t *testing.T) {
	_, c, done := startTurn(t, permissionScript(protocol.StopReasonEndTurn))

	if _, err := c.nextRequest(protocol.MethodRequestPermission, askTimeout); err != nil {
		t.Fatalf("没收到权限请求: %v", err)
	}

	// 干等 2 秒，什么都不应答
	if f, ok := c.tryAwait(done, 2*time.Second); ok {
		t.Fatalf("等了 2s 这一轮自己结束了（%s）——"+
			"Fake 给权限请求加了超时，上层「等用户裁决」的逻辑就测不出来", f.Result)
	}
}

// ★★ R4：`optionId` **原样回传**，不按类别匹配。
//
// 这条专门用一组 **id 与类别语义对不上** 的选项：`opt-allow` 的类别是
// reject_once。按类别去猜 id 的实现会选错，按 id 原样回传的不受影响。
//
// 真 Agent 的 optionId 是它自己定义的不透明字符串
// （`protocol.PermissionOption` 的文档写着「客户端原样回传，不要自己造」）。
func TestAskPermission_R4_OptionIDRoundTripsVerbatim(t *testing.T) {
	rt, c, done := startTurn(t, trickyOptionScript())

	ask, err := c.nextRequest(protocol.MethodRequestPermission, askTimeout)
	if err != nil {
		t.Fatalf("没收到权限请求: %v", err)
	}

	var req protocol.RequestPermissionRequest
	if err := json.Unmarshal(ask.Params, &req); err != nil {
		t.Fatalf("解析权限请求失败: %v", err)
	}
	if len(req.Options) != 2 {
		t.Fatalf("收到 %d 个选项, 想要 2", len(req.Options))
	}
	// 选项的 id 与类别故意对不上——Fake 不许「顺手纠正」它
	if req.Options[0].OptionID != "opt-allow" ||
		req.Options[0].Kind != protocol.PermissionRejectOnce {
		t.Errorf("第一个选项 = %+v，脚本里写的 id 或 kind 被 Fake 改过了", req.Options[0])
	}

	c.respond(ask.ID, selected("opt-allow"))
	if _, err := c.await(done, askTimeout); err != nil {
		t.Fatalf("这一轮没结束: %v", err)
	}

	// Fake 记下的必须是我们回的那个 id，**一个字符都不差**
	if got := rt.LastPermissionOptionID(); got != "opt-allow" {
		t.Errorf("Fake 收到的 optionId = %q, 想要 %q——"+
			"中间有人按类别猜 id 而不是原样传", got, "opt-allow")
	}
}

// ★ R3：声明的能力**表现为真实协议行为**。
//
// 「不支持会话模式」不能只是一个内部标志位——`session/new` 的响应里
// 必须**真的没有** modes 字段。否则测的是我们自己的探针代码，
// 而不是「Agent 没给这个字段时我们怎么办」。
func TestNewSession_R3_UndeclaredCapabilityIsAbsentOnTheWire(t *testing.T) {
	newSession := func(t *testing.T, script *fake.Script) map[string]json.RawMessage {
		t.Helper()
		rt := fake.New(fake.Options{Script: script, Clock: testutil.FixedClock(testutil.T0)})
		t.Cleanup(func() { _ = rt.Close() })
		c := newClient(t, rt.Transport())

		if _, err := c.call(protocol.MethodInitialize,
			map[string]any{"protocolVersion": 1}, askTimeout); err != nil {
			t.Fatal(err)
		}
		f, err := c.call(protocol.MethodSessionNew,
			map[string]any{"cwd": "/tmp", "mcpServers": []any{}}, askTimeout)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(f.Result, &fields); err != nil {
			t.Fatalf("解析 session/new 响应失败: %v (%s)", err, f.Result)
		}
		return fields
	}

	t.Run("不声明模式时线上真的没有 modes 字段", func(t *testing.T) {
		fields := newSession(t, permissionScript(protocol.StopReasonEndTurn))
		if _, ok := fields["modes"]; ok {
			t.Errorf("响应里出现了 modes 字段：%v\n"+
				"假的能力声明必须表现为真的协议行为，"+
				"否则测的是我们自己的探针代码", fields)
		}
	})

	t.Run("声明了模式时字段真的在线上", func(t *testing.T) {
		script := permissionScript(protocol.StopReasonEndTurn)
		script.NewSession = &fake.NewSessionBehavior{
			CurrentModeID: "plan",
			Modes: []protocol.SessionMode{
				{ID: "plan", Name: "计划"},
				{ID: "execute", Name: "执行"},
			},
		}
		if _, ok := newSession(t, script)["modes"]; !ok {
			t.Error("声明了模式，响应里却没有 modes 字段")
		}
	})
}

// 权限请求要带上**是哪一次工具调用**——不带的话界面上只能问
// 「要不要允许？」而说不出允许什么。
func TestAskPermission_CarriesToolCall(t *testing.T) {
	_, c, _ := startTurn(t, permissionScript(protocol.StopReasonEndTurn))

	ask, err := c.nextRequest(protocol.MethodRequestPermission, askTimeout)
	if err != nil {
		t.Fatalf("没收到权限请求: %v", err)
	}

	var req protocol.RequestPermissionRequest
	if err := json.Unmarshal(ask.Params, &req); err != nil {
		t.Fatal(err)
	}
	if req.ToolCall.ToolCallID != "tool-1" {
		t.Errorf("toolCallId = %q, 想要 tool-1——界面上说不出要允许的是什么",
			req.ToolCall.ToolCallID)
	}
	if req.SessionID == "" {
		t.Error("权限请求没带 sessionId，客户端无法把它归到哪条会话")
	}
}

// 一轮里问两次，两次都要各自阻塞、各自等到应答。
//
// 只处理第一次的实现在这里会挂住——而真 Agent 一轮里问三五次是常态。
func TestAskPermission_TwoAsksInOneTurn(t *testing.T) {
	script := permissionScript(protocol.StopReasonEndTurn)
	second := *script.Turns[0].Steps[0].Ask
	second.ToolCallID = "tool-2"
	second.Title = "执行 go test"
	script.Turns[0].Steps = append(script.Turns[0].Steps, fake.Step{Ask: &second})

	rt := fake.New(fake.Options{Script: script, Clock: testutil.FixedClock(testutil.T0)})
	t.Cleanup(func() { _ = rt.Close() })
	c := newClient(t, rt.Transport())

	if _, err := c.call(protocol.MethodInitialize,
		map[string]any{"protocolVersion": 1}, askTimeout); err != nil {
		t.Fatal(err)
	}
	if _, err := c.call(protocol.MethodSessionNew,
		map[string]any{"cwd": "/tmp", "mcpServers": []any{}}, askTimeout); err != nil {
		t.Fatal(err)
	}
	done := c.callAsync(protocol.MethodSessionPrompt, promptParams())

	for _, want := range []string{"tool-1", "tool-2"} {
		ask, err := c.nextRequest(protocol.MethodRequestPermission, askTimeout)
		if err != nil {
			t.Fatalf("第二次权限请求没来（真 Agent 一轮里问三五次是常态）: %v", err)
		}
		var req protocol.RequestPermissionRequest
		if err := json.Unmarshal(ask.Params, &req); err != nil {
			t.Fatal(err)
		}
		if req.ToolCall.ToolCallID != want {
			t.Errorf("toolCallId = %q, 想要 %q", req.ToolCall.ToolCallID, want)
		}
		c.respond(ask.ID, selected("opt-allow"))
	}

	if _, err := c.await(done, askTimeout); err != nil {
		t.Fatalf("两次都应答了，这一轮还没结束: %v", err)
	}
}

// ── 脚本与小工具 ─────────────────────────────────────────

func permissionScript(stop protocol.StopReason) *fake.Script {
	return &fake.Script{
		Name: "permission",
		Turns: []fake.Turn{{
			Steps: []fake.Step{{
				Ask: &fake.PermissionAsk{
					ToolCallID: "tool-1",
					Title:      "写入 README.md",
					Kind:       protocol.ToolKindEdit,
					Options: []protocol.PermissionOption{
						{OptionID: "opt-allow", Name: "允许一次", Kind: protocol.PermissionAllowOnce},
						{OptionID: "opt-deny", Name: "拒绝", Kind: protocol.PermissionRejectOnce},
					},
				},
			}},
			StopReason: stop,
		}},
	}
}

// trickyOptionScript 的 optionId 与 kind **故意语义相反**——
// 谁想按类别猜 id，就会在这里翻车。
func trickyOptionScript() *fake.Script {
	s := permissionScript(protocol.StopReasonEndTurn)
	s.Turns[0].Steps[0].Ask.Options = []protocol.PermissionOption{
		{OptionID: "opt-allow", Name: "其实是拒绝", Kind: protocol.PermissionRejectOnce},
		{OptionID: "opt-deny", Name: "其实是允许", Kind: protocol.PermissionAllowOnce},
	}
	return s
}

func promptParams() map[string]any {
	return map[string]any{
		"sessionId": "sess_fake_0001",
		"prompt":    []any{map[string]any{"type": "text", "text": "改一下 README"}},
	}
}

func selected(optionID string) map[string]any {
	return map[string]any{
		"outcome": map[string]any{"outcome": "selected", "optionId": optionID},
	}
}

func stopReasonOf(t *testing.T, raw json.RawMessage) protocol.StopReason {
	t.Helper()
	var resp protocol.PromptResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("解析 prompt 响应失败: %v (%s)", err, raw)
	}
	return resp.StopReason
}
