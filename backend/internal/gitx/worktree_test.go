package gitx_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/gitx"
	"github.com/HuLuca1998/acp-flows/backend/tests/testutil"
)

// U2.4.1 · worktree 隔离（验收点 V5 的 R1 R2）
//
// ★★ 这是 Duet **唯一会大量写文件**的地方。写错位置的话，用户的仓库里
// 会突然多出一堆分支和目录——而他把代码目录交给我们时并没同意这个。

// ★★ R2：worktree 建在 `~/.acpflows/worktrees`，**不在用户项目里**。
//
// 这条比「隔离得对不对」更靠前：位置错了的话，隔离做得再好也是在用户的
// 仓库里制造混乱。
func TestAddWorktree_LivesOutsideUserProject(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	root := t.TempDir() // 扮演 ~/.acpflows/worktrees
	before := testutil.SnapshotDir(t, repo)

	wt, err := gitx.AddWorktree(context.Background(), gitx.WorktreeSpec{
		Repo: repo, Root: root, WorkID: "work-01", Branch: "duet/work-01",
	})
	if err != nil {
		t.Fatalf("建 worktree 失败: %v", err)
	}
	t.Cleanup(func() { _ = gitx.RemoveWorktree(context.Background(), repo, wt.Path) })

	// 路径必须在 root 下，不能在用户项目里
	if !strings.HasPrefix(wt.Path, root) {
		t.Errorf("worktree 建在了 %q，不在 %q 下面", wt.Path, root)
	}
	if strings.HasPrefix(wt.Path, repo) {
		t.Errorf("worktree 建进了用户的项目目录：%q", wt.Path)
	}

	// ★ 用户项目里**不能多出任何文件**。
	// 注意 .git 内部会因为 worktree 登记而变化，那是 git 自己的账本，
	// SnapshotDir 已经跳过它——用户看得见的东西必须原样。
	testutil.AssertUnchanged(t, repo, before)
}

// ★ R1：两个工作互不干扰。
//
// 共用一个目录的话，两个 AI 会同时改同一份文件——用户看到的是一堆
// 互相覆盖的改动，而没有任何提示说这是两个工作撞在一起了。
func TestAddWorktree_IsolatesWorks(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	root := t.TempDir()
	ctx := context.Background()

	a, err := gitx.AddWorktree(ctx, gitx.WorktreeSpec{
		Repo: repo, Root: root, WorkID: "work-01", Branch: "duet/work-01",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = gitx.RemoveWorktree(ctx, repo, a.Path) })

	b, err := gitx.AddWorktree(ctx, gitx.WorktreeSpec{
		Repo: repo, Root: root, WorkID: "work-02", Branch: "duet/work-02",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = gitx.RemoveWorktree(ctx, repo, b.Path) })

	if a.Path == b.Path {
		t.Fatal("两个工作拿到了同一个目录")
	}

	// 在 a 里改一个文件，b 里不该看得见
	target := filepath.Join(a.Path, "only-in-a.txt")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(b.Path, "only-in-a.txt")); err == nil {
		t.Error("在 work-01 里建的文件出现在了 work-02 里——两个 AI 会互相覆盖改动")
	}
}

// worktree 里要有仓库的内容，不是个空目录。
func TestAddWorktree_HasRepoContent(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	root := t.TempDir()

	wt, err := gitx.AddWorktree(context.Background(), gitx.WorktreeSpec{
		Repo: repo, Root: root, WorkID: "work-01", Branch: "duet/work-01",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = gitx.RemoveWorktree(context.Background(), repo, wt.Path) })

	// NewGitRepo 造的仓库里有 README.md
	if _, err := os.Stat(filepath.Join(wt.Path, "README.md")); err != nil {
		t.Errorf("worktree 里没有仓库内容，AI 无从下手: %v", err)
	}
}

