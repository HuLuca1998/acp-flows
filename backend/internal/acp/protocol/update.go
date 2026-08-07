// Package protocol 定义 ACP v1 的线格式：类型、枚举、编解码。
//
// **零 IO、零状态。** 本包不认识「会话正在进行中」这种概念，
// 也不 import 本包内任何其他子包（acp-integration.md §3.3 第 1 条硬规则）。
//
// 它存在的理由是给 fake.Runtime 一个**不依赖被测代码**的协议参照物：
// Fake 若复用了 session 或 jsonrpc 的实现，测试就变成「用被测代码验证被测代码」，
// 参照物资格当场失效（同上，第 2 条硬规则）。
//
// 类型与字段名逐条对照 @agentclientprotocol/sdk@1.3.0 的
// dist/schema/types.gen.d.ts（PROTOCOL_VERSION = 1）抄写，
// 裁定过程见 docs/notes/acp-field-notes.md §7.2。
// **SDK 里另有 dist/v2/schema，本包不实现它** —— 真机协商出来的版本是 1。
package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ProtocolVersionV1 是本包实现的协议版本。
//
// 真机实测（2026-08-07，codex-acp 1.1.7 / claude-agent-acp 0.63.0）两端协商结果都是 1。
// 协议版本以**协商结果**为准，不以 SDK 目录里存在什么为准。
const ProtocolVersionV1 = 1

// SessionUpdateKind 是 session/update 通知里 update.sessionUpdate 的判别值。
type SessionUpdateKind string

// v1 schema 的 SessionUpdate 判别值全集，顺序照官方 union 的声明顺序。
const (
	UpdateUserMessageChunk        SessionUpdateKind = "user_message_chunk"
	UpdateAgentMessageChunk       SessionUpdateKind = "agent_message_chunk"
	UpdateAgentThoughtChunk       SessionUpdateKind = "agent_thought_chunk"
	UpdateToolCall                SessionUpdateKind = "tool_call"
	UpdateToolCallUpdate          SessionUpdateKind = "tool_call_update"
	UpdatePlan                    SessionUpdateKind = "plan"
	UpdatePlanUpdate              SessionUpdateKind = "plan_update"
	UpdatePlanRemoved             SessionUpdateKind = "plan_removed"
	UpdateAvailableCommandsUpdate SessionUpdateKind = "available_commands_update"
	UpdateCurrentModeUpdate       SessionUpdateKind = "current_mode_update"
	UpdateConfigOptionUpdate      SessionUpdateKind = "config_option_update"
	UpdateSessionInfoUpdate       SessionUpdateKind = "session_info_update"
	UpdateUsageUpdate             SessionUpdateKind = "usage_update"
)

// allSessionUpdateKinds 是全集的唯一真源。AllSessionUpdateKinds 返回它的副本。
var allSessionUpdateKinds = [...]SessionUpdateKind{
	UpdateUserMessageChunk,
	UpdateAgentMessageChunk,
	UpdateAgentThoughtChunk,
	UpdateToolCall,
	UpdateToolCallUpdate,
	UpdatePlan,
	UpdatePlanUpdate,
	UpdatePlanRemoved,
	UpdateAvailableCommandsUpdate,
	UpdateCurrentModeUpdate,
	UpdateConfigOptionUpdate,
	UpdateSessionInfoUpdate,
	UpdateUsageUpdate,
}

// experimentalUpdateKinds 是官方标了 UNSTABLE / @experimental 的判别值。
//
// 「认得」和「可依赖」是两件事：认得是为了不把它们当未知变体刷 warn 日志，
// 标注是为了不让上层建立任何映射——它们随时可能改或删。
var experimentalUpdateKinds = map[SessionUpdateKind]bool{
	UpdatePlanUpdate:  true,
	UpdatePlanRemoved: true,
}

// AllSessionUpdateKinds 返回 v1 schema 的判别值全集，按官方声明顺序。
func AllSessionUpdateKinds() []SessionUpdateKind {
	out := make([]SessionUpdateKind, len(allSessionUpdateKinds))
	copy(out, allSessionUpdateKinds[:])
	return out
}

// IsValid 报告该判别值是否属于 v1 schema 全集。
//
// 不在全集里**不是错误**：能力是可扩展的，新增变体不算破坏性变更。
// 调用方应记 warn 并丢弃，而不是断开连接——否则每次 Runtime 升级都炸一次。
func (k SessionUpdateKind) IsValid() bool {
	for _, v := range allSessionUpdateKinds {
		if v == k {
			return true
		}
	}
	return false
}

