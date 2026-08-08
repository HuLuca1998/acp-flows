package checkpoint_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/checkpoint"
	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/internal/constant"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
	"github.com/HuLuca1998/acp-flows/backend/internal/gitx"
	"github.com/HuLuca1998/acp-flows/backend/tests/testutil"
)

// U4.1.2 · 检查点与启动时恢复（验收点 V10）
//
// ★ 这一层回答两个问题：「有哪些工作能接着做」「接着做之前要不要先提醒他」。
//
// 最要紧的一条是 R3：**工作区被手工改动过时先告知，不静默覆盖**。
// 用户在 Duet 关着的时候自己改了几行，恢复时被冲掉——那是不可挽回的损失，
// 而他甚至不知道发生过。

// memWorks 是内存仓储。
type memWorks struct {
	mu    sync.Mutex
	items []*model.Work
}

func (r *memWorks) SaveWork(_ context.Context, w *model.Work) error {
	r.mu.Lock()
	defer r.mu.Unlock()
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
			// 重建新对象——真 store 每次都经 mapper 重建，交指针的话
			// 这个替身比真实现「更共享」，测出来的竞态在生产里不存在
			return model.NewWorkAt(w.ID(), w.State()), nil
		}
	}
	return nil, model.ErrNotFound
}

// errNoWorktree 是「找不到工作区」的哨兵，用来验它有没有被原样传上去。
var errNoWorktree = errors.New("工作区没了")

// stubWorktrees 记着每个工作的工作区路径。
type stubWorktrees struct {
	paths map[string]string
	err   error
}

func (s stubWorktrees) WorktreePath(_ context.Context, workID string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	p, ok := s.paths[workID]
	if !ok {
		return "", errors.New("没有这个工作的工作区")
	}
	return p, nil
}

func seed(t *testing.T, repo *memWorks, id string, state constant.WorkState) {
	t.Helper()
	if err := repo.SaveWork(context.Background(), model.NewWorkAt(id, state)); err != nil {
		t.Fatal(err)
	}
}

// ★ 用**真 gitx.IsDirty**，不塞假的：本单元最要紧的一条是
// 「用户手工改过的工作区要先告知」，而假实现只会回一个我们自己设的布尔值——
// 那条断言会永远绿，「怎么算脏」这件事根本没被验证。
func newService(repo *memWorks, wt port.WorktreeLocator) *checkpoint.Service {
	return checkpoint.New(repo, wt, checkpoint.WithDirtyChecker(gitx.IsDirty))
}

// ★★ R1：**只列出真能接着做的**。
//
// 把跑完的、失败的也列出来的话，用户对着一串条目不知道该点哪个——
// 而点进去发现「没什么可恢复的」比列表里没有它更让人困惑。
func TestListResumable_R1_OnlyPausedWorks(t *testing.T) {
	repo := &memWorks{}
	// 穷举全部状态，只有 paused 该出现
	for _, s := range constant.AllWorkStates() {
		seed(t, repo, "work-"+string(s), s)
	}
	svc := newService(repo, stubWorktrees{paths: map[string]string{}})

	got, err := svc.ListResumable(context.Background())
	if err != nil {
		t.Fatalf("列出失败: %v", err)
	}

	if len(got) != 1 {
		ids := make([]string, 0, len(got))
		for _, r := range got {
			ids = append(ids, r.WorkID)
		}
		t.Fatalf("列出了 %d 条 %v, 想要 1（只有 paused）——\n"+
			"把跑完的、失败的也列出来的话，用户对着一串条目不知道该点哪个", len(got), ids)
	}
	if got[0].WorkID != "work-paused" {
		t.Errorf("列出的是 %q, 想要 work-paused", got[0].WorkID)
	}
}

// 一个可恢复的都没有时返回**空切片**而不是 nil。
//
// api 层要序列化成 `[]` 而不是 `null`——前端对 null 调 .map() 会白屏。
func TestListResumable_EmptyIsASlice(t *testing.T) {
	svc := newService(&memWorks{}, stubWorktrees{paths: map[string]string{}})

	got, err := svc.ListResumable(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Error("返回了 nil——api 层会序列化成 null，前端对它调 .map() 会白屏")
	}
	if len(got) != 0 {
		t.Errorf("空库却列出了 %d 条", len(got))
	}
}

