package work

import (
	"context"
	"errors"
	"fmt"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/internal/constant"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// ErrNoCanceller 表示没人能执行取消（装配漏了一根线）。
//
// ★ 明确报错而不是静静成功：静静成功的话界面显示「已停止」而 AI 照跑，
// 用户以为自己停住了，而账单继续涨。
var ErrNoCanceller = errors.New("work: 没有配置取消能力")

// SetCanceller 装上取消能力。
//
// ★ 单独一个 setter 而不是塞进 New：AgentCanceller 的实现需要拿到
// 「哪个工作对应哪个 Agent 进程」，而那份映射是在 Service 建好之后
// 才随着第一个工作产生的。塞进构造函数会逼出一个先有鸡还是先有蛋。
func (s *Service) SetCanceller(c port.AgentCanceller) { s.canceller = c }

// Cancel 停掉一个工作正在跑的那一轮。
//
// 三种收场：
//
//   - 当前状态不许取消 → 返回 model.ErrCancelNotAllowed，**不碰 Agent**
//   - 正常停下 → 状态推到 paused，落一条检查点事件
//   - 停不下来 → **杀进程**，状态推到 failed，事件里带原因码
//
// ★ 第三条是「界面说已取消、后台还在烧钱改文件」的唯一防线。
// 只报错不杀的话，用户以为什么都没发生。
func (s *Service) Cancel(ctx context.Context, workID string) error {
	w, err := s.repo.FindWork(ctx, workID)
	if err != nil {
		return fmt.Errorf("查工作 %s: %w", workID, err)
	}

	// ★ 先问规则再动手。反过来的话，审查中的工作已经被掐掉了才发现不该掐。
	if !model.CanCancel(w.State()) {
		return fmt.Errorf("%w: 工作 %s 处于 %s", model.ErrCancelNotAllowed, workID, w.State())
	}
	if s.canceller == nil {
		return fmt.Errorf("%w: 工作 %s", ErrNoCanceller, workID)
	}

	// ★ 先立旗：告诉后台那一轮「这是用户主动停的」。
	//
	// 不立的话，那一轮会因为 stopReason=cancelled 返回错误，被
	// 「AI 跑挂了」那条路径抢先推到 failed——而这里想推的 paused
	// 会被状态机拒（failed 是终态）。用户看到的是：
	// 我明明主动停的，界面说「失败」。真机走查撞到过。
	// ★ **不在这里清**：后台那一轮还在跑，它要靠这个标记判断
	// 「这次失败是用户主动停的」。在 Cancel 返回时清掉的话，
	// 那一轮结束时标记已经没了，照样会被推到 failed——
	// 序列会是 [... paused failed]，用户看到的还是「失败」。
	// 清理归 runTurn 的 goroutine（它知道自己什么时候结束）。
	s.markCancelling(workID)

	mustKill, cancelErr := s.canceller.CancelTurn(ctx, workID)
	if mustKill {
		// ★ 停不下来就杀。杀在前、落库在后——先把它按住，
		// 再去记录发生了什么。
		s.canceller.KillAgent(workID)
		s.markFailed(ctx, w, cancelErr)
		return fmt.Errorf("取消工作 %s 超时，已强制结束: %w", workID, cancelErr)
	}
	if cancelErr != nil {
		return fmt.Errorf("取消工作 %s: %w", workID, cancelErr)
	}

	if err := w.Transition(constant.WorkStatePaused); err != nil {
		return fmt.Errorf("工作 %s 状态迁移: %w", workID, err)
	}
	if err := s.repo.SaveWork(ctx, w); err != nil {
		return fmt.Errorf("保存工作 %s: %w", workID, err)
	}
	s.emit(ctx, workID, "state_change", map[string]any{"to": string(w.State())})
	// ★ 落检查点：用户回头想接着干，得有东西告诉他「上次停在哪」。
	s.emit(ctx, workID, "checkpoint", map[string]any{"reason": "user_cancelled"})
	return nil
}

// markFailed 把工作推到 failed 并把原因发出去。
func (s *Service) markFailed(ctx context.Context, w *model.Work, cause error) {
	reason := "cancel_timeout"
	if cause != nil {
		reason = "cancel_timeout: " + cause.Error()
	}
	if err := w.Transition(constant.WorkStateFailed); err == nil {
		_ = s.repo.SaveWork(ctx, w)
	}
	// 落库失败也照样发事件：用户至少要知道出事了
	s.emit(ctx, w.ID(), "state_change", map[string]any{
		"to": string(constant.WorkStateFailed), "reason": reason,
	})
}

// ErrorCode 把错误翻成**机器可读的码**，界面按它查 i18n 词条。
//
// ★ 认不出的一律给兜底码，绝不把 error 的文字摊给用户——
// 那里面有路径、有内部状态名，对他没有意义。
func ErrorCode(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, model.ErrCancelNotAllowed):
		return "work_cancel_not_allowed"
	case errors.Is(err, ErrNoCanceller):
		return "work_cancel_unavailable"
	case errors.Is(err, model.ErrNotFound):
		return "work_not_found"
	default:
		return "work_operation_failed"
	}
}

// markCancelling / clearCancelling / isCancelling 标记「这个工作正在被用户停」。
//
// ★ 后台那一轮据此区分「用户主动停」与「AI 跑挂了」——两者都表现为
// RunTurn 返回错误，但对用户是完全不同的两件事。
func (s *Service) markCancelling(workID string) {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	if s.cancelling == nil {
		s.cancelling = make(map[string]bool)
	}
	s.cancelling[workID] = true
}

func (s *Service) clearCancelling(workID string) {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	delete(s.cancelling, workID)
}

func (s *Service) isCancelling(workID string) bool {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	return s.cancelling[workID]
}
