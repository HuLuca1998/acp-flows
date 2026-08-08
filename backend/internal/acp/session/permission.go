package session

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/HuLuca1998/acp-flows/backend/internal/acp/protocol"
)

// Policy 是权限裁决策略，由用户按角色配置。
type Policy string

// 三种策略。取值会原样出现在配置与 API 里，改名等于破坏用户已有的配置。
const (
	// PolicyAsk 每次都问。默认值，也是认不出配置时的回落。
	PolicyAsk Policy = "ask"
	// PolicyAutoAllowReadonly 自动允许**只读**工具，其余仍然问。
	PolicyAutoAllowReadonly Policy = "auto_allow_readonly"
	// PolicyRejectAll 一律拒绝，包括只读——用户明确表达「我先看着，什么都别动」。
	PolicyRejectAll Policy = "reject_all"
)

var allPolicies = [...]Policy{PolicyAsk, PolicyAutoAllowReadonly, PolicyRejectAll}

// AllPolicies 返回策略全集。界面上的选择器照它渲染，加一种只加一处。
func AllPolicies() []Policy {
	out := make([]Policy, len(allPolicies))
	copy(out, allPolicies[:])
	return out
}

// 裁决理由码。
//
// ★ **机器可读的枚举，不是人话。** 界面按它查 i18n 词条、排查时按它过滤日志。
// 塞一句中文的话，改文案就会把过滤规则改坏，而且没法翻译。
const (
	ReasonPolicyAsk         = "policy_ask"
	ReasonAutoAllowReadonly = "auto_allow_readonly"
	ReasonNotReadonly       = "not_readonly"
	ReasonRejectAll         = "reject_all"
	ReasonNoMatchingOption  = "no_matching_option"
	ReasonUnknownPolicy     = "unknown_policy"
	// ReasonNoAskUser：需要问用户，但没人接这根线（装配漏了）。
	ReasonNoAskUser = "no_ask_user"
	// ReasonUserCancelled：问了，但用户没选（界面关了 / 出错）。
	ReasonUserCancelled = "user_cancelled"
	// ReasonUserChose：用户自己选的。
	ReasonUserChose = "user_chose"
)

// Decision 是一次裁决的结果。
type Decision struct {
	// Deferred 为 true 表示**交给用户**：这一轮要挂着等他点。
	Deferred bool
	// OptionID 是选中的选项，Deferred 时**必定为空**——
	// 带着值的话，调用方可能顺手把它发出去。
	OptionID string
	// Reason 是机器可读的理由码，会进事件载荷（R4）。
	Reason string
}

// readonlyKinds 是「只看不动」的工具类别。
//
// ★ 这张表是**白名单**：不在里面的一律当成会改东西。
// 反过来做（列出危险的）的话，Agent 新增一类工具时会默认落进「自动允许」——
// 那正是最坏的方向：用户以为自己开的是「让它随便看」，
// 实际开的是「让它随便改」，而他不会发现，直到文件被改了。
var readonlyKinds = map[protocol.ToolKind]bool{
	protocol.ToolKindRead:   true,
	protocol.ToolKindSearch: true,
	protocol.ToolKindThink:  true,
}

// Decide 按策略裁决一次权限请求。
//
// ★ 纯函数：没有 IO、没有等待。「交给用户」之后怎么等，是 app 层的事。
//
// ★ **绝不猜 optionId。** Agent 的 optionId 是它自己定义的不透明字符串；
// 策略想「允许」而选项里没有 allow 类的时候，唯一正确的动作是交给用户——
// 猜一个的话，我们可能替他点了「永久允许」。
func Decide(policy Policy, req protocol.RequestPermissionRequest) Decision {
	switch policy {
	case PolicyAsk:
		return askUser(ReasonPolicyAsk)

	case PolicyAutoAllowReadonly:
		if !readonlyKinds[req.ToolCall.Kind] {
			return askUser(ReasonNotReadonly)
		}
		return pick(req.Options,
			protocol.PermissionAllowOnce, protocol.PermissionAllowAlways,
			ReasonAutoAllowReadonly)

	case PolicyRejectAll:
		return pick(req.Options,
			protocol.PermissionRejectOnce, protocol.PermissionRejectAlways,
			ReasonRejectAll)

	default:
		// 配置被手改坏、或者旧版本遗留一个已删除的策略名。
		// 默认放行会是灾难，所以回落到最保守的那条。
		return askUser(ReasonUnknownPolicy)
	}
}

