package store

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
	"github.com/HuLuca1998/acp-flows/backend/internal/store/entity"
	"github.com/HuLuca1998/acp-flows/backend/internal/store/mapper"
)

// RequirementRepo 是需求快照的持久化实现。
//
// ★★ **没有 Update，也没有 Delete**（INV-REQ-2）：对外只有「存一版」和
// 「读」。已冻结的版本改不动，要改就 `SaveRequirement` 一条新版本。
// 未冻结的草稿走的还是同一个入口——调用方不需要知道那次落盘是 INSERT
// 还是 UPDATE，它只知道「把这一版存下来」。
//
// 加一个 `UpdateRequirement` 毫不费力，而加完之后所有测试还是绿的——
// 直到有人想查「上周那版需求说的是什么」，答案已经被覆盖掉了。
// 有反射测试守着这个类型不出现改写类方法。
type RequirementRepo struct {
	db  *gorm.DB
	clk Clock
}

const requirementColumns = "work_id, version, items, open_facts, frozen, created_at, updated_at"

// SaveRequirement 存一个版本。
//
// ★★ 分界线是**冻结**，不是版本号：
//
//   - **未冻结**的版本是草稿，原地覆盖。需求分析师追问一轮就划掉几条待确认
//     事实、改几个字，这是常态。每动一下就升一个版本号的话，版本链记的
//     就不再是「需求变过几次」而是「问过几个问题」——而用户看版本链
//     是为了回答「上周那版说的是什么」，草稿的中间态对这个问题没有意义。
//
//   - **已冻结**的版本一个字都不能改（INV-REQ-2）。那一版被计划、契约、
//     单元引用着，改它就是改历史：事后没人说得清「当时到底要做什么」。
//     连 frozen 本身也不能回退——解冻是没有的事。
func (r *RequirementRepo) SaveRequirement(ctx context.Context, req *model.RequirementSnapshot) error {
	e := mapper.RequirementToEntity(req)
	now := r.clk.Now()
	e.CreatedAt, e.UpdatedAt = now, now

	var existing entity.Requirement
	err := r.db.WithContext(ctx).Select(requirementColumns).
		First(&existing, "work_id = ? AND version = ?", e.WorkID, e.Version).Error

	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		if createErr := r.db.WithContext(ctx).Create(e).Error; createErr != nil {
			return fmt.Errorf("store: 存需求 %s v%d: %w", e.WorkID, e.Version, createErr)
		}
		return nil

	case err != nil:
		return fmt.Errorf("store: 查需求 %s v%d: %w", e.WorkID, e.Version, err)
	}

	if existing.Frozen {
		// ★★ 已冻结：只有**一模一样**才放行（重复保存要幂等，
		// 用户手快点两下、或者一次重试，都不该报错）。
		if existing.Items == e.Items && existing.OpenFacts == e.OpenFacts && e.Frozen {
			return nil
		}
		return fmt.Errorf("%w: %s v%d 已冻结，改它只能出新版本",
			model.ErrRequirementFrozen, e.WorkID, e.Version)
	}

	// 未冻结：草稿，原地覆盖。
	updateErr := r.db.WithContext(ctx).Model(&entity.Requirement{}).
		Where("work_id = ? AND version = ?", e.WorkID, e.Version).
		// ★ 用 map 而不是 struct：GORM 的 Updates 传 struct 会静默丢零值——
		// 「最后一条待确认事实被划掉」正好是零值（空串），传 struct 会丢掉这次更新。
		Updates(map[string]any{
			"items":      e.Items,
			"open_facts": e.OpenFacts,
			"frozen":     e.Frozen,
			"updated_at": now,
		}).Error
	if updateErr != nil {
		return fmt.Errorf("store: 更新需求草稿 %s v%d: %w", e.WorkID, e.Version, updateErr)
	}
	return nil
}

// LatestRequirement 取一个工作的最新一版。查不到返回 model.ErrNotFound。
func (r *RequirementRepo) LatestRequirement(
	ctx context.Context, workID string,
) (*model.RequirementSnapshot, error) {
	var e entity.Requirement
	err := r.db.WithContext(ctx).Select(requirementColumns).
		Where("work_id = ?", workID).Order("version DESC").First(&e).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("%w: 工作 %s 还没有需求", model.ErrNotFound, workID)
	}
	if err != nil {
		return nil, fmt.Errorf("store: 查最新需求 %s: %w", workID, err)
	}
	return mapper.RequirementToModel(&e), nil
}

// RequirementVersions 列出一个工作的全部版本，**从新到旧**。
//
// ★ 全部留着才叫版本链：用户要能回答「上周那版说的是什么」。
func (r *RequirementRepo) RequirementVersions(
	ctx context.Context, workID string,
) ([]*model.RequirementSnapshot, error) {
	var rows []entity.Requirement
	err := r.db.WithContext(ctx).Select(requirementColumns).
		Where("work_id = ?", workID).Order("version DESC").Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("store: 列出需求版本 %s: %w", workID, err)
	}

	// ★ 空集合返回空切片而不是 nil，且不是错误——
	// 「还没提需求」是新工作的常态。
	out := make([]*model.RequirementSnapshot, 0, len(rows))
	for i := range rows {
		out = append(out, mapper.RequirementToModel(&rows[i]))
	}
	return out, nil
}
