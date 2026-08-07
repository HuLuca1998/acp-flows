package port

import (
	"context"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// WorkRepo 是工作的持久化抽象。
type WorkRepo interface {
	SaveWork(ctx context.Context, w *model.Work) error
	ListWorks(ctx context.Context) ([]*model.Work, error)
	// FindWork 查不到时返回 model.ErrNotFound。
	FindWork(ctx context.Context, id string) (*model.Work, error)
}

// Worktrees 管理每个工作的独立工作区。
//
// ★ 实现必须把工作区建在**用户项目之外**（`~/.acpflows/worktrees`，
// 见 open-questions.md Q30）。用户把代码目录交给 Duet 时，
// 并没有同意我们在他的仓库里造一堆分支和目录。
type Worktrees interface {
	// CreateWorktree 返回工作区路径。同一个 workID 重复调用返回同一个路径。
	CreateWorktree(ctx context.Context, repo, workID string) (string, error)
	// RemoveWorktree 移除工作区。移除不存在的不报错。
	RemoveWorktree(ctx context.Context, repo, path string) error
}

// WorkEvent 是一条与工作相关的事件，发给界面用。
type WorkEvent struct {
	WorkID string
	// Source 取值 acp | app，与 api/openapi.yaml 的 Event.source 一致。
	Source string
	// Type 是 13 类之一，见契约的 Event.type。
	Type    string
	Payload map[string]any
}

// WorkEventBus 把工作的进展推给界面。
type WorkEventBus interface {
	PublishWorkEvent(ctx context.Context, e WorkEvent) error
}
