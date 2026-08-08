package session_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/acp/fake"
	"github.com/HuLuca1998/acp-flows/backend/internal/acp/protocol"
	"github.com/HuLuca1998/acp-flows/backend/internal/acp/session"
	"github.com/HuLuca1998/acp-flows/backend/tests/testutil"
)

// U3.1.2 · 三种裁决策略（验收点 V8）
//
// ★ 这一层是**纯裁决**：给定策略、工具类别、选项集合，算出「选哪个 / 交给用户」。
// 没有 IO、没有等待——那些在 app 层。放在这里是为了能穷举所有组合。
//
// ★ 本单元的 forbidden_changes 只有一条：**在选项集合不认识时替用户拍板**。
// 下面每一条断言都在守它。

func ask(kind protocol.ToolKind, options ...protocol.PermissionOption) protocol.RequestPermissionRequest {
	return protocol.RequestPermissionRequest{
		SessionID: "sess-1",
		ToolCall:  protocol.ToolCallUpdate{ToolCallID: "tool-1", Kind: kind, Title: "干点什么"},
		Options:   options,
	}
}

func opt(id string, kind protocol.PermissionOptionKind) protocol.PermissionOption {
	return protocol.PermissionOption{OptionID: id, Name: id, Kind: kind}
}

// 一组齐全的选项：四种类别都有。
func fullOptions() []protocol.PermissionOption {
	return []protocol.PermissionOption{
		opt("a1", protocol.PermissionAllowOnce),
		opt("a2", protocol.PermissionAllowAlways),
		opt("r1", protocol.PermissionRejectOnce),
		opt("r2", protocol.PermissionRejectAlways),
	}
}

// ★★ R1：自动允许只读**只对读类工具生效**。
//
// 这是整个权限体系里最要紧的一条。放宽到写类工具的话，用户以为自己开的是
// 「让它随便看」，实际开的是「让它随便改」——而他不会发现，直到文件被改了。
func TestDecide_R1_AutoAllowReadonlyOnlyCoversReadKinds(t *testing.T) {
	// 只有这三类是「看」：读文件、搜索、想。其余一律要问。
	readonly := map[protocol.ToolKind]bool{
		protocol.ToolKindRead:   true,
		protocol.ToolKindSearch: true,
		protocol.ToolKindThink:  true,
	}

	// ★ 穷举 ToolKind 全集：Agent 那边新增一类工具时，protocol 会多一个常量，
	// 而这条测试会逼人做一次决定「它算不算只读」。
	// 漏掉的表现是新工具默认落进「自动允许」——那正是最坏的方向。
	for _, kind := range protocol.AllToolKinds() {
		t.Run(string(kind), func(t *testing.T) {
			got := session.Decide(session.PolicyAutoAllowReadonly, ask(kind, fullOptions()...))

			if readonly[kind] {
				if got.Deferred {
					t.Errorf("%s 是只读类，却仍然去问用户——「自动允许只读」等于没开", kind)
				}
				if got.OptionID != "a1" {
					t.Errorf("optionID = %q, 想要 allow_once 的那个", got.OptionID)
				}
				return
			}

			if !got.Deferred {
				t.Errorf("%s 不是只读类，却被自动放过了（选了 %q）——\n"+
					"用户以为自己开的是「让它随便看」，实际开的是「让它随便改」，"+
					"而他不会发现，直到文件被改了", kind, got.OptionID)
			}
		})
	}
}