// pick 从选项里挑一个，**优先 once 而非 always**。
//
// ★ 替用户做的决定要尽量小。选 always 的话，一次自动裁决会永久改变
// 后续所有请求的处理方式，而用户根本不知道发生过这件事。
//
// 两类都没有时交给用户，绝不退而求其次去猜。
func pick(
	options []protocol.PermissionOption,
	once, always protocol.PermissionOptionKind,
	reason string,
) Decision {
	var fallback string
	for _, o := range options {
		switch o.Kind {
		case once:
			return Decision{OptionID: o.OptionID, Reason: reason}
		case always:
			// 先记着，继续找 once；找不到才用它
			if fallback == "" {
				fallback = o.OptionID
			}
		}
	}
	if fallback != "" {
		return Decision{OptionID: fallback, Reason: reason}
	}
	return askUser(ReasonNoMatchingOption)
}

// askUser 构造「交给用户」的裁决。OptionID 必定为空——
// 带着值的话，调用方可能顺手把它发出去。
func askUser(reason string) Decision {
	return Decision{Deferred: true, Reason: reason}
}

// PermissionAsk 是交给用户的一次权限请求。
//
// ★ 是**协议原文的转述**，不是重新解释：Options 原样带上，
// 让界面照 Agent 给的名字渲染按钮。我们自己造一套「允许/拒绝」的话，
// Agent 提供的第三种选项就消失了，而用户根本不知道自己少了一个选择。
type PermissionAsk struct {
	SessionID  string
	ToolCallID string
	Title      string
	Kind       protocol.ToolKind
	// Path 是要动的文件。**卡片上必须说清楚动的是哪个文件**——
	// 只写「AI 请求写入」的话，用户没法判断该不该允许。
	// Agent 不一定给得出（比如执行命令），那时留空。
	Path    string
	Options []protocol.PermissionOption
	// Reason 是走到「问用户」这一步的理由码，界面可以据此解释「为什么问你」。
	Reason string
}

// Answer 是用户的选择。
type Answer struct {
	// OptionID 必须是 Options 里的某个 id，**原样回传**。
	//
	// 空串表示「没选」——按取消处理，绝不退而求其次挑一个。
	OptionID string
}

// AskUserFunc 把权限请求交给用户，阻塞到他做出选择。
//
// ★ **没有超时**。用户可能去泡了杯咖啡；替他超时等于替他做决定。
// 要中止只能靠 ctx——那时会回 cancelled 给 Agent。
type AskUserFunc func(context.Context, PermissionAsk) (Answer, error)

// Permission 是会话的权限配置。
type Permission struct {
	// Policy 留空时按 PolicyAsk 处理——最保守的那条。
	Policy Policy
	// AskUser 为 nil 时，所有需要问用户的请求一律回 cancelled。
	//
	// ★ 装配漏了一根线的表现必须是「什么都干不了」，不能是「什么都放行」。
	AskUser AskUserFunc
}

