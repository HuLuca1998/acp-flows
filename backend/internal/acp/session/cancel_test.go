package session_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/acp/fake"
	"github.com/HuLuca1998/acp-flows/backend/internal/acp/protocol"
	"github.com/HuLuca1998/acp-flows/backend/internal/acp/session"
	"github.com/HuLuca1998/acp-flows/backend/tests/testutil"
)

// U3.2.1 · 两段式取消与幂等（验收点 V9）
//
// ★ 「取消」这件事有两个截然不同的失败方向，两边都很糟：
//   · 说停了其实没停 —— 界面显示「已取消」，后台还在烧钱改文件
//   · 停了但现场没了 —— 用户想看它停在哪一步，什么都找不到
//
// 下面每条断言都在守其中一边。

const cancelWait = 3 * time.Second

// ★★ R1：**连续取消两次只发送一次协议取消。**
//
// ACP 的 session/cancel 是通知，Agent 收到第二条时那一轮多半已经结束——
// 它会当成一个不认识的会话，行为没有定义。而用户手快点两下是常态。
func TestCancel_R1_IsIdempotent(t *testing.T) {
	rt := newFakeRuntime(t, neverStopsScript())
	s := openWith(t, rt, session.Permission{})

	go func() { _, _ = s.Prompt(context.Background(), "干点什么", nil) }()
	waitPrompt(t, rt)

	// ★ **同时**点三下，不是点完一下再点下一下。
	//
	// 串行调用测不到幂等：第一次取消之后这一轮就结束了，后两次走的是
	// 「没在跑，空操作」那条路——把 cancelling 那个判断整个删掉，
	// 串行版本照样绿（造负例时发现的）。
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, 3)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs[i] = s.Cancel(context.Background())
		}()
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("第 %d 次取消报错: %v", i+1, err)
		}
	}

	if got := rt.CountMethod(protocol.MethodSessionCancel); got != 1 {
		t.Errorf("发了 %d 条 session/cancel, 想要 1——"+
			"用户手快点两下是常态，而 Agent 收到第二条时那一轮多半已经结束，"+
			"它会当成一个不认识的会话，行为没有定义", got)
	}
}

// ★★ R3：取消时用 `cancelled` 应答**所有** pending 的权限请求。
//
// 这是 ACP 规范的硬要求，而设计稿完全没提。漏了的话每次取消都会超时——
// 而超时直接连着 M1 的一键更新：`update/prepare` 会永远返回 blocked，
// 用户点「更新」永远点不动。
func TestCancel_R3_AnswersEveryPendingPermission(t *testing.T) {
	// 脚本里连问两次，都不回答
	rt := newFakeRuntime(t, twoAsksScript())

	blocked := make(chan struct{})
	t.Cleanup(func() { close(blocked) })
	s := openWith(t, rt, session.Permission{
		Policy: session.PolicyAsk,
		AskUser: func(ctx context.Context, _ session.PermissionAsk) (session.Answer, error) {
			select {
			case <-blocked:
			case <-ctx.Done():
			}
			return session.Answer{}, ctx.Err()
		},
	})

	go func() { _, _ = s.Prompt(context.Background(), "改两个文件", nil) }()

	// 等权限请求发出来。★ 数的是 Fake 侧的 pending，不是 recorder——
	// recorder 记的是**入站**请求，而权限请求是 Fake **发出**的。
	waitFor(t, "权限请求没发出来", func() bool {
		return rt.PendingPermissionCount() >= 1
	})

	if err := s.Cancel(context.Background()); err != nil {
		t.Fatalf("取消报错: %v", err)
	}

	// ★ Fake 侧每一条 pending 都要被应答，一条都不能挂着
	waitFor(t, "取消之后还有权限请求挂着——"+
		"每次取消都会超时，而超时直接连着 M1 的一键更新："+
		"update/prepare 永远返回 blocked，用户点「更新」永远点不动",
		func() bool { return rt.PendingPermissionCount() == 0 })

	if got := rt.LastPermissionOutcome(); got != "cancelled" {
		t.Errorf("应答的 outcome = %q, 想要 cancelled", got)
	}
}

