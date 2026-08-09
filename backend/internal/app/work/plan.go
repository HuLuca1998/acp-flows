package work

import (
	"context"
	"errors"
	"fmt"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/internal/constant"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// ErrPlansUnavailable 表示没装配计划存储。
var ErrPlansUnavailable = errors.New("work: 没有配置计划存储")

// ErrRequirementNotFrozen 表示需求还没冻结，不能开始规划（INV-REQ-1）。
//
// ★★ 需求还在变的时候做出来的计划，做完也对不上——而那时用户已经等了
// 一整轮，还得从头再来一次。
var ErrRequirementNotFrozen = errors.New("work: 需求还没冻结，不能开始规划")

// SetPlans 装上计划存储。
func (s *Service) SetPlans(p port.Plans) { s.plans = p }

// UnitView 是交给上层的单元视图。
type UnitView struct {
	ID    string
	Title string
	// RoleID / RoleName 是**由谁做**（裁定三）。
	//
	// ★ 显示名一并给出而不是让前端查表：前端查表的话，
	// 认不出的角色会显示成一个原始 id。
	RoleID         string
	RoleName       string
	DependsOn      []string
	ContractFrozen bool
	Accepted       bool
}

// SubplanView 是交给上层的子计划视图。
type SubplanView struct {
	ID    string
	Title string
	// Status / Done / Total 全是**算出来的**，不是存的字段。
	Status string
	Done   int
	Total  int
	Units  []UnitView
}

// PlanView 是交给上层的计划视图。
type PlanView struct {
	Version int
	Title   string
	// SubplanCount / UnitCount 就是设计稿的「N 子计划 · M 单元」。
	SubplanCount int
	UnitCount    int
	Dispositions map[string]string
	Subplans     []SubplanView
}

// StartPlanning 让计划架构师产出一版计划。
//
// ★★ **需求没冻结就拒绝**（INV-REQ-1）：需求还在变的时候做出来的计划，
// 做完也对不上，而那时用户已经等了一整轮还得从头再来。
func (s *Service) StartPlanning(ctx context.Context, workID string) error {
	if s.plans == nil {
		return fmt.Errorf("%w: %s", ErrPlansUnavailable, workID)
	}
	if s.requirements == nil {
		return fmt.Errorf("%w: %s", ErrRequirementsUnavailable, workID)
	}

	req, err := s.requirements.LatestRequirement(ctx, workID)
	if err != nil {
		return fmt.Errorf("查需求 %s: %w", workID, err)
	}
	if !req.CanStartPlanning() {
		return fmt.Errorf("%w: %s 的需求 v%d 还是草稿",
			ErrRequirementNotFrozen, workID, req.Version())
	}

	w, err := s.repo.FindWork(ctx, workID)
	if err != nil {
		return fmt.Errorf("查工作 %s: %w", workID, err)
	}
	if err := w.Transition(constant.WorkStatePlanning); err != nil {
		return fmt.Errorf("工作 %s 状态迁移: %w", workID, err)
	}
	if err := s.repo.SaveWork(ctx, w); err != nil {
		return fmt.Errorf("保存工作 %s: %w", workID, err)
	}
	s.emit(ctx, workID, "state_change", map[string]any{"to": string(w.State())})

	// ★★ 这一轮由**计划架构师**跑——`roleForState` 已经按状态映射好了，
	// 而工作刚刚进了 planning。
	prompt := planPromptFor(req)
	s.emit(ctx, workID, "user_message", map[string]any{"text": prompt})
	s.runTurn(ctx, workID, w.WorktreePath(), prompt, w.State())
	return nil
}

// planPromptFor 把冻结的需求变成给计划架构师的那句话。
//
// ★ 把**需求原文**贴进去，而不是只给一个 workID：Agent 那侧没有我们的库，
// 它只能看到我们发过去的字。
func planPromptFor(req *model.RequirementSnapshot) string {
	out := fmt.Sprintf("需求快照 v%d 已冻结，请把它拆成子计划与单元。\n\n需求条目：\n", req.Version())
	for i, item := range req.Items() {
		out += fmt.Sprintf("%d. %s\n", i+1, item)
	}
	// ★★ 明写「每个单元都要派角色」：不说的话 AI 会给出一份没人认领的计划，
	// 而那要到执行时才发现（裁定三）。
	out += "\n每个单元都必须写明由哪个角色做（需求分析师 / 计划架构师 / 单元设计师 / " +
		"实现工程师 / 测试执行者 / 单元审查员 / 决策顾问 / 记忆管理员）。"
	return out
}

// SavePlanVersion 落一版计划。
//
// ★ 存之前 domain 已经校验过 DAG（角色存在、依赖存在、不成环）——
// 那是 `PlanVersion.WithSubplans` 做的，这里只管落库。
func (s *Service) SavePlanVersion(ctx context.Context, workID string, v model.PlanVersion) error {
	if s.plans == nil {
		return fmt.Errorf("%w: %s", ErrPlansUnavailable, workID)
	}
	if err := s.plans.SavePlan(ctx, workID, v); err != nil {
		return err
	}
	subs, units := v.Counts()
	s.emit(ctx, workID, "plan_version", map[string]any{
		"version": v.Version(), "title": v.Title(),
		"subplans": subs, "units": units,
	})
	return nil
}

// PlanOf 读出一个工作的当前计划。查不到返回 model.ErrNotFound。
func (s *Service) PlanOf(ctx context.Context, workID string) (PlanView, error) {
	if s.plans == nil {
		return PlanView{}, fmt.Errorf("%w: %s", ErrPlansUnavailable, workID)
	}
	v, err := s.plans.LatestPlan(ctx, workID)
	if err != nil {
		return PlanView{}, fmt.Errorf("查计划 %s: %w", workID, err)
	}
	return toPlanView(v), nil
}

// PlanHistoryOf 列出全部版本，从新到旧——设计稿的「变更历史」。
func (s *Service) PlanHistoryOf(ctx context.Context, workID string) ([]PlanView, error) {
	if s.plans == nil {
		return nil, fmt.Errorf("%w: %s", ErrPlansUnavailable, workID)
	}
	all, err := s.plans.PlanVersions(ctx, workID)
	if err != nil {
		return nil, fmt.Errorf("列出计划版本 %s: %w", workID, err)
	}
	out := make([]PlanView, 0, len(all))
	for _, v := range all {
		out = append(out, toPlanView(v))
	}
	return out, nil
}

func toPlanView(v model.PlanVersion) PlanView {
	subs, units := v.Counts()
	view := PlanView{
		Version: v.Version(), Title: v.Title(),
		SubplanCount: subs, UnitCount: units,
		Dispositions: map[string]string{},
		Subplans:     make([]SubplanView, 0, subs),
	}
	for id, d := range v.Dispositions() {
		view.Dispositions[id] = string(d)
	}

	for _, sp := range v.Subplans() {
		done, total := sp.Progress()
		sv := SubplanView{
			ID: sp.ID(), Title: sp.Title(), Status: sp.Status(),
			Done: done, Total: total,
			Units: make([]UnitView, 0, total),
		}
		for _, u := range sp.Units() {
			uv := UnitView{
				ID: u.ID(), Title: u.Title(), RoleID: u.RoleID(),
				DependsOn:      u.DependsOn(),
				ContractFrozen: u.ContractFrozen(), Accepted: u.Accepted(),
			}
			// ★ 认不出的角色**原样给 id**，不编一个显示名：
			// 编出来的名字与角色页那张表对不上，用户会以为有两个不同的角色。
			if role, err := model.RoleByID(u.RoleID()); err == nil {
				uv.RoleName = role.DisplayName()
			}
			sv.Units = append(sv.Units, uv)
		}
		view.Subplans = append(view.Subplans, sv)
	}
	return view
}
