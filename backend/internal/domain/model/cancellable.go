package model

import (
	"errors"

	"github.com/HuLuca1998/acp-flows/backend/internal/constant"
)

// ErrCancelNotAllowed 表示当前状态不允许取消。
//
// ★ 界面要按它给一句人话（「正在审查，等它出完结果再停」），
// 而不是把状态名摊给用户。
var ErrCancelNotAllowed = errors.New("model: 当前状态不允许取消")

// cancellableStates 逐个写明每个状态能不能取消。
//
// ★ **不用「不在表里就是能取消」这类默认值。** 新增一个状态却忘了表态时，
// 默认行为无论倒向哪边都会在某个真实场景里咬人：
// 默认能取消 → 一个我们完全不了解的状态被强行推走；
// 默认不能 → 用户点了停毫无反应，而他不知道为什么。
//
// `TestCanCancel_CoversEveryState` 对着 `constant.AllWorkStates()` 逐个核，
// 漏一个就红。
var cancellableStates = map[constant.WorkState]bool{
	// 还在切 worktree：可以停，那时还没有现场可言
	constant.WorkStateInitializing: true,
	// 终态，本来就不在跑
	constant.WorkStateInitializingFailed: false,

	constant.WorkStateClarifying: true,
	constant.WorkStatePlanning:   true,
	constant.WorkStateReady:      true,
	constant.WorkStateExecuting:  true,

	// ★ 审查中不许停。独立审查正在产出证据——半路掐掉的话，那个单元
	// 既没通过也没被驳回，卡在一个说不清的状态里；用户回头看
	// 「它到底做完没有」，答案是「不知道」。
	constant.WorkStateReviewingUnit: false,

	// 已经在等用户了，没有正在跑的东西可停
	constant.WorkStateWaitingUser: false,
	// 已经停了 / 已经结束
	constant.WorkStatePaused:    false,
	constant.WorkStateCompleted: false,
	constant.WorkStateFailed:    false,
}

// CanCancel 报告这个状态下允不允许取消。
//
// 认不出的状态一律**不许**——数据库里存了个我们不认识的值时
// （旧版本遗留、手改坏了），默认允许会把一个完全不了解的状态强行推走。
func CanCancel(s constant.WorkState) bool {
	return cancellableStates[s]
}
