package store

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
	"github.com/HuLuca1998/acp-flows/backend/internal/store/entity"
)

// ContractRepo 是单元契约的持久化实现。
//
// ★★ **没有 Update，也没有 Delete**（INV-UC-2）：契约冻结后不能改，
// 要改就存一条新版本。冻结的那一版被证据、验收、决策引用着——
// 改它就是改历史。有反射测试守着。
type ContractRepo struct {
	db  *gorm.DB
	clk Clock
}

const (
	contractColumns  = "unit_id, version, frozen, created_at, updated_at"
	criterionColumns = "unit_id, version, id, text, ord"
	boundaryColumns  = "unit_id, version, kind, prefix, ord"

	boundaryAllowed   = "allowed"
	boundaryForbidden = "forbidden"
)

// SaveContract 存一份契约。
//
// ★★ 分界线是**冻结**，与需求快照同一套规矩：
//
//   - 未冻结的是草稿，原地覆盖（单元设计师还在往里加标准）
//   - 已冻结的一个字都不能改，连 frozen 本身也不能回退
func (r *ContractRepo) SaveContract(ctx context.Context, c *model.UnitContract) error {
	var existing entity.Contract
	err := r.db.WithContext(ctx).Select(contractColumns).
		First(&existing, "unit_id = ? AND version = ?", c.UnitID(), c.Version()).Error

	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		// 新的一版，往下走
	case err != nil:
		return fmt.Errorf("store: 查契约 %s v%d: %w", c.UnitID(), c.Version(), err)
	case existing.Frozen:
		// ★ 已冻结：只有原样重存才放行（重试与手快点两下都是常态）
		if c.IsFrozen() && r.sameContent(ctx, c) {
			return nil
		}
		return fmt.Errorf("%w: %s v%d 已冻结，改它只能出新版本",
			model.ErrContractFrozen, c.UnitID(), c.Version())
	}

	now := r.clk.Now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 草稿：整版重写。★ 先清后写而不是逐条 diff——
		// diff 的代码里每一个分支都是一次「这条到底该不该删」的判断，
		// 而判断错的后果是一条验收标准悄悄消失。
		for _, m := range []any{&entity.ContractCriterion{}, &entity.ContractBoundary{}} {
			if delErr := tx.Where("unit_id = ? AND version = ?", c.UnitID(), c.Version()).
				Delete(m).Error; delErr != nil {
				return fmt.Errorf("store: 清契约 %s v%d: %w", c.UnitID(), c.Version(), delErr)
			}
		}

		row := &entity.Contract{
			UnitID: c.UnitID(), Version: c.Version(), Frozen: c.IsFrozen(),
			CreatedAt: now, UpdatedAt: now,
		}
		if saveErr := tx.Save(row).Error; saveErr != nil {
			return fmt.Errorf("store: 存契约 %s v%d: %w", c.UnitID(), c.Version(), saveErr)
		}

		for i, crit := range c.Criteria() {
			if createErr := tx.Create(&entity.ContractCriterion{
				UnitID: c.UnitID(), Version: c.Version(),
				ID: crit.ID, Text: crit.Text, Ord: i,
			}).Error; createErr != nil {
				return fmt.Errorf("store: 存验收标准 %s: %w", crit.ID, createErr)
			}
		}

		b := c.Boundary()
		for kind, prefixes := range map[string][]string{
			boundaryAllowed:   b.Allowed,
			boundaryForbidden: b.Forbidden,
		} {
			for i, prefix := range prefixes {
				if createErr := tx.Create(&entity.ContractBoundary{
					UnitID: c.UnitID(), Version: c.Version(),
					Kind: kind, Prefix: prefix, Ord: i,
				}).Error; createErr != nil {
					return fmt.Errorf("store: 存边界 %s: %w", prefix, createErr)
				}
			}
		}
		return nil
	})
}

// LatestContract 取一个单元的最新一版。查不到返回 model.ErrNotFound。
func (r *ContractRepo) LatestContract(
	ctx context.Context, unitID string,
) (*model.UnitContract, error) {
	var row entity.Contract
	err := r.db.WithContext(ctx).Select(contractColumns).
		Where("unit_id = ?", unitID).Order("version DESC").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("%w: 单元 %s 还没有契约", model.ErrNotFound, unitID)
	}
	if err != nil {
		return nil, fmt.Errorf("store: 查最新契约 %s: %w", unitID, err)
	}
	return r.hydrate(ctx, row)
}

// ContractVersions 列出一个单元的全部版本，**从新到旧**。
func (r *ContractRepo) ContractVersions(
	ctx context.Context, unitID string,
) ([]*model.UnitContract, error) {
	var rows []entity.Contract
	if err := r.db.WithContext(ctx).Select(contractColumns).
		Where("unit_id = ?", unitID).Order("version DESC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("store: 列出契约版本 %s: %w", unitID, err)
	}

	// ★ 空集合返回空切片而不是 nil：「还没有契约」是新单元的常态
	out := make([]*model.UnitContract, 0, len(rows))
	for _, row := range rows {
		c, err := r.hydrate(ctx, row)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

func (r *ContractRepo) hydrate(
	ctx context.Context, row entity.Contract,
) (*model.UnitContract, error) {
	var critRows []entity.ContractCriterion
	if err := r.db.WithContext(ctx).Select(criterionColumns).
		Where("unit_id = ? AND version = ?", row.UnitID, row.Version).
		Order("ord").Find(&critRows).Error; err != nil {
		return nil, fmt.Errorf("store: 查验收标准 %s v%d: %w", row.UnitID, row.Version, err)
	}
	criteria := make([]model.Criterion, 0, len(critRows))
	for _, c := range critRows {
		criteria = append(criteria, model.Criterion{ID: c.ID, Text: c.Text})
	}

	var bRows []entity.ContractBoundary
	if err := r.db.WithContext(ctx).Select(boundaryColumns).
		Where("unit_id = ? AND version = ?", row.UnitID, row.Version).
		Order("kind, ord").Find(&bRows).Error; err != nil {
		return nil, fmt.Errorf("store: 查边界 %s v%d: %w", row.UnitID, row.Version, err)
	}
	var boundary model.WriteBoundary
	for _, b := range bRows {
		if b.Kind == boundaryForbidden {
			boundary.Forbidden = append(boundary.Forbidden, b.Prefix)
			continue
		}
		boundary.Allowed = append(boundary.Allowed, b.Prefix)
	}

	return model.RestoreUnitContract(row.UnitID, row.Version, criteria, boundary, row.Frozen), nil
}

// sameContent 报告库里那一版与传进来的是不是一模一样。
func (r *ContractRepo) sameContent(ctx context.Context, c *model.UnitContract) bool {
	stored, err := r.LatestContract(ctx, c.UnitID())
	if err != nil || stored.Version() != c.Version() {
		return false
	}
	if len(stored.Criteria()) != len(c.Criteria()) {
		return false
	}
	for i, crit := range stored.Criteria() {
		if crit != c.Criteria()[i] {
			return false
		}
	}
	sb, cb := stored.Boundary(), c.Boundary()
	return sameStrings(sb.Allowed, cb.Allowed) && sameStrings(sb.Forbidden, cb.Forbidden)
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