// ★★ R2：选项集合不认识时**走保守分支，绝不猜一个 id**。
//
// Agent 的 optionId 是它自己定义的不透明字符串。策略想「允许」而选项里
// 没有 allow 类的时候，唯一正确的动作是交给用户——猜一个的话，
// 我们可能替他点了「永久允许」。
func TestDecide_R2_NeverGuessesAnOptionID(t *testing.T) {
	tests := []struct {
		name    string
		policy  session.Policy
		options []protocol.PermissionOption
	}{
		{
			"想允许，但一个 allow 类选项都没有",
			session.PolicyAutoAllowReadonly,
			[]protocol.PermissionOption{opt("r1", protocol.PermissionRejectOnce)},
		},
		{
			"想拒绝，但一个 reject 类选项都没有",
			session.PolicyRejectAll,
			[]protocol.PermissionOption{opt("a1", protocol.PermissionAllowOnce)},
		},
		{
			"一个选项都没给",
			session.PolicyRejectAll,
			nil,
		},
		{
			"选项的类别我们不认识",
			session.PolicyAutoAllowReadonly,
			[]protocol.PermissionOption{opt("x1", protocol.PermissionOptionKind("allow_if_tuesday"))},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := session.Decide(tt.policy, ask(protocol.ToolKindRead, tt.options...))

			if !got.Deferred {
				t.Fatalf("替用户拍板了（选了 %q）——\n"+
					"optionId 是 Agent 自定义的不透明字符串，猜错的话"+
					"我们可能替他点了「永久允许」", got.OptionID)
			}
			if got.OptionID != "" {
				t.Errorf("交给用户了却还带着 optionID=%q，调用方可能照着它发出去", got.OptionID)
			}
			if got.Reason != session.ReasonNoMatchingOption {
				t.Errorf("reason = %q, 想要 %q", got.Reason, session.ReasonNoMatchingOption)
			}
		})
	}
}

// ★ 「每次都问」就是每次都问，不因为是只读就放过。
func TestDecide_AskAlwaysDefers(t *testing.T) {
	for _, kind := range protocol.AllToolKinds() {
		got := session.Decide(session.PolicyAsk, ask(kind, fullOptions()...))
		if !got.Deferred {
			t.Errorf("策略是「每次都问」，%s 却被自动放过了（选了 %q）", kind, got.OptionID)
		}
		if got.Reason != session.ReasonPolicyAsk {
			t.Errorf("%s 的 reason = %q, 想要 %q", kind, got.Reason, session.ReasonPolicyAsk)
		}
	}
}

// ★ 「一律拒绝」对所有类别都拒绝，包括只读。
//
// 这是用户明确表达的「我先看着，什么都别动」，读也不许。
func TestDecide_RejectAllCoversEveryKind(t *testing.T) {
	for _, kind := range protocol.AllToolKinds() {
		got := session.Decide(session.PolicyRejectAll, ask(kind, fullOptions()...))
		if got.Deferred {
			t.Errorf("策略是「一律拒绝」，%s 却去问用户了", kind)
		}
		if got.OptionID != "r1" {
			t.Errorf("%s 的 optionID = %q, 想要 reject_once 的那个", kind, got.OptionID)
		}
	}
}

// ★★ 优先「一次性」而非「永久」。
//
// allow_once 与 allow_always 都在时选 once：替用户做的决定要尽量小。
// 选了 always 的话，一次自动裁决会永久改变后续所有请求的处理方式，
// 而用户根本不知道发生过这件事。
func TestDecide_PrefersOnceOverAlways(t *testing.T) {
	// 故意把 always 排在前面——按「第一个匹配的」实现会翻车
	options := []protocol.PermissionOption{
		opt("a2", protocol.PermissionAllowAlways),
		opt("a1", protocol.PermissionAllowOnce),
		opt("r2", protocol.PermissionRejectAlways),
		opt("r1", protocol.PermissionRejectOnce),
	}

	allow := session.Decide(session.PolicyAutoAllowReadonly, ask(protocol.ToolKindRead, options...))
	if allow.OptionID != "a1" {
		t.Errorf("允许时选了 %q, 想要 allow_once——\n"+
			"选 always 的话，一次自动裁决会永久改变后续所有请求的处理方式，"+
			"而用户根本不知道发生过这件事", allow.OptionID)
	}

	reject := session.Decide(session.PolicyRejectAll, ask(protocol.ToolKindEdit, options...))
	if reject.OptionID != "r1" {
		t.Errorf("拒绝时选了 %q, 想要 reject_once", reject.OptionID)
	}
}

