package model_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// M6 U6.1.1 · 子计划与单元
//
// ★★ 裁定三：**每个单元都要分配角色**。不分配的后果是到执行时才发现
// 没人认领——而那时用户已经等了几分钟；更糟的是随手派一个，
// 让实现方审查自己的产出（INV-ATT-8 明令禁止）。

func mustUnit(t *testing.T, id, role string, deps ...string) model.Unit {
	t.Helper()
	u, err := model.NewUnit(id, "取消后证据读取", role, deps)
	if err != nil {
		t.Fatalf("造单元 %s: %v", id, err)
	}
	return u
}

// ★★ R1 · 单元**必须**有角色，且角色要在角色库里。
func TestUnit_R1_RoleIsRequiredAndMustExist(t *testing.T) {
	if _, err := model.NewUnit("unit-012", "取消", "", nil); !errors.Is(err, model.ErrUnitRoleRequired) {
		t.Errorf("没写角色却造出来了：%v——到执行时才发现没人认领，"+
			"而那时用户已经等了几分钟", err)
	}

	_, err := model.NewUnit("unit-012", "取消", "senior_vibe_coder", nil)
	if !errors.Is(err, model.ErrUnknownRole) {
		t.Fatalf("不存在的角色却造出来了：%v", err)
	}
	// ★ 错误里要带上**角色 id 与单元 id**：只说「角色不存在」的话，
	// 用户要在几十个单元里挨个找是哪一个写错了。
	for _, want := range []string{"unit-012", "senior_vibe_coder"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("错误里没有 %q：%v", want, err)
		}
	}

	if _, err := model.NewUnit("unit-012", "取消", "implementer", nil); err != nil {
		t.Errorf("真实存在的角色被拒了：%v", err)
	}
}

// ★★ R2 · 单元与子计划**不可变**。
func TestSubplan_R2_IsImmutable(t *testing.T) {
	for _, tc := range []struct {
		name    string
		rt      reflect.Type
		allowed map[string]bool
	}{
		{"Unit", reflect.TypeOf(&model.Unit{}), map[string]bool{
			"ID": true, "Title": true, "RoleID": true,
			"DependsOn": true, "ContractFrozen": true, "Accepted": true,
		}},
		{"Subplan", reflect.TypeOf(&model.Subplan{}), map[string]bool{
			"ID": true, "Title": true, "Units": true,
			"Progress": true, "Status": true,
		}},
	} {
		for i := range tc.rt.NumMethod() {
			name := tc.rt.Method(i).Name
			if !tc.allowed[name] {
				t.Errorf("%s 多了一个方法 %q——计划改了就出新版本，不在旧版本上动手",
					tc.name, name)
			}
		}
	}

	// 返回的是副本
	u := mustUnit(t, "unit-013", "implementer", "unit-012")
	deps := u.DependsOn()
	deps[0] = "篡改"
	if u.DependsOn()[0] == "篡改" {
		t.Error("DependsOn() 返回了内部切片")
	}
}

// ★★ R3 · 依赖必须指向**存在的**单元。
//
// 静静忽略的话那条依赖永远不生效：一个本该等着的单元会提前开工，
// 而它依赖的东西还没做——表现是 AI 对着一个不存在的接口写代码。
func TestSubplan_R3_DependencyMustExist(t *testing.T) {
	sp, err := model.NewSubplan("subplan-01", "ACP Runtime 抽象层", []model.Unit{
		mustUnit(t, "unit-013", "implementer", "unit-999"),
	})
	if err != nil {
		t.Fatal(err)
	}

	err = model.ValidateSubplans([]model.Subplan{sp})
	if !errors.Is(err, model.ErrUnknownDependency) {
		t.Fatalf("指向不存在的单元却过了：%v", err)
	}
	if !strings.Contains(err.Error(), "unit-999") {
		t.Errorf("错误里没说是哪个依赖：%v", err)
	}
}

