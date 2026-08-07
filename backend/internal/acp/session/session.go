// Package session 是一条 ACP 会话：建会话、发需求、把 Agent 说的话流式交出去。
//
// 它只管**一轮对话怎么跑通**。取消是 M3、Agent 差异是 U2.2.3，都不在这里。
//
// ★ 对手方在测试里是 Fake ACP Runtime——它按官方规范独立说话（只许 import
// protocol），所以「我们理解错了协议」这件事测得出来。换成 mock 的话，
// mock 和被测代码会一起错、测试照样绿。
package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/HuLuca1998/acp-flows/backend/internal/acp/jsonrpc"
	"github.com/HuLuca1998/acp-flows/backend/internal/acp/protocol"
)

// acpProtocolVersion 是我们实现的 ACP 版本。
//
// 常量而非配置：协议版本变了要改的是代码，不是某个配置文件——
// 让它可配的话，用户改一个数字就能让握手"成功"而实际语义对不上。
const acpProtocolVersion = 1

// 会话相关的错误。
var (
	// ErrCwdNotAbsolute 表示工作目录不是绝对路径。
	ErrCwdNotAbsolute = errors.New("session: cwd must be an absolute path")
	// ErrCwdNotFound 表示工作目录不存在或不是目录。
	ErrCwdNotFound = errors.New("session: cwd does not exist")
	// ErrTurnIncomplete 表示这一轮不是正常收尾。
	//
	// ★ 只有 end_turn 算正常。max_tokens 意味着产出是半成品——当成功处理的话，
	// 用户会拿到一个截断的改动而界面显示「完成」。
	ErrTurnIncomplete = errors.New("session: turn did not end normally")
)

// Event 是交给上层的一条会话事件。
//
// ★ **保留原始载荷**：上层按 Kind 决定解成哪个变体，没见过的变体也能原样
// 记进日志排查，而不是只知道「有个东西没认出来」。
type Event struct {
	// Kind 是判别值，13 类之一（或将来新增的）。
	Kind protocol.SessionUpdateKind
	// Text 是三种 chunk 变体的文本内容；其余类型为空。
	Text string
	// Raw 是完整的原始载荷。
	Raw json.RawMessage
	// Unhandled 表示这个判别值不在已知的 13 类里。
	//
	// 不是丢弃而是**标出来交上去**：静默吞掉的话，Agent 新增一类事件时
	// 界面上表现为「AI 好像少说了点什么」，没有任何报错。
	Unhandled bool
}

// Options 是开一条会话需要的东西。
type Options struct {
	// Transport 是与 Agent 的双工通道（进程的 stdin/stdout，或测试里的管道）。
	Transport io.ReadWriteCloser
	// Cwd 是 Agent 的工作目录，必须是**已存在的绝对路径**。
	Cwd string
	// ClientName / ClientVersion 报给 Agent，留空时用默认值。
	ClientName    string
	ClientVersion string
}

// Session 是一条已经建好的 ACP 会话。
type Session struct {
	conn      *jsonrpc.Conn
	transport io.ReadWriteCloser
	id        string

	// onEvent 是当前这一轮的事件回调，由 Prompt 装上、返回前摘掉。
	//
	// ★ **同步调用，不过 channel。** 第一版用带缓冲的 channel + goroutine
	// 转发，结果丢事件：session/prompt 的响应回来后 Prompt 就关掉 channel，
	// 而最后几条通知可能还在 jsonrpc 的读循环里没处理完。
	// 同步投递让顺序天然正确——通知在响应之前到达，就一定在 Prompt 返回前交出去。
	//
	// 代价是 onEvent 跑在 jsonrpc 的读 goroutine 上：**里面不要做慢操作**，
	// 阻塞它等于阻塞整条会话。
	mu      sync.Mutex
	onEvent func(Event)

	serveDone chan struct{}
	serveErr  error
	closeOnce sync.Once
}

// Open 建立会话：initialize → session/new。
//
// ★ **先校验 cwd 再发任何请求。** 反过来的话，Agent 那边已经开了一个会话
// 而我们这边报了错——它会挂在那儿占着资源，没人再去关它。
func Open(ctx context.Context, opts Options) (*Session, error) {
	if !filepath.IsAbs(opts.Cwd) {
		return nil, fmt.Errorf("%w: %q", ErrCwdNotAbsolute, opts.Cwd)
	}
	st, err := os.Stat(opts.Cwd)
	if err != nil || !st.IsDir() {
		return nil, fmt.Errorf("%w: %s", ErrCwdNotFound, opts.Cwd)
	}

	s := &Session{transport: opts.Transport, serveDone: make(chan struct{})}
	s.conn = jsonrpc.New(opts.Transport, opts.Transport, jsonrpc.HandlerFunc(s.handle))

	go func() {
		defer close(s.serveDone)
		s.serveErr = s.conn.Serve(ctx)
	}()

	name, version := opts.ClientName, opts.ClientVersion
	if name == "" {
		name, version = "duet", "0.0.0"
	}
	var initResp protocol.InitializeResponse
	if err := s.conn.CallInto(ctx, protocol.MethodInitialize, protocol.InitializeRequest{
		ProtocolVersion: acpProtocolVersion,
		ClientInfo:      &protocol.Implementation{Name: name, Version: version},
	}, &initResp); err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("initialize: %w", err)
	}

	var newResp protocol.NewSessionResponse
	if err := s.conn.CallInto(ctx, protocol.MethodSessionNew, protocol.NewSessionRequest{
		Cwd: opts.Cwd,
		// ★ 不能是 nil：nil slice 会写成 null，而 claude 用 null 覆盖 thread config
		// 的 mcp_servers 键，禁用条目全部丢失（acp-field-notes.md §4）。
		MCPServers: []protocol.MCPServer{},
	}, &newResp); err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("session/new: %w", err)
	}

	s.id = newResp.SessionID
	return s, nil
}

