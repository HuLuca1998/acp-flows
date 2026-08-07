package port

import (
	"context"
	"errors"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// ErrPathNotFound 表示用户给的路径不存在、或不是目录。
//
// ★ 这是**契约的一部分**：GitProbe 的实现必须让 errors.Is 能判定出它来。
// 不区分的话，路径打错和探测崩了会显示成同一句话，
// 而用户能自己解决的只有前者。
var ErrPathNotFound = errors.New("port: project path not found")

// ProjectRepo 是项目的持久化抽象。
//
// R1 要求「重启 duetd 后项目仍在」——所以这不是内存里的一个 map。
type ProjectRepo interface {
	// SaveProject 新增或覆盖一条项目记录。
	SaveProject(ctx context.Context, p *model.Project) error
	// ListProjects 返回全部项目，按添加顺序。
	ListProjects(ctx context.Context) ([]*model.Project, error)
	// FindProjectByPath 按规整后的路径查，用来挡住重复添加。
	// 查不到时返回 model.ErrNotFound。
	FindProjectByPath(ctx context.Context, path string) (*model.Project, error)
	// DeleteProject 取消登记。**不删用户的任何文件。**
	DeleteProject(ctx context.Context, id string) error
}

// GitInfo 是对一个目录的探测结论。
type GitInfo struct {
	IsRepo        bool
	DefaultBranch string
}

// GitProbe 探测一个本地目录是不是 git 仓库。
//
// ★ 实现**必须只读**：用户刚把自己的仓库交给 Duet，
// 加进来这个动作让他的 `git status` 多出东西是最快失去信任的方式。
type GitProbe interface {
	// ProbeGit 探测目录。路径不存在或不是目录时返回错误；
	// **不是 git 仓库不算错误**，返回 IsRepo: false 即可。
	ProbeGit(ctx context.Context, path string) (GitInfo, error)
}
