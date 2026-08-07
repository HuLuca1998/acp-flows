package fake_test

// M0 U0.4.1 · Fake Runtime 脚本回放与时序控制
//
// 验收标准 R1–R6 见 docs/plan/milestones/M0-acp-foundation.md § S0.4 U0.4.1，
// 详细设计见 docs/spec/acp-integration.md §12。
//
// ★ Fake 是所有上层测试的地基。它自己必须自证 —— 地基歪了上面全歪。

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/acp/fake"
	"github.com/HuLuca1998/acp-flows/backend/internal/acp/protocol"
	"github.com/HuLuca1998/acp-flows/backend/tests/testutil"
)

const (
	// 等一条本应立刻到达的消息。给足余量，但远小于「永不到达」的判定窗口。
	shortWait = 2 * time.Second
	// 判定「确实没来」用的窗口。
	quietWindow = 300 * time.Millisecond
)

func newRuntime(t *testing.T, s *fake.Script, presets ...func(*fake.Runtime)) (*fake.Runtime, *client) {
	t.Helper()
	rt := fake.New(fake.Options{
		Script: s,
		Clock:  testutil.FixedClock(testutil.T0),
	})
	for _, p := range presets {
		p(rt)
	}
	t.Cleanup(func() { _ = rt.Close() })
	return rt, newClient(t, rt.Transport())
}

// handshake 走完 initialize + session/new，返回 sessionId。
func handshake(t *testing.T, c *client) string {
	t.Helper()

	resp, err := c.call(protocol.MethodInitialize, protocol.InitializeRequest{
		ProtocolVersion: protocol.ProtocolVersionV1,
	}, shortWait)
	if err != nil {
		t.Fatalf("initialize 失败: %v", err)
	}
	var init protocol.InitializeResponse
	if err := json.Unmarshal(resp.Result, &init); err != nil {
		t.Fatalf("解析 initialize 响应失败: %v", err)
	}
	if init.ProtocolVersion != protocol.ProtocolVersionV1 {
		t.Fatalf("protocolVersion: want %d, got %d", protocol.ProtocolVersionV1, init.ProtocolVersion)
	}

	resp, err = c.call(protocol.MethodSessionNew, protocol.NewSessionRequest{
		Cwd:        t.TempDir(),
		MCPServers: []protocol.MCPServer{},
	}, shortWait)
	if err != nil {
		t.Fatalf("session/new 失败: %v", err)
	}
	var sess protocol.NewSessionResponse
	if err := json.Unmarshal(resp.Result, &sess); err != nil {
		t.Fatalf("解析 session/new 响应失败: %v", err)
	}
	if sess.SessionID == "" {
		t.Fatal("session/new 必须返回非空 sessionId")
	}
	return sess.SessionID
}

// recvUpdate 收一条 session/update 通知并解出判别值。
func recvUpdate(t *testing.T, c *client, timeout time.Duration) protocol.SessionNotification {
	t.Helper()
	f, err := c.nextNotification(timeout)
	if err != nil {
		t.Fatalf("等 session/update 失败: %v", err)
	}
	if f.Method != protocol.MethodSessionUpdate {
		t.Fatalf("want %s, got %s", protocol.MethodSessionUpdate, f.Method)
	}
	var n protocol.SessionNotification
	if err := json.Unmarshal(f.Params, &n); err != nil {
		t.Fatalf("解析 session/update 失败: %v (%s)", err, f.Params)
	}
	return n
}

// R1：按脚本顺序推送**全部 13 类** sessionUpdate，且每条都能被 protocol 反序列化。
//
// 数量由 protocol.AllSessionUpdateKinds() 驱动而不是写死 13 ——
// 协议加了新变体时，这里会因为 scriptedUpdates 少一条而红，
// 逼着补脚本样本，而不是让 Fake 悄悄少支持一个变体。
func TestRuntime_R1_ReplaysEverySessionUpdateKind(t *testing.T) {
	updates := scriptedUpdates(t)

	b := fake.NewScript("R1 全部变体").Session("sess_fake_0001").Turn()
	for _, u := range updates {
		b = b.Emit(u.payload)
	}
	script := b.Stop(protocol.StopReasonEndTurn).Build()

	rt, c := newRuntime(t, script)
	sessionID := handshake(t, c)

	go func() {
		_, _ = c.call(protocol.MethodSessionPrompt, protocol.PromptRequest{
			SessionID: sessionID,
			Prompt:    []protocol.ContentBlock{protocol.TextBlock("跑一遍全部变体")},
		}, shortWait)
	}()

	for i, want := range updates {
		n := recvUpdate(t, c, shortWait)
		if n.SessionID != sessionID {
			t.Errorf("第 %d 条：sessionId want %q, got %q", i, sessionID, n.SessionID)
		}
		if got := n.Update.Kind(); got != want.kind {
			t.Fatalf("第 %d 条：判别值 want %q, got %q", i, want.kind, got)
		}
		if !n.Update.Kind().IsValid() {
			t.Errorf("第 %d 条：Fake 发出了不在 v1 全集里的判别值 %q", i, n.Update.Kind())
		}
	}

	// 全部变体都覆盖到了 —— 这条断言让「加了判别值却没加样本」变成红。
	if len(updates) != len(protocol.AllSessionUpdateKinds()) {
		t.Fatalf("脚本样本 %d 个，v1 判别值 %d 个 —— 必须一一对应",
			len(updates), len(protocol.AllSessionUpdateKinds()))
	}
	_ = rt
}

