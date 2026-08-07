package protocol

import "encoding/json"

// PlanEntryStatus 是 Agent TODO 清单里一条待办的状态。
type PlanEntryStatus string

// v1 schema 的 PlanEntryStatus 全集。
const (
	PlanEntryPending    PlanEntryStatus = "pending"
	PlanEntryInProgress PlanEntryStatus = "in_progress"
	PlanEntryCompleted  PlanEntryStatus = "completed"
)

// PlanEntryPriority 是一条待办的相对重要性。
type PlanEntryPriority string

// v1 schema 的 PlanEntryPriority 全集。
const (
	PlanPriorityHigh   PlanEntryPriority = "high"
	PlanPriorityMedium PlanEntryPriority = "medium"
	PlanPriorityLow    PlanEntryPriority = "low"
)

// PlanEntry 是 Agent TODO 清单里的一条。
//
// ★ 再说一遍：这是 **Agent 的临时待办**，不是 Duet 的 PlanVersion。
type PlanEntry struct {
	Content  string            `json:"content"`
	Priority PlanEntryPriority `json:"priority"`
	Status   PlanEntryStatus   `json:"status"`
	Meta     json.RawMessage   `json:"_meta,omitempty"`
}

// PlanContent 是 plan_update 变体的载荷，三种类型：items / file / markdown。
//
// **官方标记 UNSTABLE / @experimental**，且上层一律丢弃（acp-integration.md §11.2）。
// 所以这里只做无损转发，**故意不建强类型**：给一个随时会改的实验特性做映射，
// 下次 SDK 升级就得跟着改一遍，而漏改的字段会静默丢失。
type PlanContent = json.RawMessage
