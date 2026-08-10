package runtime_test

import (
	"errors"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/acp/runtime"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// 契约来源：docs/spec/domain-model.md §16（INV-ROLE-1/3/4）
//           docs/notes/acp-field-notes.md §3（档位实测）§7 裁定 2（档名版本差）
//
// ★ 这一族测试守的是**收权真的收住了**。它错了不会崩、不会报错，
// 只会让「只读」的角色在沙箱里安静地改掉用户的文件。

// R3 · 同一个语义档，两端档名一个字都不一样。
//
// ★★ 硬编码成某一端的取值，换 Runtime 时就会发出一个对方不认识的档名。
// 而收权失败的表现**不是报错**：codex 默认档是 workspace-write 沙箱，
// 沙箱内的写操作根本不触发审批（acp-field-notes.md §3 实测：
// 客户端全拒 + 默认档 → 权限请求 0 次、文件建了）。
func TestModeNameOn_R3_SameIntentDiffersPerRuntime(t *testing.T) {
	cases := []struct {
		mode          model.SessionMode
		claude, codex string
	}{
		{model.SessionModeReadOnly, "plan", "read-only"},
		{model.SessionModeGuardedWrite, "default", "agent"},
		{model.SessionModeUnrestricted, "bypassPermissions", "agent-full-access"},
	}

	for _, c := range cases {
		gotClaude, err := runtime.ModeNameOn("claude", c.mode)
		if err != nil {
			t.Fatalf("%q 在 claude 上: %v", c.mode, err)
		}
		gotCodex, err := runtime.ModeNameOn("codex", c.mode)
		if err != nil {
			t.Fatalf("%q 在 codex 上: %v", c.mode, err)
		}

		if gotClaude != c.claude {
			t.Errorf("%q 在 claude 上 = %q，想要 %q", c.mode, gotClaude, c.claude)
		}
		if gotCodex != c.codex {
			t.Errorf("%q 在 codex 上 = %q，想要 %q", c.mode, gotCodex, c.codex)
		}
		// ★ 两端相同说明映射表被填成了同一个值，那就等于没有映射
		if gotClaude == gotCodex {
			t.Errorf("%q 两端档名都是 %q——映射表填错了", c.mode, gotClaude)
		}
	}
}

// R3b · codex 上不能再出现 `auto`。
//
// ★ 它是 codex **0.16.0** 的旧档名，1.1.7 已改。设计稿的角色表还写着它——
// 照抄的话发过去 codex 不认，收权失败，而失败的表现是「沙箱照旧放行」。
func TestModeNameOn_R3b_CodexHasNoAutoMode(t *testing.T) {
	for _, m := range []model.SessionMode{
		model.SessionModeReadOnly, model.SessionModeGuardedWrite, model.SessionModeUnrestricted,
	} {
		name, err := runtime.ModeNameOn("codex", m)
		if err != nil {
			t.Fatalf("%q: %v", m, err)
		}
		if name == "auto" {
			t.Errorf("%q 映射到了 codex 的 auto——那是 0.16.0 的旧档名，1.1.7 已改", m)
		}
	}
}

// R3c · 认不出的 Runtime 报错，不猜一个档名出来。
func TestModeNameOn_R3c_UnknownRuntimeErrsNoGuess(t *testing.T) {
	name, err := runtime.ModeNameOn("gemini", model.SessionModeReadOnly)
	if !errors.Is(err, runtime.ErrUnknownRuntime) {
		t.Errorf("未知 Runtime 的错误 = %v，想要 ErrUnknownRuntime", err)
	}
	if name != "" {
		t.Errorf("报错时还返回了档名 %q——调用方可能会拿它去发请求", name)
	}
}

// R3d · 每个语义档在每个已登记的 Runtime 上都翻译得出来。
//
// ★ 少了这条的话，映射表漏填一格要到真的开会话时才发现——
// 而那时用户已经点了「开始」。
func TestModeNameOn_R3d_MapHasNoHoles(t *testing.T) {
	modes := []model.SessionMode{
		model.SessionModeReadOnly, model.SessionModeGuardedWrite, model.SessionModeUnrestricted,
	}
	for _, rt := range []string{"claude", "codex"} {
		for _, m := range modes {
			name, err := runtime.ModeNameOn(rt, m)
			if err != nil {
				t.Errorf("%s 上的 %q 翻译不出来: %v", rt, m, err)
			}
			if name == "" {
				t.Errorf("%s 上的 %q 翻译成了空串", rt, m)
			}
		}
	}
}

// R2 · 八个预置角色都有推荐绑定，取值对上设计稿。
func TestRecommendedRuntimeFor_R2_EightPresetBindings(t *testing.T) {
	want := map[string]string{
		"requirement_analyst": "claude",
		"plan_architect":      "claude",
		"unit_designer":       "claude",
		"implementer":         "codex",
		"test_runner":         "codex",
		"unit_reviewer":       "claude",
		"decision_advisor":    "claude",
		"memory_curator":      "claude",
	}

	for _, r := range model.PresetRoles() {
		got, err := runtime.RecommendedRuntimeFor(r.ID())
		if err != nil {
			t.Errorf("%s（%s）没有推荐绑定: %v", r.ID(), r.DisplayName(), err)
			continue
		}
		if got != want[r.ID()] {
			t.Errorf("%s 的推荐 Runtime = %q，设计稿是 %q", r.ID(), got, want[r.ID()])
		}
	}
}

// R2b · 没登记的角色报错，不给一个「反正 claude 能干」的默认值。
func TestRecommendedRuntimeFor_R2b_UnlistedRoleErrs(t *testing.T) {
	got, err := runtime.RecommendedRuntimeFor("my_custom_role")
	if !errors.Is(err, runtime.ErrNoRecommendation) {
		t.Errorf("未登记角色的错误 = %v，想要 ErrNoRecommendation", err)
	}
	if got != "" {
		t.Errorf("没登记却返回了 %q——悄悄派给某一端，用户不会知道", got)
	}
}

// R2c · 每个角色在它推荐的 Runtime 上，档位翻译得出来。
//
// ★★ 这条把两张表**连起来**验：单独看两张表都对，
// 合起来却可能是「某个角色推荐用 codex，而 codex 上没有它要的那一档」。
// 那种错只在真的开会话时才暴露。
func TestPresetRoles_R2c_BindingAndModeFitTogether(t *testing.T) {
	for _, r := range model.PresetRoles() {
		rt, err := runtime.RecommendedRuntimeFor(r.ID())
		if err != nil {
			t.Fatalf("%s: %v", r.ID(), err)
		}
		name, err := runtime.ModeNameOn(rt, r.SessionMode())
		if err != nil {
			t.Errorf("%s（%s）推荐 %s，但它要的 %q 档在那端翻译不出来: %v",
				r.ID(), r.DisplayName(), rt, r.SessionMode(), err)
			continue
		}
		t.Logf("%-20s %-6s %s → %s", r.ID(), rt, r.SessionMode(), name)
	}
}