// R2：可配置每条事件的延迟，两条事件的到达间隔 ≥ 配置值。
func TestRuntime_R2_StepDelayIsHonored(t *testing.T) {
	const gap = 120 * time.Millisecond

	script := fake.NewScript("R2 延迟").Session("sess_fake_0001").
		Turn().
		Say("msg_1", "第一条立刻到").
		After(gap).Say("msg_1", "第二条要等").
		Stop(protocol.StopReasonEndTurn).
		Build()

	_, c := newRuntime(t, script)
	sessionID := handshake(t, c)

	go func() {
		_, _ = c.call(protocol.MethodSessionPrompt, protocol.PromptRequest{
			SessionID: sessionID,
			Prompt:    []protocol.ContentBlock{protocol.TextBlock("测延迟")},
		}, shortWait)
	}()

	recvUpdate(t, c, shortWait)
	start := time.Now()
	recvUpdate(t, c, shortWait)
	elapsed := time.Since(start)

	if elapsed < gap {
		t.Errorf("第二条事件来得太早：want ≥ %s, got %s", gap, elapsed)
	}
}

// R3：可配置乱序推送，且乱序由 Seed 驱动、可复现。
//
// 测试里禁止不可复现的随机 —— 同一个 seed 跑两次必须给出同一个顺序，
// 否则失败无法复现，这个开关就只会制造 flaky 测试。
func TestRuntime_R3_ReorderIsDeterministic(t *testing.T) {
	const seed = 42

	build := func() *fake.Script {
		return fake.NewScript("R3 乱序").Session("sess_fake_0001").
			Turn().
			Say("msg_1", "A").
			Say("msg_1", "B").
			Say("msg_1", "C").
			Say("msg_1", "D").
			Stop(protocol.StopReasonEndTurn).
			Build()
	}

	collect := func(t *testing.T, presets ...func(*fake.Runtime)) []string {
		t.Helper()
		_, c := newRuntime(t, build(), presets...)
		sessionID := handshake(t, c)
		go func() {
			_, _ = c.call(protocol.MethodSessionPrompt, protocol.PromptRequest{
				SessionID: sessionID,
				Prompt:    []protocol.ContentBlock{protocol.TextBlock("测乱序")},
			}, shortWait)
		}()

		var got []string
		for range 4 {
			n := recvUpdate(t, c, shortWait)
			var chunk protocol.ContentChunkUpdate
			if err := json.Unmarshal(n.Update.Payload(), &chunk); err != nil {
				t.Fatalf("解析 chunk 失败: %v", err)
			}
			text, ok := chunk.Content.Text()
			if !ok {
				t.Fatalf("want text 块, got %q", chunk.Content.Type())
			}
			got = append(got, text)
		}
		return got
	}

	inOrder := collect(t)
	if want := []string{"A", "B", "C", "D"}; !equalStrings(inOrder, want) {
		t.Fatalf("不开乱序时必须按脚本顺序：want %v, got %v", want, inOrder)
	}

	first := collect(t, fake.Reorder(seed))
	second := collect(t, fake.Reorder(seed))

	if !equalStrings(first, second) {
		t.Errorf("同一个 seed 必须复现同一个顺序：第一次 %v, 第二次 %v", first, second)
	}
	if equalStrings(first, inOrder) {
		t.Errorf("Reorder(%d) 没有真的打乱顺序，仍然是 %v —— 这个开关是失效的", seed, first)
	}
	// 乱序只换顺序，不丢事件也不造事件。
	if len(first) != 4 || !sameSet(first, inOrder) {
		t.Errorf("乱序改变了事件集合：want 同一批 %v, got %v", inOrder, first)
	}
}

