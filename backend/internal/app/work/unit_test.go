package work_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/work"
	"github.com/HuLuca1998/acp-flows/backend/internal/constant"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
	"github.com/HuLuca1998/acp-flows/backend/tests/testutil"
)

// M7 U7.3.1 · 单元开工
//
// ★★ 契约没冻结就开工的话，AI 干到一半契约变了——而它已经照着旧的那份
// 改了十几个文件。产出对不上任何一版契约，是最难排查的一类问题。

// unitSetup 建一个工作、落一版计划，返回 service 与 workID。
func unitSetup(t *testing.T, runner *fakeRunner) (*work.Service, *memContracts, string) {
	t.Helper()
	project := testutil.NewGitRepo(t)
	svc := newServiceWithRunner(t, &memWorks{}, &recordingBus{}, runner)
	contracts := newMemContracts()
	svc.SetPlans(newMemPlans())
	svc.SetContracts(contracts)
	ctx := context.Background()

	svc.SetRequirements(newMemRequirements())
	view, err := svc.Start(ctx, project, "做点事", "")
	if err != nil {
		t.Fatal(err)
	}
	// ★ 走**真实流程**：冻结需求 → 规划 → 落计划。
	// 直接落计划的话工作还停在 clarifying，而状态机不许它跳到 executing——
	// 那条限制是对的，绕过它等于让测试跑在一条真实路径产生不了的状态上。
	if err := svc.FreezeRequirement(ctx, view.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.StartPlanning(ctx, view.ID); err != nil {
		t.Fatal(err)
	}

	// ★ 单元派给**审查员**：真实场景里一个单元可以派给任何角色，
	// 而「按工作状态选角色」会把它错派成实现工程师。
	// ★ 依赖指向的单元也得在计划里——DAG 校验会拦住指向外部的依赖，
	// 而那正是它该做的事（一个本该等着的单元会提前开工）
	dep, err := model.NewUnit("unit-012", "取消协议", "implementer", nil)
	if err != nil {
		t.Fatal(err)
	}
	u, err := model.NewUnit("unit-013", "取消后证据读取", "unit_reviewer", []string{"unit-012"})
	if err != nil {
		t.Fatal(err)
	}
	sp, err := model.NewSubplan("subplan-01", "抽象层", []model.Unit{dep, u})
	if err != nil {
		t.Fatal(err)
	}
	v1, err := model.NewPlanVersion(1, "取消", nil).WithSubplans([]model.Subplan{sp})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SavePlanVersion(ctx, view.ID, v1); err != nil {
		t.Fatal(err)
	}
	return svc, contracts, view.ID
}

func frozenContract(t *testing.T) *model.UnitContract {
	t.Helper()
	c := model.NewUnitContract("unit-013", 1)
	if err := c.AddCriterion("ac-1", "取消后 diff 与最后事件游标可读"); err != nil {
		t.Fatal(err)
	}
	if err := c.SetBoundary(model.WriteBoundary{
		Allowed:   []string{"internal/acp/"},
		Forbidden: []string{"internal/acp/gen/"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := c.Freeze(); err != nil {
		t.Fatal(err)
	}
	return c
}

// ★★ R1 · 契约**没冻结**时拒绝开工，且一轮都不跑。
func TestStartUnit_R1_RefusesWhileContractIsDraft(t *testing.T) {
	runner := &fakeRunner{}
	svc, contracts, workID := unitSetup(t, runner)
	ctx := context.Background()
	waitFor(t, "前两轮没跑起来", func() bool { return len(runner.snapshot()) == 2 })

	draft := model.NewUnitContract("unit-013", 1)
	if err := draft.AddCriterion("ac-1", "随便"); err != nil {
		t.Fatal(err)
	}
	if err := contracts.SaveContract(ctx, draft); err != nil {
		t.Fatal(err)
	}

	err := svc.StartUnit(ctx, workID, "unit-013")
	if !errors.Is(err, work.ErrContractNotFrozen) {
		t.Fatalf("契约还是草稿却开工了：%v——AI 干到一半契约变了，"+
			"而它已经照着旧的那份改了十几个文件", err)
	}
	if n := len(runner.snapshot()); n != 2 {
		t.Errorf("被拒之后还是跑了：%d 轮", n)
	}
}

// ★★ R2 · 这一轮用**单元自己派的那个角色**（裁定三），不是按工作状态选。
func TestStartUnit_R2_RunsAsTheUnitsOwnRole(t *testing.T) {
	runner := &fakeRunner{}
	svc, contracts, workID := unitSetup(t, runner)
	ctx := context.Background()
	waitFor(t, "前两轮没跑起来", func() bool { return len(runner.snapshot()) == 2 })

	if err := contracts.SaveContract(ctx, frozenContract(t)); err != nil {
		t.Fatal(err)
	}
	if err := svc.StartUnit(ctx, workID, "unit-013"); err != nil {
		t.Fatalf("冻结之后却开不了工：%v", err)
	}
	waitFor(t, "执行轮没跑起来", func() bool { return len(runner.snapshot()) == 3 })

	turn := runner.snapshot()[2]
	if turn.RoleID != "unit_reviewer" {
		t.Errorf("角色 = %q，想要 unit_reviewer——计划里写着这个单元派给审查员，"+
			"按工作状态选的话会错派成实现工程师，"+
			"而实现方审查自己的产出是 INV-ATT-8 明令禁止的", turn.RoleID)
	}
}

// ★★ R5 · prompt 里带着**验收标准与写入边界**。
//
// 不贴边界的话，AI 不知道哪些文件不该碰——而越界会在权限卡片上被标出来，
// 那时用户看到「它想动不该动的东西」，实际上是我们从没告诉过它边界在哪。
func TestStartUnit_R5_PromptCarriesTheContract(t *testing.T) {
	runner := &fakeRunner{}
	svc, contracts, workID := unitSetup(t, runner)
	ctx := context.Background()
	waitFor(t, "前两轮没跑起来", func() bool { return len(runner.snapshot()) == 2 })

	if err := contracts.SaveContract(ctx, frozenContract(t)); err != nil {
		t.Fatal(err)
	}
	if err := svc.StartUnit(ctx, workID, "unit-013"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "执行轮没跑起来", func() bool { return len(runner.snapshot()) == 3 })

	prompt := runner.snapshot()[2].Prompt
	for _, want := range []string{
		"unit-013", // 是哪个单元
		"取消后 diff 与最后事件游标可读", // 验收标准原文
		"internal/acp/",     // 允许的边界
		"internal/acp/gen/", // 禁止的边界
		"unit-012",          // 它依赖谁
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt 里没有 %q：\n%s", want, prompt)
		}
	}
}

// ★★ 没有允许项时**明说**，别让那一段空着。
//
// 空着的话 AI 会以为没有限制，而实际上一个字节都不许改。
func TestStartUnit_EmptyBoundarySaysSo(t *testing.T) {
	runner := &fakeRunner{}
	svc, contracts, workID := unitSetup(t, runner)
	ctx := context.Background()
	waitFor(t, "前两轮没跑起来", func() bool { return len(runner.snapshot()) == 2 })

	c := model.NewUnitContract("unit-013", 1)
	if err := c.AddCriterion("ac-1", "随便"); err != nil {
		t.Fatal(err)
	}
	if err := c.Freeze(); err != nil {
		t.Fatal(err)
	}
	if err := contracts.SaveContract(ctx, c); err != nil {
		t.Fatal(err)
	}
	if err := svc.StartUnit(ctx, workID, "unit-013"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "执行轮没跑起来", func() bool { return len(runner.snapshot()) == 3 })

	if !strings.Contains(runner.snapshot()[2].Prompt, "不要改动任何文件") {
		t.Errorf("边界为空却没明说——AI 会以为没有限制，"+
			"而实际上一个字节都不许改：\n%s", runner.snapshot()[2].Prompt)
	}
}

// ★ 不在计划里的单元**不凭空造**：造的话它没有角色、没有契约，
// 而 AI 会照着一份不存在的说明开始改文件。
func TestStartUnit_UnknownUnitIsRejected(t *testing.T) {
	runner := &fakeRunner{}
	svc, _, workID := unitSetup(t, runner)
	ctx := context.Background()

	if err := svc.StartUnit(ctx, workID, "unit-999"); !errors.Is(err, work.ErrUnitNotInPlan) {
		t.Errorf("计划里没有的单元却开工了：%v", err)
	}
}

// 开工之后工作知道自己在做哪个单元——边界判定要靠它。
func TestStartUnit_WorkRemembersTheUnit(t *testing.T) {
	runner := &fakeRunner{}
	svc, contracts, workID := unitSetup(t, runner)
	ctx := context.Background()
	waitFor(t, "前两轮没跑起来", func() bool { return len(runner.snapshot()) == 2 })

	if err := contracts.SaveContract(ctx, frozenContract(t)); err != nil {
		t.Fatal(err)
	}
	if err := svc.StartUnit(ctx, workID, "unit-013"); err != nil {
		t.Fatal(err)
	}

	// ★ 判据落在**边界判定**上，不是「字段等于 unit-013」——
	// 后者只证明字段被赋值了，前者证明这条链真的通了
	if got := svc.BoundaryFor(ctx, workID, "internal/acp/x.go"); got != model.BoundaryInside {
		t.Errorf("边界判定 = %q，想要 in_boundary——工作没记住自己在做哪个单元", got)
	}
}

// 终态的工作切不动单元。
func TestStartUnit_TerminalWorkRefuses(t *testing.T) {
	w := model.NewWorkAt("work-done", constant.WorkStateCompleted)
	if err := w.StartUnit("unit-013"); !errors.Is(err, model.ErrTerminalState) {
		t.Errorf("已完成的工作却开始了一个单元：%v——"+
			"那会让边界判定拿到一份过期的契约", err)
	}
}
