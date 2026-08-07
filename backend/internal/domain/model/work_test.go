package model_test

// Work 状态机（原 U0.9.1，编号已废弃）
// 原单元编号 U0.9.1（已废弃）。Work 状态机服务于 M2/M3/M4，见 docs/plan/roadmap.md
//
// 这些测试先于实现写就，先跑一次确认是红的（铁律 1）。

import (
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/constant"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// R1 · 九个状态取值与 AGENTS.md §8 术语表一字不差。
//
// 这是穷举测试：新增状态而忘了登记时它会红。
func TestWorkState_R1_ExhaustiveAndMatchesGlossary(t *testing.T) {
	// 术语表原文，顺序即状态机的推进顺序。改这里必须同时改 AGENTS.md §8。
	//
	// initializing / initializing_failed 是 ADR 0006 Q1 加入的：
	// 设计稿 §09 列的 9 个是「对话状态行显示的状态词」，不是全集——
	// initializing 阶段还没有对话，自然不在那张表里。
	want := []constant.WorkState{
		"initializing", "initializing_failed",
		"clarifying", "planning", "ready", "executing",
		"reviewing_unit", "waiting_user", "paused", "completed", "failed",
	}

	got := constant.AllWorkStates()

	if len(got) != len(want) {
		t.Fatalf("状态数量不符: 有 %d 个，术语表是 %d 个\n实际: %v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("第 %d 个状态不符: got %q, want %q", i, got[i], w)
		}
	}

	// 每个登记的状态都必须自认合法
	for _, s := range got {
		if !s.IsValid() {
			t.Errorf("%q 在 AllWorkStates 里但 IsValid() 为 false", s)
		}
	}

	// 没登记的一律非法
	for _, s := range []constant.WorkState{"", "running", "执行中", "Executing"} {
		if s.IsValid() {
			t.Errorf("%q 不在术语表里但 IsValid() 为 true", s)
		}
	}
}

// R2/R3 · 全部合法迁移可达，全部非法迁移被拒且错误可辨识。
func TestWork_R2R3_Transition(t *testing.T) {
	tests := []struct {
		name    string
		from    constant.WorkState
		to      constant.WorkState
		wantErr error
	}{
		// ── 初始化（ADR 0006 Q1）──────────────────────────────
		{"worktree 切好进入澄清", constant.WorkStateInitializing, constant.WorkStateClarifying, nil},
		{"worktree 创建失败", constant.WorkStateInitializing, constant.WorkStateInitializingFailed, nil},

		// ── 正常推进 ──────────────────────────────────────────
		{"澄清完进入规划", constant.WorkStateClarifying, constant.WorkStatePlanning, nil},
		{"规划完就绪", constant.WorkStatePlanning, constant.WorkStateReady, nil},
		{"就绪后开始执行", constant.WorkStateReady, constant.WorkStateExecuting, nil},
		{"执行完进入审查", constant.WorkStateExecuting, constant.WorkStateReviewingUnit, nil},
		{"审查通过后回到执行下一个单元", constant.WorkStateReviewingUnit, constant.WorkStateExecuting, nil},
		{"审查通过且无剩余单元则完成", constant.WorkStateReviewingUnit, constant.WorkStateCompleted, nil},

		// ── 决策阻塞 ──────────────────────────────────────────
		{"执行中触发 D2 进入等待", constant.WorkStateExecuting, constant.WorkStateWaitingUser, nil},
		{"审查中触发 D2 进入等待", constant.WorkStateReviewingUnit, constant.WorkStateWaitingUser, nil},
		{"用户决策后回到执行", constant.WorkStateWaitingUser, constant.WorkStateExecuting, nil},
		{"用户决策后要求重规划", constant.WorkStateWaitingUser, constant.WorkStatePlanning, nil},

		// ── 暂停与恢复（M1 的 update prepare 依赖这条）────────
		{"执行中被暂停", constant.WorkStateExecuting, constant.WorkStatePaused, nil},
		{"审查中被暂停", constant.WorkStateReviewingUnit, constant.WorkStatePaused, nil},
		{"等待用户时被暂停", constant.WorkStateWaitingUser, constant.WorkStatePaused, nil},
		{"从检查点恢复", constant.WorkStatePaused, constant.WorkStateExecuting, nil},

		// ── 失败 ──────────────────────────────────────────────
		{"执行中失败", constant.WorkStateExecuting, constant.WorkStateFailed, nil},
		{"规划中失败", constant.WorkStatePlanning, constant.WorkStateFailed, nil},

		// ── 非法：跳过审查直接完成 ★ 核心不变量 ────────────────
		{"执行中不能直接完成", constant.WorkStateExecuting, constant.WorkStateCompleted, model.ErrInvalidTransition},
		{"就绪不能直接完成", constant.WorkStateReady, constant.WorkStateCompleted, model.ErrInvalidTransition},

		// ── 非法：终态不可离开 ────────────────────────────────
		{"完成后不能再执行", constant.WorkStateCompleted, constant.WorkStateExecuting, model.ErrTerminalState},
		{"完成后不能失败", constant.WorkStateCompleted, constant.WorkStateFailed, model.ErrTerminalState},
		{"失败后不能执行", constant.WorkStateFailed, constant.WorkStateExecuting, model.ErrTerminalState},
		// initializing_failed 是终态：worktree 没切成，没有可执行的现场，
		// 用户只能删掉重建，不能"恢复"（ADR 0006 Q1）
		{"初始化失败后不能澄清", constant.WorkStateInitializingFailed, constant.WorkStateClarifying, model.ErrTerminalState},
		{"初始化失败后不能重试初始化", constant.WorkStateInitializingFailed, constant.WorkStateInitializing, model.ErrTerminalState},

		// ── 非法：跳级 ────────────────────────────────────────
		{"初始化不能跳过澄清直接规划", constant.WorkStateInitializing, constant.WorkStatePlanning, model.ErrInvalidTransition},
		{"澄清不能直接执行", constant.WorkStateClarifying, constant.WorkStateExecuting, model.ErrInvalidTransition},
		{"规划不能直接执行", constant.WorkStatePlanning, constant.WorkStateExecuting, model.ErrInvalidTransition},

		// ── 非法：自迁移 ──────────────────────────────────────
		{"不能迁移到自身", constant.WorkStateExecuting, constant.WorkStateExecuting, model.ErrInvalidTransition},

		// ── 非法：目标状态不合法 ──────────────────────────────
		{"目标状态不在术语表里", constant.WorkStateExecuting, constant.WorkState("running"), model.ErrUnknownState},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := model.NewWorkAt("work-01", tt.from)

			err := w.Transition(tt.to)

			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("期望迁移成功，得到错误: %v", err)
				}
				if w.State() != tt.to {
					t.Errorf("迁移后状态错误: got %q, want %q", w.State(), tt.to)
				}
				return
			}

			if err == nil {
				t.Fatalf("期望被拒绝(%v)，但迁移成功了", tt.wantErr)
			}
			if !isErr(err, tt.wantErr) {
				t.Fatalf("错误类型不符: got %v, want %v", err, tt.wantErr)
			}
			// 被拒绝时状态必须原样不动
			if w.State() != tt.from {
				t.Errorf("迁移被拒后状态被改动了: got %q, want %q", w.State(), tt.from)
			}
			// 错误信息要能辨识出 from/to，否则线上排查全靠猜
			msg := err.Error()
			if !contains(msg, string(tt.from)) || !contains(msg, string(tt.to)) {
				t.Errorf("错误信息未包含 from/to: %q", msg)
			}
		})
	}
}

