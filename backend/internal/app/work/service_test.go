package work_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/internal/app/work"
	"github.com/HuLuca1998/acp-flows/backend/internal/constant"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
	"github.com/HuLuca1998/acp-flows/backend/internal/gitx"
	"github.com/HuLuca1998/acp-flows/backend/tests/testutil"
)

// U2.4.1 · 新建工作（验收点 V5 + V6）
//
// ★ worktree 用**真 gitx 实现**，不塞假的：本单元最要紧的一条是
// 「不往用户项目里写」，而假实现什么都不写——那条断言会永远绿。

// memWorks 是内存仓储。
//
// ★ **它必须记下「每次保存时的状态」**，而不只是存指针。
// 存指针的话，`w.Transition(...)` 改的是同一个对象——即使代码忘了调
// SaveWork，仓储里那个指针指向的对象状态也已经变了，于是
// 「忘了落库」这类 bug 完全测不出来。真实的 store 会序列化，没有这个语义。
//
// 造负例（删掉失败分支里的 SaveWork）时它两次都没红，才发现这个洞。
type memWorks struct {
	mu    sync.Mutex
	items []*model.Work
	// savedStates 是每次 SaveWork 时的状态快照，按调用顺序
	savedStates map[string][]constant.WorkState
}

func (r *memWorks) SaveWork(_ context.Context, w *model.Work) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.savedStates == nil {
		r.savedStates = map[string][]constant.WorkState{}
	}
	r.savedStates[w.ID()] = append(r.savedStates[w.ID()], w.State())

	for i, existing := range r.items {
		if existing.ID() == w.ID() {
			r.items[i] = w
			return nil
		}
	}
	r.items = append(r.items, w)
	return nil
}

func (r *memWorks) ListWorks(context.Context) ([]*model.Work, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]*model.Work(nil), r.items...), nil
}

func (r *memWorks) FindWork(_ context.Context, id string) (*model.Work, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, w := range r.items {
		if w.ID() == id {
			return w, nil
		}
	}
	return nil, model.ErrNotFound
}

// recordingBus 记下发布过的事件。
type recordingBus struct {
	mu     sync.Mutex
	events []port.WorkEvent
}

func (b *recordingBus) PublishWorkEvent(_ context.Context, e port.WorkEvent) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, e)
	return nil
}

func (b *recordingBus) types() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]string, 0, len(b.events))
	for _, e := range b.events {
		out = append(out, e.Type)
	}
	return out
}

// realWorktrees 用真 gitx——本单元的第一条禁令就是「不往用户项目里写」。
type realWorktrees struct{ root string }

func (w realWorktrees) CreateWorktree(ctx context.Context, repo, workID string) (string, error) {
	wt, err := gitx.AddWorktree(ctx, gitx.WorktreeSpec{
		Repo: repo, Root: w.root, WorkID: workID, Branch: "duet/" + workID,
	})
	return wt.Path, err
}

func (w realWorktrees) RemoveWorktree(ctx context.Context, repo, path string) error {
	return gitx.RemoveWorktree(ctx, repo, path)
}

type seqIDs struct {
	mu sync.Mutex
	n  int
}

func (g *seqIDs) NextID(prefix string) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.n++
	return prefix + "-0" + string(rune('0'+g.n))
}
func (g *seqIDs) NextULID() string { return g.NextID("ulid") }

func newService(t *testing.T, repo *memWorks, bus *recordingBus) *work.Service {
	t.Helper()
	return work.New(repo, realWorktrees{root: t.TempDir()}, bus, &seqIDs{})
}

// ★★ R2：建工作**不往用户项目里写一个字节**。
func TestStart_WritesNothingIntoUserProject(t *testing.T) {
	project := testutil.NewGitRepo(t)
	before := testutil.SnapshotDir(t, project)

	svc := newService(t, &memWorks{}, &recordingBus{})
	_, err := svc.Start(context.Background(), project, "帮我加个功能")
	if err != nil {
		t.Fatalf("建工作失败: %v", err)
	}

	testutil.AssertUnchanged(t, project, before)
}

