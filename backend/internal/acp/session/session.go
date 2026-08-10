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
	"time"

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
	// Kind 是判别值，13 类之一（或将来新增的）；权限裁决用 KindPermissionDecided。
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
	// Decision 只在 Kind == KindPermissionDecided 时有值。
	Decision Decision
}

// KindPermissionDecided 是我们自己发的事件：一次权限裁决的结果。
//
// ★ 不是 ACP 的判别值，是我们加的——所以带 `duet.` 前缀，
// 永远不会和 Agent 将来新增的类型撞名。
const KindPermissionDecided protocol.SessionUpdateKind = "duet.permission_decided"

// Options 是开一条会话需要的东西。
type Options struct {
	// Transport 是与 Agent 的双工通道（进程的 stdin/stdout，或测试里的管道）。
	Transport io.ReadWriteCloser
	// Cwd 是 Agent 的工作目录，必须是**已存在的绝对路径**。
	Cwd string
	// ClientName / ClientVersion 报给 Agent，留空时用默认值。
	ClientName    string
	ClientVersion string
	// Permission 决定收到权限请求时怎么办。零值 = 每次都问、且没人接线
	// （于是一律 cancelled）——最保守的那条。
	Permission Permission

	// RequiredModeID 是这条会话必须运行在的档位，**已经翻译成这一端的档名**
	// （claude 的 `plan` / codex 的 `read-only`）。翻译在
	// `acp/runtime.ModeNameOn`，session 层只管协议。
	//
	// ★★ 非空时**收不了权就开不了会话**。留空表示不限制——
	// 只有明确不需要限制的场景才留空，别拿它当「先跑起来再说」的后门：
	// 实测证明收权失败**没有任何症状**（codex 默认档是沙箱，
	// 沙箱内的写不触发审批，权限请求 0 次、文件照建）。
	RequiredModeID string
}

// Session 是一条已经建好的 ACP 会话。
type Session struct {
	conn       *jsonrpc.Conn
	transport  io.ReadWriteCloser
	id         string
	permission Permission

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
	// fresh 表示这是一条全新会话（不是恢复出来的）。
	// resumeErr 记着降级的原因。
	//
	// ★ 两者一起构成「恢复失败」的**显式信号**。假装恢复成功的话，
	// 用户以为 AI 记得之前的事，实际它一无所知——他接着上文提问，
	// AI 答非所问，双方都不知道发生了什么。
	fresh     bool
	resumeErr error

	// inTurn 表示正有一轮在跑。
	//
	// ★ 不能拿 onEvent 是否为 nil 当判据：调用方完全可以不关心事件而传 nil
	// （比如只想让它跑完），那时 Cancel 会以为「没在跑」而什么都不做——
	// 用户点了停，界面毫无反应，而 AI 照跑。
	inTurn bool

	// ★ 取消的状态。**幂等靠它**：cancelling 为真时不再发第二条协议取消。
	// 用户手快点两下是常态，而 Agent 收到第二条时那一轮多半已经结束，
	// 它会当成一个不认识的会话，行为没有定义。
	cancelling bool
	cancelDone chan struct{}
	// releasePermissions 用 cancelled 应答所有 pending 的权限请求。
	// 由 handlePermission 串上、这一轮结束时摘掉。
	releasePermissions func()

	serveDone chan struct{}
	serveErr  error
	closeOnce sync.Once
}

// Open 建立会话：initialize → session/new。
//
// ★ **先校验 cwd 再发任何请求。** 反过来的话，Agent 那边已经开了一个会话
// 而我们这边报了错——它会挂在那儿占着资源，没人再去关它。
func Open(ctx context.Context, opts Options) (*Session, error) {
	s, err := dial(ctx, opts)
	if err != nil {
		return nil, err
	}
	if err := s.initialize(ctx, opts); err != nil {
		_ = s.Close()
		return nil, err
	}
	if err := s.newSession(ctx, opts); err != nil {
		_ = s.Close()
		return nil, err
	}
	return s, nil
}

// dial 校验 cwd、建连接、起读循环。**还没说过一句话。**
//
// ★ 拆出来是为了让 Resume 复用：它要在同一条连接上先试 session/load，
// 失败再退回 session/new。重新连一次的话，那个 Agent 进程要重拉，
// 几秒钟就没了。
//
// ★ **先校验 cwd 再建连接。** 反过来的话，Agent 那边已经起来了
// 而我们这边报了错——它会挂在那儿占着资源，没人再去关它。
func dial(ctx context.Context, opts Options) (*Session, error) {
	if !filepath.IsAbs(opts.Cwd) {
		return nil, fmt.Errorf("%w: %q", ErrCwdNotAbsolute, opts.Cwd)
	}
	st, err := os.Stat(opts.Cwd)
	if err != nil || !st.IsDir() {
		return nil, fmt.Errorf("%w: %s", ErrCwdNotFound, opts.Cwd)
	}

	s := &Session{
		transport:  opts.Transport,
		permission: opts.Permission,
		serveDone:  make(chan struct{}),
	}
	s.conn = jsonrpc.New(opts.Transport, opts.Transport, jsonrpc.HandlerFunc(s.handle))

	go func() {
		defer close(s.serveDone)
		s.serveErr = s.conn.Serve(ctx)
	}()
	return s, nil
}

