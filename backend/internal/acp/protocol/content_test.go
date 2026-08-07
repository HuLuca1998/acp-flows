package protocol_test

// M0 U0.2.3 · protocol 线格式包 —— 内容块
//
// 这里的构造函数是 Fake Runtime 的 builder DSL（acp-integration.md §12.4）直接要用的：
//   .Say("msg_1", "…")                    → TextBlock
//   .ToolDone("call_001", fake.Diff(…))   → DiffContent
// 先在本包定型并测住，U0.4.1 就不必回头改这个包。

import (
	"encoding/json"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/acp/protocol"
)

// 构造出来的内容块必须与官方 shape 逐字一致。
//
// ★ 这条挡的是「字段名写成 snake_case」这类 bug：oldText 写成 old_text
// 编译器不会报错，agent 那边只会安静地少显示一个 diff。
func TestTextBlock_MatchesOfficialShape(t *testing.T) {
	block := protocol.TextBlock("先读 acp-integration.md §12 的脚本格式。")

	out, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	const want = `{"type":"text","text":"先读 acp-integration.md §12 的脚本格式。"}`
	if string(out) != want {
		t.Errorf("want %s\n got %s", want, out)
	}

	if block.Type() != "text" {
		t.Errorf("Type(): want %q, got %q", "text", block.Type())
	}
	text, ok := block.Text()
	if !ok {
		t.Fatal("text 块的 Text() 必须能取到值")
	}
	if text != "先读 acp-integration.md §12 的脚本格式。" {
		t.Errorf("Text(): got %q", text)
	}
}

func TestDiffContent_MatchesOfficialShape(t *testing.T) {
	diff := protocol.DiffContent(
		"/w/backend/internal/acp/fake/script.go",
		"",
		"type Script struct{}\n",
	)

	out, err := json.Marshal(diff)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	const want = `{"type":"diff","path":"/w/backend/internal/acp/fake/script.go","oldText":"","newText":"type Script struct{}\n"}`
	if string(out) != want {
		t.Errorf("want %s\n got %s", want, out)
	}
	if diff.Type() != "diff" {
		t.Errorf("Type(): want %q, got %q", "diff", diff.Type())
	}
}

// 没有强类型访问器的内容块必须原样转发，不能被吃掉。
//
// M0 只用到 text 与 diff，但 agent 随时可能发 image / resource_link。
// 转发无损这条契约成立，上层才敢直接把载荷交给前端渲染。
func TestContentBlock_UnhandledTypesSurviveRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{
			name: "image",
			raw:  `{"type":"image","data":"iVBORw0KGgo=","mimeType":"image/png"}`,
		},
		{
			name: "resource_link",
			raw:  `{"type":"resource_link","name":"cancel.go","uri":"file:///w/backend/internal/acp/session/cancel.go","mimeType":"text/x-go"}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var block protocol.ContentBlock
			if err := json.Unmarshal([]byte(tc.raw), &block); err != nil {
				t.Fatalf("解析失败: %v", err)
			}
			if block.Type() != tc.name {
				t.Errorf("Type(): want %q, got %q", tc.name, block.Type())
			}
			// 非 text 块取 Text() 必须明确说「没有」，而不是返回空串让调用方以为是空消息。
			if _, ok := block.Text(); ok {
				t.Errorf("%s 块不该有 Text()", tc.name)
			}

			out, err := json.Marshal(block)
			if err != nil {
				t.Fatalf("序列化失败: %v", err)
			}
			var want, got any
			if err := json.Unmarshal([]byte(tc.raw), &want); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(out, &got); err != nil {
				t.Fatal(err)
			}
			if !jsonEqual(want, got) {
				t.Errorf("转发有损\n want=%s\n  got=%s", tc.raw, out)
			}
		})
	}
}

// 零值内容块不能被静静写成 "null"。
//
// Fake 的脚本里漏填一个 content 时，我们要在测试里立刻看到错误，
// 而不是让对端收到 null 再报一个语焉不详的解析失败。
func TestContentBlocks_ZeroValueRefusesToMarshal(t *testing.T) {
	t.Run("ContentBlock", func(t *testing.T) {
		var block protocol.ContentBlock
		if _, err := json.Marshal(block); err == nil {
			t.Error("未初始化的 ContentBlock 必须拒绝序列化")
		}
	})
	t.Run("ToolCallContent", func(t *testing.T) {
		var content protocol.ToolCallContent
		if _, err := json.Marshal(content); err == nil {
			t.Error("未初始化的 ToolCallContent 必须拒绝序列化")
		}
	})
}

func jsonEqual(a, b any) bool {
	x, err1 := json.Marshal(a)
	y, err2 := json.Marshal(b)
	return err1 == nil && err2 == nil && string(x) == string(y)
}