// IsExperimental 报告该判别值是否被官方标为 UNSTABLE / @experimental。
//
// 未知判别值返回 false —— 「不认识」和「认识但不稳定」的处理路径不同，不能混。
func (k SessionUpdateKind) IsExperimental() bool { return experimentalUpdateKinds[k] }

// ErrMissingDiscriminator 表示一条 session/update 缺少 sessionUpdate 判别字段。
//
// 这与「判别值不认识」是两回事：缺字段说明报文本身不合协议，
// 而不认识的判别值属于正常的协议演进。
var ErrMissingDiscriminator = errors.New("protocol: session/update 缺少 sessionUpdate 判别字段")

// SessionUpdate 是一条会话更新的判别式包装。
//
// 它**保留完整原始载荷**，未知判别值也能无损转发。
// 上层据此把没见过的变体原样记进日志排查，而不是只知道「有个东西没认出来」。
type SessionUpdate struct {
	kind SessionUpdateKind
	raw  json.RawMessage
}

// Kind 返回判别值。未知判别值原样返回，不会被规整成空值。
func (u SessionUpdate) Kind() SessionUpdateKind { return u.kind }

// Payload 返回完整的原始载荷（含 sessionUpdate 字段本身）。
//
// 调用方按 Kind 决定解成哪个变体类型，例如：
//
//	var chunk protocol.ContentChunkUpdate
//	err := json.Unmarshal(u.Payload(), &chunk)
func (u SessionUpdate) Payload() json.RawMessage { return u.raw }

// UnmarshalJSON 解析一条 session/update，只读出判别值，载荷原样留存。
func (u *SessionUpdate) UnmarshalJSON(b []byte) error {
	var probe struct {
		SessionUpdate *SessionUpdateKind `json:"sessionUpdate"`
	}
	if err := json.Unmarshal(b, &probe); err != nil {
		return fmt.Errorf("protocol: 解析 session/update 失败: %w", err)
	}
	if probe.SessionUpdate == nil {
		return ErrMissingDiscriminator
	}
	u.kind = *probe.SessionUpdate
	u.raw = append(json.RawMessage(nil), b...)
	return nil
}

// MarshalJSON 原样写回载荷。
func (u SessionUpdate) MarshalJSON() ([]byte, error) {
	if len(u.raw) == 0 {
		return nil, fmt.Errorf("protocol: SessionUpdate 未初始化（kind=%q）", u.kind)
	}
	return u.raw, nil
}

// NewSessionUpdate 把一个变体载荷包成 SessionUpdate。
//
// 判别值从序列化结果里读回，而不是由调用方另外传一遍 ——
// 传两遍就会出现「结构体说自己是 tool_call、参数说是 plan」的不一致。
func NewSessionUpdate(payload any) (SessionUpdate, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return SessionUpdate{}, fmt.Errorf("protocol: 序列化 session/update 载荷失败: %w", err)
	}
	var u SessionUpdate
	if err := u.UnmarshalJSON(b); err != nil {
		return SessionUpdate{}, err
	}
	return u, nil
}

// SessionNotification 是 session/update 通知的 params。
//
// ★ SessionID 必须端到端穿透。前一个项目最严重的一条错误（H-1）就是它在某一层丢了，
// 表现为「第 2 轮不记得第 1 轮」（acp-field-notes.md §1）。
type SessionNotification struct {
	SessionID string          `json:"sessionId"`
	Update    SessionUpdate   `json:"update"`
	Meta      json.RawMessage `json:"_meta,omitempty"`
}

// ── 13 个变体的载荷 ────────────────────────────────────────────────

// ContentChunkUpdate 是三个 chunk 变体的载荷：
// user_message_chunk / agent_message_chunk / agent_thought_chunk。
// 三者字段完全相同，只有判别值不同。
//
// MessageID 用于分组：同 id 属同一条消息，id 变了就是新消息。
type ContentChunkUpdate struct {
	SessionUpdate SessionUpdateKind `json:"sessionUpdate"`
	Content       ContentBlock      `json:"content"`
	MessageID     string            `json:"messageId,omitempty"`
	Meta          json.RawMessage   `json:"_meta,omitempty"`
}

