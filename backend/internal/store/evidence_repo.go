package store

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
	"github.com/HuLuca1998/acp-flows/backend/internal/store/entity"
)

// EvidenceRepo 是证据的持久化实现。
//
// ★★ **没有 Update，也没有 Delete**：证据被改写过就不再是证据了——
// 「当时到底跑出了什么」没有第二个地方可查。有反射测试守着。
type EvidenceRepo struct {
	db  *gorm.DB
	clk Clock
}

const (
	evidenceColumns     = "id, work_id, unit_id, kind, source, summary, body, created_at"
	evidenceCritColumns = "evidence_id, criterion_id, ord"
)

// SaveEvidence 存一条证据，连同它支持的验收标准。
//
// ★ 两张表一起写，用事务：只写了证据没写关系的话，
// 那条证据在「标准 ✓ ev-441」里不会出现——用户以为它没派上用场。
func (r *EvidenceRepo) SaveEvidence(
	ctx context.Context, workID string, e model.Evidence,
) error {
	now := r.clk.Now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row := &entity.Evidence{
			ID: e.ID(), WorkID: workID, UnitID: e.UnitID(),
			Kind: string(e.Kind()), Source: string(e.Source()),
			Summary: e.Summary(), Body: e.Body(), CreatedAt: now,
		}
		if err := tx.Create(row).Error; err != nil {
			return fmt.Errorf("store: 存证据 %s: %w", e.ID(), err)
		}
		for i, cID := range e.Criteria() {
			if err := tx.Create(&entity.EvidenceCriterion{
				EvidenceID: e.ID(), CriterionID: cID, Ord: i,
			}).Error; err != nil {
				return fmt.Errorf("store: 存证据关系 %s→%s: %w", e.ID(), cID, err)
			}
		}
		return nil
	})
}

// EvidenceOf 列出一个单元的全部证据，按采集顺序。
func (r *EvidenceRepo) EvidenceOf(
	ctx context.Context, workID, unitID string,
) ([]model.Evidence, error) {
	var rows []entity.Evidence
	if err := r.db.WithContext(ctx).Select(evidenceColumns).
		Where("work_id = ? AND unit_id = ?", workID, unitID).
		Order("created_at, id").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("store: 查证据 %s/%s: %w", workID, unitID, err)
	}

	// ★ 空集合返回空切片而不是 nil：「还没有证据」是新单元的常态
	out := make([]model.Evidence, 0, len(rows))
	for _, row := range rows {
		var critRows []entity.EvidenceCriterion
		if err := r.db.WithContext(ctx).Select(evidenceCritColumns).
			Where("evidence_id = ?", row.ID).Order("ord").Find(&critRows).Error; err != nil {
			return nil, fmt.Errorf("store: 查证据关系 %s: %w", row.ID, err)
		}
		criteria := make([]string, 0, len(critRows))
		for _, c := range critRows {
			criteria = append(criteria, c.CriterionID)
		}

		e, err := model.NewEvidence(row.ID, row.UnitID,
			model.EvidenceKind(row.Kind), model.EvidenceSource(row.Source),
			row.Summary, row.Body, criteria)
		if err != nil {
			// ★ 读不出来就**报错**，不跳过：跳过的话「证据 3 条」会变成 2 条，
			// 而没有任何地方说明那一条去哪了
			return nil, fmt.Errorf("store: 还原证据 %s: %w", row.ID, err)
		}
		out = append(out, e)
	}
	return out, nil
}
