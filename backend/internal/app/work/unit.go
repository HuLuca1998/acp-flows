package work

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/HuLuca1998/acp-flows/backend/internal/constant"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// ErrContractNotFrozen 表示契约还没冻结，不能开工。
//
// ★★ 没冻结就开工的话，AI 干到一半契约变了——而它已经照着旧的那份
// 改了十几个文件。「这次到底要做什么」在执行过程中变化，
// 是最难排查的一类问题：产出对不上任何一版契约。
var ErrContractNotFrozen = errors.New("work: 契约还没冻结，不能开工")

// ErrUnitNotInPlan 表示这个单元不在当前计划里。
var ErrUnitNotInPlan = errors.New("work: 当前计划里没有这个单元")

// StartUnit 让一个单元开工。
//
// ★★ 这一轮由**单元自己派的那个角色**跑（裁定三），不是固定的实现工程师：
// 计划里写着 `unit-013 · unit_reviewer` 就该由审查员来跑——
// 派错人的话，实现方会审查自己的产出（INV-ATT-8 明令禁止）。
func (s *Service) StartUnit(ctx context.Context, workID, unitID string) error {
	if s.contracts == nil {
		return fmt.Errorf("%w: %s", ErrContractsUnavailable, workID)
	}

	unit, err := s.findUnit(ctx, workID, unitID)
	if err != nil {
		return err
	}

	c, err := s.contracts.LatestContract(ctx, unitID)
	if err != nil {
		return fmt.Errorf("查契约 %s: %w", unitID, err)
	}
	if !c.IsFrozen() {
		return fmt.Errorf("%w: %s v%d", ErrContractNotFrozen, unitID, c.Version())
	}

	w, err := s.repo.FindWork(ctx, workID)
	if err != nil {
		return fmt.Errorf("查工作 %s: %w", workID, err)
	}
	// ★ 先切单元再迁状态：反过来的话，中间失败会留下一个
	// 「executing 但不知道在做哪个单元」的工作——那时边界判定说不清。
	if err := w.StartUnit(unitID); err != nil {
		return fmt.Errorf("工作 %s 开始单元 %s: %w", workID, unitID, err)
	}
	if w.State() != constant.WorkStateExecuting {
		if tErr := w.Transition(constant.WorkStateExecuting); tErr != nil {
			return fmt.Errorf("工作 %s 状态迁移: %w", workID, tErr)
		}
	}
	if err := s.repo.SaveWork(ctx, w); err != nil {
		return fmt.Errorf("保存工作 %s: %w", workID, err)
	}
	s.emit(ctx, workID, "state_change", map[string]any{
		"to": string(w.State()), "unit": unitID,
	})

	prompt := unitPrompt(unit, c)
	s.emit(ctx, workID, "user_message", map[string]any{"text": prompt})
	// ★★ 用**单元自己派的角色**，不是按工作状态选——
	// 一个单元可能派给审查员，而工作状态是 executing。
	s.runTurnAs(ctx, workID, w.WorktreePath(), prompt, unit.RoleID())
	return nil
}

// ErrContractsUnavailable 表示没装配契约存储。
var ErrContractsUnavailable = errors.New("work: 没有配置契约存储")

// findUnit 在当前计划里找一个单元。
func (s *Service) findUnit(ctx context.Context, workID, unitID string) (model.Unit, error) {
	if s.plans == nil {
		return model.Unit{}, fmt.Errorf("%w: %s", ErrPlansUnavailable, workID)
	}
	plan, err := s.plans.LatestPlan(ctx, workID)
	if err != nil {
		return model.Unit{}, fmt.Errorf("查计划 %s: %w", workID, err)
	}
	for _, sp := range plan.Subplans() {
		for _, u := range sp.Units() {
			if u.ID() == unitID {
				return u, nil
			}
		}
	}
	// ★ 不在计划里就报错，**不凭空造一个单元**：凭空造的话它没有角色、
	// 没有契约，而 AI 会照着一份不存在的说明开始改文件。
	return model.Unit{}, fmt.Errorf("%w: %s", ErrUnitNotInPlan, unitID)
}

// unitPrompt 把契约变成给执行者的那段话。
//
// ★★ **验收标准与写入边界都贴进去**：Agent 那侧没有我们的库，
// 它只看得到我们发过去的字。不贴边界的话，它不知道哪些文件不该碰——
// 而越界会在权限卡片上被标出来，那时用户看到的是「它想动不该动的东西」，
// 而实际上是我们从没告诉过它边界在哪。
func unitPrompt(u model.Unit, c *model.UnitContract) string {
	var b strings.Builder
	fmt.Fprintf(&b, "开始做单元 %s：%s\n\n", u.ID(), u.Title())
	fmt.Fprintf(&b, "契约 v%d 的验收标准：\n", c.Version())
	for _, crit := range c.Criteria() {
		fmt.Fprintf(&b, "- [%s] %s\n", crit.ID, crit.Text)
	}

	boundary := c.Boundary()
	b.WriteString("\n写入边界（**只能改这些**）：\n")
	for _, p := range boundary.Allowed {
		fmt.Fprintf(&b, "- 允许：%s\n", p)
	}
	for _, p := range boundary.Forbidden {
		fmt.Fprintf(&b, "- 禁止：%s\n", p)
	}
	if len(boundary.Allowed) == 0 {
		// ★ 没有允许项时**明说**，别让这一段空着：空着的话 AI 会以为
		// 没有限制，而实际上一个字节都不许改。
		b.WriteString("- （这份契约没有写允许项——不要改动任何文件，先问用户）\n")
	}

	if deps := u.DependsOn(); len(deps) > 0 {
		fmt.Fprintf(&b, "\n它依赖：%s\n", strings.Join(deps, " · "))
	}
	return b.String()
}