// ToolCallStartUpdate 是 tool_call 变体：一次工具调用的首次声明。
type ToolCallStartUpdate struct {
	SessionUpdate SessionUpdateKind `json:"sessionUpdate"`
	ToolCall
}

// ToolCallDeltaUpdate 是 tool_call_update 变体：工具调用的增量更新。
//
// 除 toolCallId 外全部字段都是可选增量，本层原样上抛，
// **字段级合并是 session 层的职责**（acp-integration.md §11.2）。
type ToolCallDeltaUpdate struct {
	SessionUpdate SessionUpdateKind `json:"sessionUpdate"`
	ToolCallUpdate
}

// PlanSnapshotUpdate 是 plan 变体：Agent 的 TODO 清单**全量快照**。
//
// ★ 同名陷阱：这**不是** Duet 的 PlanVersion。映射过去会把 Agent 的临时待办
// 写进只增不改的计划版本链，是严重数据污染。上层一律丢弃（§11.2）。
type PlanSnapshotUpdate struct {
	SessionUpdate SessionUpdateKind `json:"sessionUpdate"`
	Entries       []PlanEntry       `json:"entries"`
	Meta          json.RawMessage   `json:"_meta,omitempty"`
}

// PlanContentUpdate 是 plan_update 变体。**官方标记 UNSTABLE / @experimental。**
type PlanContentUpdate struct {
	SessionUpdate SessionUpdateKind `json:"sessionUpdate"`
	Plan          PlanContent       `json:"plan"`
	Meta          json.RawMessage   `json:"_meta,omitempty"`
}

// PlanRemovedUpdate 是 plan_removed 变体。**官方标记 UNSTABLE / @experimental。**
type PlanRemovedUpdate struct {
	SessionUpdate SessionUpdateKind `json:"sessionUpdate"`
	PlanID        string            `json:"planId"`
	Meta          json.RawMessage   `json:"_meta,omitempty"`
}

// AvailableCommandsUpdate 是 available_commands_update 变体。M0 不用 slash command。
type AvailableCommandsUpdate struct {
	SessionUpdate     SessionUpdateKind  `json:"sessionUpdate"`
	AvailableCommands []AvailableCommand `json:"availableCommands"`
	Meta              json.RawMessage    `json:"_meta,omitempty"`
}

// CurrentModeUpdate 是 current_mode_update 变体。
//
// Agent 可以自己换 mode，所以这条会主动到来。本层不产事件，
// 由 session 层更新会话元数据（§11.2）。
type CurrentModeUpdate struct {
	SessionUpdate SessionUpdateKind `json:"sessionUpdate"`
	CurrentModeID string            `json:"currentModeId"`
	Meta          json.RawMessage   `json:"_meta,omitempty"`
}

// ConfigOptionUpdate 是 config_option_update 变体。
type ConfigOptionUpdate struct {
	SessionUpdate SessionUpdateKind `json:"sessionUpdate"`
	ConfigOptions []ConfigOption    `json:"configOptions"`
	Meta          json.RawMessage   `json:"_meta,omitempty"`
}

// SessionInfoUpdate 是 session_info_update 变体：标题与时间戳，与事件流无关。
type SessionInfoUpdate struct {
	SessionUpdate SessionUpdateKind `json:"sessionUpdate"`
	Title         string            `json:"title,omitempty"`
	UpdatedAt     string            `json:"updatedAt,omitempty"`
	Meta          json.RawMessage   `json:"_meta,omitempty"`
}

// UsageUpdate 是 usage_update 变体：上下文窗口用量与成本。
//
// ★ 注意它与 PromptResponse 里的 Usage **不是同一个类型**：
// 这里是「窗口用了多少」，那里是 token 明细。
// 成本一律用 Cost.Amount —— totalTokens 会严重高估（缓存读远大于新增 token，
// acp-field-notes.md §6）。
type UsageUpdate struct {
	SessionUpdate SessionUpdateKind `json:"sessionUpdate"`
	Used          int64             `json:"used"`
	Size          int64             `json:"size"`
	Cost          *Cost             `json:"cost,omitempty"`
	Meta          json.RawMessage   `json:"_meta,omitempty"`
}

// Cost 是一次会话累计的成本。**只有 claude 会给**（codex 没有）。
type Cost struct {
	Amount   float64         `json:"amount"`
	Currency string          `json:"currency"`
	Meta     json.RawMessage `json:"_meta,omitempty"`
}