// ★★ R3：工作区被手工改动过时**先告知，不静默覆盖**。
//
// 用户在 Duet 关着的时候自己改了几行，恢复时被冲掉——那是不可挽回的损失，
// 而他甚至不知道发生过。
func TestResume_R3_DirtyWorktreeIsReported(t *testing.T) {
	repo := &memWorks{}
	seed(t, repo, "work-01", constant.WorkStatePaused)

	wt := testutil.NewGitRepo(t)
	// 用户在 Duet 关着的时候改了一行
	if err := os.WriteFile(filepath.Join(wt, "README.md"), []byte("# 我自己改的\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := newService(repo, stubWorktrees{paths: map[string]string{"work-01": wt}})

	_, err := svc.Resume(context.Background(), "work-01")
	if !errors.Is(err, checkpoint.ErrWorktreeDirty) {
		t.Fatalf("错误 = %v, 想要 ErrWorktreeDirty——\n"+
			"用户在 Duet 关着时改的几行会被冲掉，而他甚至不知道发生过", err)
	}
	// 状态**不能**被改动：告知的意思是「还没恢复」
	w, _ := repo.FindWork(context.Background(), "work-01")
	if w.State() != constant.WorkStatePaused {
		t.Errorf("报了冲突却已经把状态改成 %q——那叫「先斩后奏」", w.State())
	}
}

// ★★ **未跟踪的新文件也算脏。**
//
// 只看已跟踪文件的改动（`git diff`）会漏掉它——而「用户在 Duet 关着的时候
// 新建了一个文件」正是最典型的场景。恢复之后 AI 在同一个目录里干活，
// 很可能覆盖同名文件。
//
// ★ 这条与上一条分开写：上一条改的是已跟踪的 README.md，
// 把 `status --porcelain` 换成 `diff --name-only` 它照样绿（造负例时发现的）。
func TestResume_UntrackedFileCountsAsDirty(t *testing.T) {
	repo := &memWorks{}
	seed(t, repo, "work-01", constant.WorkStatePaused)

	wt := testutil.NewGitRepo(t)
	// 用户新建了一个文件，还没 add
	if err := os.WriteFile(filepath.Join(wt, "我的笔记.md"), []byte("随手记的\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := newService(repo, stubWorktrees{paths: map[string]string{"work-01": wt}})

	if _, err := svc.Resume(context.Background(), "work-01"); !errors.Is(err, checkpoint.ErrWorktreeDirty) {
		t.Fatalf("错误 = %v, 想要 ErrWorktreeDirty——\n"+
			"只看已跟踪文件的改动会漏掉新建的文件，而恢复之后 AI 在同一个目录里干活，"+
			"很可能覆盖它", err)
	}
}

// ★★ 确认之后才恢复，**并且不碰用户的改动**。
func TestResume_ForcedProceedsAndKeepsChanges(t *testing.T) {
	repo := &memWorks{}
	seed(t, repo, "work-01", constant.WorkStatePaused)

	wt := testutil.NewGitRepo(t)
	mine := filepath.Join(wt, "README.md")
	if err := os.WriteFile(mine, []byte("# 我自己改的\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := newService(repo, stubWorktrees{paths: map[string]string{"work-01": wt}})

	if _, err := svc.ResumeForce(context.Background(), "work-01"); err != nil {
		t.Fatalf("确认后恢复失败: %v", err)
	}

	// ★ 恢复**不许覆盖用户的改动**（本单元 forbidden_changes 明写）
	raw, err := os.ReadFile(mine)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "# 我自己改的\n" {
		t.Errorf("用户的改动被冲掉了（现在是 %q）——不可挽回，而他甚至不知道", raw)
	}
}

// ★★ R2：恢复之后状态回到**能接着跑**的那一个。
func TestResume_R2_RestoresRunnableState(t *testing.T) {
	repo := &memWorks{}
	seed(t, repo, "work-01", constant.WorkStatePaused)
	wt := testutil.NewGitRepo(t)
	svc := newService(repo, stubWorktrees{paths: map[string]string{"work-01": wt}})

	view, err := svc.Resume(context.Background(), "work-01")
	if err != nil {
		t.Fatalf("恢复失败: %v", err)
	}

	if view.State == constant.WorkStatePaused {
		t.Error("恢复之后还停在 paused——用户点了「接着做」，它什么都没做")
	}
	w, _ := repo.FindWork(context.Background(), "work-01")
	if w.State() != view.State {
		t.Errorf("返回的状态 %q 与落库的 %q 对不上", view.State, w.State())
	}
}

// 恢复一个不是 paused 的工作要拒，而不是把它强行推走。
func TestResume_NonPausedIsRejected(t *testing.T) {
	repo := &memWorks{}
	seed(t, repo, "work-01", constant.WorkStateExecuting)
	svc := newService(repo, stubWorktrees{paths: map[string]string{"work-01": testutil.NewGitRepo(t)}})

	_, err := svc.Resume(context.Background(), "work-01")
	// ★ 断言**具体的错误**，不是「有错就行」：状态检查删掉之后，
	// 后面的 Transition 也会失败，「有错就行」照样绿（造负例时发现的）。
	if !errors.Is(err, checkpoint.ErrNotResumable) {
		t.Errorf("错误 = %v, 想要 ErrNotResumable——\n"+
			"一个正在跑的工作被「恢复」，会把它推到一个说不清的状态", err)
	}
}

// 恢复不存在的工作要报错，不是静静成功。
func TestResume_UnknownWorkIsRejected(t *testing.T) {
	svc := newService(&memWorks{}, stubWorktrees{paths: map[string]string{}})

	if _, err := svc.Resume(context.Background(), "work-nope"); err == nil {
		t.Error("恢复一个不存在的工作却成功了")
	}
}

// ★ 工作区**找不到**时报错，不当成「干净」放行。
//
// 当成干净的话，恢复会在一个不存在的目录上继续——AI 的第一个工具调用就炸，
// 而错误离原因已经很远了。
func TestResume_MissingWorktreeIsAnError(t *testing.T) {
	repo := &memWorks{}
	seed(t, repo, "work-01", constant.WorkStatePaused)
	svc := newService(repo, stubWorktrees{err: errNoWorktree})

	_, err := svc.Resume(context.Background(), "work-01")
	if err == nil {
		t.Fatal("工作区找不到却恢复成功了")
	}
	// ★ 断言**上游给的真实原因**传上来了，不是「有错就行」。
	// 不校验 WorktreePath 的错误时，路径变成空串、脏检查在空路径上失败，
	// 照样返回一个错误——而那句错误说的是「查工作区状态失败」，
	// 真正的原因（工作区没了）不见了。造负例时发现的。
	if !errors.Is(err, errNoWorktree) {
		t.Errorf("错误 = %v——上游说的「%v」没传上来，"+
			"排查时看到的是「查工作区状态失败」而不是「工作区没了」", err, errNoWorktree)
	}
}

// 列出的条目要带上**暂停时间**——用户靠它认出「哪个是我刚才那个」。
func TestListResumable_R1_CarriesPausedAt(t *testing.T) {
	repo := &memWorks{}
	seed(t, repo, "work-01", constant.WorkStatePaused)
	svc := newService(repo, stubWorktrees{paths: map[string]string{}})

	got, err := svc.ListResumable(context.Background())
	if err != nil || len(got) != 1 {
		t.Fatalf("列出失败: %v (%d 条)", err, len(got))
	}
	if got[0].PausedAt.IsZero() {
		t.Error("没带暂停时间——开着三四个工作时，用户认不出哪个是刚才那个")
	}
	if got[0].PausedAt.After(time.Now().Add(time.Minute)) {
		t.Errorf("暂停时间在未来：%v", got[0].PausedAt)
	}
}
