package store

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/HuLuca1998/acp-flows/backend/internal/store/entity"
)

// SkillHitsRepo 记 Skill 的命中计数。
//
// ★★ **没有 skills 表**——Skill 是扫盘产物，用户可以随时增删改，
// 我们不留副本。这里只存我们自己观察到的一件事：它被注入过几次。
type SkillHitsRepo struct {
	db  *gorm.DB
	clk Clock
}

// BumpSkillHits 给一批 Skill 的计数各加一。
//
// ★ 用 upsert：第一次被注入时那一行还不存在。分成「查了再插」两步的话，
// 两轮并发会撞在一起，而计数是并发下最容易悄悄少数的东西。
func (r *SkillHitsRepo) BumpSkillHits(ctx context.Context, refs []string) error {
	if len(refs) == 0 {
		return nil
	}
	now := r.clk.Now()
	rows := make([]entity.SkillHit, 0, len(refs))
	for _, ref := range refs {
		rows = append(rows, entity.SkillHit{Ref: ref, HitCount: 1, UpdatedAt: now})
	}
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "ref"}},
		DoUpdates: clause.Assignments(map[string]any{
			"hit_count":  gorm.Expr("skill_hits.hit_count + 1"),
			"updated_at": now,
		}),
	}).Create(&rows).Error
	if err != nil {
		return fmt.Errorf("store: Skill 命中计数: %w", err)
	}
	return nil
}

// SkillHits 读一批 Skill 的计数。
//
// ★★ 没有记录的返回 **0，不是缺席**：界面上「从没用过」和
// 「不知道用过几次」是两件事，后者会让用户以为数据丢了。
func (r *SkillHitsRepo) SkillHits(ctx context.Context, refs []string) (map[string]int, error) {
	out := make(map[string]int, len(refs))
	for _, ref := range refs {
		out[ref] = 0
	}
	if len(refs) == 0 {
		return out, nil
	}
	var rows []entity.SkillHit
	if err := r.db.WithContext(ctx).Where("ref IN ?", refs).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("store: 读 Skill 命中计数: %w", err)
	}
	for _, row := range rows {
		out[row.Ref] = row.HitCount
	}
	return out, nil
}
