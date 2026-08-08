package model_test

import (
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/constant"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// U3.2.3 · 什么时候不许停（验收点 V9）
//
// ★ 这一层是纯规则：给一个状态，答「能不能取消」。
// 放在 domain 是因为它是**业务判断**，不是协议细节——
// acp/session 不知道工作处于什么状态，也不该知道。

// ★★ R3：每个状态都要**明确表态**。
//
// 新增一个状态却忘了在这里表态时，这条测试会红——而漏掉的默认行为
// （无论是「都能取消」还是「都不能」）都会在某个真实场景里咬人。
func TestCanCancel_CoversEveryState(t *testing.T) {
	// 逐个写清楚，不用 map 的默认值兜底
	want := map[constant.WorkState]bool{
		// 还在切 worktree：可以停，那时还没有现场可言
		constant.WorkStateInitializing: true,
		// 终态，本来就不在跑
		constant.WorkStateInitializingFailed: false,
		constant.WorkStateClarifying:         true,
		constant.WorkStatePlanning:           true,
		constant.WorkStateReady:              true,
		constant.WorkStateExecuting:          true,
		// ★ 审查中不许停：独立审查正在产出证据，
		// 半路掐掉的话，那个单元既没通过也没被驳回，卡在一个说不清的状态里
		constant.WorkStateReviewingUnit: false,
		// 已经在等用户了，没有正在跑的东西可停
		constant.WorkStateWaitingUser: false,
		// 已经停了
		constant.WorkStatePaused:    false,
		constant.WorkStateCompleted: false,
		constant.WorkStateFailed:    false,
	}

	all := constant.AllWorkStates()
	if len(want) != len(all) {
		t.Fatalf("表里有 %d 个状态，全集有 %d 个——"+
			"新增状态时忘了在这里表态，而漏掉的默认行为会在某个真实场景里咬人",
			len(want), len(all))
	}

	for _, s := range all {
		expected, ok := want[s]
		if !ok {
			t.Errorf("状态 %q 没有表态「能不能取消」", s)
			continue
		}
		if got := model.CanCancel(s); got != expected {
			t.Errorf("CanCancel(%q) = %v, 想要 %v", s, got, expected)
		}
	}
}

// ★★ 审查中拒绝取消——这是这条规则存在的理由。
//
// 独立审查正在产出证据。半路掐掉的话，那个单元既没通过也没被驳回，
// 卡在一个说不清的状态里；用户回头看「它到底做完没有」，答案是「不知道」。
func TestCanCancel_RejectsReviewing(t *testing.T) {
	if model.CanCancel(constant.WorkStateReviewingUnit) {
		t.Error("审查中允许取消了——那个单元会卡在既没通过也没被驳回的状态里，" +
			"用户回头看「它到底做完没有」，答案是「不知道」")
	}
}

// 认不出的状态**不许取消**：最保守的那条。
//
// 数据库里存了一个我们不认识的值时（旧版本遗留、手改坏了），
// 默认允许取消会让一个我们完全不了解的状态被强行推走。
func TestCanCancel_UnknownStateIsRefused(t *testing.T) {
	if model.CanCancel(constant.WorkState("量子叠加态")) {
		t.Error("认不出的状态允许取消了——数据库里存了个不认识的值时，" +
			"默认允许会把一个我们完全不了解的状态强行推走")
	}
}

// ★★ 「能取消」与「迁得到 paused」必须**一致**。
//
// 真机走查撞到的：CanCancel(clarifying) 是 true，而迁移表里
// clarifying 没有 paused 这条出边——用户点了停，规则放行，
// 状态机拒绝，他看到一句「invalid work state transition」。
//
// 两处是同一件事的两种表达，谁都可能被单独改动。
func TestCanCancel_MatchesTransitionTable(t *testing.T) {
	for _, s := range constant.AllWorkStates() {
		w := model.NewWorkAt("work-01", s)
		canReach := w.Transition(constant.WorkStatePaused) == nil

		if model.CanCancel(s) && !canReach {
			t.Errorf("CanCancel(%q) 说能停，但状态机不让它迁到 paused——\n"+
				"用户点了停，规则放行，状态机拒绝，"+
				"他看到一句「invalid work state transition」", s)
		}
		// ★ 反方向**不要求**：`paused` 的入边不止「用户点停」这一条。
		// `reviewing_unit` / `waiting_user` 也能迁到 paused——那是
		// 「更新前把工作按下来」（M1 的 update/prepare）走的路，
		// 不是用户主动取消。把两者绑成双向的话，
		// 一键更新的暂停会被这条测试逼着改坏。
	}
}
