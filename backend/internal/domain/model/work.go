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
// 三条不变量值得单独说明：
//   - completed 只能从 reviewing_unit 进入 —— 不经过审查就不能算完成
//   - completed / failed / initializing_failed 是终态，没有出边
//   - initializing_failed 不可恢复：worktree 没切成就没有可执行的现场（ADR 0006 Q1）
var workTransitions = map[constant.WorkState][]constant.WorkState{
	constant.WorkStateInitializing: {
		constant.WorkStateClarifying,         // worktree 切好了
		constant.WorkStateInitializingFailed, // 切失败，终态
		constant.WorkStatePaused,             // 用户在切 worktree 时就点了停
	},
	constant.WorkStateClarifying: {
		constant.WorkStatePlanning,
		constant.WorkStatePaused, // 用户点了停
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
	constant.WorkStateInitializingFailed: nil,
	constant.WorkStateCompleted:          nil,
	constant.WorkStateFailed:             nil,
}

// Work 是一次完整的开发任务，独占一个 git worktree 与分支。
type Work struct {
	id    string
	state constant.WorkState

	// worktreePath 与 baseCommit 是这个工作的 git 现场。
	//
	// ★★ **基线必须记下来**：右栏的「领先几个 commit」与验收时的 diff
	// 都拿它当起点。不记的话，「AI 到底干了什么」只能靠猜——
	// 而猜出来的答案会随着仓库变化而漂移。
	worktreePath string
	baseCommit   string
	branch       string
	// currentUnitID 是**现在在做哪个单元**。
	//
	// ★★ 有它才谈得上「这次写入在不在边界内」——边界写在单元的契约里，
	// 不知道是哪个单元就查不到契约，而那时权限卡片只能说「不知道」。
	//
	// ★ 空表示还没开始做任何单元（澄清、规划阶段都是空的）。
	currentUnitID string
	// projectPath 是这个工作属于哪个项目（**绝对路径**）。
	//
	// ★★ 左栏的项目树按它把工作挂到项目下。不记的话，用户打开应用看到
	// 一个空荡荡的项目——而工作明明就在库里。
	// （`design/PARITY.md` 开篇记的正是这一类：「数据有却不显示，
	// 等于界面说谎」。）
	//
	// ★ 用**路径**而不是项目 id：项目可以被移除再加回来，那时 id 变了
	// 而路径没变——按 id 关联的话，那些工作会集体失去归属。
	projectPath string
}

// ProjectPath 返回这个工作属于哪个项目；空表示还没记（老数据）。
func (w *Work) ProjectPath() string { return w.projectPath }

// SetProject 记下它属于哪个项目。
func (w *Work) SetProject(path string) { w.projectPath = path }

// CurrentUnitID 返回现在在做的单元；空表示还没开始做任何单元。
func (w *Work) CurrentUnitID() string { return w.currentUnitID }

// StartUnit 把「现在在做哪个单元」切到 unitID。
//
// ★ 终态的工作切不动：一个已经完成的工作又「开始做某个单元」说不清是什么
// 意思，而它会让边界判定拿到一份过期的契约。
func (w *Work) StartUnit(unitID string) error {
	if IsTerminal(w.state) {
		return fmt.Errorf("work %s: %s 状态下不能开始单元: %w",
			w.id, w.state, ErrTerminalState)
	}
	w.currentUnitID = unitID
	return nil
}

// WorktreePath 返回工作区路径；还没切时为空。
func (w *Work) WorktreePath() string { return w.worktreePath }

// BaseCommit 返回这个工作的基线 commit；还没切时为空。
func (w *Work) BaseCommit() string { return w.baseCommit }

// Branch 返回工作所在的分支；还没切时为空。
func (w *Work) Branch() string { return w.branch }

// SetWorktree 记下切好的工作区。
//
// ★ 只在 worktree **真的建好之后**调用——记一个还没建出来的路径的话，
// 恢复时会指向一个不存在的目录。
func (w *Work) SetWorktree(path, branch, baseCommit string) {
	w.worktreePath, w.branch, w.baseCommit = path, branch, baseCommit
}

// NewWorkAt 用给定状态构造一个 Work。
//
// 主要供持久化层重建聚合与测试使用；新建工作走 NewWork。
func NewWorkAt(id string, state constant.WorkState) *Work {
	return &Work{id: id, state: state}
}

// NewWork 新建一个处于初始状态的工作。
//
// 初始状态是 initializing 而不是 clarifying —— 此时 worktree 还没切，
// 对话还没开始（ADR 0006 Q1）。
func NewWork(id string) *Work {
	return &Work{id: id, state: constant.WorkStateInitializing}
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
	return s == constant.WorkStateCompleted ||
		s == constant.WorkStateFailed ||
		s == constant.WorkStateInitializingFailed
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
