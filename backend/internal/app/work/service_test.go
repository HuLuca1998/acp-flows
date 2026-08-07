package work_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

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

// FindWork 返回**重建出来的新对象**，不是仓储里那个指针。
//
// ★ 真 store 每次都经 mapper.WorkToModel 重建（行 → 模型），
// 直接交出指针的话这个替身比真实现「更共享」：后台 goroutine 改它，
// 请求 goroutine 读它，测出来的竞态在生产里根本不存在——
// 假警报和漏报一样有害。
func (r *memWorks) FindWork(_ context.Context, id string) (*model.Work, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, w := range r.items {
		if w.ID() == id {
			return model.NewWorkAt(w.ID(), w.State()), nil
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

func (b *recordingBus) snapshot() []port.WorkEvent {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]port.WorkEvent(nil), b.events...)
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
	return work.New(repo, realWorktrees{root: t.TempDir()}, bus, &seqIDs{}, nil)
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

// ── U2.4.1 · 工作建好之后，AI 得真的开口 ────────────────────────

// fakeRunner 记下被要求跑的每一轮，并可以模拟「跑很久」和「跑挂了」。
type fakeRunner struct {
	mu    sync.Mutex
	turns []port.AgentTurn
	// ctxErr 记下每一轮**跑完时**传进来的 ctx 状态。
	ctxErr []error

	delay time.Duration
	err   error
	done  chan struct{}
}

func (r *fakeRunner) RunTurn(ctx context.Context, t port.AgentTurn) error {
	if r.delay > 0 {
		time.Sleep(r.delay)
	}
	r.mu.Lock()
	r.turns = append(r.turns, t)
	r.ctxErr = append(r.ctxErr, ctx.Err())
	r.mu.Unlock()
	if r.done != nil {
		close(r.done)
	}
	return r.err
}

func (r *fakeRunner) snapshot() []port.AgentTurn {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]port.AgentTurn(nil), r.turns...)
}

func newServiceWithRunner(t *testing.T, repo *memWorks, bus *recordingBus, runner port.AgentRunner) *work.Service {
	t.Helper()
	return work.New(repo, realWorktrees{root: t.TempDir()}, bus, &seqIDs{}, runner)
}

// waitFor 等一个条件成立，超时就让测试红。
//
// 后台那一轮是异步的，直接断言会稳定地在它跑起来之前就查。
func waitFor(t *testing.T, why string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("等超时了：%s", why)
}

// ★★ 建好工作之后要**真的把需求送给 AI**，而且送到 worktree 里去跑。
//
// 不送的话，用户提了需求、界面上建出一个条目，然后就再也没有下文了——
// 这正是 V5 之前的样子。
// cwd 传成用户的仓库路径同样不行：那等于让 AI 直接在他的分支上改文件。
func TestStart_RunsTurnInWorktree(t *testing.T) {
	project := testutil.NewGitRepo(t)
	runner := &fakeRunner{done: make(chan struct{})}

	view, err := newServiceWithRunner(t, &memWorks{}, &recordingBus{}, runner).
		Start(context.Background(), project, "帮我加个功能")
	if err != nil {
		t.Fatalf("建工作失败: %v", err)
	}

	waitFor(t, "AI 那一轮一直没跑起来——用户提了需求却没有下文", func() bool {
		return len(runner.snapshot()) == 1
	})

	turn := runner.snapshot()[0]
	if turn.Cwd != view.Worktree {
		t.Errorf("cwd = %q, 想要 worktree %q——传用户仓库等于让 AI 直接改他的分支",
			turn.Cwd, view.Worktree)
	}
	if turn.Cwd == project {
		t.Error("cwd 就是用户的项目目录，AI 会直接在他的分支上改文件")
	}
	if turn.Prompt != "帮我加个功能" {
		t.Errorf("prompt = %q, 用户提的需求丢了", turn.Prompt)
	}
	if turn.WorkID != view.ID {
		t.Errorf("work_id = %q, 想要 %q——不带对的话前端过滤不出这一轮的事件",
			turn.WorkID, view.ID)
	}
}