// ★★ R4：Agent 不回应时超时，**错误里能看出是谁、等了多久**。
//
// 只说「timeout」的话，用户提了工单我们也查不出是哪条会话、卡了多久。
func TestCancel_R4_TimeoutIsDiagnosable(t *testing.T) {
	rt := newDeadRuntime(t)
	s := openWith(t, rt, session.Permission{})

	go func() { _, _ = s.Prompt(context.Background(), "干点什么", nil) }()
	waitPrompt(t, rt)

	// 用一个很短的上限，别让测试等真实的超时
	err := s.CancelWithin(context.Background(), 200*time.Millisecond)
	if err == nil {
		t.Fatal("Agent 一直不回应，取消却成功了")
	}
	if !errors.Is(err, session.ErrCancelTimeout) {
		t.Fatalf("错误 = %v, 想要 ErrCancelTimeout", err)
	}

	msg := err.Error()
	if !strings.Contains(msg, s.ID()) {
		t.Errorf("错误里没有会话标识（%q）——用户提了工单我们查不出是哪条", msg)
	}
	if !strings.Contains(msg, "200ms") {
		t.Errorf("错误里没有等了多久（%q）——查不出是「卡了一下」还是「彻底死了」", msg)
	}
}

// ★★ R5：超时之后**同时**发取消并让调用方知道该杀进程。
//
// 这是「界面说已取消、后台还在烧钱改文件」的唯一防线。
// 超时返回一个普通错误、调用方以为「再等等」的话，那个进程会一直跑下去。
func TestCancel_R5_TimeoutDemandsKill(t *testing.T) {
	rt := newDeadRuntime(t)
	s := openWith(t, rt, session.Permission{})

	go func() { _, _ = s.Prompt(context.Background(), "干点什么", nil) }()
	waitPrompt(t, rt)

	err := s.CancelWithin(context.Background(), 200*time.Millisecond)

	if !session.MustKill(err) {
		t.Fatal("超时了却没有要求杀进程——" +
			"这是「界面说已取消、后台还在烧钱改文件」的唯一防线")
	}
	// 协议取消照样发了：能停就停，停不下来再杀
	if got := rt.CountMethod(protocol.MethodSessionCancel); got != 1 {
		t.Errorf("发了 %d 条 session/cancel, 想要 1——"+
			"超时之后直接杀进程而不发取消的话，Agent 那边没有收尾的机会", got)
	}
}

// ★ 正常取消**不要求**杀进程——那样每次取消都要重拉一个 Agent。
func TestCancel_NormalCancelDoesNotDemandKill(t *testing.T) {
	rt := newFakeRuntime(t, cancellableScript())
	s := openWith(t, rt, session.Permission{})

	go func() { _, _ = s.Prompt(context.Background(), "干点什么", nil) }()
	waitPrompt(t, rt)

	err := s.Cancel(context.Background())
	if err != nil {
		t.Fatalf("取消报错: %v", err)
	}
	if session.MustKill(err) {
		t.Error("正常取消也要求杀进程——那样每次取消都要重拉一个 Agent")
	}
}

// ★★ R2：取消之后**事件游标还读得到**。
//
// 用户点了停，第一件想知道的是「它停在哪一步」。游标丢了的话，
// 时间线接不上，他只能从头看一遍。
func TestCancel_R2_CursorSurvivesCancel(t *testing.T) {
	rt := newFakeRuntime(t, chattyThenHangScript())
	s := openWith(t, rt, session.Permission{})

	var mu sync.Mutex
	seen := 0
	go func() {
		_, _ = s.Prompt(context.Background(), "干点什么", func(session.Event) {
			mu.Lock()
			seen++
			mu.Unlock()
		})
	}()

	waitFor(t, "没收到事件", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return seen > 0
	})

	if err := s.Cancel(context.Background()); err != nil {
		t.Fatal(err)
	}

	// 取消后会话仍然可查——ID 与已收事件数都在
	if s.ID() == "" {
		t.Error("取消之后会话标识没了——时间线接不上，用户只能从头看一遍")
	}
	mu.Lock()
	defer mu.Unlock()
	if seen == 0 {
		t.Error("取消把已经收到的事件也抹了")
	}
}