// ★ R4：理由码是**机器可读的枚举**，不是一句人话。
//
// 界面要按它查 i18n 词条、排查时要按它过滤日志。塞一句中文的话，
// 改文案就会把过滤规则改坏，而且没法翻译。
func TestDecide_R4_ReasonIsAMachineCode(t *testing.T) {
	cases := []struct {
		policy session.Policy
		kind   protocol.ToolKind
		want   string
	}{
		{session.PolicyAsk, protocol.ToolKindRead, session.ReasonPolicyAsk},
		{session.PolicyAutoAllowReadonly, protocol.ToolKindRead, session.ReasonAutoAllowReadonly},
		{session.PolicyAutoAllowReadonly, protocol.ToolKindEdit, session.ReasonNotReadonly},
		{session.PolicyRejectAll, protocol.ToolKindRead, session.ReasonRejectAll},
	}

	for _, c := range cases {
		got := session.Decide(c.policy, ask(c.kind, fullOptions()...))
		if got.Reason != c.want {
			t.Errorf("策略 %s + %s 的 reason = %q, 想要 %q", c.policy, c.kind, got.Reason, c.want)
		}
		for _, r := range got.Reason {
			if r > 127 {
				t.Errorf("reason %q 里有非 ASCII 字符——"+
					"理由码要机器可读，界面按它查词条", got.Reason)
				break
			}
		}
	}
}

// 认不出的策略走**最保守的那条**：交给用户。
//
// 配置文件被手改坏、或者旧版本遗留一个已删除的策略名时，
// 默认「自动允许」会是灾难。
func TestDecide_UnknownPolicyDefersToUser(t *testing.T) {
	got := session.Decide(session.Policy("随便放行"), ask(protocol.ToolKindEdit, fullOptions()...))

	if !got.Deferred {
		t.Errorf("认不出的策略却自动放过了（选了 %q）——"+
			"配置被改坏时默认放行是灾难", got.OptionID)
	}
	if got.Reason != session.ReasonUnknownPolicy {
		t.Errorf("reason = %q, 想要 %q", got.Reason, session.ReasonUnknownPolicy)
	}
}

// 策略全集要能穷举——加一种策略时，界面上的选择器与这里必须同步。
func TestAllPolicies_CoversEveryDecideBranch(t *testing.T) {
	all := session.AllPolicies()
	if len(all) < 3 {
		t.Fatalf("策略全集只有 %d 种，至少该有「每次都问 / 自动允许只读 / 一律拒绝」", len(all))
	}
	for _, p := range all {
		got := session.Decide(p, ask(protocol.ToolKindRead, fullOptions()...))
		if got.Reason == session.ReasonUnknownPolicy {
			t.Errorf("%q 在全集里，Decide 却不认识它——"+
				"加策略时漏了一处，界面上选了它会变成「交给用户」", p)
		}
	}
}

// ── 接进会话：收到反向请求要真的裁决并回应答 ──────────────

// ★★ 策略能自动裁决时，**这一轮不该停下来等人**。
//
// 「自动允许只读」开着，AI 读一个文件却弹出一张卡片让用户点——
// 那这个开关等于没有。
func TestSession_AutoDecidesWithoutAskingUser(t *testing.T) {
	rt := newFakeWithAsk(t, protocol.ToolKindRead)
	asked := 0

	s := openWith(t, rt, session.Permission{
		Policy: session.PolicyAutoAllowReadonly,
		AskUser: func(context.Context, session.PermissionAsk) (session.Answer, error) {
			asked++
			return session.Answer{}, errors.New("不该问到用户")
		},
	})

	reason, err := s.Prompt(context.Background(), "读一下 README", nil)
	if err != nil {
		t.Fatalf("这一轮失败了: %v", err)
	}
	if reason != protocol.StopReasonEndTurn {
		t.Errorf("stopReason = %q, 想要 end_turn", reason)
	}
	if asked != 0 {
		t.Errorf("问了用户 %d 次——「自动允许只读」开着却还在弹卡片，这个开关等于没有", asked)
	}
	if got := rt.LastPermissionOptionID(); got != "opt-allow" {
		t.Errorf("回给 Agent 的 optionId = %q, 想要 opt-allow", got)
	}
}

