package mapper

import (
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
	"github.com/HuLuca1998/acp-flows/backend/internal/store/entity"
)

// ProjectToEntity 把领域模型转成行结构。
//
// 时间戳由调用方（repo）用注入的 Clock 填，映射本身不取时间——
// 取了就没法写确定性测试。
func ProjectToEntity(p *model.Project) *entity.Project {
	return &entity.Project{
		ID:            p.ID(),
		Name:          p.Name(),
		Path:          p.Path(),
		IsGitRepo:     p.IsGitRepo(),
		DefaultBranch: p.DefaultBranch(),
	}
}

// ProjectToModel 把行结构转回领域模型。
//
// ★ 走 model.Restore 而不是 model.NewProject：后者会校验用户输入
// （路径必须绝对之类），而这里读的是已经存过的东西——
// 校验规则将来变严的话，用 NewProject 会让列表里的老项目突然读不出来。
func ProjectToModel(e *entity.Project) *model.Project {
	return model.Restore(e.ID, e.Name, e.Path, e.IsGitRepo, e.DefaultBranch)
}
