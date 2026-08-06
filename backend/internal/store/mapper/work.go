// Package mapper 在领域模型与 GORM 实体之间双向映射。
//
// 它是两个世界的唯一接触点：领域模型不认识 gorm，实体不认识业务规则。
package mapper

import (
	"github.com/HuLuca1998/acp-flows/backend/internal/constant"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
	"github.com/HuLuca1998/acp-flows/backend/internal/store/entity"
)

// WorkToEntity 把领域模型转成行结构。
//
// 时间戳由调用方（repo）用注入的 Clock 填，映射本身不取时间——
// 取了就没法写确定性测试。
func WorkToEntity(w *model.Work) *entity.Work {
	return &entity.Work{
		ID:    w.ID(),
		State: string(w.State()),
	}
}

// WorkToModel 把行结构转回领域模型。
//
// 用 NewWorkAt 而不是 NewWork：从数据库重建聚合时状态是既定事实，
// 不该再走一遍初始状态。
func WorkToModel(e *entity.Work) *model.Work {
	return model.NewWorkAt(e.ID, constant.WorkState(e.State))
}

// WorksToModels 批量转换。空输入返回空切片而不是 nil——
// 调用方 range 空切片是安全的，而 nil 会让「没有」与「出错」难以区分。
func WorksToModels(es []entity.Work) []*model.Work {
	out := make([]*model.Work, 0, len(es))
	for i := range es {
		out = append(out, WorkToModel(&es[i]))
	}
	return out
}
