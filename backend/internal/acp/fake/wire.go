package fake

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// wireFrame 是线上一帧的通用形态。
//
// ★ 本包**故意不复用** internal/acp/jsonrpc 的消息类型：
// Fake 是那个包的参照物，共用类型的话分帧或字段名写错时两边一起错，
// 测试照样绿（acp-integration.md §3.3 硬规则 2）。
type wireFrame struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// wireResponse 是一条响应。
type wireResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *wireError      `json:"error,omitempty"`
}

type wireError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// errStreamClosed 表示流已断，调用方应安静退出而不是继续写。
var errStreamClosed = fmt.Errorf("fake: 流已关闭")

// frameWriter 按 ndjson 写帧。
//
// ★ 分帧是**换行分隔**：每行一条完整 JSON，行内不含未转义的换行。
// 不是 LSP 的 Content-Length —— 搞错了对方一条都解析不出来。
//
// 写入必须串行化：脚本回放 goroutine 与响应可能同时写，
// 交错的话会产出两个半行拼起来的畸形帧。
type frameWriter struct {
	mu     sync.Mutex
	w      io.Writer
	closed bool
}

func (fw *frameWriter) writeFrame(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("fake: 序列化出站帧失败: %w", err)
	}

	fw.mu.Lock()
	defer fw.mu.Unlock()
	if fw.closed {
		return errStreamClosed
	}
	// 一次写完整行：分两次写的话，另一个 goroutine 可能插在中间。
	if _, err := fw.w.Write(append(b, '\n')); err != nil {
		return fmt.Errorf("fake: 写出站帧失败: %w", err)
	}
	return nil
}

// respond 回一条成功响应。
func (fw *frameWriter) respond(id json.RawMessage, result any) error {
	raw, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("fake: 序列化响应结果失败: %w", err)
	}
	return fw.writeFrame(wireResponse{JSONRPC: "2.0", ID: id, Result: raw})
}

// respondError 回一条错误响应。
func (fw *frameWriter) respondError(id json.RawMessage, code int, message string) error {
	return fw.writeFrame(wireResponse{
		JSONRPC: "2.0", ID: id,
		Error: &wireError{Code: code, Message: message},
	})
}

// notify 发一条通知（不带 id，不等响应）。
func (fw *frameWriter) notify(method string, params any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("fake: 序列化通知参数失败: %w", err)
	}
	return fw.writeFrame(wireFrame{JSONRPC: "2.0", Method: method, Params: raw})
}

// close 断流。后续写入一律返回 errStreamClosed，而不是 panic 或静默成功。
func (fw *frameWriter) close() {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	if fw.closed {
		return
	}
	fw.closed = true
	if c, ok := fw.w.(io.Closer); ok {
		_ = c.Close()
	}
}
