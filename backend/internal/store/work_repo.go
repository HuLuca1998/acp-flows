package store

import (
	"context"

	"gorm.io/gorm"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
	"github.com/HuLuca1998/acp-flows/backend/internal/store/entity"
	"github.com/HuLuca1998/acp-flows/backend/internal/store/mapper"
)

// WorkRepo 是 Work 聚合的持久化实现。
//
// 它实现 app/port 里的 WorkRepo 接口——Go 是结构化类型，
// 不需要 import 那个包。
type WorkRepo struct {
	db  *gorm.DB
	clk Clock
}

// 查询时显式列出列，不用 SELECT *：加列时不会静默改变返回结构。
const workColumns = "id, project_id, state, branch, worktree, created_at, updated_at"

// CreateWork 新增一条工作记录。
func (r *WorkRepo) CreateWork(ctx context.Context, w *model.Work) error {
	e := mapper.WorkToEntity(w)
	now := r.clk.Now()
	e.CreatedAt, e.UpdatedAt = now, now

	return translate("create work "+w.ID(),
		r.db.WithContext(ctx).Create(e).Error)
}

// FindWork 按 ID 查一条工作，不存在时返回 model.ErrNotFound。
//
// 用 First 而不是 Find：Find 在记录不存在时 err 是 nil，
// 会让「查不到」静默变成「查到了一个零值」。
func (r *WorkRepo) FindWork(ctx context.Context, id string) (*model.Work, error) {
	var e entity.Work
	err := r.db.WithContext(ctx).
		Select(workColumns).
		Where("id = ?", id).
		First(&e).Error
	if err != nil {
		return nil, translate("find work "+id, err)
	}
	return mapper.WorkToModel(&e), nil
}

// UpdateWork 更新一条工作的可变字段。
//
// 用 map 而不是 struct：GORM 的 Updates 传 struct 时会忽略零值字段，
// 状态机把状态改成看起来像零值的值时会静默丢更新。
func (r *WorkRepo) UpdateWork(ctx context.Context, w *model.Work) error {
	res := r.db.WithContext(ctx).
		Model(&entity.Work{}).
		Where("id = ?", w.ID()).
		Updates(map[string]any{
			"state":      string(w.State()),
			"updated_at": r.clk.Now(),
		})
	if res.Error != nil {
		return translate("update work "+w.ID(), res.Error)
	}
	if res.RowsAffected == 0 {
		// 没有行被更新意味着记录不存在——不报错的话调用方会以为更新成功了
		return translate("update work "+w.ID(), gorm.ErrRecordNotFound)
	}
	return nil
}

// ListWorks 列出全部工作。
//
// 空集合返回空切片而不是 nil：调用方 range 空切片是安全的，
// 而 nil 会让「没有」与「出错」难以区分。
func (r *WorkRepo) ListWorks(ctx context.Context) ([]*model.Work, error) {
	var es []entity.Work
	err := r.db.WithContext(ctx).
		Select(workColumns).
		Order("created_at DESC").
		Find(&es).Error
	if err != nil {
		return nil, translate("list works", err)
	}
	return mapper.WorksToModels(es), nil
}