// ★★ 交给用户时，**用户选的那个 id 原样回给 Agent**。
//
// 按类别重新匹配的话，用户点「拒绝」而 Agent 收到「允许」——
// 这是整个权限体系里后果最严重的一种错。
func TestSession_UserChoiceGoesBackVerbatim(t *testing.T) {
	// 选项的 id 与 kind 故意语义相反：按类别猜的实现会翻车
	rt := newFakeWithTrickyAsk(t)

	s := openWith(t, rt, session.Permission{
		Policy: session.PolicyAsk,
		AskUser: func(_ context.Context, a session.PermissionAsk) (session.Answer, error) {
			// 用户点了名字叫「其实是拒绝」的那个，它的 id 是 opt-allow
			return session.Answer{OptionID: a.Options[0].OptionID}, nil
		},
	})

	if _, err := s.Prompt(context.Background(), "改一下 README", nil); err != nil {
		t.Fatalf("这一轮失败了: %v", err)
	}
	if got := rt.LastPermissionOptionID(); got != "opt-allow" {
		t.Errorf("回给 Agent 的 optionId = %q, 想要用户选的 opt-allow——"+
			"按类别重新匹配的话，用户点「拒绝」而 Agent 收到「允许」", got)
	}
}

// ★ 用户那边出错（界面关了、超时）时回 **cancelled**，不是随便选一个。
//
// 随便选的话，用户关掉窗口就等于默认同意了。
func TestSession_AskUserFailureAnswersCancelled(t *testing.T) {
	rt := newFakeWithAsk(t, protocol.ToolKindEdit)

	s := openWith(t, rt, session.Permission{
		Policy: session.PolicyAsk,
		AskUser: func(context.Context, session.PermissionAsk) (session.Answer, error) {
			return session.Answer{}, errors.New("界面关了")
		},
	})

	reason, _ := s.Prompt(context.Background(), "改一下 README", nil)
	if reason != protocol.StopReasonCancelled {
		t.Errorf("stopReason = %q, 想要 cancelled", reason)
	}
	if got := rt.LastPermissionOutcome(); got != "cancelled" {
		t.Errorf("回给 Agent 的 outcome = %q, 想要 cancelled——"+
			"随便选一个的话，用户关掉窗口就等于默认同意了", got)
	}
}

// ★★ 没配 AskUser 时回 cancelled，**不是自动允许**。
//
// 装配漏了一根线的表现必须是「什么都干不了」，不能是「什么都放行」。
func TestSession_NoAskUserAnswersCancelled(t *testing.T) {
	rt := newFakeWithAsk(t, protocol.ToolKindEdit)
	s := openWith(t, rt, session.Permission{Policy: session.PolicyAsk})

	if _, _ = s.Prompt(context.Background(), "改一下 README", nil); true {
		if got := rt.LastPermissionOutcome(); got != "cancelled" {
			t.Errorf("没配 AskUser 时 outcome = %q, 想要 cancelled——"+
				"装配漏了一根线，表现必须是「什么都干不了」而不是「什么都放行」", got)
		}
	}
}

// ★ R4：裁决要发一条**带理由码**的事件出去。
//
// 用户回头问「它为什么没问我就改了文件」，答案得在时间线上找得到。
func TestSession_R4_EmitsDecisionWithReasonCode(t *testing.T) {
	rt := newFakeWithAsk(t, protocol.ToolKindRead)
	s := openWith(t, rt, session.Permission{Policy: session.PolicyAutoAllowReadonly})

	var events []session.Event
	if _, err := s.Prompt(context.Background(), "读一下", func(e session.Event) {
		events = append(events, e)
	}); err != nil {
		t.Fatalf("这一轮失败了: %v", err)
	}

	for _, e := range events {
		if e.Kind != session.KindPermissionDecided {
			continue
		}
		if e.Decision.Reason != session.ReasonAutoAllowReadonly {
			t.Errorf("理由码 = %q, 想要 %q", e.Decision.Reason, session.ReasonAutoAllowReadonly)
		}
		if e.Decision.OptionID != "opt-allow" {
			t.Errorf("事件里的 optionID = %q", e.Decision.OptionID)
		}
		return
	}
	t.Errorf("没发裁决事件——用户回头问「它为什么没问我就改了文件」，"+
		"答案得在时间线上找得到（收到的是 %v）", kindsOf(events))
}

