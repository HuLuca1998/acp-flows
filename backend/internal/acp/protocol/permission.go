package protocol

import (
	"encoding/json"
	"fmt"
)

// PermissionOptionKind 是权限选项的类别。
//
// ★ 真实取值是 allow_once / allow_always / reject_once / reject_always，
// **没有裸 allow / reject**。写错的话客户端选的选项 Agent 认不出来。
type PermissionOptionKind string

// v1 schema 的 PermissionOptionKind 全集，顺序照官方声明顺序。
const (
	PermissionAllowOnce    PermissionOptionKind = "allow_once"
	PermissionAllowAlways  PermissionOptionKind = "allow_always"
	PermissionRejectOnce   PermissionOptionKind = "reject_once"
	PermissionRejectAlways PermissionOptionKind = "reject_always"
)

var allPermissionOptionKinds = [...]PermissionOptionKind{
	PermissionAllowOnce, PermissionAllowAlways,
	PermissionRejectOnce, PermissionRejectAlways,
}

// AllPermissionOptionKinds 返回全集，按官方声明顺序。
func AllPermissionOptionKinds() []PermissionOptionKind {
	out := make([]PermissionOptionKind, len(allPermissionOptionKinds))
	copy(out, allPermissionOptionKinds[:])
	return out
}

// IsValid 报告取值是否在全集内。
func (k PermissionOptionKind) IsValid() bool {
	for _, v := range allPermissionOptionKinds {
		if v == k {
			return true
		}
	}
	return false
}

// PermissionOption 是 Agent 给出的一个可选应答。
//
// OptionID 由 Agent 定义，客户端**原样回传**，不要自己造。
type PermissionOption struct {
	OptionID string               `json:"optionId"`
	Name     string               `json:"name"`
	Kind     PermissionOptionKind `json:"kind"`
	Meta     json.RawMessage      `json:"_meta,omitempty"`
}

// RequestPermissionRequest 是 session/request_permission 的 params。
//
// 这是一个**反向请求**：Agent 调我们。它会阻塞当前轮直到我们应答。
type RequestPermissionRequest struct {
	SessionID string             `json:"sessionId"`
	ToolCall  ToolCallUpdate     `json:"toolCall"`
	Options   []PermissionOption `json:"options"`
	Meta      json.RawMessage    `json:"_meta,omitempty"`
}

// RequestPermissionOutcome 是权限请求的应答结果，两种：cancelled / selected。
type RequestPermissionOutcome struct {
	outcome  string
	optionID string
}

// CancelledOutcome 构造「已取消」应答。
//
// ★ 取消时必须用它应答**所有 pending 的权限请求**（acp-integration.md §6.3 义务 2）。
// 漏了会导致每次取消都超时、M1 的 update/prepare 永远返回 blocked。
// 规范硬要求，设计稿完全没提。
func CancelledOutcome() RequestPermissionOutcome {
	return RequestPermissionOutcome{outcome: "cancelled"}
}

// SelectedOutcome 构造「选了某个选项」应答。optionID 必须是 Agent 给过的值。
func SelectedOutcome(optionID string) RequestPermissionOutcome {
	return RequestPermissionOutcome{outcome: "selected", optionID: optionID}
}

// IsCancelled 报告这是不是一个「已取消」应答。
func (o RequestPermissionOutcome) IsCancelled() bool { return o.outcome == "cancelled" }

// SelectedOptionID 返回选中的 optionId；不是 selected 应答时返回空串。
func (o RequestPermissionOutcome) SelectedOptionID() string {
	if o.outcome != "selected" {
		return ""
	}
	return o.optionID
}

// UnmarshalJSON 解析判别式应答。
func (o *RequestPermissionOutcome) UnmarshalJSON(b []byte) error {
	var wire struct {
		Outcome  string `json:"outcome"`
		OptionID string `json:"optionId"`
	}
	if err := json.Unmarshal(b, &wire); err != nil {
		return fmt.Errorf("protocol: 解析权限应答失败: %w", err)
	}
	o.outcome = wire.Outcome
	o.optionID = wire.OptionID
	return nil
}

// MarshalJSON 按判别式写出。selected 才带 optionId。
func (o RequestPermissionOutcome) MarshalJSON() ([]byte, error) {
	if o.outcome == "selected" {
		return json.Marshal(struct {
			Outcome  string `json:"outcome"`
			OptionID string `json:"optionId"`
		}{Outcome: o.outcome, OptionID: o.optionID})
	}
	return json.Marshal(struct {
		Outcome string `json:"outcome"`
	}{Outcome: o.outcome})
}

// RequestPermissionResponse 是 session/request_permission 的响应。
type RequestPermissionResponse struct {
	Outcome RequestPermissionOutcome `json:"outcome"`
	Meta    json.RawMessage          `json:"_meta,omitempty"`
}
