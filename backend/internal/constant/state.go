// Package constant 存放跨包共享的常量与枚举取值，按主题分文件。
//
// 只在一个文件里用到的常量应就地定义，不要污染这里的共享命名空间。
// 规则见 docs/rules/coding-standards.md §1.2。
package constant

import "slices"

// WorkState 是一次工作的状态。
//
// 取值与 AGENTS.md §8 术语表一字不差，界面上原样等宽显示、不翻译。
// 改动这里必须同时改术语表与 api/openapi.yaml。
type WorkState string

const (
	// WorkStateInitializing 正在为这个工作切 git worktree 与分支。
	//
	// 这是新建工作的初始状态——此时对话还没开始。
	WorkStateInitializing WorkState = "initializing"
	// WorkStateInitializingFailed worktree 创建失败。**终态**。
	//
	// 没切成 worktree 就没有可执行的现场，用户只能删掉重建，不能"恢复"。
	// 设计稿：「创建失败时工作进入 initializing_failed，不会退回原目录执行」。
	WorkStateInitializingFailed WorkState = "initializing_failed"
	// WorkStateClarifying 需求澄清中，尚未产出可冻结的需求快照。
	WorkStateClarifying WorkState = "clarifying"
	// WorkStatePlanning 规划中，正在产出或修订 PlanVersion。
	WorkStatePlanning WorkState = "planning"
	// WorkStateReady 计划已冻结，等待开始执行第一个单元。
	WorkStateReady WorkState = "ready"
	// WorkStateExecuting 某个单元正在执行。
	WorkStateExecuting WorkState = "executing"
	// WorkStateReviewingUnit 单元执行完毕，独立审查中。
	WorkStateReviewingUnit WorkState = "reviewing_unit"
	// WorkStateWaitingUser 被 D2/D3 决策阻塞，等用户裁决。
	WorkStateWaitingUser WorkState = "waiting_user"
	// WorkStatePaused 已暂停并落检查点，可恢复。应用更新前会把工作推到这个状态。
	WorkStatePaused WorkState = "paused"
	// WorkStateCompleted 全部单元已验收。终态。
	WorkStateCompleted WorkState = "completed"
	// WorkStateFailed 失败且不再继续。终态。
	WorkStateFailed WorkState = "failed"
)

// workStates 是全部合法取值，顺序即状态机的推进顺序。
//
// initializing / initializing_failed 由 ADR 0006 Q1 加入：设计规范 §09 列的
// 9 个状态词是「对话状态行显示的」子集，不是全集——initializing 阶段还没有对话。
//
// 新增状态必须同时：加到这里 · 加进 model 的迁移表 · 改 AGENTS.md §8 · 改 openapi.yaml。
// 漏掉任何一处，穷举测试会红。
var workStates = []WorkState{
	WorkStateInitializing,
	WorkStateInitializingFailed,
	WorkStateClarifying,
	WorkStatePlanning,
	WorkStateReady,
	WorkStateExecuting,
	WorkStateReviewingUnit,
	WorkStateWaitingUser,
	WorkStatePaused,
	WorkStateCompleted,
	WorkStateFailed,
}

// AllWorkStates 返回全部合法状态的副本。
//
// 返回副本而不是内部切片，避免调用方改坏共享状态。
func AllWorkStates() []WorkState {
	out := make([]WorkState, len(workStates))
	copy(out, workStates)
	return out
}

// IsValid 报告 s 是否是登记过的合法状态。
func (s WorkState) IsValid() bool {
	return slices.Contains(workStates, s)
}

// String 让 WorkState 可直接用于格式化。
func (s WorkState) String() string { return string(s) }
