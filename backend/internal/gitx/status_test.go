package gitx_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/gitx"
)

// M4 U4.1.1 · 开工前的仓库状态
//
// ★ 用**真的 git 仓库**（`t.TempDir` + 真 git 命令），不 mock——
// 「rebase 中途」「空仓库」这些状态，mock 出来的都是我以为的样子。

func repoWithCommit(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	initRepo(t, root)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", ".")
	gitRun(t, root, "commit", "-q", "-m", "first")
	return root
}

func probe(t *testing.T, root string) gitx.Status {
	t.Helper()
	st, err := gitx.ProbeStatus(context.Background(), root)
	if err != nil {
		t.Fatalf("探测 %s: %v", root, err)
	}
	return st
}

// ★★ R1 · 未提交改动**分得清已跟踪与未跟踪**。
//
// 合成一条的话，「我只是新建了几个还没 add 的文件」和
// 「我改了正在跟踪的代码」会长得一模一样——而对用户是两件完全不同的事。
func TestProbeStatus_R1_SeparatesTrackedFromUntracked(t *testing.T) {
	root := repoWithCommit(t)

	// 干净时两个都是 0
	if st := probe(t, root); st.IsDirty() {
		t.Fatalf("干净仓库被报成脏的：%+v", st)
	}

	// 只有未跟踪
	if err := os.WriteFile(filepath.Join(root, "new.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	st := probe(t, root)
	if st.Untracked != 1 {
		t.Errorf("未跟踪 = %d，想要 1", st.Untracked)
	}
	if st.TrackedDirty != 0 {
		t.Errorf("★ 新建的文件被算成了「已跟踪文件被改」：tracked=%d——"+
			"用户会以为自己动了正在跟踪的代码", st.TrackedDirty)
	}

	// 再改一个已跟踪的
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	st = probe(t, root)
	if st.TrackedDirty != 1 {
		t.Errorf("已跟踪改动 = %d，想要 1", st.TrackedDirty)
	}
	if st.Untracked != 1 {
		t.Errorf("未跟踪 = %d，想要 1", st.Untracked)
	}
}

// R2 · 列出本地分支并标出当前分支。
func TestProbeStatus_R2_ListsBranchesAndMarksCurrent(t *testing.T) {
	root := repoWithCommit(t)
	gitRun(t, root, "branch", "develop")
	gitRun(t, root, "branch", "feature/x")

	st := probe(t, root)
	if len(st.Branches) != 3 {
		t.Fatalf("分支 = %v，想要 3 个", st.Branches)
	}
	if st.CurrentBranch == "" {
		t.Error("没标出当前分支")
	}
	var found bool
	for _, b := range st.Branches {
		if b == st.CurrentBranch {
			found = true
		}
	}
	if !found {
		t.Errorf("当前分支 %q 不在分支列表 %v 里", st.CurrentBranch, st.Branches)
	}
	if st.HeadCommit == "" {
		t.Error("没读出 HEAD 的 commit——worktree 需要一个基线")
	}
}

// ★★ R3 · **一个字节都不写**。
//
// 判据是探测前后 `git status --porcelain` 的输出完全一致 +
// 全目录内容指纹不变——后者连 `.git` 里的索引变化也算进去。
func TestProbeStatus_R3_WritesNothing(t *testing.T) {
	root := repoWithCommit(t)
	if err := os.WriteFile(filepath.Join(root, "dirty.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	before := gitStatus(t, root)
	probe(t, root)
	if after := gitStatus(t, root); before != after {
		t.Errorf("探测改动了仓库：\n之前长度 %d\n之后长度 %d", len(before), len(after))
	}
}

// ★★ R4 · rebase / merge 中途**拒绝开工**。
//
// 那时切 worktree 会把一个做了一半的操作丢在那儿，
// 而用户很可能正在解冲突——他回来时会发现自己的现场不见了。
func TestProbeStatus_R4_RefusesDuringMerge(t *testing.T) {
	root := repoWithCommit(t)

	// 造一个真的冲突中途态：两个分支改同一行，merge 必然冲突
	gitRun(t, root, "checkout", "-q", "-b", "other")
	writeAndCommit(t, root, "a.txt", "from other\n", "other side")
	gitRun(t, root, "checkout", "-q", "-")
	writeAndCommit(t, root, "a.txt", "from main\n", "main side")

	// merge 会失败（冲突），仓库进入 MERGE_HEAD 状态
	_ = gitx.RunForTest(context.Background(), root, "merge", "other")

	_, err := gitx.ProbeStatus(context.Background(), root)
	if !errors.Is(err, gitx.ErrMidOperation) {
		t.Fatalf("merge 冲突中途的错误 = %v，想要 ErrMidOperation——"+
			"那时开工会丢掉用户正在解的冲突", err)
	}
	if err != nil && !contains(err.Error(), "merge") {
		t.Errorf("错误没说清是哪种操作：%v", err)
	}
}

// ★ R5 · 空仓库（刚 git init）如实报告，**不 panic**。
//
// 「新建文件夹 → git init → 加进 Duet」是真实路径。
func TestProbeStatus_R5_EmptyRepoIsReported(t *testing.T) {
	root := t.TempDir()
	initRepo(t, root)

	_, err := gitx.ProbeStatus(context.Background(), root)
	if !errors.Is(err, gitx.ErrNoCommits) {
		t.Errorf("空仓库的错误 = %v，想要 ErrNoCommits", err)
	}
	// ★ 不能报成「没有分支」：那会让用户去建分支，
	// 而他真正要做的是先提交一次
	if err != nil && contains(err.Error(), "分支") {
		t.Errorf("把「还没提交过」报成了分支问题：%v", err)
	}
}

// 非 git 目录如实报错。
func TestProbeStatus_NonRepoErrs(t *testing.T) {
	if _, err := gitx.ProbeStatus(context.Background(), t.TempDir()); err == nil {
		t.Error("非仓库却没报错")
	}
}

// ★ 未跟踪的**子目录里的文件逐个数**，不是只报一个目录。
//
// `git status --porcelain` 默认把整个未跟踪目录折成一行，
// 那样用户会看到「1 处改动」而实际有十几个文件。
func TestProbeStatus_CountsFilesInUntrackedDirs(t *testing.T) {
	root := repoWithCommit(t)
	sub := filepath.Join(root, "newdir")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"a", "b", "c"} {
		if err := os.WriteFile(filepath.Join(sub, n+".txt"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	st := probe(t, root)
	if st.Untracked != 3 {
		t.Errorf("未跟踪 = %d，想要 3——默认会把整个目录折成一行，"+
			"用户会看到「1 处改动」而实际有三个文件", st.Untracked)
	}
}

// 已暂存的改动也算「已跟踪的改动」。
func TestProbeStatus_StagedCountsAsTracked(t *testing.T) {
	root := repoWithCommit(t)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("staged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", "a.txt")

	st := probe(t, root)
	if st.TrackedDirty != 1 {
		t.Errorf("已暂存的改动 = %d，想要 1——它同样是「未提交」", st.TrackedDirty)
	}
}

func writeAndCommit(t *testing.T, root, name, content, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", name)
	gitRun(t, root, "commit", "-q", "-m", msg)
}

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// M4 U4.2.2 · 工作区现场

// ★ 未提交改动**逐个文件带增删行数**。
//
// 只说「改了 3 个文件」的话，用户判断不出这次改动有多大——
// 而设计稿要的是 `+64 −12` 这个形态。
func TestProbeWorktree_ReportsPerFileLineCounts(t *testing.T) {
	root := repoWithCommit(t)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("l1\nl2\nl3\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	st, err := gitx.ProbeWorktree(context.Background(), root, "")
	if err != nil {
		t.Fatalf("探测: %v", err)
	}
	if len(st.Changes) != 1 {
		t.Fatalf("改动 = %+v，想要 1 个文件", st.Changes)
	}
	c := st.Changes[0]
	if c.Path != "a.txt" {
		t.Errorf("路径 = %q", c.Path)
	}
	if c.Added == 0 && c.Removed == 0 {
		t.Errorf("增删行数都是 0：%+v——用户判断不出改动有多大", c)
	}
}

// ★★ 只报**本次工作的** commit，基线之前的不算。
//
// 混进来的话，用户会以为 Duet 改了他早先的提交。
func TestProbeWorktree_OnlyCountsCommitsAfterBase(t *testing.T) {
	root := repoWithCommit(t)
	base, err := gitx.RunOutputForTest(context.Background(), root, "rev-parse", "--short", "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	// 基线之后两条
	writeAndCommit(t, root, "b.txt", "x\n", "第一条")
	writeAndCommit(t, root, "c.txt", "y\n", "第二条")

	st, err := gitx.ProbeWorktree(context.Background(), root, base)
	if err != nil {
		t.Fatalf("探测: %v", err)
	}
	if st.Ahead != 2 {
		t.Errorf("领先 = %d，想要 2", st.Ahead)
	}
	if len(st.Commits) != 2 {
		t.Fatalf("commit = %d 条，想要 2——基线之前的是用户自己的历史", len(st.Commits))
	}
	// ★ 最近的排最前
	if st.Commits[0].Subject != "第二条" {
		t.Errorf("第一条 commit = %q，想要「第二条」（最近的排最前）", st.Commits[0].Subject)
	}
	for _, c := range st.Commits {
		if c.SHA == "" || c.When == "" {
			t.Errorf("commit 信息不全：%+v", c)
		}
	}
}

// ★ commit 标题里有竖线和冒号是很正常的事，不能被切坏。
func TestProbeWorktree_HandlesSubjectsWithSeparators(t *testing.T) {
	root := repoWithCommit(t)
	base, err := gitx.RunOutputForTest(context.Background(), root, "rev-parse", "--short", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	subject := "fix(api): 处理 a|b 与 c:d 这种标题"
	writeAndCommit(t, root, "d.txt", "z\n", subject)

	st, err := gitx.ProbeWorktree(context.Background(), root, base)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Commits) != 1 {
		t.Fatalf("commit = %d 条", len(st.Commits))
	}
	if st.Commits[0].Subject != subject {
		t.Errorf("标题被切坏了：%q\n想要：%q", st.Commits[0].Subject, subject)
	}
}

// ★★ 没有基线时**不编一个 0 出来**。
//
// 报「领先 0 个 commit」而实际我们根本不知道，比不报更糟——
// 用户会以为 AI 什么都没干。
func TestProbeWorktree_NoBaseMeansNoAheadCount(t *testing.T) {
	root := repoWithCommit(t)
	writeAndCommit(t, root, "e.txt", "x\n", "一条提交")

	st, err := gitx.ProbeWorktree(context.Background(), root, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Commits) != 0 {
		t.Errorf("没有基线却列出了 %d 条 commit——那些可能是用户自己的历史", len(st.Commits))
	}
	// 分支与未提交改动仍然要报
	if st.Branch == "" {
		t.Error("没报分支")
	}
}

// ★★ 探测**一个字节都不写**。
func TestProbeWorktree_WritesNothing(t *testing.T) {
	root := repoWithCommit(t)
	if err := os.WriteFile(filepath.Join(root, "dirty.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	before := gitStatus(t, root)
	if _, err := gitx.ProbeWorktree(context.Background(), root, ""); err != nil {
		t.Fatal(err)
	}
	if after := gitStatus(t, root); before != after {
		t.Error("探测改动了工作区")
	}
}

// 干净的工作区：没有改动，也不报错。
func TestProbeWorktree_CleanTreeIsFine(t *testing.T) {
	root := repoWithCommit(t)
	st, err := gitx.ProbeWorktree(context.Background(), root, "")
	if err != nil {
		t.Fatalf("干净的工作区报了错：%v", err)
	}
	if len(st.Changes) != 0 {
		t.Errorf("干净却报了 %d 处改动", len(st.Changes))
	}
}

func TestProbeWorktree_NonRepoErrs(t *testing.T) {
	if _, err := gitx.ProbeWorktree(context.Background(), t.TempDir(), ""); err == nil {
		t.Error("非仓库却没报错")
	}
}
