// Package model 存放领域模型：聚合、实体、值对象，以及只依赖自身状态的方法。
//
// 这一层是纯计算：不做 IO、不认识 SQL/HTTP/JSON-RPC、不直接取时间与随机数。
// 依赖方向由 depguard 强制，规则见 backend/internal/domain/AGENTS.md。
//
// 模型是充血的——状态流转与不变量校验都写在模型上。如果这里全是纯字段的 struct、
// 逻辑都跑到 service 里去了，说明分层做反了，停下来重新设计。
package model

import (
	"errors"
	"fmt"
	"slices"

	"github.com/HuLuca1998/acp-flows/backend/internal/constant"
)

// 领域错误。api 层统一把它们映射成 Problem 的机器可读错误码。
var (
	// ErrInvalidTransition 表示这次状态迁移不在允许的迁移表里。
	ErrInvalidTransition = errors.New("invalid work state transition")
	// ErrTerminalState 表示源状态是终态，不允许再迁移出去。
	ErrTerminalState = errors.New("work is in a terminal state")
	// ErrUnknownState 表示目标状态不是登记过的合法取值。
	ErrUnknownState = errors.New("unknown work state")
)

// workTransitions 是允许的状态迁移表。
//
// 只列**允许**的边；没列的一律拒绝。这样新增状态时忘了接进来，
// 穷举测试（TestWork_R4_EveryStateIsReachableOrTerminal）会直接红。
//
// 两条不变量值得单独说明：
//   - completed 只能从 reviewing_unit 进入 —— 不经过审查就不能算完成
//   - completed / failed 是终态，没有出边
var workTransitions = map[constant.WorkState][]constant.WorkState{
	constant.WorkStateClarifying: {
		constant.WorkStatePlanning,
		constant.WorkStateFailed,
	},
	constant.WorkStatePlanning: {
		constant.WorkStateReady,
		constant.WorkStateWaitingUser,
		constant.WorkStatePaused,
		constant.WorkStateFailed,
	},
	constant.WorkStateReady: {
		constant.WorkStateExecuting,
		constant.WorkStatePaused,
		constant.WorkStateFailed,
	},
	constant.WorkStateExecuting: {
		constant.WorkStateReviewingUnit,
		constant.WorkStateWaitingUser,
		constant.WorkStatePaused,
		constant.WorkStateFailed,
	},
	constant.WorkStateReviewingUnit: {
		constant.WorkStateExecuting, // 审查通过，继续下一个单元
		constant.WorkStateCompleted, // 审查通过且无剩余单元
		constant.WorkStateWaitingUser,
		constant.WorkStatePaused,
		constant.WorkStateFailed,
	},
	constant.WorkStateWaitingUser: {
		constant.WorkStateExecuting, // 用户裁决后继续
		constant.WorkStatePlanning,  // 用户要求重规划
		constant.WorkStatePaused,
		constant.WorkStateFailed,
	},
	constant.WorkStatePaused: {
		constant.WorkStateExecuting, // 从检查点恢复
		constant.WorkStateWaitingUser,
		constant.WorkStateFailed,
	},
	// 终态没有出边。
	constant.WorkStateCompleted: nil,
	constant.WorkStateFailed:    nil,
}

// Work 是一次完整的开发任务，独占一个 git worktree 与分支。
type Work struct {
	id    string
	state constant.WorkState
}

// NewWorkAt 用给定状态构造一个 Work。
//
// 主要供持久化层重建聚合与测试使用；新建工作走 NewWork。
func NewWorkAt(id string, state constant.WorkState) *Work {
	return &Work{id: id, state: state}
}

// NewWork 新建一个处于初始状态的工作。
func NewWork(id string) *Work {
	return &Work{id: id, state: constant.WorkStateClarifying}
}

// ID 返回工作标识（形如 work-08）。
func (w *Work) ID() string { return w.id }

// State 返回当前状态。
func (w *Work) State() constant.WorkState { return w.state }

// Transition 把工作迁移到 to。
//
// 迁移被拒时状态原样不动，且错误信息包含 from 与 to —— 线上排查靠它。
func (w *Work) Transition(to constant.WorkState) error {
	if !to.IsValid() {
		return fmt.Errorf("work %s: %s -> %s: %w", w.id, w.state, to, ErrUnknownState)
	}
	if IsTerminal(w.state) {
		return fmt.Errorf("work %s: %s -> %s: %w", w.id, w.state, to, ErrTerminalState)
	}
	if !slices.Contains(workTransitions[w.state], to) {
		return fmt.Errorf("work %s: %s -> %s: %w", w.id, w.state, to, ErrInvalidTransition)
	}
	w.state = to
	return nil
}

// IsTerminal 报告 s 是否是终态（没有任何出边）。
func IsTerminal(s constant.WorkState) bool {
	return s == constant.WorkStateCompleted || s == constant.WorkStateFailed
}

// AllowedTransitionsFrom 返回从 s 出发允许到达的状态。
func AllowedTransitionsFrom(s constant.WorkState) []constant.WorkState {
	out := make([]constant.WorkState, len(workTransitions[s]))
	copy(out, workTransitions[s])
	return out
}

// AllowedTransitionsTo 返回能够到达 s 的状态。
//
// 用于穷举测试：没有入边且不是初始状态的状态，永远不可达。
func AllowedTransitionsTo(s constant.WorkState) []constant.WorkState {
	var out []constant.WorkState
	for from, tos := range workTransitions {
		if slices.Contains(tos, s) {
			out = append(out, from)
		}
	}
	return out
}

// ErrNotFound 表示请求的聚合不存在。
//
// 持久化层把各自的「记录不存在」翻译成它——GORM 的哨兵错误不出 store 包。
var ErrNotFound = errors.New("not found")

// ErrAlreadyExists 表示唯一约束冲突。
var ErrAlreadyExists = errors.New("already exists")