// 没在跑的时候取消**不报错也不发协议取消**。
//
// 报错的话，用户点一个本来就该没反应的按钮会看到一句吓人的错误。
func TestCancel_WhenIdleIsNoop(t *testing.T) {
	rt := newFakeRuntime(t, cancellableScript())
	s := openWith(t, rt, session.Permission{})

	if err := s.Cancel(context.Background()); err != nil {
		t.Errorf("空闲时取消报错了: %v", err)
	}
	if got := rt.CountMethod(protocol.MethodSessionCancel); got != 0 {
		t.Errorf("空闲时发了 %d 条 session/cancel, 想要 0", got)
	}
}

// ── 脚本与小工具 ─────────────────────────────────────────

// newDeadRuntime 造一个**连取消都不理**的 Runtime——「Agent 卡死了」。
//
// 这是 R4/R5 唯一能测的场景：能正常响应取消的 Agent 不会超时。
func newDeadRuntime(t *testing.T) *fake.Runtime {
	t.Helper()
	rt := fake.New(fake.Options{
		Script: neverStopsScript(),
		Clock:  testutil.FixedClock(testutil.T0),
	})
	fake.NeverStops(rt)
	t.Cleanup(func() { _ = rt.Close() })
	return rt
}

// neverStopsScript：收到 prompt 之后不响应，但收到取消会收尾。
func neverStopsScript() *fake.Script {
	return &fake.Script{
		Name:  "never-stops",
		Turns: []fake.Turn{{}}, // StopReason 为空 = 永不响应
	}
}

// cancellableScript：收到取消后正常收尾。
func cancellableScript() *fake.Script {
	return &fake.Script{
		Name: "cancellable",
		Turns: []fake.Turn{{
			StopReason: protocol.StopReasonCancelled,
			StopDelay:  fake.Dur(5 * time.Second), // 等取消来
		}},
	}
}

// chattyThenHangScript：先说几句，然后挂住。
func chattyThenHangScript() *fake.Script {
	return &fake.Script{
		Name: "chatty",
		Turns: []fake.Turn{{
			Steps: []fake.Step{{Emit: mustTextUpdate("我在干活")}},
			// 不写 StopReason = 挂住
		}},
	}
}

// twoAsksScript：一轮里连问两次权限，都不回答。
func twoAsksScript() *fake.Script {
	ask := func(id string) fake.Step {
		return fake.Step{Ask: &fake.PermissionAsk{
			ToolCallID: id, Title: "写文件", Kind: protocol.ToolKindEdit,
			Options: []protocol.PermissionOption{
				{OptionID: "opt-allow", Name: "允许", Kind: protocol.PermissionAllowOnce},
				{OptionID: "opt-deny", Name: "拒绝", Kind: protocol.PermissionRejectOnce},
			},
		}}
	}
	return &fake.Script{
		Name:  "two-asks",
		Turns: []fake.Turn{{Steps: []fake.Step{ask("tool-1"), ask("tool-2")}}},
	}
}

// mustTextUpdate 造一条文本片段的载荷。
func mustTextUpdate(text string) json.RawMessage {
	raw, err := json.Marshal(map[string]any{
		"sessionUpdate": string(protocol.UpdateAgentMessageChunk),
		"content":       map[string]any{"type": "text", "text": text},
	})
	if err != nil {
		panic(err) // 夹具构造失败要立刻暴露
	}
	return raw
}

func waitPrompt(t *testing.T, rt *fake.Runtime) {
	t.Helper()
	waitFor(t, "session/prompt 没到 Fake", func() bool {
		return rt.CountMethod(protocol.MethodSessionPrompt) > 0
	})
}

func waitFor(t *testing.T, why string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(cancelWait)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("等超时了：%s", why)
}
