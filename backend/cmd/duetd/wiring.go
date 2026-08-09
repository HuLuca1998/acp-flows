package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/HuLuca1998/acp-flows/backend/internal/acp/protocol"
	"github.com/HuLuca1998/acp-flows/backend/internal/acp/session"
	"github.com/HuLuca1998/acp-flows/backend/internal/app/permission"
	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/internal/eventbus"
	"github.com/HuLuca1998/acp-flows/backend/internal/gitx"
	"github.com/HuLuca1998/acp-flows/backend/internal/store"
)

// 端口适配器：把具体实现接到 app 层定义的 port 接口上。
//
// ★ 与 main.go 分开，是因为它们变化的原因不同：main.go 管进程怎么起来
// （参数、路径、监听、优雅退出），这里管「谁实现了哪个 port」。
// 加一个新的基础设施只动这个文件。
//
// ★ cmd 是**唯一**能同时看见 acp 层与 app 层的地方——两边互不认识
// （depguard 挡着），装配只能在这里做。

// gitProbe 把 gitx 接到 app/port 上。
//
// cmd 是唯一做依赖装配的地方，所以这层薄适配器放这里——
// 让 gitx 直接返回 port 的类型会让基础设施依赖 app 的数据结构。
type gitProbe struct{}

func (gitProbe) ProbeGit(ctx context.Context, path string) (port.GitInfo, error) {
	info, err := gitx.Probe(ctx, path)
	if errors.Is(err, gitx.ErrNotADirectory) {
		// 基础设施的错误类型不许穿到 app/api——翻成契约里的哨兵，
		// 让界面能说出「这个文件夹找不到」而不是一句通用错误
		return port.GitInfo{}, fmt.Errorf("%w: %s", port.ErrPathNotFound, path)
	}
	return port.GitInfo{IsRepo: info.IsRepo, DefaultBranch: info.DefaultBranch}, err
}

// eventStore 把 store 的事件仓储接到 eventbus 与 api 上。
//
// ★ 为什么需要这层翻译：store.Event 与 eventbus.Event 字段完全一致，
// 但**具名结构体之间不能互相赋值**——Go 的结构化类型只对 interface 生效。
// 而 depguard 的 infra 规则不许基础设施之间互相 import（store 不能认识
// eventbus，反之亦然），所以接缝只能落在 cmd —— 唯一做装配的地方。
type eventStore struct{ repo *store.EventRepo }

func (s eventStore) AppendEvent(ctx context.Context, e *eventbus.Event) error {
	row := &store.Event{
		ID: e.ID, WorkID: e.WorkID, Source: e.Source,
		Type: e.Type, TS: e.TS, Payload: e.Payload,
		// ★★ 这几个**极易漏抄**——漏了的话事件照样落库、照样能读回来，
		// 只是角色标签没了，而所有单测都绿（它们不过这层）。
		// 有反射测试守着「两个结构体字段数一致」。
		Role: e.Role, RoleDisplayName: e.RoleDisplayName, Runtime: e.Runtime,
		RequirementVersion: e.RequirementVersion,
		RequirementFrozen:  e.RequirementFrozen,
	}
	if err := s.repo.AppendEvent(ctx, row); err != nil {
		return err
	}
	e.Seq = row.Seq // 序号由数据库发放，写回给调用方
	return nil
}

func (s eventStore) MaxSeq(ctx context.Context) (int64, error) {
	return s.repo.MaxSeq(ctx)
}

func (s eventStore) EventsAfter(ctx context.Context, after int64, limit int) ([]eventbus.Event, error) {
	rows, err := s.repo.EventsAfter(ctx, after, limit)
	if err != nil {
		return nil, err
	}
	out := make([]eventbus.Event, 0, len(rows))
	for _, r := range rows {
		out = append(out, eventbus.Event{
			ID: r.ID, Seq: r.Seq, WorkID: r.WorkID,
			Source: r.Source, Type: r.Type, TS: r.TS, Payload: r.Payload,
			// ★ 回来的方向同样别漏——用户重开应用看到的就是这条路。
			Role: r.Role, RoleDisplayName: r.RoleDisplayName, Runtime: r.Runtime,
			RequirementVersion: r.RequirementVersion,
			RequirementFrozen:  r.RequirementFrozen,
		})
	}
	return out, nil
}

// worktrees 把 gitx 的 worktree 操作接到 app/port 上。
//
// ★ root 是 `~/.acpflows/worktrees`——**用户项目之外**（open-questions Q30）。
// 这一层存在的意义就是把那个路径决定钉死在装配处，
// 不让 app 层自己去拼路径（拼错了就写进用户仓库了）。
type worktrees struct{ root string }

