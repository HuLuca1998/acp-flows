package protocol

import (
	"encoding/json"
	"fmt"
)

// ContentBlock 是 ACP 的内容块，五种类型：
// text / image / audio / resource_link / resource。
//
// 本层对 text 提供强类型访问（M0 只用到它），其余**原样保留**。
// 转发无损这条契约成立，上层才敢把载荷直接交给前端渲染器；
// 需要新类型时加访问器即可，已有调用方不受影响。
type ContentBlock struct {
	typ  string
	raw  json.RawMessage
	text string
}

// TextBlock 构造一个 text 内容块。
func TextBlock(text string) ContentBlock {
	// 这里手工拼 JSON 会踩转义的坑（正文里常有引号和换行），一律走 Marshal。
	raw, err := json.Marshal(struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}{Type: "text", Text: text})
	if err != nil {
		// 只有 string 字段，Marshal 不可能失败；真失败了说明标准库出了问题。
		panic(fmt.Sprintf("protocol: 构造 text 内容块失败: %v", err))
	}
	return ContentBlock{typ: "text", raw: raw, text: text}
}

// Type 返回内容块类型判别值。
func (c ContentBlock) Type() string { return c.typ }

// Text 返回文本内容。第二个返回值为 false 表示这不是 text 块 ——
// 非 text 块返回空串会让调用方误以为收到了一条空消息。
func (c ContentBlock) Text() (string, bool) {
	if c.typ != "text" {
		return "", false
	}
	return c.text, true
}

// UnmarshalJSON 读出类型判别值，载荷原样留存。
func (c *ContentBlock) UnmarshalJSON(b []byte) error {
	var probe struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(b, &probe); err != nil {
		return fmt.Errorf("protocol: 解析内容块失败: %w", err)
	}
	c.typ = probe.Type
	c.text = probe.Text
	c.raw = append(json.RawMessage(nil), b...)
	return nil
}

// MarshalJSON 原样写回载荷。
func (c ContentBlock) MarshalJSON() ([]byte, error) {
	if len(c.raw) == 0 {
		return nil, fmt.Errorf("protocol: ContentBlock 未初始化（type=%q）", c.typ)
	}
	return c.raw, nil
}

// ToolCallContent 是工具调用的产出，三种类型：content / diff / terminal。
//
// 与 ContentBlock 同样的处理：只对本阶段用得到的 diff 提供构造函数，其余无损转发。
type ToolCallContent struct {
	typ string
	raw json.RawMessage
}

// DiffContent 构造一个 diff 产出块。
//
// oldText 为空串表示新建文件 —— 这与「没有 oldText 字段」语义不同，
// 所以它按值写出，不用 omitempty。
func DiffContent(path, oldText, newText string) ToolCallContent {
	raw, err := json.Marshal(struct {
		Type    string `json:"type"`
		Path    string `json:"path"`
		OldText string `json:"oldText"`
		NewText string `json:"newText"`
	}{Type: "diff", Path: path, OldText: oldText, NewText: newText})
	if err != nil {
		panic(fmt.Sprintf("protocol: 构造 diff 产出块失败: %v", err))
	}
	return ToolCallContent{typ: "diff", raw: raw}
}

// Type 返回产出块类型判别值。
func (c ToolCallContent) Type() string { return c.typ }

// UnmarshalJSON 读出类型判别值，载荷原样留存。
func (c *ToolCallContent) UnmarshalJSON(b []byte) error {
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(b, &probe); err != nil {
		return fmt.Errorf("protocol: 解析工具产出块失败: %w", err)
	}
	c.typ = probe.Type
	c.raw = append(json.RawMessage(nil), b...)
	return nil
}

// MarshalJSON 原样写回载荷。
func (c ToolCallContent) MarshalJSON() ([]byte, error) {
	if len(c.raw) == 0 {
		return nil, fmt.Errorf("protocol: ToolCallContent 未初始化（type=%q）", c.typ)
	}
	return c.raw, nil
}