// R4 · 新增状态而未在迁移表里处理时必须红。
//
// 做法：每个合法状态都必须至少作为一次迁移的起点或终点出现在迁移表里。
// 加了状态却忘了接进状态机时，这条会失败。
func TestWork_R4_EveryStateIsReachableOrTerminal(t *testing.T) {
	for _, s := range constant.AllWorkStates() {
		t.Run(string(s), func(t *testing.T) {
			outs := model.AllowedTransitionsFrom(s)
			ins := model.AllowedTransitionsTo(s)

			if len(outs) == 0 && len(ins) == 0 {
				t.Fatalf("状态 %q 既进不去也出不来——它没有被接进状态机", s)
			}
			if model.IsTerminal(s) && len(outs) != 0 {
				t.Errorf("终态 %q 不该有出边，却有 %v", s, outs)
			}
			if !model.IsTerminal(s) && s != constant.WorkStateInitializing && len(ins) == 0 {
				t.Errorf("非初始的非终态 %q 没有入边，永远不可达", s)
			}
		})
	}
}

// 领域层是纯计算：不做 IO、不碰时间源。
// 这条由 depguard 与 lint 强制，这里只做一个最基本的自证：
// 构造与迁移都不需要 context。
// ADR 0006 Q1：新建工作的初始状态是 initializing，不是 clarifying——
// worktree 还没切，对话还没开始。
func TestNewWork_StartsInInitializing(t *testing.T) {
	w := model.NewWork("work-01")
	if w.State() != constant.WorkStateInitializing {
		t.Errorf("新建工作的初始状态 = %q, want %q", w.State(), constant.WorkStateInitializing)
	}
}

func TestWork_DomainIsPure(t *testing.T) {
	w := model.NewWorkAt("work-01", constant.WorkStateReady)
	if err := w.Transition(constant.WorkStateExecuting); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if w.ID() != "work-01" {
		t.Errorf("ID() = %q, want %q", w.ID(), "work-01")
	}
}

// ── 小工具（测试内部用，不进 util）──────────────────────────

func isErr(got, want error) bool {
	for e := got; e != nil; {
		if e == want {
			return true
		}
		u, ok := e.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		e = u.Unwrap()
	}
	return false
}

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
