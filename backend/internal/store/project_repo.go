package store

import (
	"context"
	"strconv"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
	"github.com/HuLuca1998/acp-flows/backend/internal/store/entity"
	"github.com/HuLuca1998/acp-flows/backend/internal/store/mapper"
)

// ProjectRepo 是 Project 的持久化实现。
//
// 它实现 app/port 里的 ProjectRepo 接口——Go 是结构化类型，
// 不需要 import 那个包。
type ProjectRepo struct {
	db  *gorm.DB
	clk Clock
}

// 查询时显式列出列，不用 SELECT *：加列时不会静默改变返回结构。
const projectColumns = "id, name, path, is_git_repo, default_branch, created_at, updated_at"

// SaveProject 新增或更新一条项目记录。
//
// 用 upsert 而不是「先查再决定 insert/update」：后者在两个请求同时进来时
// 会双双查到「不存在」，然后一起插入。
func (r *ProjectRepo) SaveProject(ctx context.Context, p *model.Project) error {
	e := mapper.ProjectToEntity(p)
	now := r.clk.Now()
	e.CreatedAt, e.UpdatedAt = now, now

	// ★ 冲突只按主键处理。**path 的唯一冲突要如实报错**——
	// 把它也 upsert 掉的话，「用另一个 ID 添加同一个路径」会静默改掉已有记录，
	// 而调用方以为自己新建了一条。
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "path", "is_git_repo", "default_branch", "updated_at"}),
		}).
		Create(e).Error

	return translate("save project "+p.ID(), err)
}

// ListProjects 返回全部项目，**按添加顺序**。
//
// 不按 ID 或 path 排序：顺序每次不一样的话，用户会觉得列表在自己乱跳。
func (r *ProjectRepo) ListProjects(ctx context.Context) ([]*model.Project, error) {
	var rows []entity.Project
	if err := r.db.WithContext(ctx).
		Select(projectColumns).
		Order("created_at, id").
		Find(&rows).Error; err != nil {
		return nil, translate("list projects", err)
	}

	// ★ 空结果返回空切片而不是 nil：api 层要序列化成 `[]` 而不是 `null`，
	// 前端对 null 调 .map() 会白屏。
	out := make([]*model.Project, 0, len(rows))
	for i := range rows {
		out = append(out, mapper.ProjectToModel(&rows[i]))
	}
	return out, nil
}

// FindProjectByPath 按规整后的路径查，用来挡住重复添加。
//
// 用 First 而不是 Find：Find 在记录不存在时 err 是 nil，
// 会让「查不到」静默变成「查到了一个零值」——那样重复添加检查就形同虚设。
func (r *ProjectRepo) FindProjectByPath(ctx context.Context, path string) (*model.Project, error) {
	var e entity.Project
	if err := r.db.WithContext(ctx).
		Select(projectColumns).
		Where("path = ?", path).
		First(&e).Error; err != nil {
		return nil, translate("find project by path "+path, err)
	}
	return mapper.ProjectToModel(&e), nil
}

// DeleteProject 取消登记。**不删用户的任何文件。**
func (r *ProjectRepo) DeleteProject(ctx context.Context, id string) error {
	res := r.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&entity.Project{})
	if res.Error != nil {
		return translate("delete project "+id, res.Error)
	}
	if res.RowsAffected == 0 {
		// 删一个不存在的要报错。静默成功的话界面会显示「已移除」，
		// 而用户下次打开发现它还在。
		return translate("delete project "+id, gorm.ErrRecordNotFound)
	}
	return nil
}

// MaxProjectSeq 返回已有项目 ID 里的最大序号，供启动时预热 IDGen。
//
// ★ **不预热的话，重启后第一次添加项目就会撞主键。** IDGen 的序号在内存里，
// 进程一重启就归零，于是一个已经有 proj-01 的库会再发一次 proj-01。
//
// 这个坑在开发机上几乎撞不到——数据库总是空的；它要等用户用了一阵子、
// 重启一次应用才炸，那时现场离原因已经很远。
//
// 解析不出来的 ID 一律跳过（手工改过、或将来 ID 格式变了）：
// 宁可序号往后多跳几个，也不能让 duetd 起不来。
func (r *ProjectRepo) MaxProjectSeq(ctx context.Context) (int, error) {
	var ids []string
	if err := r.db.WithContext(ctx).
		Model(&entity.Project{}).
		Pluck("id", &ids).Error; err != nil {
		return 0, translate("max project seq", err)
	}

	maxSeq := 0
	for _, id := range ids {
		_, digits, ok := strings.Cut(id, "-")
		if !ok {
			continue
		}
		n, err := strconv.Atoi(digits)
		if err != nil {
			continue
		}
		maxSeq = max(maxSeq, n)
	}
	return maxSeq, nil
}
