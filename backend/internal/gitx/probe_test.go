package gitx_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/gitx"
	"github.com/HuLuca1998/acp-flows/backend/tests/testutil"
)

// U2.1.1 · git 探测（验收点 V4）
//
// ★ 这一层的铁律是**只读**。用真 git 仓库测，不 mock：
// mock 掉之后，「探测有没有往用户仓库里写东西」这个问题就永远测不出来了，
// 而那正是本单元的 forbidden_changes 第一条。

func TestProbe(t *testing.T) {
	t.Run("git 仓库：报出 is_repo 与默认分支", func(t *testing.T) {
		repo := testutil.NewGitRepo(t)

		got, err := gitx.Probe(context.Background(), repo)
		if err != nil {
			t.Fatalf("探测失败: %v", err)
		}
		if !got.IsRepo {
			t.Error("IsRepo = false，但这就是个真 git 仓库")
		}
		if got.DefaultBranch != "main" {
			t.Errorf("DefaultBranch = %q, 想要 main", got.DefaultBranch)
		}
	})

	t.Run("普通目录：不是仓库，但不报错", func(t *testing.T) {
		// ★ 不是 git 仓库**也允许添加**。直接当错误处理的话，用户得先去命令行
		// git init 才能用这个产品——而他很可能正是不想碰命令行才来用的。
		plain := t.TempDir()

		got, err := gitx.Probe(context.Background(), plain)
		if err != nil {
			t.Fatalf("普通目录不该报错: %v", err)
		}
		if got.IsRepo {
			t.Error("IsRepo = true，但这只是个普通目录")
		}
		if got.DefaultBranch != "" {
			t.Errorf("DefaultBranch = %q, 非仓库时应为空", got.DefaultBranch)
		}
	})

	t.Run("路径不存在：报错，不当成普通目录", func(t *testing.T) {
		// 静默当成「普通目录」的话，用户会把一个打错的路径加进列表，
		// 直到真正开工作时才发现——那时错误离原因已经很远了。
		missing := filepath.Join(t.TempDir(), "nope")

		if _, err := gitx.Probe(context.Background(), missing); err == nil {
			t.Error("路径不存在却没报错")
		}
	})

	t.Run("路径是文件不是目录：报错", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "a.txt")
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}

		if _, err := gitx.Probe(context.Background(), f); err == nil {
			t.Error("路径是文件却没报错")
		}
	})
}

// ★★ 本单元最要紧的一条：探测**不往用户仓库写一个字节**。
//
// 比 git status 是不够的——往 .gitignore 加一行、或写进已被忽略的路径，
// status 依然干净而目录确实被动过了。所以连文件列表一起比。
func TestProbe_WritesNothing(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	before := testutil.SnapshotDir(t, repo)

	for range 3 { // 多探几次：某些 git 子命令会在首次调用时建缓存
		if _, err := gitx.Probe(context.Background(), repo); err != nil {
			t.Fatalf("探测失败: %v", err)
		}
	}

	testutil.AssertUnchanged(t, repo, before)
}

// 仓库还没有任何 commit 时（刚 git init），HEAD 指向一个不存在的引用。
// 这是用户「新建文件夹 → git init → 加进 Duet」的真实路径，不能崩。
func TestProbe_EmptyRepo(t *testing.T) {
	dir := t.TempDir()
	if err := runGit(t, dir, "init"); err != nil {
		t.Fatal(err)
	}

	got, err := gitx.Probe(context.Background(), dir)
	if err != nil {
		t.Fatalf("空仓库探测失败: %v", err)
	}
	if !got.IsRepo {
		t.Error("IsRepo = false，但已经 git init 过了")
	}
	// 分支名此时来自 init.defaultBranch，允许为空但不能崩
}

func runGit(t *testing.T, dir string, args ...string) error {
	t.Helper()
	return gitx.RunForTest(t.Context(), dir, args...)
}