// 同一个 WorkID 重复建：返回已有的而不是报错。
//
// 重启后恢复一个工作时会走到这条路——报错的话用户的工作就再也打不开了。
func TestAddWorktree_IsIdempotent(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	root := t.TempDir()
	ctx := context.Background()
	spec := gitx.WorktreeSpec{Repo: repo, Root: root, WorkID: "work-01", Branch: "duet/work-01"}

	first, err := gitx.AddWorktree(ctx, spec)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = gitx.RemoveWorktree(ctx, repo, first.Path) })

	second, err := gitx.AddWorktree(ctx, spec)
	if err != nil {
		t.Fatalf("重复建应当返回已有的，而不是报错: %v", err)
	}
	if second.Path != first.Path {
		t.Errorf("两次拿到不同路径：%q vs %q", first.Path, second.Path)
	}
}

// ★ 移除 worktree **不碰用户项目里的任何文件**。
func TestRemoveWorktree_LeavesUserProjectAlone(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	root := t.TempDir()
	ctx := context.Background()

	wt, err := gitx.AddWorktree(ctx, gitx.WorktreeSpec{
		Repo: repo, Root: root, WorkID: "work-01", Branch: "duet/work-01",
	})
	if err != nil {
		t.Fatal(err)
	}

	before := testutil.SnapshotDir(t, repo)
	if err := gitx.RemoveWorktree(ctx, repo, wt.Path); err != nil {
		t.Fatalf("移除失败: %v", err)
	}

	if _, err := os.Stat(wt.Path); err == nil {
		t.Error("worktree 目录还在")
	}
	testutil.AssertUnchanged(t, repo, before)
}

// 移除一个不存在的：不报错。
// 清理路径可能被走两遍（正常结束 + 崩溃后重启清理）。
func TestRemoveWorktree_IsIdempotent(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	missing := filepath.Join(t.TempDir(), "never-existed")

	if err := gitx.RemoveWorktree(context.Background(), repo, missing); err != nil {
		t.Errorf("移除不存在的 worktree 报错了：%v", err)
	}
}

// 非 git 仓库不能建 worktree——要给明确的错误，不是让 git 吐一句英文。
func TestAddWorktree_RejectsNonRepo(t *testing.T) {
	plain := t.TempDir()

	_, err := gitx.AddWorktree(context.Background(), gitx.WorktreeSpec{
		Repo: plain, Root: t.TempDir(), WorkID: "work-01", Branch: "duet/work-01",
	})
	if err == nil {
		t.Fatal("非 git 目录却建成功了")
	}
}

// M4 U4.1.2 · worktree 创建与失败处置

// ★★ R2 · 分支已存在时**不覆盖**，报错。
//
// 用户可能自己手建过同名分支，上面躺着他的提交。
// `git worktree add -B` 会把它强制复位到 HEAD——那些提交就没了，
// 而分支还在，他不会立刻发现。
func TestAddWorktree_R2_RefusesToStealAnExistingBranch(t *testing.T) {
	repo := repoWithCommit(t)
	root := t.TempDir()

	// 用户自己建了一个同名分支，并在上面提交了东西
	gitRun(t, repo, "branch", "duet/work-01")
	gitRun(t, repo, "checkout", "-q", "duet/work-01")
	writeAndCommit(t, repo, "his-work.txt", "我辛苦写的\n", "用户自己的提交")
	his, err := gitx.RunOutputForTest(context.Background(), repo, "rev-parse", "duet/work-01")
	if err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "checkout", "-q", "-")

	_, addErr := gitx.AddWorktree(context.Background(), gitx.WorktreeSpec{
		Repo: repo, Root: root, WorkID: "work-01", Branch: "duet/work-01",
	})
	if !errors.Is(addErr, gitx.ErrBranchTaken) {
		t.Fatalf("撞上已有分支的错误 = %v，想要 ErrBranchTaken", addErr)
	}

	// ★★ 判据：他那条分支**一个 commit 都没少**
	after, err := gitx.RunOutputForTest(context.Background(), repo, "rev-parse", "duet/work-01")
	if err != nil {
		t.Fatal(err)
	}
	if after != his {
		t.Errorf("★ 用户的分支被复位了：%s → %s——他的提交没了，而分支还在，"+
			"他不会立刻发现", his, after)
	}
}

