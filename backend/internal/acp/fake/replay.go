package fake

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/acp/protocol"
)

// 脚本回放：一轮 session/prompt 收到之后，按脚本一步步演出来。
//
// ★ 与 runtime.go 分开，是因为它们变化的原因不同：runtime.go 管连接与分发，
// 这里管「脚本怎么解释」。加一种步骤（比如 U3.1.1 的 Ask）只动这个文件。

// nextTurn 取下一轮的脚本，并把轮次计数往前推。
func (r *Runtime) nextTurn() *Turn {
	r.mu.Lock()
	defer r.mu.Unlock()
	turn := r.script.turnAt(r.turnSeq)
	r.turnSeq++
	return turn
}

// replay 回放一轮：按脚本推事件，然后（或不）响应 session/prompt。
func (r *Runtime) replay(ctx context.Context, w *frameWriter, promptID json.RawMessage, turn *Turn) {
	if turn == nil {
		// 脚本没写这一轮 —— 不响应比编一个 end_turn 诚实：
		// 编出来的话，测试会以为脚本覆盖到了实际没覆盖的轮次。
		_, _ = fmt.Fprintf(r.stderr, "fake: 脚本没有第 %d 轮，session/prompt 不响应\n", r.turnSeq)
		return
	}

	// 断流计时从**首个 prompt** 开始 —— 语义是「跑着跑着断了」，
	// 从 Serve 起算的话握手耗时会算进去，测试时序变得不可控。
	r.armSilence()

	steps := turn.Steps
	if r.latency.Reorder {
		steps = reorderSteps(steps, r.latency.seed())
	}

	sessionID := r.script.sessionID()
	for i, step := range steps {
		if !sleepCtx(ctx, r.latency.delayFor(i, time.Duration(step.Delay))) {
			return
		}
		// 取消已经来了：别再往下演，直接收尾
		select {
		case <-r.cancelSignal():
			_ = w.respond(promptID, protocol.PromptResponse{
				StopReason: protocol.StopReasonCancelled,
			})
			return
		default:
		}

		if step.Ask != nil {
			outcome, ok := r.ask(ctx, w, step.Ask)
			if !ok {
				// ★ 被取消解除时要**回 cancelled**，不能静静 return：
				// 不回的话 session/prompt 永远挂着，调用方等到超时——
				// 而真正的行为（取消生效了）被误判成「Agent 卡死」。
				if outcome.Outcome == "cancelled" {
					_ = w.respond(promptID, protocol.PromptResponse{
						StopReason: protocol.StopReasonCancelled,
					})
				}
				return // ctx 结束或断流，安静收场
			}
			// ★ 客户端说取消，这一轮就以 cancelled 收尾，**不照脚本里的
			// stop_reason 走**。照走的话，用户按了「拒绝」而界面显示「完成」——
			// 比不问更糟。
			if outcome.Outcome == "cancelled" {
				_ = w.respond(promptID, protocol.PromptResponse{
					StopReason: protocol.StopReasonCancelled,
				})
				return
			}
			continue
		}
		if len(step.Emit) == 0 {
			continue
		}
		var update protocol.SessionUpdate
		if err := json.Unmarshal(step.Emit, &update); err != nil {
			_, _ = fmt.Fprintf(r.stderr, "fake: 第 %d 步的 emit 载荷非法: %v\n", i, err)
			continue
		}
		if err := w.notify(protocol.MethodSessionUpdate, protocol.SessionNotification{
			SessionID: sessionID,
			Update:    update,
		}); err != nil {
			return // 流已断，安静退出
		}
	}

	// ★ 不回 stopReason：NeverStops 预设，或脚本里这一轮没写 stop_reason。
	// 这是测 ErrCancelTimeout 的开关（testing-strategy.md §3.5）。
	//
	// 但**收到 session/cancel 之后要收尾**——真 Agent 就是这么做的。
	// 一直挂着的话，「取消之后这一轮真的停了吗」根本测不出来：
	// 被测代码会一直等一个永远不来的收尾信号。
	// ★ NeverStops 预设 = **连取消也不理**。那才是「Agent 卡死了」的样子，
	// 也是 U3.2.1 R4/R5（超时可诊断、超时要杀进程）唯一能测的场景。
	if r.neverStops {
		return
	}
	// 脚本里这一轮没写 stop_reason：正常情况下挂着，收到取消就收尾——
	// 真 Agent 就是这么做的。
	if turn.StopReason == "" {
		select {
		case <-r.cancelSignal():
			_ = w.respond(promptID, protocol.PromptResponse{
				StopReason: protocol.StopReasonCancelled,
			})
		case <-ctx.Done():
		}
		return
	}
	// 取消先到就照取消收尾，不等脚本里的延迟跑完
	select {
	case <-r.cancelSignal():
		_ = w.respond(promptID, protocol.PromptResponse{
			StopReason: protocol.StopReasonCancelled,
		})
		return
	case <-time.After(time.Duration(turn.StopDelay)):
	case <-ctx.Done():
		return
	}
	_ = w.respond(promptID, protocol.PromptResponse{StopReason: turn.StopReason})
}

// armSilence 在首个 prompt 之后启动断流计时。
func (r *Runtime) armSilence() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.silentAfter <= 0 || r.closed {
		return
	}
	d := r.silentAfter
	r.silentAfter = 0 // 只装一次
	closeOut := r.closeOut
	time.AfterFunc(d, func() {
		if closeOut != nil {
			closeOut()
		}
	})
}

// Close 停止 Fake 并断开管道。可重复调用。