func (w worktrees) CreateWorktree(
	ctx context.Context, repo, workID, baseRef string,
) (port.Worktree, error) {
	wt, err := gitx.AddWorktree(ctx, gitx.WorktreeSpec{
		Repo: repo, Root: w.root, WorkID: workID,
		Branch: "duet/" + workID, BaseRef: baseRef,
	})
	if err != nil {
		return port.Worktree{}, err
	}
	return port.Worktree{Path: wt.Path, Branch: wt.Branch, BaseCommit: wt.BaseCommit}, nil
}

// repoStatus 实现 port.RepoStatusProbe。
type repoStatus struct{}

func (repoStatus) ProbeRepoStatus(ctx context.Context, path string) (port.RepoStatus, error) {
	st, err := gitx.ProbeStatus(ctx, path)
	if err != nil {
		return port.RepoStatus{}, err
	}
	return port.RepoStatus{
		CurrentBranch: st.CurrentBranch,
		Branches:      st.Branches,
		HeadCommit:    st.HeadCommit,
		TrackedDirty:  st.TrackedDirty,
		Untracked:     st.Untracked,
	}, nil
}

// WorktreePath 实现 port.WorktreeLocator：算出某个工作的工作区在哪。
//
// ★ 路径是**算出来的**，不查库：worktree 的位置由 root + workID 唯一确定
// （见 gitx.AddWorktree）。存一份的话，两处就会漂移——而漂移的表现是
// 「恢复到了一个不存在的目录」。
func (w worktrees) WorktreePath(_ context.Context, workID string) (string, error) {
	path := filepath.Join(w.root, workID)
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("工作区 %s 不可用: %w", path, err)
	}
	return path, nil
}

func (w worktrees) RemoveWorktree(ctx context.Context, repo, path string) error {
	return gitx.RemoveWorktree(ctx, repo, path)
}

// workBus 把 app 层的工作事件接到事件总线上。
//
// 两个 Event 类型字段一致但不能互相赋值（结构化类型只对 interface 生效），
// 而 depguard 不许 app 与 eventbus 互相依赖——接缝落在 cmd。
type workBus struct{ bus *eventbus.Bus }

func (b workBus) PublishWorkEvent(ctx context.Context, e port.WorkEvent) error {
	payload, err := json.Marshal(e.Payload)
	if err != nil {
		// 载荷编不出来不该让工作本身失败——事件是给界面看的
		payload = []byte("{}")
	}
	return b.bus.Publish(ctx, eventbus.Event{
		ID: "evt_" + e.WorkID, WorkID: e.WorkID,
		Source: e.Source, Type: e.Type, Payload: payload,
		// ★ 角色一路穿到界面：断在这里的话，前端只能按 Runtime 名猜，
		// 而一个 Runtime 承担多个角色时那两个角色会长得一模一样。
		Role: e.Role, RoleDisplayName: e.RoleDisplayName, Runtime: e.Runtime,
		// ★ 需求版本同理：断在这里的话，用户看不出「说这句话时需求是第几版」。
		RequirementVersion: e.RequirementVersion,
		RequirementFrozen:  e.RequirementFrozen,
	})
}

// toBrokerOptions 把协议层的选项转成 Broker 的形状。
//
// ★ optionId 与 name **一个字符都不改**：id 是 Agent 定义的不透明字符串，
// name 是它给的按钮文字。任何「顺手规整」都可能把用户的「拒绝」变成「允许」。
func toBrokerOptions(in []protocol.PermissionOption) []permission.Option {
	out := make([]permission.Option, 0, len(in))
	for _, o := range in {
		out = append(out, permission.Option{
			OptionID: o.OptionID, Name: o.Name, Kind: string(o.Kind),
		})
	}
	return out
}

// runtimeNameOf 取「是哪个 Agent 在问」。
//
// 会话层不带这个信息（它只认识一条连接），暂时用一个中性值——
// 界面上宁可说「AI」，也不要写死 claude 或 codex（上层不许出现品牌名，
// 见 check-naming 第 10 节）。真正的名字等 U4.x 的多 Runtime 并行再接。
func runtimeNameOf(_ session.PermissionAsk) string { return "AI" }

func (repoStatus) ProbeWorktreeState(
	ctx context.Context, path, base string,
) (port.WorktreeState, error) {
	st, err := gitx.ProbeWorktree(ctx, path, base)
	if err != nil {
		return port.WorktreeState{}, err
	}
	out := port.WorktreeState{
		Branch: st.Branch, BaseCommit: st.BaseCommit, Ahead: st.Ahead,
		Changes: make([]port.FileChange, 0, len(st.Changes)),
		Commits: make([]port.CommitInfo, 0, len(st.Commits)),
	}
	for _, c := range st.Changes {
		out.Changes = append(out.Changes, port.FileChange{
			Path: c.Path, Added: c.Added, Removed: c.Removed,
		})
	}
	for _, c := range st.Commits {
		out.Commits = append(out.Commits, port.CommitInfo{
			SHA: c.SHA, Subject: c.Subject, When: c.When,
		})
	}
	return out, nil
}