// R4：可配置中途断流，消费方**感知到断开**而不是永久阻塞。
//
// 永久阻塞的症状是测试挂住而不是失败 —— 那比失败更难查。
func TestRuntime_R4_SilentAfterEndsTheStream(t *testing.T) {
	script := fake.NewScript("R4 断流").Session("sess_fake_0001").
		Turn().
		Say("msg_1", "断流前的最后一条").
		After(time.Hour).Say("msg_1", "这条永远不该到").
		Stop(protocol.StopReasonEndTurn).
		Build()

	_, c := newRuntime(t, script, fake.SilentAfter(150*time.Millisecond))
	sessionID := handshake(t, c)

	go func() {
		_, _ = c.call(protocol.MethodSessionPrompt, protocol.PromptRequest{
			SessionID: sessionID,
			Prompt:    []protocol.ContentBlock{protocol.TextBlock("测断流")},
		}, shortWait)
	}()

	recvUpdate(t, c, shortWait)

	// 断流后必须收到 EOF，而不是干等到测试超时。
	_, err := c.nextNotification(shortWait)
	if !errors.Is(err, io.EOF) {
		t.Errorf("断流后应感知到 EOF，got %v", err)
	}
}

// R5：不回 stopReason 模式 —— session/prompt 永不 resolve，供 S0.6 测取消超时。
//
// ★ 关键区分：是「不回 stopReason」，不是「整个挂了」。
// 事件必须照常流出，否则测出来的超时是连接断了，不是 Runtime 不收尾。
func TestRuntime_R5_NeverStopsLeavesPromptPending(t *testing.T) {
	script := fake.NewScript("R5 不收尾").Session("sess_fake_0001").
		Turn().
		Say("msg_1", "我开始干活了").
		Stop(protocol.StopReasonEndTurn). // 脚本说要收尾，NeverStops 覆盖它
		Build()

	_, c := newRuntime(t, script, fake.NeverStops)
	sessionID := handshake(t, c)

	done := make(chan error, 1)
	go func() {
		_, err := c.call(protocol.MethodSessionPrompt, protocol.PromptRequest{
			SessionID: sessionID,
			Prompt:    []protocol.ContentBlock{protocol.TextBlock("测不收尾")},
		}, quietWindow)
		done <- err
	}()

	// 事件照常来 —— 证明连接是活的。
	n := recvUpdate(t, c, shortWait)
	if n.Update.Kind() != protocol.UpdateAgentMessageChunk {
		t.Errorf("want %q, got %q", protocol.UpdateAgentMessageChunk, n.Update.Kind())
	}

	// 但 prompt 不收尾。
	if err := <-done; err == nil {
		t.Fatal("NeverStops 下 session/prompt 必须永不 resolve，却收到了响应")
	}
}

// R6：记录收到的全部请求，**且绝不去重**。
//
// ★ 最容易写反的一条。Fake 若自己去重，「连续取消两次只发一次协议请求」
// （U0.6.1 R1）就永远绿 —— 去重是**被测代码**的职责，不是 Fake 的。
// 这正是「测试制造虚假安全感」的典型。
func TestRuntime_R6_RecordsEveryRequestWithoutDeduping(t *testing.T) {
	script := fake.NewScript("R6 记录").Session("sess_fake_0001").
		Turn().Say("msg_1", "干活中").Stop(protocol.StopReasonEndTurn).Build()

	rt, c := newRuntime(t, script)
	sessionID := handshake(t, c)

	// 客户端故意发两次取消 —— Fake 必须如实记两条。
	c.notify(protocol.MethodSessionCancel, protocol.CancelNotification{SessionID: sessionID})
	c.notify(protocol.MethodSessionCancel, protocol.CancelNotification{SessionID: sessionID})

	ctx, cancel := context.WithTimeout(context.Background(), shortWait)
	defer cancel()

	seen := 0
	if _, err := rt.WaitFor(ctx, func(r fake.Recorded) bool {
		if r.Method == protocol.MethodSessionCancel {
			seen++
		}
		return seen == 2
	}); err != nil {
		t.Fatalf("等两条 session/cancel 失败: %v", err)
	}

	if got := rt.CountMethod(protocol.MethodSessionCancel); got != 2 {
		t.Errorf("★ Fake 不做去重：want 2 条 session/cancel, got %d", got)
	}

	got := rt.Requests()
	wantMethods := []string{
		protocol.MethodInitialize,
		protocol.MethodSessionNew,
		protocol.MethodSessionCancel,
		protocol.MethodSessionCancel,
	}
	if len(got) != len(wantMethods) {
		t.Fatalf("收到的消息条数: want %d, got %d (%+v)", len(wantMethods), len(got), got)
	}
	for i, want := range wantMethods {
		if got[i].Method != want {
			t.Errorf("第 %d 条: want %q, got %q", i, want, got[i].Method)
		}
		if got[i].N != i+1 {
			t.Errorf("第 %d 条: 接收序号应从 1 开始递增, got %d", i, got[i].N)
		}
	}

	// 请求带 id、通知不带 —— 分不清的话没法断言「取消是通知不是请求」。
	if got[0].Kind != fake.KindRequest {
		t.Errorf("initialize 是请求, got %v", got[0].Kind)
	}
	if got[2].Kind != fake.KindNotification {
		t.Errorf("session/cancel 是**通知**不是请求, got %v", got[2].Kind)
	}

	// 原始参数要留着，测试才能断言「取消发的是哪个 session」。
	var cancelParams protocol.CancelNotification
	if err := json.Unmarshal(got[2].Params, &cancelParams); err != nil {
		t.Fatalf("解析记录的 params 失败: %v", err)
	}
	if cancelParams.SessionID != sessionID {
		t.Errorf("记录的 sessionId: want %q, got %q", sessionID, cancelParams.SessionID)
	}
}

