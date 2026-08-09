package store

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
	"github.com/HuLuca1998/acp-flows/backend/internal/store/entity"
)

// PlanRepo 是计划版本的持久化实现。
//
// ★★ **没有 Update，也没有 Delete**（INV-PLAN-4）：计划改了就存一条新版本。
// 用户打开计划面板要能回答「上周那版拆成了什么、为什么改」——
// 覆盖掉的话那个问题永远没有答案。有反射测试守着。
type PlanRepo struct {
	db  *gorm.DB
	clk Clock
}

const (
	planColumns    = "work_id, version, title, dispositions, created_at"
	subplanColumns = "work_id, version, id, title, ord"
	unitColumns    = "work_id, version, subplan_id, id, title, role_id, " +
		"depends_on, contract_frozen, accepted, ord"
	// lineSep 是多值列的连接符。
	//
	// ★ 用换行而不是逗号：标识里出现逗号不是不可能，
	// 而拆错的后果是一条依赖凭空消失，且没有任何报错。
	lineSep = "\n"
)

// SavePlan 存一个计划版本，连同它的子计划与单元。
//
// ★★ **同一版存两次会被拒**，不是静默覆盖：一次重试就能把用户看过的
// 那一版换掉，而没有任何痕迹。
//
// ★ 三张表一起写，用事务：写了计划却没写单元的话，界面上会出现一个
// 「0 子计划 · 0 单元」的版本，而用户以为 AI 什么都没规划出来。
func (r *PlanRepo) SavePlan(ctx context.Context, workID string, v model.PlanVersion) error {
	var existing entity.Plan
	err := r.db.WithContext(ctx).Select(planColumns).
		First(&existing, "work_id = ? AND version = ?", workID, v.Version()).Error
	switch {
	case err == nil:
		return fmt.Errorf("%w: %s v%d 已经存过", model.ErrPlanVersionNotNext, workID, v.Version())
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return fmt.Errorf("store: 查计划 %s v%d: %w", workID, v.Version(), err)
	}

	now := r.clk.Now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		plan := &entity.Plan{
			WorkID: workID, Version: v.Version(), Title: v.Title(),
			Dispositions: joinDispositions(v.Dispositions()), CreatedAt: now,
		}
		if createErr := tx.Create(plan).Error; createErr != nil {
			return fmt.Errorf("store: 存计划 %s v%d: %w", workID, v.Version(), createErr)
		}

		for i, sp := range v.Subplans() {
			row := &entity.Subplan{
				WorkID: workID, Version: v.Version(),
				ID: sp.ID(), Title: sp.Title(), Ord: i,
			}
			if createErr := tx.Create(row).Error; createErr != nil {
				return fmt.Errorf("store: 存子计划 %s: %w", sp.ID(), createErr)
			}
			for j, u := range sp.Units() {
				unit := &entity.Unit{
					WorkID: workID, Version: v.Version(), SubplanID: sp.ID(),
					ID: u.ID(), Title: u.Title(), RoleID: u.RoleID(),
					DependsOn:      strings.Join(u.DependsOn(), lineSep),
					ContractFrozen: u.ContractFrozen(), Accepted: u.Accepted(),
					Ord: j,
				}
				if createErr := tx.Create(unit).Error; createErr != nil {
					return fmt.Errorf("store: 存单元 %s: %w", u.ID(), createErr)
				}
			}
		}
		return nil
	})
}

// LatestPlan 取一个工作的最新一版。查不到返回 model.ErrNotFound。
func (r *PlanRepo) LatestPlan(ctx context.Context, workID string) (model.PlanVersion, error) {
	var row entity.Plan
	err := r.db.WithContext(ctx).Select(planColumns).
		Where("work_id = ?", workID).Order("version DESC").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.PlanVersion{}, fmt.Errorf("%w: 工作 %s 还没有计划", model.ErrNotFound, workID)
	}
	if err != nil {
		return model.PlanVersion{}, fmt.Errorf("store: 查最新计划 %s: %w", workID, err)
	}
	return r.hydrate(ctx, row)
}

// PlanVersions 列出一个工作的全部版本，**从新到旧**。
//
// ★ 全部留着才叫版本链：用户要能回答「上周那版拆成了什么」。
func (r *PlanRepo) PlanVersions(ctx context.Context, workID string) ([]model.PlanVersion, error) {
	var rows []entity.Plan
	err := r.db.WithContext(ctx).Select(planColumns).
		Where("work_id = ?", workID).Order("version DESC").Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("store: 列出计划版本 %s: %w", workID, err)
	}

	// ★ 空集合返回空切片而不是 nil：「还没规划」是新工作的常态
	out := make([]model.PlanVersion, 0, len(rows))
	for _, row := range rows {
		v, hydrateErr := r.hydrate(ctx, row)
		if hydrateErr != nil {
			return nil, hydrateErr
		}
		out = append(out, v)
	}
	return out, nil
}

// hydrate 把一行计划连同它的子计划与单元还原成领域对象。
func (r *PlanRepo) hydrate(ctx context.Context, row entity.Plan) (model.PlanVersion, error) {
	var subRows []entity.Subplan
	if err := r.db.WithContext(ctx).Select(subplanColumns).
		Where("work_id = ? AND version = ?", row.WorkID, row.Version).
		Order("ord").Find(&subRows).Error; err != nil {
		return model.PlanVersion{}, fmt.Errorf("store: 查子计划 %s v%d: %w",
			row.WorkID, row.Version, err)
	}

	var unitRows []entity.Unit
	if err := r.db.WithContext(ctx).Select(unitColumns).
		Where("work_id = ? AND version = ?", row.WorkID, row.Version).
		Order("ord").Find(&unitRows).Error; err != nil {
		return model.PlanVersion{}, fmt.Errorf("store: 查单元 %s v%d: %w",
			row.WorkID, row.Version, err)
	}

	bySubplan := map[string][]model.Unit{}
	for _, u := range unitRows {
		bySubplan[u.SubplanID] = append(bySubplan[u.SubplanID], model.RestoreUnit(
			u.ID, u.Title, u.RoleID, splitLines(u.DependsOn), u.ContractFrozen, u.Accepted))
	}

	subplans := make([]model.Subplan, 0, len(subRows))
	for _, sp := range subRows {
		built, err := model.NewSubplan(sp.ID, sp.Title, bySubplan[sp.ID])
		if err != nil {
			return model.PlanVersion{}, fmt.Errorf("store: 还原子计划 %s: %w", sp.ID, err)
		}
		subplans = append(subplans, built)
	}

	v := model.NewPlanVersion(row.Version, row.Title, splitDispositions(row.Dispositions))
	// ★ 走 WithSubplans 而不是自己拼：它带着 DAG 校验，
	// 而一份存坏了的计划（比如成了环）应该在**读出来时**就被发现，
	// 不是等到调度时表现成「没有可执行的单元」。
	return v.WithSubplans(subplans)
}

func joinDispositions(d map[string]model.Disposition) string {
	if len(d) == 0 {
		return ""
	}
	// ★ 排序后再连接：map 遍历顺序每次都不同，不排的话同一份计划
	// 存两次得到两个不同的字符串——diff 出来全是噪音。
	keys := make([]string, 0, len(d))
	for k := range d {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		lines = append(lines, k+"="+string(d[k]))
	}
	return strings.Join(lines, lineSep)
}

func splitDispositions(s string) map[string]model.Disposition {
	out := map[string]model.Disposition{}
	for _, line := range splitLines(s) {
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out[k] = model.Disposition(v)
	}
	return out
}

// splitLines 拆多值列；空串返回 nil 而不是 [""]。
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, lineSep)
}