// handlePermission 处理 Agent 发来的 session/request_permission。
//
// 返回值直接作为反向请求的响应体。
func (s *Session) handlePermission(ctx context.Context, params json.RawMessage) (any, error) {
	var req protocol.RequestPermissionRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, fmt.Errorf("解析 session/request_permission: %w", err)
	}

	policy := s.permission.Policy
	if policy == "" {
		policy = PolicyAsk
	}
	decision := Decide(policy, req)

	if !decision.Deferred {
		s.emitDecision(decision)
		return permissionResponse(protocol.SelectedOutcome(decision.OptionID)), nil
	}

	// ★ 交给用户。没配 AskUser 就回 cancelled——不是自动允许。
	if s.permission.AskUser == nil {
		s.emitDecision(Decision{Reason: ReasonNoAskUser})
		return permissionResponse(protocol.CancelledOutcome()), nil
	}

	// ★ 登记「怎么放掉这一条」：取消时要用 cancelled 应答**所有**
	// pending 的请求，否则 Agent 一直等我们回答，这一轮永远不会结束。
	askCtx, releaseAsk := context.WithCancel(ctx)
	defer releaseAsk()
	s.addPermissionRelease(releaseAsk)

	answer, err := s.permission.AskUser(askCtx, PermissionAsk{
		SessionID:  req.SessionID,
		ToolCallID: req.ToolCall.ToolCallID,
		Title:      req.ToolCall.Title,
		Kind:       req.ToolCall.Kind,
		Path:       pathOf(req.ToolCall),
		Options:    req.Options,
		Reason:     decision.Reason,
	})
	if err != nil || answer.OptionID == "" {
		// 界面关了、超时、用户没选——一律 cancelled。
		// 随便挑一个的话，用户关掉窗口就等于默认同意了。
		s.emitDecision(Decision{Reason: ReasonUserCancelled})
		return permissionResponse(protocol.CancelledOutcome()), nil
	}

	// ★ 用户选的 id **原样回传**，不按类别重新匹配——
	// 重新匹配的话，用户点「拒绝」而 Agent 收到「允许」。
	s.emitDecision(Decision{OptionID: answer.OptionID, Reason: ReasonUserChose})
	return permissionResponse(protocol.SelectedOutcome(answer.OptionID)), nil
}

// permissionResponse 把 outcome 包成响应体。
//
// ★ 直接用协议类型：它自带 MarshalJSON（判别式写出，selected 才带 optionId）。
// 在这里手拼 map 的话，判别式的规则就有了两份实现，必然漂移。
func permissionResponse(outcome protocol.RequestPermissionOutcome) protocol.RequestPermissionResponse {
	return protocol.RequestPermissionResponse{Outcome: outcome}
}

// emitDecision 把裁决作为一条事件交出去（R4）。
//
// ★ 用户回头问「它为什么没问我就改了文件」，答案得在时间线上找得到。
func (s *Session) emitDecision(d Decision) {
	s.mu.Lock()
	cb := s.onEvent
	s.mu.Unlock()
	if cb == nil {
		return
	}
	cb(Event{Kind: KindPermissionDecided, Decision: d})
}

// pathOf 从工具调用里取出要动的文件。
//
// ★ 优先 locations：那是 Agent **明确标出来的位置**；rawInput 是它收到的
// 原始参数，字段名各家不一样（file_path / path / filePath 都见过）。
// 两个都没有时返回空串——Agent 确实不一定给得出（比如执行一条命令）。
func pathOf(tc protocol.ToolCallUpdate) string {
	for _, loc := range tc.Locations {
		if loc.Path != "" {
			return loc.Path
		}
	}
	if len(tc.RawInput) == 0 {
		return ""
	}
	var raw map[string]any
	if err := json.Unmarshal(tc.RawInput, &raw); err != nil {
		return ""
	}
	// 见过的几种写法，按常见程度排
	for _, key := range []string{"file_path", "path", "filePath"} {
		if v, ok := raw[key].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// addPermissionRelease 把「放掉这一条待裁决请求」串进 releasePermissions。
//
// ★ 串起来而不是覆盖：一轮里可能同时挂着好几条（真 Agent 一轮问三五次是
// 常态），只记最后一条的话，取消时前面几条会永远挂着——那条会话再也不会结束。
func (s *Session) addPermissionRelease(release func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.releasePermissions
	s.releasePermissions = func() {
		if prev != nil {
			prev()
		}
		release()
	}
}