// ★ R5 · 基线 commit 被记下来。
//
// 记不下来的话，「领先几个 commit」与验收时的 diff 都没有起点。
func TestAddWorktree_R5_RecordsBaseCommit(t *testing.T) {
	repo := repoWithCommit(t)
	root := t.TempDir()

	wt, err := gitx.AddWorktree(context.Background(), gitx.WorktreeSpec{
		Repo: repo, Root: root, WorkID: "work-base", Branch: "duet/work-base",
	})
	if err != nil {
		t.Fatalf("建 worktree: %v", err)
	}
	if wt.BaseCommit == "" {
		t.Fatal("没记下基线 commit——「领先几个 commit」与验收 diff 都没有起点")
	}

	head, err := gitx.RunOutputForTest(context.Background(), repo, "rev-parse", "--short", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if wt.BaseCommit != head {
		t.Errorf("基线 = %q，仓库 HEAD = %q", wt.BaseCommit, head)
	}
}

// ★ 能从**指定的基线**开分支，不总是当前 HEAD。
//
// 用户可能想从 `develop` 开工，而当前分支上正躺着他没提交完的东西。
func TestAddWorktree_ForksFromChosenBase(t *testing.T) {
	repo := repoWithCommit(t)
	root := t.TempDir()

	gitRun(t, repo, "checkout", "-q", "-b", "develop")
	writeAndCommit(t, repo, "on-develop.txt", "x\n", "develop 上的提交")
	developHead, err := gitx.RunOutputForTest(context.Background(), repo, "rev-parse", "--short", "develop")
	if err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "checkout", "-q", "-")

	wt, err := gitx.AddWorktree(context.Background(), gitx.WorktreeSpec{
		Repo: repo, Root: root, WorkID: "work-dev", Branch: "duet/work-dev",
		BaseRef: "develop",
	})
	if err != nil {
		t.Fatalf("从 develop 开工: %v", err)
	}
	if wt.BaseCommit != developHead {
		t.Errorf("基线 = %q，想要 develop 的 %q——用户选了基线却没用上", wt.BaseCommit, developHead)
	}
	// worktree 里应该有 develop 上那个文件
	if _, statErr := os.Stat(filepath.Join(wt.Path, "on-develop.txt")); statErr != nil {
		t.Errorf("worktree 不是从 develop 切的：%v", statErr)
	}
}

// ★ 基线解析不出来就**不建**。
//
// 用一个我们没搞懂的起点开工，之后「领先几个 commit」「本次工作的 diff」
// 全都是错的。
func TestAddWorktree_RefusesUnknownBase(t *testing.T) {
	repo := repoWithCommit(t)
	root := t.TempDir()

	_, err := gitx.AddWorktree(context.Background(), gitx.WorktreeSpec{
		Repo: repo, Root: root, WorkID: "work-bad", Branch: "duet/work-bad",
		BaseRef: "根本不存在的分支",
	})
	if err == nil {
		t.Fatal("基线不存在却照建了")
	}
	// ★ 判据：**目录没建出来**——半个 worktree 比没有更糟
	if _, statErr := os.Stat(filepath.Join(root, "work-bad")); statErr == nil {
		t.Error("基线解析失败却留下了 worktree 目录")
	}
	// ★★ 错误信息要说得出**是基线的问题**。
	//
	// 光靠 git 自己报错的话，用户看到的是一句
	// `fatal: invalid reference: 根本不存在的分支`——他不知道那是
	// 「你选的基线」而不是「Duet 内部出错了」。
	// 这就是我们在动手之前先解析一次基线的理由。
	if !contains(err.Error(), "基线") {
		t.Errorf("错误没说清是基线的问题：%v", err)
	}
}