// ID 返回会话标识。后续的取消、恢复都靠它。
func (s *Session) ID() string { return s.id }

// Prompt 发一条需求，把 Agent 说的话**边说边**交给 onEvent，返回这一轮的结束原因。
//
// ★ onEvent 在事件到达时立即调用，不攒批。攒完再吐的话，用户盯着一个不动的
// 界面等好几秒，而 AI 其实早就开口了——这是 V5 的用户可感部分，不是内部优化。
//
// 返回的 StopReason **总是**真实值（即便同时返回 error），调用方要据此
// 区分「被截断」和「被拒绝」。
func (s *Session) Prompt(
	ctx context.Context,
	text string,
	onEvent func(Event),
) (protocol.StopReason, error) {
	s.mu.Lock()
	s.onEvent = onEvent
	s.mu.Unlock()

	var resp protocol.PromptResponse
	callErr := s.conn.CallInto(ctx, protocol.MethodSessionPrompt, protocol.PromptRequest{
		SessionID: s.id,
		Prompt:    []protocol.ContentBlock{protocol.TextBlock(text)},
	}, &resp)

	// 摘掉回调：轮次之间 Agent 若多发一条通知，不该再打到上一轮的调用方身上。
	s.mu.Lock()
	s.onEvent = nil
	s.mu.Unlock()

	if callErr != nil {
		return resp.StopReason, fmt.Errorf("session/prompt: %w", callErr)
	}
	if resp.StopReason != protocol.StopReasonEndTurn {
		// 把真实取值一起返回：上层要区分「被截断」（可以续）与「被拒绝」（不能续）
		return resp.StopReason, fmt.Errorf("%w: %s", ErrTurnIncomplete, resp.StopReason)
	}
	return resp.StopReason, nil
}

// Close 关掉会话与底层通道。可重复调用。
func (s *Session) Close() error {
	var err error
	s.closeOnce.Do(func() {
		err = s.transport.Close()
		<-s.serveDone
	})
	return err
}

// handle 处理 Agent 发来的请求与通知。
func (s *Session) handle(_ context.Context, method string, params json.RawMessage) (any, error) {
	if method != protocol.MethodSessionUpdate {
		// 权限请求是 M3 的事。这里明确回不支持，而不是静默丢弃——
		// 静默的话 Agent 会一直等一个永远不来的响应。
		return nil, &jsonrpc.Error{Code: -32601, Message: "method not supported yet: " + method}
	}

	var notif protocol.SessionNotification
	if err := json.Unmarshal(params, &notif); err != nil {
		return nil, fmt.Errorf("解析 session/update: %w", err)
	}

	s.mu.Lock()
	cb := s.onEvent
	s.mu.Unlock()
	if cb == nil {
		// 不在任何一轮里收到更新：丢弃而不是报错——Agent 在轮次边界
		// 多发一条不该让整条会话失败。
		return nil, nil
	}

	cb(toEvent(notif.Update))
	return nil, nil
}

// toEvent 把协议层的更新翻成交给上层的事件。
func toEvent(u protocol.SessionUpdate) Event {
	e := Event{Kind: u.Kind(), Raw: u.Payload()}

	switch u.Kind() {
	// 三种文本块：内容是用户直接看到的东西，抽出来省得每个调用方各解一次
	case protocol.UpdateUserMessageChunk,
		protocol.UpdateAgentMessageChunk,
		protocol.UpdateAgentThoughtChunk:
		var chunk protocol.ContentChunkUpdate
		if err := json.Unmarshal(u.Payload(), &chunk); err == nil {
			// Text() 的第二个返回值区分「不是文本块」与「文本是空串」
			if t, ok := chunk.Content.Text(); ok {
				e.Text = t
			}
		}

	// 其余各类原样交上去：这一层不解释它们的语义，
	// 那是渲染层与领域层的事（U2.3.x / M3）。
	case protocol.UpdateToolCall,
		protocol.UpdateToolCallUpdate,
		protocol.UpdatePlan,
		protocol.UpdatePlanUpdate,
		protocol.UpdatePlanRemoved,
		protocol.UpdateAvailableCommandsUpdate,
		protocol.UpdateCurrentModeUpdate,
		protocol.UpdateConfigOptionUpdate,
		protocol.UpdateSessionInfoUpdate,
		protocol.UpdateUsageUpdate:

	default:
		// ★ 没见过的判别值**标出来交上去**，不静默吞掉。
		// 吞掉的话，Agent 新增一类事件时界面表现为「AI 好像少说了点什么」，
		// 而没有任何报错。
		e.Unhandled = true
	}
	return e
}
