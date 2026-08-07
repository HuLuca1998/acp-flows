// Package gitx 是 git 的基础设施封装：探测、worktree、diff、commit。
//
// ★ **本包对用户的仓库只读，除非调用方明确要求写。**
// 用户把自己的代码目录交给 Duet，加进来这个动作让他的 `git status` 多出东西，
// 是最快失去信任的方式。探测类函数一个字节都不写。
//
// worktree 建在 `~/.acpflows/worktrees`（open-questions Q30），
// **不在用户项目里**。
package gitx

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// probeTimeout 是每条 git 子命令的上限。
//
// git 在网络文件系统或超大仓库上会慢，但探测只跑本地元数据操作，
// 5 秒足够；卡住时宁可报错也不能让设置页一直转圈。
const probeTimeout = 5 * time.Second

// ErrNotADirectory 表示路径不存在或不是目录。
var ErrNotADirectory = errors.New("gitx: path is not a directory")

// Info 是一次探测的结论。
type Info struct {
	// IsRepo 报告这个目录是不是 git 仓库（含它是某个仓库的子目录的情况）。
	IsRepo bool
	// DefaultBranch 是当前 HEAD 指向的分支名；非仓库或空仓库时可能为空。
	DefaultBranch string
}

// Probe 探测一个本地目录。**只读，不写任何文件。**
//
// ★ 不是 git 仓库**不算错误**——返回 `IsRepo: false` 就好。
// 当成错误处理的话，用户得先去命令行 `git init` 才能用这个产品，
// 而他很可能正是不想碰命令行才来用的。
//
// 路径不存在或不是目录**才**是错误：静默当成「普通目录」的话，
// 用户会把一个打错的路径加进列表，直到真正开工时才发现。
func Probe(ctx context.Context, path string) (Info, error) {
	st, err := os.Stat(path)
	if err != nil {
		return Info{}, fmt.Errorf("%w: %s: %w", ErrNotADirectory, path, err)
	}
	if !st.IsDir() {
		return Info{}, fmt.Errorf("%w: %s", ErrNotADirectory, path)
	}

	// rev-parse --is-inside-work-tree 是最轻的判定，且不会创建任何东西。
	out, err := run(ctx, path, "rev-parse", "--is-inside-work-tree")
	if err != nil || strings.TrimSpace(out) != "true" {
		// 非仓库时 git 以非 0 退出，这是正常结论不是故障
		return Info{IsRepo: false}, nil
	}

	// 空仓库（刚 git init，还没有 commit）时 HEAD 指向一个不存在的引用，
	// 这条会以非 0 退出——那时分支名留空，不当作失败。
	// 这是「新建文件夹 → git init → 加进 Duet」的真实路径，不能崩。
	branch, _ := run(ctx, path, "rev-parse", "--abbrev-ref", "HEAD")

	return Info{IsRepo: true, DefaultBranch: strings.TrimSpace(branch)}, nil
}

// run 在 dir 下跑一条 git 子命令，返回 stdout。
func run(ctx context.Context, dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	// git 在需要凭据时会尝试从终端读——继承来的 stdin 会让它一直等下去。
	cmd.Stdin = nil
	// ctx 到期时 CommandContext 只 kill 直接子进程；git 会 fork
	// （credential helper、pager），孙子进程握着管道能把超时架空。
	// 见 scripts/check/check-naming.sh 第 8 节。
	cmd.WaitDelay = time.Second
	// 交互式凭据提示在服务进程里永远等不到人来输
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// RunForTest 供本包的测试搭建夹具用（例如现场 git init 一个空仓库）。
//
// 导出它是为了让测试不必自己再写一份 exec 封装——那份封装会漏掉
// WaitDelay 与 GIT_TERMINAL_PROMPT，于是测试在 CI 上偶发卡住。
func RunForTest(ctx context.Context, dir string, args ...string) error {
	_, err := run(ctx, dir, args...)
	return err
}
