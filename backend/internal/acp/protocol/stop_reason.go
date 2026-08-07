package protocol

// StopReason 是一轮结束的原因。
//
// ★ 它**不是通知，是 session/prompt 的响应字段**。前端看到的 turn_end
// 是应用侧合成的事件（acp-integration.md §2.2-C7）。
type StopReason string

// v1 schema 的 StopReason 全集，顺序照官方声明顺序。
const (
	// StopReasonEndTurn 是唯一算正常收尾的取值。
	StopReasonEndTurn StopReason = "end_turn"
	// StopReasonMaxTokens 表示被 token 上限截断 —— 产出是半成品。
	StopReasonMaxTokens StopReason = "max_tokens"
	// StopReasonMaxTurnRequests 表示轮内请求数超限。
	StopReasonMaxTurnRequests StopReason = "max_turn_requests"
	// StopReasonRefusal 表示模型拒绝执行。
	StopReasonRefusal StopReason = "refusal"
	// StopReasonCancelled 是两段式取消的同步点：取消完成的唯一凭据。
	StopReasonCancelled StopReason = "cancelled"
)

var allStopReasons = [...]StopReason{
	StopReasonEndTurn,
	StopReasonMaxTokens,
	StopReasonMaxTurnRequests,
	StopReasonRefusal,
	StopReasonCancelled,
}

// AllStopReasons 返回 StopReason 全集，按官方声明顺序。
func AllStopReasons() []StopReason {
	out := make([]StopReason, len(allStopReasons))
	copy(out, allStopReasons[:])
	return out
}

// IsValid 报告取值是否在全集内。
func (r StopReason) IsValid() bool {
	for _, v := range allStopReasons {
		if v == r {
			return true
		}
	}
	return false
}

// IsSuccess 报告这一轮是否正常收尾。**只有 end_turn 算。**
//
// ★ 前一个项目的 H-5：把所有 stopReason 都当成功处理，
// 结果 max_tokens 截断的半成品被当作已完成验收（acp-field-notes.md §1）。
// 判断放在协议层，是为了让上层不可能「忘了判断」。
func (r StopReason) IsSuccess() bool { return r == StopReasonEndTurn }
