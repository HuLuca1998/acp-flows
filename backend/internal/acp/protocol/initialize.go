package protocol

import "encoding/json"

// Implementation 是一端的自我介绍（clientInfo / agentInfo）。
type Implementation struct {
	Name    string          `json:"name"`
	Title   string          `json:"title,omitempty"`
	Version string          `json:"version"`
	Meta    json.RawMessage `json:"_meta,omitempty"`
}

// AuthMethod 是 Agent 声明的一种认证方式。
//
// codex 给出 api-key 与 chat-gpt 两种（实测）；claude 的 authMethods 是空数组。
// 「支持纯环境变量认证」是 CI / 批量场景优先选 codex 的理由。
type AuthMethod struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Meta        json.RawMessage `json:"_meta,omitempty"`
}

// InitializeRequest 是 initialize 的 params。
type InitializeRequest struct {
	ProtocolVersion    int             `json:"protocolVersion"`
	ClientCapabilities json.RawMessage `json:"clientCapabilities,omitempty"`
	ClientInfo         *Implementation `json:"clientInfo,omitempty"`
	Meta               json.RawMessage `json:"_meta,omitempty"`
}

// InitializeResponse 是 initialize 的响应。
//
// AgentCapabilities 保持 json.RawMessage：能力声明是**可扩展**的，
// 建强类型映射意味着每次 Runtime 升级都要跟着改一遍，
// 而漏改的字段会静默丢失。能力判断一律走 capability 包的探针（S0.7），
// 不在这里解析（acp-integration.md §8）。
type InitializeResponse struct {
	ProtocolVersion   int             `json:"protocolVersion"`
	AgentCapabilities json.RawMessage `json:"agentCapabilities,omitempty"`
	AuthMethods       []AuthMethod    `json:"authMethods,omitempty"`
	AgentInfo         *Implementation `json:"agentInfo,omitempty"`
	Meta              json.RawMessage `json:"_meta,omitempty"`
}
