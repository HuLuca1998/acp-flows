package store

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
	"github.com/HuLuca1998/acp-flows/backend/internal/store/entity"
	"github.com/HuLuca1998/acp-flows/backend/internal/store/mapper"
)

// MemoryRepo 是 Memory 的持久化实现。
//
// ★★ **没有 Delete**（INV-MEM-6）：失效 ≠ 删除。
// 加一个删除方法毫不费力，而加完之后所有测试还是绿的——
// 直到有人想查「半年前那次运行用的是哪条记忆」，答案已经不在任何地方了。
// 有测试用反射守着这个类型不出现删除类方法。
type MemoryRepo struct {
	db  *gorm.DB
	clk Clock
}

// 显式列出列，不用 SELECT *：加列时不会静默改变返回结构。
const memoryColumns = "id, kind, scope, status, source_refs, created_by, " +
	"confirmed_by, reason, supersedes, history_len, created_at, updated_at"

// SaveMemory 新增或更新一条记忆索引。
//
// ★ **不写正文**（INV-MEM-8）：这张表里根本没有那一列。
func (r *MemoryRepo) SaveMemory(ctx context.Context, m *model.Memory) error {
	e := mapper.MemoryToEntity(m)
	now := r.clk.Now()
	e.CreatedAt, e.UpdatedAt = now, now

	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"kind", "scope", "status", "source_refs",
				"confirmed_by", "reason", "supersedes", "history_len", "updated_at",
			}),
		}).
		Create(e).Error
	if err != nil {
		return fmt.Errorf("store: 保存记忆 %s: %w", m.ID(), err)
	}
	return nil
}

// FindMemory 按 id 取一条。查不到返回 model.ErrNotFound。
func (r *MemoryRepo) FindMemory(ctx context.Context, id string) (*model.Memory, error) {
	var e entity.Memory
	err := r.db.WithContext(ctx).Select(memoryColumns).First(&e, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// ★ 不把 GORM 的错误泄漏出去：上层不该认识 ORM。
		return nil, fmt.Errorf("%w: 记忆 %s", model.ErrNotFound, id)
	}
	if err != nil {
		return nil, fmt.Errorf("store: 查记忆 %s: %w", id, err)
	}
	return mapper.MemoryToModel(&e), nil
}

// ListMemories 按条件列出记忆。
//
// ★ 空集合返回**空切片而不是 nil**，且不是错误——
// 「一条记忆都没有」是新用户的常态，当成错误的话记忆页一打开就是报错。
func (r *MemoryRepo) ListMemories(ctx context.Context, q port.MemoryFilter) ([]*model.Memory, error) {
	tx := r.db.WithContext(ctx).Select(memoryColumns)
	if q.Scope != "" {
		tx = tx.Where("scope = ?", q.Scope)
	}
	if q.Status != "" {
		tx = tx.Where("status = ?", q.Status)
	}

	var rows []entity.Memory
	// 按创建时间倒序：最近提的候选排在最上面，那是最需要人看的。
	if err := tx.Order("created_at DESC, id DESC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("store: 列出记忆: %w", err)
	}

	out := make([]*model.Memory, 0, len(rows))
	for i := range rows {
		out = append(out, mapper.MemoryToModel(&rows[i]))
	}
	return out, nil
}