// ★ R1：两个工作有各自的 worktree，互不干扰。
func TestStart_EachWorkGetsItsOwnWorktree(t *testing.T) {
	project := testutil.NewGitRepo(t)
	svc := newService(t, &memWorks{}, &recordingBus{})
	ctx := context.Background()

	a, err := svc.Start(ctx, project, "第一件事")
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.Start(ctx, project, "第二件事")
	if err != nil {
		t.Fatal(err)
	}

	if a.Worktree == b.Worktree {
		t.Fatal("两个工作共用一个目录——两个 AI 会同时改同一份文件")
	}

	// 在 a 的工作区建个文件，b 里看不见
	if err := os.WriteFile(filepath.Join(a.Worktree, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(b.Worktree, "a.txt")); err == nil {
		t.Error("工作之间的文件改动互相可见")
	}
}

// 新工作从 initializing 起步，worktree 切好之后进 clarifying。
//
// ★ 状态不能一步跳到 clarifying：worktree 可能切失败，
// 那时用户要看到「初始化失败」而不是「正在澄清需求」——后者会让他
// 对着一个永远等不到回应的界面干等。
func TestStart_TransitionsThroughInitializing(t *testing.T) {
	project := testutil.NewGitRepo(t)
	bus := &recordingBus{}
	svc := newService(t, &memWorks{}, bus)

	w, err := svc.Start(context.Background(), project, "做点事")
	if err != nil {
		t.Fatal(err)
	}

	if w.State != constant.WorkStateClarifying {
		t.Errorf("State = %q, 想要 clarifying", w.State)
	}

	// 状态变化要发事件，界面才知道该刷新
	types := bus.types()
	if len(types) == 0 {
		t.Fatal("一个事件都没发——界面不会知道有新工作")
	}
	if types[0] != "state_change" {
		t.Errorf("第一个事件是 %q，想要 state_change", types[0])
	}
}

// ★ worktree 切失败时进 initializing_failed（**终态，不可恢复**），
// 并且**不留下半个工作**让用户去猜。
func TestStart_WorktreeFailureIsTerminal(t *testing.T) {
	notARepo := t.TempDir() // 不是 git 仓库
	repo := &memWorks{}
	svc := newService(t, repo, &recordingBus{})

	_, err := svc.Start(context.Background(), notARepo, "做点事")
	if err == nil {
		t.Fatal("非 git 目录却建成功了")
	}
	if !errors.Is(err, gitx.ErrNotARepo) {
		t.Errorf("err = %v，想要能判定成 ErrNotARepo——上层要据此提示用户先 git init", err)
	}

	// 失败的工作也要落库：用户看得到「这次没起来」，而不是点了之后什么都没发生
	works, err := repo.ListWorks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(works) != 1 {
		t.Fatalf("失败后落了 %d 条记录，想要 1 条（用户要看得到这次没起来）", len(works))
	}
	// ★ 断言**最后一次落库时的状态**是 initializing_failed。
	//
	// 不能只看 works[0].State()：内存仓储存的是指针，Transition 改的是同一个
	// 对象——即使代码忘了调 SaveWork，那个断言照样绿。
	// 造负例（删掉失败分支里的 SaveWork）时两次都没红，才发现这个洞。
	states := repo.savedStates[works[0].ID()]
	if len(states) == 0 {
		t.Fatal("一次都没落库")
	}
	if last := states[len(states)-1]; last != constant.WorkStateInitializingFailed {
		t.Errorf("最后一次落库的 State = %q, 想要 initializing_failed——"+
			"停在 initializing 的话，用户看到的是一个永远「正在初始化」的条目", last)
	}
}

// 相对路径在建工作之前就该被拒——它会在 duetd 的工作目录下解析。
func TestStart_RejectsRelativePath(t *testing.T) {
	svc := newService(t, &memWorks{}, &recordingBus{})

	if _, err := svc.Start(context.Background(), "work/app", "做点事"); err == nil {
		t.Error("相对路径却建成功了")
	}
}

// 空需求被拒：没有需求的工作没有意义，而它会占着一个 worktree。
func TestStart_RejectsEmptyPrompt(t *testing.T) {
	project := testutil.NewGitRepo(t)
	svc := newService(t, &memWorks{}, &recordingBus{})

	for _, blank := range []string{"", "   ", "\t\n"} {
		if _, err := svc.Start(context.Background(), project, blank); err == nil {
			t.Errorf("空需求 %q 却建成功了", blank)
		}
	}
}

// R6：列出工作时按创建顺序，重启后仍在（落库由 store 层测）。
func TestList_ReturnsAll(t *testing.T) {
	project := testutil.NewGitRepo(t)
	svc := newService(t, &memWorks{}, &recordingBus{})
	ctx := context.Background()

	for range 3 {
		if _, err := svc.Start(ctx, project, "做点事"); err != nil {
			t.Fatal(err)
		}
	}

	got, err := svc.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Errorf("列出 %d 个工作，想要 3 个", len(got))
	}
}
