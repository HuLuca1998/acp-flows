package port

import (
	"context"
	"errors"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// WorkRepo 是工作的持久化抽象。
type WorkRepo interface {
	SaveWork(ctx context.Context, w *model.Work) error
	ListWorks(ctx context.Context) ([]*model.Work, error)
	// FindWork 查不到时返回 model.ErrNotFound。
	FindWork(ctx context.Context, id string) (*model.Work, error)
}

// Requirements 是需求快照的持久化抽象。
//
// ★★ **没有 Update，也没有 Delete**（INV-REQ-2）：对外只有「存一版」和「读」。
// 已冻结的版本改不动，要改就存一条新版本。
type Requirements interface {
	SaveRequirement(ctx context.Context, req *model.RequirementSnapshot) error
	// LatestRequirement 查不到时返回 model.ErrNotFound。
	LatestRequirement(ctx context.Context, workID string) (*model.RequirementSnapshot, error)
	RequirementVersions(ctx context.Context, workID string) ([]*model.RequirementSnapshot, error)
}

// Plans 是计划版本的持久化抽象。
//
// ★★ **没有 Update，也没有 Delete**（INV-PLAN-4）：计划改了就存新版本。
type Plans interface {
	SavePlan(ctx context.Context, workID string, v model.PlanVersion) error
	// LatestPlan 查不到时返回 model.ErrNotFound。
	LatestPlan(ctx context.Context, workID string) (model.PlanVersion, error)
	PlanVersions(ctx context.Context, workID string) ([]model.PlanVersion, error)
}

// Contracts 是单元契约的持久化抽象。
//
// ★★ **没有 Update，也没有 Delete**（INV-UC-2）：契约冻结后改不动。
type Contracts interface {
	SaveContract(ctx context.Context, c *model.UnitContract) error
	// LatestContract 查不到时返回 model.ErrNotFound。
	LatestContract(ctx context.Context, unitID string) (*model.UnitContract, error)
	ContractVersions(ctx context.Context, unitID string) ([]*model.UnitContract, error)
}

// Evidence 是证据的持久化抽象。
//
// ★★ **没有 Update，也没有 Delete**：证据被改写过就不再是证据了——
// 「当时到底跑出了什么」没有第二个地方可查。
type Evidence interface {
	SaveEvidence(ctx context.Context, workID string, e model.Evidence) error
	EvidenceOf(ctx context.Context, workID, unitID string) ([]model.Evidence, error)
}

// Committer 把工作区里的改动提交到工作分支。
//
// ★ 定义成 port 而不是直接调 gitx：app 层不许 import 基础设施
// （depguard 挡着）。
type Committer interface {
	// Commit 提交全部改动，返回短 sha。没有改动时返回错误——
	// ★★ **不造空提交**：一个「验收通过」却什么都没改的单元，
	// 说明该被质疑的是那次验收。
	Commit(ctx context.Context, path, message string) (string, error)
}

// Decisions 是决策的持久化抽象。
//
// ★★ **没有 Delete**，Update 只有一条路：作答（且只能一次）。
type Decisions interface {
	SaveDecision(ctx context.Context, d *model.Decision) error
	// FindDecision 查不到时返回 model.ErrNotFound。
	FindDecision(ctx context.Context, id string) (*model.Decision, error)
	PendingDecisions(ctx context.Context, workID string) ([]*model.Decision, error)
}

// Worktrees 管理每个工作的独立工作区。
//
// ★ 实现必须把工作区建在**用户项目之外**（`~/.acpflows/worktrees`，
// 见 open-questions.md Q30）。用户把代码目录交给 Duet 时，
// 并没有同意我们在他的仓库里造一堆分支和目录。
type Worktrees interface {
	// CreateWorktree 返回工作区路径。同一个 workID 重复调用返回同一个路径。
	//
	// ★ baseRef 是用户选的基线（分支名或 commit），留空时用仓库当前 HEAD。
	// 他可能想从 `develop` 开工，而当前分支上正躺着他没提交完的东西。
	CreateWorktree(ctx context.Context, repo, workID, baseRef string) (Worktree, error)
	// RemoveWorktree 移除工作区。移除不存在的不报错。
	RemoveWorktree(ctx context.Context, repo, path string) error
}

// Worktree 是一个建好的工作区。
type Worktree struct {
	Path   string
	Branch string
	// BaseCommit 是创建时的基线。★ 记下来才能回答「这个工作从哪儿开始的」——
	// 右栏的「领先几个 commit」与验收 diff 都要它当起点。
	BaseCommit string
}

// RepoStatus 是开工前的仓库状态。
type RepoStatus struct {
	CurrentBranch string
	Branches      []string
	HeadCommit    string
	// ★ 已跟踪与未跟踪**分开数**：合成一条的话，
	// 「新建了几个还没 add 的文件」和「改了正在跟踪的代码」长得一模一样。
	TrackedDirty int
	Untracked    int
}

// RepoStatusProbe 探测开工前的仓库状态。**只读。**
type RepoStatusProbe interface {
	ProbeRepoStatus(ctx context.Context, path string) (RepoStatus, error)
	// ProbeWorktreeState 读出一个工作区的现场。
	//
	// ★ base 为空时**不报** ahead 与 commits——不知道基线就算不出
	// 「本次工作改了什么」，而编一个 0 出来比不报更糟。
	ProbeWorktreeState(ctx context.Context, path, base string) (WorktreeState, error)
}

// WorktreeState 是一个工作区的 git 现场，右栏照它渲染。
type WorktreeState struct {
	Branch     string
	BaseCommit string
	Ahead      int
	Changes    []FileChange
	Commits    []CommitInfo
}

// FileChange 是一个文件的改动。
type FileChange struct {
	Path    string
	Added   int
	Removed int
}

// CommitInfo 是一条 commit。
type CommitInfo struct {
	SHA     string
	Subject string
	// When 是相对时间原文（`2 minutes ago`），由 git 算。
	When string
}

// WorkEvent 是一条与工作相关的事件，发给界面用。
type WorkEvent struct {
	WorkID string
	// Source 取值 acp | app，与 api/openapi.yaml 的 Event.source 一致。
	Source string
	// Type 是 14 类之一，见契约的 Event.type。
	Type string

	// Role / RoleDisplayName / Runtime 说明**这一条是谁说的**。
	//
	// ★★ 由后端填，**前端不许按 Runtime 名猜**：一个 Runtime 可以承担
	// 多个角色（`claude` 同时是需求分析师和审查员），按名字猜的话
	// 界面上两个角色会长得一模一样——而用户正是靠这个标签判断
	// 「现在是谁在说话、他能不能动我的文件」。
	//
	// ★ 三个都可能为空：应用自己发的事件（state_change、checkpoint）
	// 没有角色。**空就是空**，别填一个「系统」上去——那会让用户
	// 以为有个叫「系统」的角色在干活。
	Role            string
	RoleDisplayName string
	Runtime         string

	// RequirementVersion / RequirementFrozen 是**说这句话的时候**需求的样子。
	//
	// ★ 设计稿把它画成角色名旁边的一枚等宽小标签（`requirement v2 已冻结`）。
	// 前端另查一次的话，拿到的是「现在」的版本，而用户看的是一条历史消息——
	// 他会以为当时就已经是 v3 了。
	//
	// 0 表示这个工作还没有需求快照。
	RequirementVersion int
	RequirementFrozen  bool

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
	// RoleID 是这一轮由哪个角色执行（`implementer` / `unit_reviewer` …）。
	//
	// ★★ **它决定这条会话有多大权限**：角色带着语义档位，
	// 开会话时会翻译成那一端的档名发过去收权。
	//
	// ★ 留空时按**实现工程师**处理（受控写）。不是「不收权」——
	// 不收权的表现是 codex 跑在 workspace-write 沙箱里，
	// 沙箱内的写操作连审批都不触发（acp-field-notes.md §3 实测）。
	// 装配漏了一根线的表现必须是「权限最小」，不能是「什么都放行」。
	RoleID string
	// RequirementVersion / RequirementFrozen 是**说这句话的时候**需求的样子。
	//
	// ★★ 跟着这一轮走，盖在它产出的每一条事件上——与 RoleID 同理：
	// 界面另查一次的话，拿到的是「现在」的版本，而用户看的是一条历史消息，
	// 他会以为当时就已经是 v3 了。
	//
	// 0 表示这个工作还没有需求快照。
	RequirementVersion int
	RequirementFrozen  bool
	// OnReply 在这一轮结束时收到 Agent 说过的**全部文本**（拼好的）。
	//
	// ★★ 有它才谈得上「把 AI 的回复变成结构化的计划」——事件流是给界面看的，
	// 而 app 层要从同一段文本里把 JSON 抠出来。让 app 层自己去订阅事件总线
	// 的话，它得知道「哪几条属于这一轮」，而那个边界只有 acp 层清楚。
	//
	// ★ 可以为 nil：大多数轮次不需要回读自己说了什么。
	OnReply func(reply string)
	// SystemPrompt 是拼在这条会话最前面的那段话（角色的职责/性格/边界/产出）。
	//
	// ★★ **不拼的话，需求分析师和实现工程师收到的是一模一样的一句需求**——
	// 角色库里那八张卡片就只是界面上的装饰，而用户以为自己在跟一个
	// 「追问式、不放过『大概』」的角色说话。
	SystemPrompt string
}

// ErrTurnQueueFull 表示这条会话上排队的轮次已经到上限。
//
// ★★ 定义在 port 而不是 acp：app 层**不许 import acp**（depguard 挡着），
// 而这件事必须让它分得出来——排队满**不是「AI 跑挂了」**，
// 把工作推到 failed 的话，用户只是手快点了几下就得重开一个工作。
var ErrTurnQueueFull = errors.New("port: 这条会话上排队的消息太多了")

// ErrTurnAbandoned 表示这一轮在排队时被取消了。
//
// ★ 同样不是失败：用户点了停，排在后面的那几句放弃是**预期行为**。
var ErrTurnAbandoned = errors.New("port: 这一轮在排队时被放弃")

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
	// ReleaseWork 放掉这个工作的**常驻会话**。
	//
	// ★★ 工作停下来时必须调（Q42）：常驻会话是为了让 AI 记得上文，
	// 而一个 paused 的工作不需要那个——留着的话它会一直占着
	// 两三个 Agent 进程，用户以为它已经停了。
	ReleaseWork(ctx context.Context, workID string)
}
