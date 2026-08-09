package work_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/internal/app/work"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
	"github.com/HuLuca1998/acp-flows/backend/tests/testutil"
)

// M6 U6.2.1 · 需求冻结后产出计划
//
// ★★ 需求还在变的时候做出来的计划，做完也对不上——而那时用户已经等了
// 一整轮，还得从头再来一次。

// memPlans 是内存版计划仓储，**与真 store 同一套规则**：同一版存两次被拒。
type memPlans struct {
	mu    sync.Mutex
	items map[string][]model.PlanVersion
}

func newMemPlans() *memPlans {
	return &memPlans{items: map[string][]model.PlanVersion{}}
}

func (p *memPlans) SavePlan(_ context.Context, workID string, v model.PlanVersion) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, existing := range p.items[workID] {
		if existing.Version() == v.Version() {
			return model.ErrPlanVersionNotNext
		}
	}
	p.items[workID] = append(p.items[workID], v)
	return nil
}

func (p *memPlans) LatestPlan(_ context.Context, workID string) (model.PlanVersion, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	list := p.items[workID]
	if len(list) == 0 {
		return model.PlanVersion{}, model.ErrNotFound
	}
	best := list[0]
	for _, v := range list {
		if v.Version() > best.Version() {
			best = v
		}
	}
	return best, nil
}

