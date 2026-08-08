package protocol

import "encoding/json"

// ACP 的方法名常量。
//
// 只在本包出现字面量 —— 别处拼字符串的话，改一个方法名要全仓库 grep。
const (
	MethodInitialize        = "initialize"
	MethodAuthenticate      = "authenticate"
	MethodSessionNew        = "session/new"
	MethodSessionLoad       = "session/load"
	MethodSessionPrompt     = "session/prompt"
	MethodSessionSetMode    = "session/set_mode"
	MethodSessionSetConfig  = "session/set_config_option"
	MethodSessionUpdate     = "session/update"             // 通知，Agent → 客户端
	MethodSessionCancel     = "session/cancel"             // ★ 通知，不是请求
	MethodRequestPermission = "session/request_permission" // 反向请求，Agent → 客户端
)

// ConfigSelectOption 是 select 型配置项的一个可选值。
type ConfigSelectOption struct {
	Value       string          `json:"value"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Meta        json.RawMessage `json:"_meta,omitempty"`
}

// ConfigOption 是会话级配置项（session/new 响应与 config_option_update 里都有）。
//
// ★ **按 Category 取，不按 ID 取。** 推理强度在 claude 是 id=effort、
// codex 是 id=reasoning_effort，但两端 category 都是 thought_level。
// 这是差异内化最漂亮的实证：协议本身给了语义层的稳定键，不需要维护映射表
// （acp-field-notes.md §7.1）。
//
// Category 是 *string 而非 string：**claude 的 agent 选项 category 是空字符串**
// （实测，field-notes N2），而「字段缺失」与「空字符串」是两回事，
// 都要能无损表达。取值时用 CategoryOrEmpty。
type ConfigOption struct {
	ID           string               `json:"id"`
	Name         string               `json:"name"`
	Description  string               `json:"description,omitempty"`
	Category     *string              `json:"category,omitempty"`
	Type         string               `json:"type"` // select | boolean
	CurrentValue json.RawMessage      `json:"currentValue"`
	Options      []ConfigSelectOption `json:"options,omitempty"`
	Meta         json.RawMessage      `json:"_meta,omitempty"`
}

// CategoryOrEmpty 返回 category，字段缺失时返回空串。
//
// 按 category 取配置项时用它 —— 直接解引用会在 claude 的某些选项上 panic。
func (o ConfigOption) CategoryOrEmpty() string {
	if o.Category == nil {
		return ""
	}
	return *o.Category
}

// AvailableCommandInput 描述一个 slash command 的输入提示。
type AvailableCommandInput struct {
	Hint string          `json:"hint"`
	Meta json.RawMessage `json:"_meta,omitempty"`
}

// AvailableCommand 是 Agent 声明的一个 slash command。M0 不用。
type AvailableCommand struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Input       *AvailableCommandInput `json:"input,omitempty"`
	Meta        json.RawMessage        `json:"_meta,omitempty"`
}

// SessionMode 是一个会话模式。
type SessionMode struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Meta        json.RawMessage `json:"_meta,omitempty"`
}

// SessionModeState 是 session/new 响应里的模式状态。
//
// ★ availableModes **不能硬编码** —— 两端档位完全不同，且会随版本变
// （acp-integration.md §5.3）。
type SessionModeState struct {
	CurrentModeID  string          `json:"currentModeId"`
	AvailableModes []SessionMode   `json:"availableModes"`
	Meta           json.RawMessage `json:"_meta,omitempty"`
}

// MCPServer 是一个 MCP 服务器声明。M0 不注入 MCP，故只做无损透传。
type MCPServer = json.RawMessage

// NewSessionRequest 是 session/new 的 params。
//
// ★ 两个坑：
//   - Cwd 必须是**已存在的绝对路径**，且要在发请求前校验（相对路径的失败信息很难懂）。
//   - MCPServers **必须显式传空数组，不能是 null**：codex 收到非空会整体覆盖
//     thread config 的 mcp_servers 键，禁用条目全部丢失（acp-field-notes.md §4）。
//     所以这个字段没有 omitempty —— 但 nil slice 会写成 null，构造时务必给 []MCPServer{}。
type NewSessionRequest struct {
	Cwd                   string          `json:"cwd"`
	AdditionalDirectories []string        `json:"additionalDirectories,omitempty"`
	MCPServers            []MCPServer     `json:"mcpServers"`
	Meta                  json.RawMessage `json:"_meta,omitempty"`
}

// NewSessionResponse 是 session/new 的响应。
//
// ★ claude 的响应顶层**没有** models 字段（实测复验，field-notes §7.1）。
// 模型与推理强度都在 ConfigOptions 里，按 category 取。
type NewSessionResponse struct {
	SessionID     string            `json:"sessionId"`
	Modes         *SessionModeState `json:"modes,omitempty"`
	ConfigOptions []ConfigOption    `json:"configOptions,omitempty"`
	Meta          json.RawMessage   `json:"_meta,omitempty"`
}

// PromptRequest 是 session/prompt 的 params。
//
// ★ 系统提示词**只在首轮发**。每轮都发会污染上下文并推高成本
// （前一个项目的 H-3，acp-field-notes.md §1）。
type PromptRequest struct {
	SessionID string          `json:"sessionId"`
	Prompt    []ContentBlock  `json:"prompt"`
	Meta      json.RawMessage `json:"_meta,omitempty"`
}

// Usage 是一轮的 token 明细。
//
// ★ 成本核算**不要用 TotalTokens** —— 缓存读远大于新增 token，会严重高估。
// 用 UsageUpdate.Cost.Amount，但**只有 claude 给**（acp-field-notes.md §6）。
type Usage struct {
	TotalTokens       int64           `json:"totalTokens"`
	InputTokens       int64           `json:"inputTokens"`
	OutputTokens      int64           `json:"outputTokens"`
	ThoughtTokens     *int64          `json:"thoughtTokens,omitempty"`
	CachedReadTokens  *int64          `json:"cachedReadTokens,omitempty"`
	CachedWriteTokens *int64          `json:"cachedWriteTokens,omitempty"`
	Meta              json.RawMessage `json:"_meta,omitempty"`
}

// PromptResponse 是 session/prompt 的响应。
//
// ★ StopReason 在**这里**，不是通知。两段式取消的同步点就是这个响应带回
// stopReason=cancelled（acp-integration.md §6.1）。
type PromptResponse struct {
	StopReason StopReason      `json:"stopReason"`
	Usage      *Usage          `json:"usage,omitempty"`
	Meta       json.RawMessage `json:"_meta,omitempty"`
}

// LoadSessionRequest 是 session/load 的 params。
//
// ★ 恢复一条**已有**会话，不是新开。Agent 会把那条会话的历史重放回来，
// 于是新一轮能接着上次继续——这是「关掉再打开，它还记得刚才做到哪」
// 的唯一实现途径。
//
// ★ 不是所有 Agent 都支持：不支持的会回 -32601，那时必须**显式降级**
// 为新会话，绝不假装恢复成功。
type LoadSessionRequest struct {
	SessionID string `json:"sessionId"`
	Cwd       string `json:"cwd"`
	// MCPServers 与 session/new 同理：**不能是 nil**，
	// nil slice 会写成 null 而覆盖掉 Agent 那边的配置。
	MCPServers []MCPServer     `json:"mcpServers"`
	Meta       json.RawMessage `json:"_meta,omitempty"`
}

// LoadSessionResponse 是 session/load 的响应。
//
// 字段与 NewSessionResponse 一致：Agent 可以借这次机会更新模式等信息。
type LoadSessionResponse struct {
	Modes *SessionModeState `json:"modes,omitempty"`
	Meta  json.RawMessage   `json:"_meta,omitempty"`
}

// CancelNotification 是 session/cancel 的 params。
//
// ★ session/cancel 是**通知，不是请求** —— 发出去不会有响应。
// 取消完成的唯一凭据是 session/prompt 的响应带回 stopReason=cancelled。
// 这就是「两段式」的由来。
type CancelNotification struct {
	SessionID string          `json:"sessionId"`
	Meta      json.RawMessage `json:"_meta,omitempty"`
}