// ★★ 这一轮**不能挂在请求的 ctx 上**。
//
// HTTP 处理函数一返回，请求的 ctx 就被取消。挂在上面的话，AI 刚说两句
// 就被砍掉——而用户看到的是时间线停在半截，没有任何报错。
func TestStart_TurnSurvivesRequestCancel(t *testing.T) {
	project := testutil.NewGitRepo(t)
	runner := &fakeRunner{delay: 50 * time.Millisecond, done: make(chan struct{})}

	ctx, cancel := context.WithCancel(context.Background())
	if _, err := newServiceWithRunner(t, &memWorks{}, &recordingBus{}, runner).
		Start(ctx, project, "帮我加个功能"); err != nil {
		t.Fatalf("建工作失败: %v", err)
	}
	// 模拟 HTTP 处理函数返回：请求的 ctx 立刻被取消
	cancel()

	select {
	case <-runner.done:
	case <-time.After(3 * time.Second):
		t.Fatal("请求取消后这一轮就没跑完——AI 说到一半被砍掉")
	}

	runner.mu.Lock()
	defer runner.mu.Unlock()
	if err := runner.ctxErr[0]; err != nil {
		t.Errorf("这一轮跑完时 ctx 已经是 %v——它挂在请求的 ctx 上，"+
			"HTTP 一返回 AI 就被砍掉", err)
	}
}

// ★ AI 那一轮跑挂了要**说出来**。
//
// 静默的话，用户看到工作停在「正在澄清需求」，永远等不到下一句，
// 而真正的原因（claude 没装、没登录）没人告诉他。
func TestStart_ReportsTurnFailure(t *testing.T) {
	project := testutil.NewGitRepo(t)
	bus := &recordingBus{}
	runner := &fakeRunner{err: errors.New("claude: 未登录"), done: make(chan struct{})}

	if _, err := newServiceWithRunner(t, &memWorks{}, bus, runner).
		Start(context.Background(), project, "帮我加个功能"); err != nil {
		t.Fatalf("建工作失败: %v", err)
	}
	<-runner.done

	waitFor(t, "AI 跑挂了却没有任何事件——用户会对着「正在澄清需求」干等", func() bool {
		for _, e := range bus.snapshot() {
			if e.Type == "state_change" && e.Payload["to"] == string(constant.WorkStateFailed) {
				return true
			}
		}
		return false
	})

	// 原因要带上，否则用户只知道「失败了」而不知道要去装个 claude
	for _, e := range bus.snapshot() {
		if e.Payload["to"] != string(constant.WorkStateFailed) {
			continue
		}
		if reason, _ := e.Payload["reason"].(string); reason == "" {
			t.Error("失败事件没带原因——用户只知道失败了，不知道该去做什么")
		}
	}
}

// worktree 都没切成就不该去跑 AI——没有现场可以让它干活。
func TestStart_NoTurnWhenWorktreeFails(t *testing.T) {
	notARepo := t.TempDir()
	runner := &fakeRunner{}

	if _, err := newServiceWithRunner(t, &memWorks{}, &recordingBus{}, runner).
		Start(context.Background(), notARepo, "帮我加个功能"); err == nil {
		t.Fatal("不是 git 仓库却建成功了")
	}

	time.Sleep(100 * time.Millisecond)
	if n := len(runner.snapshot()); n != 0 {
		t.Errorf("worktree 没切成却跑了 %d 轮——没有现场可以让 AI 干活", n)
	}
}

// 没配 runner 时（比如只跑 API 冒烟）不该崩。
func TestStart_NilRunnerDoesNotPanic(t *testing.T) {
	project := testutil.NewGitRepo(t)
	if _, err := work.New(&memWorks{}, realWorktrees{root: t.TempDir()},
		&recordingBus{}, &seqIDs{}, nil).
		Start(context.Background(), project, "帮我加个功能"); err != nil {
		t.Fatalf("建工作失败: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
}