func (p *memPlans) PlanVersions(_ context.Context, workID string) ([]model.PlanVersion, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := append([]model.PlanVersion(nil), p.items[workID]...)
	// 从新到旧
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

var _ port.Plans = (*memPlans)(nil)

// planningSetup 建一个工作、冻结需求，返回它的 id。
func planningSetup(t *testing.T, runner port.AgentRunner) (*work.Service, *memPlans, string) {
	t.Helper()
	project := testutil.NewGitRepo(t)
	svc, _ := newServiceWithRequirements(t, runner)
	plans := newMemPlans()
	svc.SetPlans(plans)

	view, err := svc.Start(context.Background(), project, "用户能取消正在运行的 turn", "")
	if err != nil {
		t.Fatal(err)
	}
	return svc, plans, view.ID
}

// ★★ R1 · 需求**没冻结**时拒绝产计划（INV-REQ-1）。
func TestStartPlanning_R1_RefusesWhileRequirementIsDraft(t *testing.T) {
	runner := &fakeRunner{}
	svc, _, workID := planningSetup(t, runner)
	waitFor(t, "第一轮没跑起来", func() bool { return len(runner.snapshot()) == 1 })

	err := svc.StartPlanning(context.Background(), workID)
	if !errors.Is(err, work.ErrRequirementNotFrozen) {
		t.Fatalf("需求还是草稿却开始规划了：%v——做出来的计划做完也对不上，"+
			"而那时用户已经等了一整轮", err)
	}
	// ★ 判据：**一轮都没多跑**
	if n := len(runner.snapshot()); n != 1 {
		t.Errorf("被拒之后还是跑了：%d 轮", n)
	}
}

// ★★ R2 R3 · 冻结之后能规划：工作进 `planning`，且这一轮由**计划架构师**跑。
func TestStartPlanning_R2R3_PlanningRunsAsThePlanArchitect(t *testing.T) {
	runner := &fakeRunner{}
	svc, _, workID := planningSetup(t, runner)
	ctx := context.Background()
	waitFor(t, "第一轮没跑起来", func() bool { return len(runner.snapshot()) == 1 })

	if err := svc.FreezeRequirement(ctx, workID); err != nil {
		t.Fatal(err)
	}
	if err := svc.StartPlanning(ctx, workID); err != nil {
		t.Fatalf("冻结之后却规划不了：%v", err)
	}
	waitFor(t, "规划那一轮没跑起来", func() bool { return len(runner.snapshot()) == 2 })

	turn := runner.snapshot()[1]
	if turn.RoleID != "plan_architect" {
		t.Errorf("角色 = %q，想要 plan_architect——派错人的话，"+
			"一个只读的需求分析师会被要求产出计划", turn.RoleID)
	}
	// ★ 需求原文要贴进 prompt：Agent 那侧没有我们的库，它只看得到我们发过去的字
	if !strings.Contains(turn.Prompt, "用户能取消正在运行的 turn") {
		t.Errorf("prompt 里没有需求原文：%s", turn.Prompt)
	}
	// ★★ 明写「每个单元都要派角色」：不说的话 AI 会给出一份没人认领的计划
	if !strings.Contains(turn.Prompt, "角色") {
		t.Errorf("prompt 没要求派角色——那要到执行时才发现没人认领：%s", turn.Prompt)
	}
}

// ★★ R5 · 落一版计划要发 `plan_version` 事件，带版本号与两个计数。
func TestSavePlanVersion_R5_EmitsPlanVersionEvent(t *testing.T) {
	project := testutil.NewGitRepo(t)
	bus := &recordingBus{}
	svc := newServiceWithRunner(t, &memWorks{}, bus, &fakeRunner{})
	svc.SetPlans(newMemPlans())
	ctx := context.Background()

	view, err := svc.Start(ctx, project, "做点事", "")
	if err != nil {
		t.Fatal(err)
	}

	u, err := model.NewUnit("unit-012", "取消协议", "implementer", nil)
	if err != nil {
		t.Fatal(err)
	}
	sp, err := model.NewSubplan("subplan-01", "抽象层", []model.Unit{u})
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

	for _, e := range bus.snapshot() {
		if e.Type != "plan_version" {
			continue
		}
		if e.Payload["version"] != 1 {
			t.Errorf("事件里的版本 = %v", e.Payload["version"])
		}
		// 设计稿的「N 子计划 · M 单元」
		if e.Payload["subplans"] != 1 || e.Payload["units"] != 1 {
			t.Errorf("计数不对：%v", e.Payload)
		}
		return
	}
	t.Fatal("没发 plan_version 事件——界面不会知道计划出来了")
}

// ★★ R4 · 重规划**必须给处置**，缺一项就拒。
func TestPlan_R4_ReplanNeedsDispositions(t *testing.T) {
	v1 := model.NewPlanVersion(1, "取消", nil)

	// 已验收 unit-012，却没给它的处置
	_, err := v1.Next(2, "重规划", []string{"unit-012"}, nil)
	if !errors.Is(err, model.ErrDispositionMissing) {
		t.Fatalf("没给处置却重规划成功了：%v——那一项会悄悄失效，而没人知道", err)
	}

	_, err = v1.Next(2, "重规划", []string{"unit-012"},
		map[string]model.Disposition{"unit-012": model.DispositionStillValid})
	if err != nil {
		t.Errorf("给了处置却被拒：%v", err)
	}
}

// 读计划：视图里带**角色显示名**，认不出的角色留空而不是编一个。
func TestPlanOf_CarriesRoleNames(t *testing.T) {
	project := testutil.NewGitRepo(t)
	svc := newServiceWithRunner(t, &memWorks{}, &recordingBus{}, &fakeRunner{})
	plans := newMemPlans()
	svc.SetPlans(plans)
	ctx := context.Background()

	view, err := svc.Start(ctx, project, "做点事", "")
	if err != nil {
		t.Fatal(err)
	}

	sp, err := model.NewSubplan("subplan-01", "抽象层", []model.Unit{
		model.RestoreUnit("unit-012", "取消", "implementer", nil, true, true),
		model.RestoreUnit("unit-013", "证据", "a_role_we_removed", []string{"unit-012"}, false, false),
	})
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

	got, err := svc.PlanOf(ctx, view.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.SubplanCount != 1 || got.UnitCount != 2 {
		t.Errorf("计数 = %d · %d，想要 1 · 2", got.SubplanCount, got.UnitCount)
	}
	units := got.Subplans[0].Units
	if units[0].RoleName != "实现工程师" {
		t.Errorf("角色显示名 = %q", units[0].RoleName)
	}
	// ★ 认不出的角色**留空**，不编一个——编出来的名字与角色页那张表对不上
	if units[1].RoleName != "" {
		t.Errorf("给认不出的角色编了个名字：%q", units[1].RoleName)
	}
	if units[1].RoleID != "a_role_we_removed" {
		t.Errorf("原始 id 丢了：%q", units[1].RoleID)
	}
	// 进度算出来的
	if got.Subplans[0].Done != 1 || got.Subplans[0].Total != 2 {
		t.Errorf("进度 = %d/%d，想要 1/2", got.Subplans[0].Done, got.Subplans[0].Total)
	}
}

// 没装配计划存储时明确报错，不静静成功。
func TestPlan_UnconfiguredSaysSo(t *testing.T) {
	svc := newServiceWithRunner(t, &memWorks{}, &recordingBus{}, &fakeRunner{})

	if _, err := svc.PlanOf(context.Background(), "work-01"); !errors.Is(err, work.ErrPlansUnavailable) {
		t.Errorf("err = %v，想要 ErrPlansUnavailable", err)
	}
}