// initialize 走协议握手。
func (s *Session) initialize(ctx context.Context, opts Options) error {
	name, version := opts.ClientName, opts.ClientVersion
	if name == "" {
		name, version = "duet", "0.0.0"
	}
	var resp protocol.InitializeResponse
	if err := s.conn.CallInto(ctx, protocol.MethodInitialize, protocol.InitializeRequest{
		ProtocolVersion: acpProtocolVersion,
		ClientInfo:      &protocol.Implementation{Name: name, Version: version},
	}, &resp); err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	return nil
}

// newSession 开一条全新会话。
func (s *Session) newSession(ctx context.Context, opts Options) error {
	var resp protocol.NewSessionResponse
	if err := s.conn.CallInto(ctx, protocol.MethodSessionNew, protocol.NewSessionRequest{
		Cwd: opts.Cwd,
		// ★ 必须是**空数组**，不能是 nil。
		//
		// nil slice 会写成 `null`；而 **codex-acp**（不是 claude——这条归因
		// 之前写错了）在 session 级 mcpServers 非空时，会用它**整体覆盖**
		// thread config 的 `mcp_servers` 键，`CODEX_CONFIG` 里那些
		// `enabled: false` 的禁用条目全部丢失（acp-field-notes.md §4 硬规则 2）。
		//
		// 后果是：机器级的 MCP Server 会在这条会话里静默复活，
		// 而 MCP 是**任意进程执行**——用户以为自己只是打开了一个项目。
		MCPServers: []protocol.MCPServer{},
	}, &resp); err != nil {
		return fmt.Errorf("session/new: %w", err)
	}
	s.id = resp.SessionID

	// ★★ **收权在这里，而且失败就整个开会话失败。**
	//
	// 顺序是刻意的：先拿到 sessionID（set_config_option 要带它），
	// 再立刻收权，**在任何 prompt 之前**。中间不留窗口——
	// codex 的默认档是 workspace-write 沙箱，那个窗口里的写操作
	// 连审批都不会触发。
	if opts.RequiredModeID != "" {
		if err := s.applyMode(ctx, resp, opts.RequiredModeID); err != nil {
			return fmt.Errorf("收权到 %q: %w", opts.RequiredModeID, err)
		}
	}
	return nil
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
	s.inTurn = true
	s.mu.Unlock()

	var resp protocol.PromptResponse
	callErr := s.conn.CallInto(ctx, protocol.MethodSessionPrompt, protocol.PromptRequest{
		SessionID: s.id,
		Prompt:    []protocol.ContentBlock{protocol.TextBlock(text)},
	}, &resp)

	// 摘掉回调：轮次之间 Agent 若多发一条通知，不该再打到上一轮的调用方身上。
	s.mu.Lock()
	s.onEvent = nil
	s.inTurn = false
	s.releasePermissions = nil
	s.mu.Unlock()

	// 这一轮收尾了：叫醒等着的 Cancel。放在摘回调之后——
	// Cancel 的调用方拿到返回值时，事件已经不会再来了。
	s.finishCancel()

	if callErr != nil {
		return resp.StopReason, fmt.Errorf("session/prompt: %w", callErr)
	}
	if resp.StopReason != protocol.StopReasonEndTurn {
		// 把真实取值一起返回：上层要区分「被截断」（可以续）与「被拒绝」（不能续）
		return resp.StopReason, fmt.Errorf("%w: %s", ErrTurnIncomplete, resp.StopReason)
	}
	return resp.StopReason, nil
}

// closeGrace 是等读循环收尾的上限。
//
// ★ 不能无限等。Agent 一轮结束后**并不退出**（它等着下一条需求），
// 而读循环要等对方关掉 stdout——两边互相等，表现是应用退不出去、
// Agent 进程越攒越多。关传输通常就能把读循环叫醒，这个上限兜住关不动的情况。
const closeGrace = 2 * time.Second

// Close 关掉会话与底层通道。可重复调用。
//
// ★ **有界等待**：见 closeGrace。超时不算错误——传输已经关了，
// 真正的收尾（杀进程组）归调用方的 runtime.Process.Stop 管。
func (s *Session) Close() error {
	var err error
	s.closeOnce.Do(func() {
		err = s.transport.Close()

		timer := time.NewTimer(closeGrace)
		defer timer.Stop()
		select {
		case <-s.serveDone:
		case <-timer.C:
		}
	})
	return err
}

// handle 处理 Agent 发来的请求与通知。
func (s *Session) handle(ctx context.Context, method string, params json.RawMessage) (any, error) {
	switch method {
	case protocol.MethodRequestPermission:
		return s.handlePermission(ctx, params)
	case protocol.MethodSessionUpdate:
		// 往下走
	default:
		// 明确回不支持，而不是静默丢弃——静默的话 Agent 会一直等
		// 一个永远不来的响应。
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
