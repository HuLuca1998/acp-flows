package session

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/acp/protocol"
)

// DefaultCancelTimeout 是等 Agent 收尾的上限。
//
// ★ 不能太短：Agent 收到取消后要把手上的工具调用收干净，几秒是正常的。
// 也不能没有：没有上限的话，一个卡死的 Agent 会让「取消」这个按钮
// 永远转圈，而用户以为是自己网不好。
const DefaultCancelTimeout = 10 * time.Second

// ErrCancelTimeout 表示发了取消但 Agent 没在上限内收尾。
//
// ★ 拿到它的调用方**必须杀进程**（见 MustKill）。当成普通错误重试的话，
// 界面显示「已取消」而后台那个 Agent 还在烧钱改文件。
var ErrCancelTimeout = errors.New("session: 取消超时")

// MustKill 报告这个错误是否要求调用方杀掉 Agent 进程。
//
// ★ 单独一个函数而不是让调用方 errors.Is：这件事太容易漏，
// 而漏了的后果是「界面说已取消、后台还在改文件」——
// 用户最不能原谅的一种。给它一个显眼的名字。
func MustKill(err error) bool {
	return errors.Is(err, ErrCancelTimeout)
}

// Cancel 取消当前这一轮，用默认上限。
func (s *Session) Cancel(ctx context.Context) error {
	return s.CancelWithin(ctx, DefaultCancelTimeout)
}

// CancelWithin 取消当前这一轮，等 Agent 收尾最多 within。
//
// 两段式：
//
//  1. 先用 `cancelled` 应答**所有** pending 的权限请求。
//     ★ ACP 的硬要求，而设计稿完全没提。漏了的话 Agent 一直等着我们回答，
//     这一轮永远不会结束——每次取消都超时，而超时直接连着 M1 的一键更新：
//     `update/prepare` 会永远返回 blocked，用户点「更新」永远点不动。
//  2. 再发 `session/cancel` 通知，等这一轮真的收尾。
//
// ★ `session/cancel` 是**通知不是请求**：发出去就完事，不等响应。
// 等响应的话会一直挂着——Agent 根本不会回。真正的收尾信号是
// `session/prompt` 那个请求返回 `stopReason: cancelled`。
//
// ★ **幂等**：连续调用只发一条协议取消。用户手快点两下是常态，
// 而 Agent 收到第二条时那一轮多半已经结束，它会当成一个不认识的会话。
func (s *Session) CancelWithin(ctx context.Context, within time.Duration) error {
	s.mu.Lock()
	// 没在跑：什么都不做。报错的话，用户点一个本来就该没反应的按钮
	// 会看到一句吓人的错误。
	if !s.inTurn {
		s.mu.Unlock()
		return nil
	}
	// 已经取消过：直接返回，不再发第二条
	if s.cancelling {
		done := s.cancelDone
		s.mu.Unlock()
		return waitCancel(ctx, done, within, s.id)
	}
	s.cancelling = true
	s.cancelDone = make(chan struct{})
	done := s.cancelDone
	s.mu.Unlock()

	// ① 先放掉所有 pending 的权限请求，否则 Agent 一直等我们回答
	s.releasePendingPermissions()

	// ② 再发取消通知。**不等响应**——它是通知。
	if err := s.conn.Notify(ctx, protocol.MethodSessionCancel, protocol.CancelNotification{
		SessionID: s.id,
	}); err != nil {
		return fmt.Errorf("session/cancel: %w", err)
	}

	return waitCancel(ctx, done, within, s.id)
}

// waitCancel 等这一轮真的收尾。
func waitCancel(ctx context.Context, done <-chan struct{}, within time.Duration, id string) error {
	timer := time.NewTimer(within)
	defer timer.Stop()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		// ★ 错误里带上**是谁、等了多久**：只说「timeout」的话，
		// 用户提了工单我们也查不出是哪条会话、卡了多久。
		return fmt.Errorf("%w: 会话 %s 等了 %s 仍未收尾", ErrCancelTimeout, id, within)
	}
}

// releasePendingPermissions 用 cancelled 应答所有还在等的权限请求。
func (s *Session) releasePendingPermissions() {
	s.mu.Lock()
	release := s.releasePermissions
	s.mu.Unlock()
	if release != nil {
		release()
	}
}

// finishCancel 在这一轮真的收尾时叫醒等着的 Cancel。
func (s *Session) finishCancel() {
	s.mu.Lock()
	done := s.cancelDone
	s.cancelDone = nil
	s.cancelling = false
	s.mu.Unlock()

	if done != nil {
		close(done)
	}
}