// ★ 依赖**可以跨子计划**——设计稿里 unit-013 依赖 unit-012 就是这样。
func TestSubplan_DependencyAcrossSubplansIsFine(t *testing.T) {
	a, err := model.NewSubplan("subplan-01", "抽象层", []model.Unit{
		mustUnit(t, "unit-012", "implementer"),
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := model.NewSubplan("subplan-02", "取消", []model.Unit{
		mustUnit(t, "unit-013", "unit_reviewer", "unit-012"),
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := model.ValidateSubplans([]model.Subplan{a, b}); err != nil {
		t.Errorf("跨子计划的依赖被误判成不存在：%v", err)
	}
}

// ★★ R4 · 依赖**不成环**，且错误里把环列出来。
//
// 有环就没有「先做哪个」的答案。不拦的话调度会挑一个下手，
// 而那个选择每次运行都可能不同——同一份计划跑两次结果不一样。
func TestSubplan_R4_RejectsDependencyCycle(t *testing.T) {
	sp, err := model.NewSubplan("subplan-01", "环", []model.Unit{
		mustUnit(t, "unit-a", "implementer", "unit-c"),
		mustUnit(t, "unit-b", "implementer", "unit-a"),
		mustUnit(t, "unit-c", "implementer", "unit-b"),
	})
	if err != nil {
		t.Fatal(err)
	}

	err = model.ValidateSubplans([]model.Subplan{sp})
	if !errors.Is(err, model.ErrDependencyCycle) {
		t.Fatalf("成环却过了：%v", err)
	}
	// ★ 判据：错误里**把环列出来**。只说「成环了」的话，
	// 用户要在几十个单元里自己找那一圈。
	for _, want := range []string{"unit-a", "unit-b", "unit-c"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("错误里没列出 %q：%v", want, err)
		}
	}
}

// 自己依赖自己也是环。
func TestSubplan_RejectsSelfDependency(t *testing.T) {
	sp, err := model.NewSubplan("subplan-01", "自环", []model.Unit{
		mustUnit(t, "unit-a", "implementer", "unit-a"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.ValidateSubplans([]model.Subplan{sp}); !errors.Is(err, model.ErrDependencyCycle) {
		t.Errorf("自己依赖自己却过了：%v", err)
	}
}

// 无环的 DAG 照常通过。
func TestSubplan_AcceptsAcyclicGraph(t *testing.T) {
	sp, err := model.NewSubplan("subplan-01", "正常", []model.Unit{
		mustUnit(t, "unit-a", "implementer"),
		mustUnit(t, "unit-b", "implementer", "unit-a"),
		mustUnit(t, "unit-c", "unit_reviewer", "unit-a", "unit-b"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.ValidateSubplans([]model.Subplan{sp}); err != nil {
		t.Errorf("正常的 DAG 被拒了：%v", err)
	}
}

// 同一版计划里不许有重名单元。
func TestSubplan_RejectsDuplicateUnitID(t *testing.T) {
	a, err := model.NewSubplan("subplan-01", "一", []model.Unit{
		mustUnit(t, "unit-012", "implementer"),
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := model.NewSubplan("subplan-02", "二", []model.Unit{
		mustUnit(t, "unit-012", "unit_reviewer"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.ValidateSubplans([]model.Subplan{a, b}); !errors.Is(err, model.ErrDuplicateUnitID) {
		t.Errorf("重名单元却过了：%v——依赖指向它时没人说得清指的是哪一个", err)
	}
}

// ★★ R5 · 进度**由单元算出来**，不单独存。
//
// 存一个字段的话它会和单元的真实状态漂移，
// 而用户看到「3/3」时以为全做完了。
func TestSubplan_R5_ProgressComesFromUnits(t *testing.T) {
	sp, err := model.NewSubplan("subplan-01", "ACP Runtime 抽象层", []model.Unit{
		model.RestoreUnit("unit-011", "抽象层", "implementer", nil, true, true),
		model.RestoreUnit("unit-012", "取消", "implementer", nil, true, true),
		model.RestoreUnit("unit-013", "证据", "implementer", []string{"unit-012"}, false, false),
	})
	if err != nil {
		t.Fatal(err)
	}

	done, total := sp.Progress()
	if done != 2 || total != 3 {
		t.Errorf("进度 = %d/%d，想要 2/3", done, total)
	}
	if got := sp.Status(); got != "in_progress" {
		t.Errorf("状态 = %q，想要 in_progress", got)
	}
}

// 全验收 → accepted；一个都没动 → pending。设计稿的 `accepted · 3/3`。
func TestSubplan_StatusFromUnits(t *testing.T) {
	for _, tc := range []struct {
		name     string
		accepted []bool
		want     string
	}{
		{"全验收", []bool{true, true, true}, "accepted"},
		{"一个都没动", []bool{false, false}, "pending"},
		{"空子计划", nil, "empty"},
	} {
		units := make([]model.Unit, 0, len(tc.accepted))
		for i, ok := range tc.accepted {
			units = append(units, model.RestoreUnit(
				"unit-"+string(rune('a'+i)), "t", "implementer", nil, true, ok))
		}
		sp, err := model.NewSubplan("subplan-01", tc.name, units)
		if err != nil {
			t.Fatal(err)
		}
		if got := sp.Status(); got != tc.want {
			t.Errorf("%s：状态 = %q，想要 %q", tc.name, got, tc.want)
		}
	}
}

// ★ RestoreUnit **不校验角色**：角色库将来删掉一个角色时，
// 不该让历史计划读不出来。
func TestRestoreUnit_DoesNotValidateRole(t *testing.T) {
	u := model.RestoreUnit("unit-012", "取消", "a_role_we_removed", nil, true, true)
	if u.RoleID() != "a_role_we_removed" {
		t.Errorf("角色 = %q，历史计划该原样读回来", u.RoleID())
	}
}

// ★★ `WithSubplans` **不改原值**——它返回的是另一个 PlanVersion。
//
// 在原值上追加的话，「v3 当时拆成了什么」会随时间变化，
// 而计划面板的「变更历史」正是靠这个回答「每次为什么改」。
func TestPlanVersion_WithSubplansLeavesTheOriginalAlone(t *testing.T) {
	v1 := model.NewPlanVersion(1, "取消运行中的 turn", nil)

	sp, err := model.NewSubplan("subplan-01", "ACP Runtime 抽象层", []model.Unit{
		mustUnit(t, "unit-012", "implementer"),
		mustUnit(t, "unit-013", "unit_reviewer", "unit-012"),
	})
	if err != nil {
		t.Fatal(err)
	}

	withSubs, err := v1.WithSubplans([]model.Subplan{sp})
	if err != nil {
		t.Fatal(err)
	}

	if n, m := withSubs.Counts(); n != 1 || m != 2 {
		t.Errorf("计数 = %d 子计划 · %d 单元，想要 1 · 2", n, m)
	}
	// ★ 判据：原来那个一个字没变
	if n, m := v1.Counts(); n != 0 || m != 0 {
		t.Errorf("原值被改了：%d 子计划 · %d 单元", n, m)
	}
}

// ★ DAG 校验在**装进计划时**就做掉，不留到调度时。
//
// 留到那时的话，用户已经等了几分钟，而错误信息会是
// 「没有可执行的单元」——他看不出是计划写错了。
func TestPlanVersion_WithSubplansValidatesTheGraph(t *testing.T) {
	v1 := model.NewPlanVersion(1, "环", nil)
	sp, err := model.NewSubplan("subplan-01", "环", []model.Unit{
		mustUnit(t, "unit-a", "implementer", "unit-b"),
		mustUnit(t, "unit-b", "implementer", "unit-a"),
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := v1.WithSubplans([]model.Subplan{sp}); !errors.Is(err, model.ErrDependencyCycle) {
		t.Errorf("成环的计划却装进去了：%v", err)
	}
}
