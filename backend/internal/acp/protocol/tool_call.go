package protocol

import "encoding/json"

// ToolKind 是工具调用的类别，决定前端用哪个图标与渲染器。
type ToolKind string

// v1 schema 的 ToolKind 全集，顺序照官方声明顺序。
const (
	ToolKindRead       ToolKind = "read"
	ToolKindEdit       ToolKind = "edit"
	ToolKindDelete     ToolKind = "delete"
	ToolKindMove       ToolKind = "move"
	ToolKindSearch     ToolKind = "search"
	ToolKindExecute    ToolKind = "execute"
	ToolKindThink      ToolKind = "think"
	ToolKindFetch      ToolKind = "fetch"
	ToolKindSwitchMode ToolKind = "switch_mode"
	ToolKindOther      ToolKind = "other"
)

var allToolKinds = [...]ToolKind{
	ToolKindRead, ToolKindEdit, ToolKindDelete, ToolKindMove, ToolKindSearch,
	ToolKindExecute, ToolKindThink, ToolKindFetch, ToolKindSwitchMode, ToolKindOther,
}

// AllToolKinds 返回 ToolKind 全集，按官方声明顺序。
func AllToolKinds() []ToolKind {
	out := make([]ToolKind, len(allToolKinds))
	copy(out, allToolKinds[:])
	return out
}

// IsValid 报告取值是否在全集内。空值一律非法——漏填 ≠ 合法取值。
func (k ToolKind) IsValid() bool {
	for _, v := range allToolKinds {
		if v == k {
			return true
		}
	}
	return false
}

// ToolCallStatus 是工具调用的执行状态。
type ToolCallStatus string

// v1 schema 的 ToolCallStatus 全集，顺序照官方声明顺序。
const (
	ToolCallStatusPending    ToolCallStatus = "pending"
	ToolCallStatusInProgress ToolCallStatus = "in_progress"
	ToolCallStatusCompleted  ToolCallStatus = "completed"
	ToolCallStatusFailed     ToolCallStatus = "failed"
)

var allToolCallStatuses = [...]ToolCallStatus{
	ToolCallStatusPending, ToolCallStatusInProgress,
	ToolCallStatusCompleted, ToolCallStatusFailed,
}

// AllToolCallStatuses 返回 ToolCallStatus 全集，按官方声明顺序。
func AllToolCallStatuses() []ToolCallStatus {
	out := make([]ToolCallStatus, len(allToolCallStatuses))
	copy(out, allToolCallStatuses[:])
	return out
}

// IsValid 报告取值是否在全集内。
//
// ★ 注意这里**没有** cancelled —— 那是 StopReason 的取值。
// 两个枚举都有「结束」的语义但取值不同，混用会静默失配。
func (s ToolCallStatus) IsValid() bool {
	for _, v := range allToolCallStatuses {
		if v == s {
			return true
		}
	}
	return false
}

// ToolCallLocation 是工具触碰到的位置，用于前端「跟随 Agent 视线」。
type ToolCallLocation struct {
	Path string          `json:"path"`
	Line *int            `json:"line,omitempty"`
	Meta json.RawMessage `json:"_meta,omitempty"`
}

// ToolCall 是一次工具调用的完整声明（tool_call 变体的载荷）。
type ToolCall struct {
	ToolCallID string             `json:"toolCallId"`
	Title      string             `json:"title"`
	Name       string             `json:"name,omitempty"`
	Kind       ToolKind           `json:"kind,omitempty"`
	Status     ToolCallStatus     `json:"status,omitempty"`
	Content    []ToolCallContent  `json:"content,omitempty"`
	Locations  []ToolCallLocation `json:"locations,omitempty"`
	RawInput   json.RawMessage    `json:"rawInput,omitempty"`
	RawOutput  json.RawMessage    `json:"rawOutput,omitempty"`
	Meta       json.RawMessage    `json:"_meta,omitempty"`
}

// ToolCallUpdate 是一次工具调用的增量更新。
//
// 除 ToolCallID 外**全部字段可选**：收到什么就是变了什么，没收到的保持原值。
// 这个类型在两处复用 —— tool_call_update 变体，以及 session/request_permission
// 的 toolCall 字段（那里 Agent 告诉你它想执行什么）。
type ToolCallUpdate struct {
	ToolCallID string             `json:"toolCallId"`
	Kind       ToolKind           `json:"kind,omitempty"`
	Status     ToolCallStatus     `json:"status,omitempty"`
	Title      string             `json:"title,omitempty"`
	Name       string             `json:"name,omitempty"`
	Content    []ToolCallContent  `json:"content,omitempty"`
	Locations  []ToolCallLocation `json:"locations,omitempty"`
	RawInput   json.RawMessage    `json:"rawInput,omitempty"`
	RawOutput  json.RawMessage    `json:"rawOutput,omitempty"`
	Meta       json.RawMessage    `json:"_meta,omitempty"`
}