// 脚本跑完一轮要能正常收尾，且 stopReason 原样带回。
//
// 这是 R1–R6 的前置：连正常路径都跑不通的话，上面几条测的都是别的东西。
func TestRuntime_CompletesAScriptedTurn(t *testing.T) {
	script := fake.NewScript("正常一轮").Session("sess_fake_0001").
		Turn().
		Say("msg_1", "开始分析 cancel.go").
		Tool("call_001", protocol.ToolKindEdit, "编辑 cancel.go").
		ToolStatus("call_001", protocol.ToolCallStatusCompleted).
		Stop(protocol.StopReasonEndTurn).
		Build()

	_, c := newRuntime(t, script)
	sessionID := handshake(t, c)
	if sessionID != "sess_fake_0001" {
		t.Errorf("脚本指定的 sessionId 应被采用: want %q, got %q", "sess_fake_0001", sessionID)
	}

	done := make(chan protocol.PromptResponse, 1)
	go func() {
		resp, err := c.call(protocol.MethodSessionPrompt, protocol.PromptRequest{
			SessionID: sessionID,
			Prompt:    []protocol.ContentBlock{protocol.TextBlock("跑一轮")},
		}, shortWait)
		if err != nil {
			t.Errorf("session/prompt 失败: %v", err)
			close(done)
			return
		}
		var pr protocol.PromptResponse
		if err := json.Unmarshal(resp.Result, &pr); err != nil {
			t.Errorf("解析 prompt 响应失败: %v", err)
			close(done)
			return
		}
		done <- pr
	}()

	wantKinds := []protocol.SessionUpdateKind{
		protocol.UpdateAgentMessageChunk,
		protocol.UpdateToolCall,
		protocol.UpdateToolCallUpdate,
	}
	for i, want := range wantKinds {
		n := recvUpdate(t, c, shortWait)
		if got := n.Update.Kind(); got != want {
			t.Fatalf("第 %d 条: want %q, got %q", i, want, got)
		}
	}

	pr, ok := <-done
	if !ok {
		t.Fatal("session/prompt 没有正常收尾")
	}
	if pr.StopReason != protocol.StopReasonEndTurn {
		t.Errorf("stopReason: want %q, got %q", protocol.StopReasonEndTurn, pr.StopReason)
	}
	if !pr.StopReason.IsSuccess() {
		t.Error("end_turn 应算正常收尾")
	}
}

// 子进程形态与进程内形态跑的必须是同一个 Fake。
//
// e2e 里跑的和单测里跑的不是同一个东西的话，
// 「单测绿 + e2e 红」时你不知道该信谁（acp-integration.md §12.1）。
func TestRuntime_ServeSpeaksTheSameProtocolAsTransport(t *testing.T) {
	script := fake.NewScript("Serve 形态").Session("sess_fake_0001").
		Turn().Say("msg_1", "来自子进程形态").Stop(protocol.StopReasonEndTurn).Build()

	rt := fake.New(fake.Options{Script: script, Clock: testutil.FixedClock(testutil.T0)})
	t.Cleanup(func() { _ = rt.Close() })

	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = rt.Serve(ctx, inR, outW) }()

	c := newClient(t, struct {
		io.Reader
		io.WriteCloser
	}{Reader: outR, WriteCloser: inW})

	resp, err := c.call(protocol.MethodInitialize, protocol.InitializeRequest{
		ProtocolVersion: protocol.ProtocolVersionV1,
	}, shortWait)
	if err != nil {
		t.Fatalf("Serve 形态下 initialize 失败: %v", err)
	}
	var init protocol.InitializeResponse
	if err := json.Unmarshal(resp.Result, &init); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if init.ProtocolVersion != protocol.ProtocolVersionV1 {
		t.Errorf("protocolVersion: want %d, got %d", protocol.ProtocolVersionV1, init.ProtocolVersion)
	}
}

