package work

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
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

// DesignContract 让**单元设计师**为一个单元产出契约。
//
// ★★ 契约由单元设计师产出，**不是由实现工程师自己写**：自己给自己定
// 验收标准与边界，等于没有边界——他会写一个刚好装得下自己想改的东西的
// 范围（INV-ATT-8 的同一条道理）。
func (s *Service) DesignContract(ctx context.Context, workID, unitID string) error {
	if s.contracts == nil {
		return fmt.Errorf("%w: %s", ErrContractsUnavailable, workID)
	}

	unit, err := s.findUnit(ctx, workID, unitID)
	if err != nil {
		return err
	}
	w, err := s.repo.FindWork(ctx, workID)
	if err != nil {
		return fmt.Errorf("查工作 %s: %w", workID, err)
	}

	next := nextContractVersion(ctx, s.contracts, unitID)
	prompt := contractPromptFor(unit, next)
	s.emit(ctx, workID, "user_message", map[string]any{"text": prompt})

	turnCtx := context.WithoutCancel(ctx)
	// ★ 用**单元设计师**跑，不是单元自己派的那个角色——
	// 派给实现工程师的单元，它的契约也该由设计师来定。
	s.runTurnAsWithReply(ctx, workID, w.WorktreePath(), prompt, "unit_designer",
		func(reply string) { s.absorbContractReply(turnCtx, workID, unitID, reply, next) })
	return nil
}

func nextContractVersion(ctx context.Context, contracts port.Contracts, unitID string) int {
	cur, err := contracts.LatestContract(ctx, unitID)
	if err != nil {
		return 1
	}
	return cur.Version() + 1
}

// contractPromptFor 把单元变成给单元设计师的那段话。
func contractPromptFor(u model.Unit, version int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "为单元 %s（%s）设计契约 v%d。\n\n", u.ID(), u.Title(), version)
	fmt.Fprintf(&b, "这个单元由**%s**执行。\n", u.RoleID())
	if deps := u.DependsOn(); len(deps) > 0 {
		fmt.Fprintf(&b, "它依赖：%s\n", strings.Join(deps, " · "))
	}
	b.WriteString("\n契约要回答两件事：**凭什么说做完了**（可验证的验收标准），" +
		"**允许改哪些文件**（写入边界）。")
	b.WriteString(contractInstructions())
	return b.String()
}

// absorbContractReply 把回复变成一份契约草稿。
//
// ★ 产出的是**草稿不是冻结的**：冻结是用户的动作（与需求快照同理）。
// 自动冻结的话，用户还没看过这份契约，AI 就已经照着它开始改文件了。
func (s *Service) absorbContractReply(ctx context.Context, workID, unitID, reply string, version int) {
	c, err := ParseContractReply(reply, unitID, version)
	if err != nil {
		s.emit(ctx, workID, "unit_contract", map[string]any{
			"unit": unitID, "version": version, "failed": true, "reason": err.Error(),
		})
		return
	}
	if saveErr := s.contracts.SaveContract(ctx, c); saveErr != nil {
		s.emit(ctx, workID, "unit_contract", map[string]any{
			"unit": unitID, "version": version, "failed": true, "reason": saveErr.Error(),
		})
		return
	}
	b := c.Boundary()
	s.emit(ctx, workID, "unit_contract", map[string]any{
		"unit": unitID, "version": version,
		"criteria": len(c.Criteria()),
		"allowed":  len(b.Allowed), "forbidden": len(b.Forbidden),
	})
}

// FreezeContract 冻结一个单元的当前契约。**由用户点。**
func (s *Service) FreezeContract(ctx context.Context, workID, unitID string) error {
	if s.contracts == nil {
		return fmt.Errorf("%w: %s", ErrContractsUnavailable, workID)
	}
	c, err := s.contracts.LatestContract(ctx, unitID)
	if err != nil {
		return fmt.Errorf("查契约 %s: %w", unitID, err)
	}
	if err := c.Freeze(); err != nil {
		return fmt.Errorf("冻结契约 %s v%d: %w", unitID, c.Version(), err)
	}
	if err := s.contracts.SaveContract(ctx, c); err != nil {
		return fmt.Errorf("保存契约 %s v%d: %w", unitID, c.Version(), err)
	}
	return nil
}

// ContractView 是交给上层的契约视图。
type ContractView struct {
	UnitID   string
	Version  int
	Frozen   bool
	Criteria []model.Criterion
	// Allowed / Forbidden 是写入边界。
	Allowed   []string
	Forbidden []string
}

// ContractOf 读出一个单元的当前契约。查不到返回 model.ErrNotFound。
func (s *Service) ContractOf(ctx context.Context, unitID string) (ContractView, error) {
	if s.contracts == nil {
		return ContractView{}, fmt.Errorf("%w: %s", ErrContractsUnavailable, unitID)
	}
	c, err := s.contracts.LatestContract(ctx, unitID)
	if err != nil {
		return ContractView{}, fmt.Errorf("查契约 %s: %w", unitID, err)
	}
	b := c.Boundary()
	return ContractView{
		UnitID: c.UnitID(), Version: c.Version(), Frozen: c.IsFrozen(),
		Criteria: c.Criteria(), Allowed: b.Allowed, Forbidden: b.Forbidden,
	}, nil
}