func kindsOf(events []session.Event) []string {
	out := []string{}
	for _, e := range events {
		out = append(out, string(e.Kind))
	}
	return out
}

// ── 夹具 ─────────────────────────────────────────────────

func newFakeWithAsk(t *testing.T, kind protocol.ToolKind) *fake.Runtime {
	t.Helper()
	return newFakeRuntime(t, &fake.Script{
		Name: "ask",
		Turns: []fake.Turn{{
			Steps: []fake.Step{{Ask: &fake.PermissionAsk{
				ToolCallID: "tool-1", Title: "干点什么", Kind: kind,
				Options: []protocol.PermissionOption{
					{OptionID: "opt-allow", Name: "允许一次", Kind: protocol.PermissionAllowOnce},
					{OptionID: "opt-deny", Name: "拒绝", Kind: protocol.PermissionRejectOnce},
				},
			}}},
			StopReason: protocol.StopReasonEndTurn,
		}},
	})
}

// newFakeWithTrickyAsk 的 optionId 与 kind 语义相反。
func newFakeWithTrickyAsk(t *testing.T) *fake.Runtime {
	t.Helper()
	return newFakeRuntime(t, &fake.Script{
		Name: "tricky",
		Turns: []fake.Turn{{
			Steps: []fake.Step{{Ask: &fake.PermissionAsk{
				ToolCallID: "tool-1", Title: "干点什么", Kind: protocol.ToolKindEdit,
				Options: []protocol.PermissionOption{
					{OptionID: "opt-allow", Name: "其实是拒绝", Kind: protocol.PermissionRejectOnce},
					{OptionID: "opt-deny", Name: "其实是允许", Kind: protocol.PermissionAllowOnce},
				},
			}}},
			StopReason: protocol.StopReasonEndTurn,
		}},
	})
}

func newFakeRuntime(t *testing.T, script *fake.Script) *fake.Runtime {
	t.Helper()
	rt := fake.New(fake.Options{Script: script, Clock: testutil.FixedClock(testutil.T0)})
	t.Cleanup(func() { _ = rt.Close() })
	return rt
}

