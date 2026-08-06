// Package jsonrpc 实现 ACP 用的 JSON-RPC 2.0 over stdio。
//
// ★ 分帧是 **ndjson（换行分隔）**，每行一条完整 JSON，行内不含未转义的换行。
// **不是** LSP 的 Content-Length 分帧——搞错了对方一条都解析不出来。
//
// 连接是**双向**的：agent 会反过来调我们（session/request_permission、
// fs/read_text_file 等），所以必须注册 Handler。不实现的话 agent 吃到 -32601，
// 整轮可能失败。
//
// 本包不认识任何 ACP 语义（session/* 这些方法名不出现在这里），只管收发。
package jsonrpc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
)

// JSON-RPC 2.0 标准错误码，以及 ACP 用到的扩展码。
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
	// CodeAuthRequired 是 ACP 的扩展：需要先 authenticate。
	CodeAuthRequired = -32000
	// CodeRequestCancelled 是 ACP 的扩展：请求被取消。
	CodeRequestCancelled = -32800
)

// Error 是远端返回的 JSON-RPC 错误。
type Error struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *Error) Error() string { return fmt.Sprintf("jsonrpc %d: %s", e.Code, e.Message) }

// Handler 处理对方发来的请求。
//
// 返回的 any 会被序列化成 result；返回 error 时会被翻译成 error 响应。
type Handler interface {
	Handle(ctx context.Context, method string, params json.RawMessage) (any, error)
}

// HandlerFunc 让普通函数满足 Handler。
type HandlerFunc func(ctx context.Context, method string, params json.RawMessage) (any, error)

func (f HandlerFunc) Handle(ctx context.Context, m string, p json.RawMessage) (any, error) {
	return f(ctx, m, p)
}

// message 是线上传输的信封。请求/响应/通知共用一个结构，靠字段存在与否区分。
type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *uint64         `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// Conn 是一条双向 JSON-RPC 连接。
type Conn struct {
	r       io.Reader
	w       io.Writer
	handler Handler
	log     *slog.Logger

	writeMu sync.Mutex // 串行化写，保证一条消息不会被另一条插进来

	nextID atomic.Uint64

	pendingMu sync.Mutex
	pending   map[uint64]chan message
}

// New 建立一条连接。handler 可以是 nil，那样收到的请求一律回 -32601。
func New(r io.Reader, w io.Writer, handler Handler) *Conn {
	return &Conn{
		r:       r,
		w:       w,
		handler: handler,
		log:     slog.Default(),
		pending: make(map[uint64]chan message),
	}
}

// Serve 读循环。它会一直跑到 ctx 结束或读端关闭。
func (c *Conn) Serve(ctx context.Context) error {
	sc := bufio.NewScanner(c.r)
	// ACP 的单条消息可能很大（tool_call 里带 diff），默认 64KB 不够。
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)

	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}

		var m message
		if err := json.Unmarshal(line, &m); err != nil {
			// R6：非法 JSON 不致命。记录后继续——一行坏帧不该让整条连接死掉。
			c.log.Warn("收到非法 JSON-RPC 帧，已跳过", "err", err, "bytes", len(line))
			continue
		}

		switch {
		case m.Method != "" && m.ID != nil:
			go c.serveRequest(ctx, m) // 反向请求：不阻塞读循环
		case m.Method != "":
			go c.serveNotification(ctx, m)
		case m.ID != nil:
			c.deliver(m)
		default:
			c.log.Warn("收到无法识别的 JSON-RPC 帧")
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	if err := sc.Err(); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("jsonrpc read: %w", err)
	}
	return nil
}

// Call 发一个请求并等响应。
func (c *Conn) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	var raw json.RawMessage
	err := c.CallInto(ctx, method, params, &raw)
	return raw, err
}

// CallInto 发一个请求，并把 result 解到 out 里。out 为 nil 时忽略 result。
func (c *Conn) CallInto(ctx context.Context, method string, params any, out any) error {
	id := c.nextID.Add(1)
	ch := make(chan message, 1)

	c.pendingMu.Lock()
	c.pending[id] = ch
	c.pendingMu.Unlock()
	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, id) // 无论成功失败都要摘掉，否则 map 无限增长
		c.pendingMu.Unlock()
	}()

	if err := c.send(message{JSONRPC: "2.0", ID: &id, Method: method, Params: mustRaw(params)}); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		// R5：超时/取消时立刻返回，pending 由 defer 摘掉，不泄漏
		return fmt.Errorf("jsonrpc call %s: %w", method, ctx.Err())
	case resp := <-ch:
		if resp.Error != nil {
			return fmt.Errorf("jsonrpc call %s: %w", method, resp.Error)
		}
		if out == nil || len(resp.Result) == 0 {
			return nil
		}
		if err := json.Unmarshal(resp.Result, out); err != nil {
			return fmt.Errorf("jsonrpc call %s: decode result: %w", method, err)
		}
		return nil
	}
}

// Notify 发一个通知，不等响应。
//
// ACP 的 session/cancel 就是通知——取消完成的同步点是 session/prompt 的响应，
// 不是这个调用返回。见 docs/acp-integration.md。
func (c *Conn) Notify(_ context.Context, method string, params any) error {
	return c.send(message{JSONRPC: "2.0", Method: method, Params: mustRaw(params)})
}

func (c *Conn) serveRequest(ctx context.Context, m message) {
	resp := message{JSONRPC: "2.0", ID: m.ID}

	if c.handler == nil {
		resp.Error = &Error{Code: CodeMethodNotFound, Message: "method not found: " + m.Method}
	} else if result, err := c.handler.Handle(ctx, m.Method, m.Params); err != nil {
		var rpcErr *Error
		if errors.As(err, &rpcErr) {
			resp.Error = rpcErr
		} else {
			resp.Error = &Error{Code: CodeInternalError, Message: err.Error()}
		}
	} else {
		resp.Result = mustRaw(result)
	}

	if err := c.send(resp); err != nil {
		c.log.Error("回复反向请求失败", "method", m.Method, "err", err)
	}
}

func (c *Conn) serveNotification(ctx context.Context, m message) {
	if c.handler == nil {
		return // 通知没人要就丢掉，这是允许的
	}
	if _, err := c.handler.Handle(ctx, m.Method, m.Params); err != nil {
		c.log.Warn("处理通知出错", "method", m.Method, "err", err)
	}
}

// deliver 把响应交给等待的调用方。
func (c *Conn) deliver(m message) {
	c.pendingMu.Lock()
	ch, ok := c.pending[*m.ID]
	c.pendingMu.Unlock()
	if !ok {
		// 调用方已经超时走了，或者对方回了个我们没发过的 id
		c.log.Warn("收到无人认领的响应", "id", *m.ID)
		return
	}
	ch <- m // 缓冲为 1，不会阻塞
}

// send 写一帧。
//
// ★ 用 json.Marshal 而不是 Encoder：Encoder 会自动追加换行，
// 但更重要的是 Marshal 保证输出里的换行都被转义成 \n，不会破坏 ndjson 分帧。
func (c *Conn) send(m message) error {
	body, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("jsonrpc encode: %w", err)
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if _, err := c.w.Write(append(body, '\n')); err != nil {
		return fmt.Errorf("jsonrpc write: %w", err)
	}
	return nil
}

func mustRaw(v any) json.RawMessage {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		// 参数序列化失败是编程错误，不是运行时状况
		return json.RawMessage(fmt.Sprintf(`{"__marshal_error":%q}`, err.Error()))
	}
	return b
}
