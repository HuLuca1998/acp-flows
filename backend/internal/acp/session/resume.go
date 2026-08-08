package session

import (
	"context"
	"fmt"

	"github.com/HuLuca1998/acp-flows/backend/internal/acp/protocol"
)

// ResumeOptions 是恢复一条会话需要的东西。
type ResumeOptions struct {
	Options
	// SessionID 是上次那条会话的标识。
	//
	// 留空表示「没有可恢复的」——直接开新的，不发一个注定失败的 load。
	SessionID string
}

// Resume 恢复一条已有会话；恢复不了就**显式**降级为新会话。
//
// ★ 「关掉再打开，它还记得刚才做到哪」全靠这一条。而它最危险的失败方式
// 不是报错，是**假装成功**：用户以为 AI 记得之前的事，实际它一无所知——
// 他接着上文提问，AI 答非所问，双方都不知道发生了什么。
//
// 所以降级路径一定是**看得见的**：`IsFresh()` 为真、`ResumeError()` 有值，
// 上层据此告诉用户「之前的上下文没了」。
//
// ★ 不是所有 Agent 都支持 `session/load`。不支持的回 -32601，
// 那不是错误而是**能力差异**——照样要给用户一条能用的会话。
func Resume(ctx context.Context, opts ResumeOptions) (*Session, error) {
	if opts.SessionID == "" {
		// 没有可恢复的：直接开新的，不发一个注定失败的 load
		s, err := Open(ctx, opts.Options)
		if err != nil {
			return nil, err
		}
		s.fresh = true
		return s, nil
	}

	s, err := dial(ctx, opts.Options)
	if err != nil {
		return nil, err
	}

	if err := s.initialize(ctx, opts.Options); err != nil {
		_ = s.Close()
		return nil, err
	}

	// 试着接上那条旧会话
	var resp protocol.LoadSessionResponse
	loadErr := s.conn.CallInto(ctx, protocol.MethodSessionLoad, protocol.LoadSessionRequest{
		SessionID: opts.SessionID,
		Cwd:       opts.Cwd,
		// 与 session/new 同理：nil slice 会写成 null 而覆盖掉 Agent 的配置
		MCPServers: []protocol.MCPServer{},
	}, &resp)
	if loadErr == nil {
		s.id = opts.SessionID
		return s, nil
	}

	// ★ 接不上：**显式**降级。这条连接已经握过手了，直接在上面开新会话，
	// 不用重连——重连的话，那个 Agent 进程要重拉一次，几秒钟就没了。
	if err := s.newSession(ctx, opts.Options); err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("恢复会话 %s 失败后开新会话也失败: %w", opts.SessionID, err)
	}
	s.fresh = true
	// 原因里带上是哪一条：排查时得看得出「哪条没恢复上」
	s.resumeErr = fmt.Errorf("恢复会话 %s 失败，已降级为新会话: %w", opts.SessionID, loadErr)
	return s, nil
}

// IsFresh 报告这是不是一条**全新**会话（而不是恢复出来的）。
//
// ★ 上层必须看它：为真时要告诉用户「之前的上下文没了」。
// 不看的话，用户接着上文提问而 AI 一无所知。
func (s *Session) IsFresh() bool { return s.fresh }

// ResumeError 返回降级的原因；没降级时为 nil。
func (s *Session) ResumeError() error { return s.resumeErr }
