package gitx_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/gitx"
	"github.com/HuLuca1998/acp-flows/backend/tests/testutil"
)

// M8 U8.2.2 · 验收通过后的提交
//
// 用**真的 git 仓库**，不 mock——「提交到了哪个分支」「有没有带上新文件」
// 这些问题假实现回答不了。

// ★★ 提交带上**未跟踪的新文件**。
//
// AI 干活时新建文件是常态，不带的话提交里少了一半东西——
// 而 diff 证据里明明有它们，用户会发现两处对不上。
func TestCommit_IncludesUntrackedFiles(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "brand-new.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	sha, err := gitx.Commit(context.Background(), repo, "feat: 新文件")
	if err != nil {
		t.Fatalf("提交: %v", err)
	}
	if sha == "" {
		t.Error("没拿到 sha")
	}

	// ★ 判据：提交之后工作区**干净了**——新文件确实进了那个 commit
	dirty, err := gitx.IsDirty(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	if dirty {
		t.Error("提交之后还是脏的——新建的文件没被带上")
	}
}

// ★★ 没有改动时**不造空提交**。
//
// 一个「验收通过」却什么都没改的单元，说明该被质疑的是那次验收，
// 而不是往历史里塞一个空 commit 把问题盖过去。
func TestCommit_RefusesToMakeAnEmptyCommit(t *testing.T) {
	repo := testutil.NewGitRepo(t)

	_, err := gitx.Commit(context.Background(), repo, "feat: 什么都没改")
	if !errors.Is(err, gitx.ErrNothingToCommit) {
		t.Fatalf("空提交却成功了：%v——那会把「验收通过但什么都没做」盖过去", err)
	}
}

// 提交信息为空时拒绝——空信息的 commit 在历史里等于没有说明。
func TestCommit_RejectsEmptyMessage(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := gitx.Commit(context.Background(), repo, "   "); err == nil {
		t.Error("空提交信息却成功了")
	}
}

// ★ 提交信息**原样进历史**——用户日后 `git log` 要能看懂那次验收。
func TestCommit_KeepsTheMessage(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	const msg = "feat(unit-013): 取消后证据读取\n\n契约 v3 · 验收通过"
	if _, err := gitx.Commit(context.Background(), repo, msg); err != nil {
		t.Fatal(err)
	}

	out := gitLog(t, repo)
	if !strings.Contains(out, "unit-013") || !strings.Contains(out, "契约 v3") {
		t.Errorf("提交信息进历史时被改了：\n%s", out)
	}
}

// gitLog 读最后一次提交的完整信息。
func gitLog(t *testing.T, repo string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", repo, "log", "-1", "--format=%B")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git log: %v\n%s", err, out)
	}
	return string(out)
}

// ★★ 没配 git 身份时给**可操作的错误**，不是 `exit status 128`。
//
// 他刚装完 git 就用 Duet 的话会卡在这里，而不知道要去配 user.email——
// 那是一条一行就能解决的路。CI 上本地全绿而远端红，根因就是这个。
func TestCommit_MissingIdentityIsExplained(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	// ★ 用**空 ident** 来构造这个条件，而不是摘掉配置。
	//
	// 摘配置在 macOS 上重现不了：git 会从用户名与主机名**自动推导**
	// 一个身份（`luca@Mini6.lan`）然后提交成功。Linux 容器里推导不出
	// 有效地址才会拒绝——本地绿而 CI 红，根因就是这个差异。
	// 空 ident 在两种系统上都被拒绝，所以这条测试到哪都有效。
	t.Setenv("GIT_AUTHOR_NAME", "")
	t.Setenv("GIT_COMMITTER_NAME", "")
	if err := os.WriteFile(filepath.Join(repo, "NEW.md"), []byte("x\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := gitx.Commit(t.Context(), repo, "试着提交")

	if !errors.Is(err, gitx.ErrNoGitIdentity) {
		t.Fatalf("错误是 %v，想要 ErrNoGitIdentity——"+
			"用户看到 exit status 128 的话，不知道自己只差一句 git config", err)
	}
}