// scriptedUpdates 为每个 v1 判别值造一个最小但合法的载荷。
//
// ★ 这张表是穷举的落点：协议加了新判别值时，末尾的完整性断言会红。
func scriptedUpdates(t *testing.T) []struct {
	kind    protocol.SessionUpdateKind
	payload any
} {
	t.Helper()
	block := protocol.TextBlock("Fake 在回放脚本")
	return []struct {
		kind    protocol.SessionUpdateKind
		payload any
	}{
		{protocol.UpdateUserMessageChunk, protocol.ContentChunkUpdate{
			SessionUpdate: protocol.UpdateUserMessageChunk, Content: block}},
		{protocol.UpdateAgentMessageChunk, protocol.ContentChunkUpdate{
			SessionUpdate: protocol.UpdateAgentMessageChunk, Content: block, MessageID: "msg_1"}},
		{protocol.UpdateAgentThoughtChunk, protocol.ContentChunkUpdate{
			SessionUpdate: protocol.UpdateAgentThoughtChunk, Content: block, MessageID: "msg_1"}},
		{protocol.UpdateToolCall, protocol.ToolCallStartUpdate{
			SessionUpdate: protocol.UpdateToolCall,
			ToolCall: protocol.ToolCall{
				ToolCallID: "call_001", Title: "编辑 cancel.go",
				Kind: protocol.ToolKindEdit, Status: protocol.ToolCallStatusPending,
			}}},
		{protocol.UpdateToolCallUpdate, protocol.ToolCallDeltaUpdate{
			SessionUpdate: protocol.UpdateToolCallUpdate,
			ToolCallUpdate: protocol.ToolCallUpdate{
				ToolCallID: "call_001", Status: protocol.ToolCallStatusCompleted,
				Content: []protocol.ToolCallContent{
					protocol.DiffContent("/w/backend/internal/acp/session/cancel.go", "", "package session\n"),
				},
			}}},
		{protocol.UpdatePlan, protocol.PlanSnapshotUpdate{
			SessionUpdate: protocol.UpdatePlan,
			Entries: []protocol.PlanEntry{{
				Content: "读规格", Priority: protocol.PlanPriorityHigh, Status: protocol.PlanEntryCompleted,
			}}}},
		{protocol.UpdatePlanUpdate, protocol.PlanContentUpdate{
			SessionUpdate: protocol.UpdatePlanUpdate,
			Plan: protocol.PlanContent(
				`{"type":"markdown","planId":"plan_01","content":"## 步骤\n"}`),
		}},
		{protocol.UpdatePlanRemoved, protocol.PlanRemovedUpdate{
			SessionUpdate: protocol.UpdatePlanRemoved, PlanID: "plan_01"}},
		{protocol.UpdateAvailableCommandsUpdate, protocol.AvailableCommandsUpdate{
			SessionUpdate:     protocol.UpdateAvailableCommandsUpdate,
			AvailableCommands: []protocol.AvailableCommand{{Name: "review", Description: "独立审查"}}}},
		{protocol.UpdateCurrentModeUpdate, protocol.CurrentModeUpdate{
			SessionUpdate: protocol.UpdateCurrentModeUpdate, CurrentModeID: "read-only"}},
		{protocol.UpdateConfigOptionUpdate, protocol.ConfigOptionUpdate{
			SessionUpdate: protocol.UpdateConfigOptionUpdate,
			ConfigOptions: []protocol.ConfigOption{{
				ID: "reasoning_effort", Name: "Reasoning effort",
				Type: "select", CurrentValue: json.RawMessage(`"medium"`),
			}}}},
		{protocol.UpdateSessionInfoUpdate, protocol.SessionInfoUpdate{
			SessionUpdate: protocol.UpdateSessionInfoUpdate, Title: "U0.4.1 Fake Runtime"}},
		{protocol.UpdateUsageUpdate, protocol.UsageUpdate{
			SessionUpdate: protocol.UpdateUsageUpdate, Used: 48213, Size: 200000}},
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sameSet(a, b []string) bool {
	count := map[string]int{}
	for _, v := range a {
		count[v]++
	}
	for _, v := range b {
		count[v]--
	}
	for _, n := range count {
		if n != 0 {
			return false
		}
	}
	return true
}
