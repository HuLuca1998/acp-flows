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

// AgentTurn 是要跑的一轮对话。
type AgentTurn struct {
	WorkID string
	// Cwd 是 Agent 的工作目录，**必须是工作自己的 worktree**。
	//
	// ★ 传用户的仓库路径就等于让 AI 直接在他的分支上改文件——
	// 他把代码目录交给 Duet 时并没有同意这件事（open-questions.md Q30）。
	Cwd string
	// Prompt 是用户提的需求。
	Prompt string
}

// AgentRunner 拉起一个 Agent 跑一轮对话，把它说的话发到事件总线。
//
// ★ 实现**阻塞到这一轮结束**（可能好几分钟）。调用方负责另起 goroutine，
// 别把它挂在 HTTP 请求上——请求一返回 ctx 就被取消，AI 说到一半会被砍掉。
type AgentRunner interface {
	RunTurn(ctx context.Context, t AgentTurn) error
}

// WorktreeLocator 找出某个工作的工作区在哪。
//
// ★ 与 Worktrees 分开：那个负责创建/移除，这个只负责「在哪」。
// 恢复流程只需要后者——它不该有能力删掉用户的工作区。
type WorktreeLocator interface {
	WorktreePath(ctx context.Context, workID string) (string, error)
}

// AgentCanceller 停掉某个工作正在跑的那一轮。
//
// ★ 用返回值而不是哨兵错误传递「要不要杀」：app 层不许 import acp
// （depguard 挡着），拿不到那边的 error 类型。而这件事太重要，
// 不能靠调用方去猜——漏了的后果是「界面说已取消、后台还在改文件」。
type AgentCanceller interface {
	// CancelTurn 取消这个工作正在跑的那一轮。
	//
	// mustKill 为 true 表示 Agent 没在上限内收尾，**调用方必须杀掉它**。
	CancelTurn(ctx context.Context, workID string) (mustKill bool, err error)
	// KillAgent 杀掉这个工作的 Agent 进程（连同它的整个进程组）。
	KillAgent(workID string)
}
