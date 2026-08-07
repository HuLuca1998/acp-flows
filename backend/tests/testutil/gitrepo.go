package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// NewGitRepo 在临时目录里现场 git init 一个真仓库，带一个初始 commit。
//
// ★ **绝不碰用户的真实仓库**（铁律 6）。这里造的是真 git、真 commit、真 HEAD——
// 换成 mock 的话，「Duet 有没有污染用户仓库」这个问题就测不了了，
// 而那正是本项目最不能出错的地方。
func NewGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// 全局 git 配置可能带 gpgsign / hooks / 模板目录，会让 commit 在别人的机器上
	// 失败或多出文件。用 -c 覆盖成确定的最小配置。
	run := func(args ...string) {
		t.Helper()
		base := []string{
			"-c", "user.email=test@duet.local",
			"-c", "user.name=Duet Test",
			"-c", "commit.gpgsign=false",
			"-c", "init.defaultBranch=main",
			"-C", dir,
		}
		cmd := exec.Command("git", append(base, args...)...)
		cmd.WaitDelay = 5 * time.Second
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s 失败: %v\n%s", strings.Join(args, " "), err, out)
		}
	}

	run("init")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatalf("写 README 失败: %v", err)
	}
	run("add", "README.md")
	run("commit", "-m", "init")

	return dir
}

// Snapshot 记录一个目录当下的样子，用来证明某个操作**什么都没改**。
type Snapshot struct {
	status string
	files  []string
}

// SnapshotDir 拍下目录的 git status 与完整文件列表。
//
// 只比 git status 是不够的：Duet 要是往 .gitignore 里加一行、或者写进一个
// 已被忽略的路径，git status 依然干净——而用户的目录确实被动过了。
func SnapshotDir(t *testing.T, dir string) Snapshot {
	t.Helper()

	cmd := exec.Command("git", "-C", dir, "status", "--porcelain")
	cmd.WaitDelay = 5 * time.Second
	out, err := cmd.CombinedOutput()
	if err != nil {
		// 非 git 目录也要能拍快照——那时只比文件列表
		out = nil
	}

	var files []string
	err = filepath.WalkDir(dir, func(path string, _ os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return relErr
		}
		// .git 内部会因为 gc、reflog 等自行变动，不作为「用户文件被改了」的证据
		if rel == ".git" || strings.HasPrefix(rel, ".git"+string(filepath.Separator)) {
			return nil
		}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		t.Fatalf("遍历 %s 失败: %v", dir, err)
	}

	return Snapshot{status: string(out), files: files}
}

// AssertUnchanged 断言目录与快照时相比一个字节都没变。
func AssertUnchanged(t *testing.T, dir string, before Snapshot) {
	t.Helper()
	after := SnapshotDir(t, dir)

	if after.status != before.status {
		t.Errorf("用户仓库的 git status 变了——Duet 往里写东西了：\n  之前: %q\n  之后: %q",
			before.status, after.status)
	}
	if strings.Join(after.files, "\n") != strings.Join(before.files, "\n") {
		t.Errorf("用户目录的文件列表变了：\n  之前: %v\n  之后: %v", before.files, after.files)
	}
}