func openWith(t *testing.T, rt *fake.Runtime, perm session.Permission) *session.Session {
	t.Helper()
	s, err := session.Open(context.Background(), session.Options{
		Transport:  rt.Transport(),
		Cwd:        t.TempDir(),
		Permission: perm,
	})
	if err != nil {
		t.Fatalf("开会话失败: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// ★★ R3：阻塞的**只是这一轮**。
//
// 一条会话等着用户裁决时，另一条会话必须照常跑完。做成全局锁的话，
// 用户开着两个工作，去泡杯咖啡再回来会发现两个都停在那儿——
// 而他只被问了一件事。
func TestSession_R3_BlockingIsPerSession(t *testing.T) {
	// A 会话：卡在权限请求上，永远不回答
	blocked := make(chan struct{})
	t.Cleanup(func() { close(blocked) })

	rtA := newFakeWithAsk(t, protocol.ToolKindEdit)
	sA := openWith(t, rtA, session.Permission{
		Policy: session.PolicyAsk,
		AskUser: func(ctx context.Context, _ session.PermissionAsk) (session.Answer, error) {
			select {
			case <-blocked:
			case <-ctx.Done():
			}
			return session.Answer{}, ctx.Err()
		},
	})

	aDone := make(chan struct{})
	go func() {
		defer close(aDone)
		_, _ = sA.Prompt(context.Background(), "改一下 README", nil)
	}()

	// 等 A 真的卡住了
	waitAsk(t, rtA)

	// B 会话：**也发权限请求**，但它的策略能自动裁决。
	//
	// ★ B 必须走到裁决那段代码，这条测试才测得到东西。
	// 让 B 跑一个没有权限请求的脚本的话，加一把全局锁它照样绿——
	// 造负例时发现的。
	rtB := newFakeWithAsk(t, protocol.ToolKindRead)
	sB := openWith(t, rtB, session.Permission{Policy: session.PolicyAutoAllowReadonly})

	bDone := make(chan protocol.StopReason, 1)
	go func() {
		reason, _ := sB.Prompt(context.Background(), "干点别的", nil)
		bDone <- reason
	}()

	select {
	case reason := <-bDone:
		if reason != protocol.StopReasonEndTurn {
			t.Errorf("B 会话的 stopReason = %q, 想要 end_turn", reason)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("A 会话在等用户裁决，B 会话也跟着停了——\n" +
			"用户开着两个工作、去泡杯咖啡回来会发现两个都不动，而他只被问了一件事")
	}

	// A 还卡着才说明这条测试测到了东西
	select {
	case <-aDone:
		t.Error("A 会话自己结束了，这条测试什么也没证明")
	default:
	}
}

// waitAsk 等 Fake 真的发出了权限请求。
func waitAsk(t *testing.T, rt *fake.Runtime) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if rt.CountMethod(protocol.MethodSessionPrompt) > 0 {
			// prompt 到了，权限请求紧随其后；再让一小会儿
			time.Sleep(100 * time.Millisecond)
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("等 A 会话卡住超时")
}

// ★★ 交给用户时要带上**动的是哪个文件**。
//
// 不带的话，卡片上只写着「AI 请求写入」——用户没法判断该不该允许。
// 真机走查撞到的：三个按钮都在，就是不知道要改什么。
func TestSession_PermissionAskCarriesPath(t *testing.T) {
	tests := []struct {
		name  string
		extra map[string]any
		want  string
	}{
		{
			"locations 里有路径",
			map[string]any{"locations": []any{map[string]any{"path": "/repo/README.md", "line": 1}}},
			"/repo/README.md",
		},
		{
			"只有 rawInput.file_path",
			map[string]any{"rawInput": map[string]any{"file_path": "/repo/main.go"}},
			"/repo/main.go",
		},
		{
			// locations 更准（它是 Agent 明确标出来的位置），rawInput 是原始参数
			"两个都有时用 locations",
			map[string]any{
				"locations": []any{map[string]any{"path": "/repo/a.go"}},
				"rawInput":  map[string]any{"file_path": "/repo/b.go"},
			},
			"/repo/a.go",
		},
		{"两个都没有时留空", map[string]any{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := newFakeAskWithToolCall(t, tt.extra)

			var got session.PermissionAsk
			s := openWith(t, rt, session.Permission{
				Policy: session.PolicyAsk,
				AskUser: func(_ context.Context, a session.PermissionAsk) (session.Answer, error) {
					got = a
					return session.Answer{OptionID: "opt-deny"}, nil
				},
			})
			if _, err := s.Prompt(context.Background(), "改一下", nil); err != nil {
				t.Fatal(err)
			}

			if got.Path != tt.want {
				t.Errorf("path = %q, 想要 %q——"+
					"卡片上只写「AI 请求写入」而不说哪个文件，用户没法判断该不该允许",
					got.Path, tt.want)
			}
		})
	}
}

// newFakeAskWithToolCall 造一个带自定义 toolCall 字段的权限请求脚本。
func newFakeAskWithToolCall(t *testing.T, extra map[string]any) *fake.Runtime {
	t.Helper()
	return newFakeRuntime(t, &fake.Script{
		Name: "ask-with-path",
		Turns: []fake.Turn{{
			Steps: []fake.Step{{Ask: &fake.PermissionAsk{
				ToolCallID: "tool-1", Title: "写文件", Kind: protocol.ToolKindEdit,
				ToolCallExtra: extra,
				Options: []protocol.PermissionOption{
					{OptionID: "opt-allow", Name: "允许", Kind: protocol.PermissionAllowOnce},
					{OptionID: "opt-deny", Name: "拒绝", Kind: protocol.PermissionRejectOnce},
				},
			}}},
			StopReason: protocol.StopReasonEndTurn,
		}},
	})
}
