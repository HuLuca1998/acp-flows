package gitx

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrNotARepo 表示目标目录不是 git 仓库。
var ErrNotARepo = errors.New("gitx: not a git repository")

// ErrBranchTaken 表示分支已存在，且**不是这个工作的 worktree 在用**。
//
// ★★ 绝不覆盖：用户可能自己手建过同名分支，上面躺着他的提交。
// `git worktree add -B` 会把它**强制复位到 HEAD**——那些提交就没了，
// 而他不会立刻发现（分支还在，只是内容变了）。
var ErrBranchTaken = errors.New("gitx: 分支已存在且被别处占用")

// WorktreeSpec 描述要建一个什么样的 worktree。
type WorktreeSpec struct {
	// Repo 是用户的项目目录。
	Repo string
	// Root 是 worktree 的存放根目录。
	//
	// ★ 这里是 `~/.acpflows/worktrees`，**不在用户项目里**
	// （open-questions.md Q30）。用户把代码目录交给 Duet 时，并没有同意
	// 我们在他的仓库里造一堆分支和目录。
	Root string
	// WorkID 决定目录名，同一个工作永远拿到同一个目录。
	WorkID string
	// Branch 是这个工作的分支名。
	Branch string
	// BaseRef 是从哪儿开分支：分支名或 commit。留空时用仓库当前 HEAD。
	//
	// ★ 让用户选基线是 `M4` 完成标志第 3 条——他可能想从 `develop`
	// 而不是当前分支开工，而当前分支上可能正躺着他没提交完的东西。
	BaseRef string
}

// Worktree 是一个已建好的工作区。
type Worktree struct {
	Path   string
	Branch string
	// BaseCommit 是创建时的基线 commit（短 SHA）。
	//
	// ★ 记下来才能回答「这个工作是从哪儿开始的」——
	// 右栏的「领先几个 commit」与验收时的 diff 都要它当起点。
	BaseCommit string
}

// AddWorktree 为一个工作建独立工作区。
//
// ★ **幂等**：同一个 WorkID 重复调用返回已有的那个，不报错。
// 重启后恢复工作时会走到这条路——报错的话用户的工作就再也打不开了。
//
// ★ 这是 Duet 唯一会大量写文件的地方，而它写的**全在 Root 下**。
// 用户项目里除了 `.git` 的 worktree 账本之外一个字节都不多。
func AddWorktree(ctx context.Context, spec WorktreeSpec) (Worktree, error) {
	info, err := Probe(ctx, spec.Repo)
	if err != nil {
		return Worktree{}, err
	}
	if !info.IsRepo {
		// 明确报错而不是让 git 吐一句英文——上层要据此告诉用户「先 git init」
		return Worktree{}, fmt.Errorf("%w: %s", ErrNotARepo, spec.Repo)
	}

	path := filepath.Join(spec.Root, spec.WorkID)

	// 已经建过就直接返回。判据是目录在不在，而不是查 git 的 worktree 列表——
	// 后者在仓库被手工清理过之后会与磁盘状态不一致。
	if st, statErr := os.Stat(path); statErr == nil && st.IsDir() {
		return Worktree{Path: path, Branch: spec.Branch}, nil
	}

	if err := os.MkdirAll(spec.Root, 0o700); err != nil {
		return Worktree{}, fmt.Errorf("建 worktree 根目录 %s: %w", spec.Root, err)
	}

	// ★★ **分支已存在时不覆盖**。
	//
	// 原来这里用 `-B`（存在就复位到 HEAD）。那对「恢复一个曾经建过的工作」
	// 是对的，但对「用户自己手建过同名分支」是灾难：它上面躺着的提交
	// 会被静默丢掉，而分支还在——他不会立刻发现。
	//
	// 走到这一步说明 worktree 目录不存在（上面已经判过），
	// 所以一个已存在的同名分支只可能是**别处的**。
	if branchExists(ctx, spec.Repo, spec.Branch) {
		return Worktree{}, fmt.Errorf("%w: %s", ErrBranchTaken, spec.Branch)
	}

	// 基线：用户选的，或仓库当前 HEAD。
	base := spec.BaseRef
	if base == "" {
		base = "HEAD"
	}
	baseCommit, err := run(ctx, spec.Repo, "rev-parse", "--short", base)
	if err != nil {
		// ★ 基线解析不出来就**不建**：用一个我们没搞懂的起点开工，
		// 之后「领先几个 commit」「本次工作的 diff」全都是错的。
		return Worktree{}, fmt.Errorf("解析基线 %q: %w", base, err)
	}

	if _, err := run(ctx, spec.Repo, "worktree", "add", "-b", spec.Branch, path, base); err != nil {
		return Worktree{}, fmt.Errorf("建 worktree %s: %w", path, err)
	}

	return Worktree{
		Path:       path,
		Branch:     spec.Branch,
		BaseCommit: strings.TrimSpace(baseCommit),
	}, nil
}

// branchExists 报告本地有没有这个分支。
func branchExists(ctx context.Context, repo, branch string) bool {
	_, err := run(ctx, repo, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

// RemoveWorktree 移除一个工作区。
//
// ★ **幂等**：移除不存在的不报错。清理路径可能被走两遍
// （正常结束一次、崩溃后重启清理一次）。
//
// 只删 worktree 目录本身，**不碰用户项目里的任何文件**。
func RemoveWorktree(ctx context.Context, repo, path string) error {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	}

	// --force：worktree 里通常有未提交的改动（AI 干到一半），
	// 不加的话 git 会拒绝，而那些改动本来就是要丢的
	if _, err := run(ctx, repo, "worktree", "remove", "--force", path); err != nil {
		// git 拒绝时兜底直接删目录：worktree 的账本可以事后 prune，
		// 而留着一个删不掉的目录会让下一次同名工作永远建不起来
		if rmErr := os.RemoveAll(path); rmErr != nil {
			return fmt.Errorf("移除 worktree %s: %w", path, err)
		}
		_, _ = run(ctx, repo, "worktree", "prune")
	}
	return nil
}

// ListWorktrees 列出这个仓库下 Duet 建的工作区路径。
//
// 用于启动时清理孤儿——上次崩溃留下的 worktree 会一直占着磁盘，
// 而用户不知道它们的存在。
func ListWorktrees(ctx context.Context, repo, root string) ([]string, error) {
	out, err := run(ctx, repo, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("列 worktree: %w", err)
	}

	var paths []string
	for line := range strings.SplitSeq(out, "\n") {
		p, ok := strings.CutPrefix(strings.TrimSpace(line), "worktree ")
		if !ok {
			continue
		}
		// 只认 root 底下的——用户自己建的 worktree 不归 Duet 管
		if strings.HasPrefix(p, root) {
			paths = append(paths, p)
		}
	}
	return paths, nil
}

// IsDirty 报告工作区有没有未提交的改动（含未跟踪文件）。
//
// ★ 用 `status --porcelain` 而不是 `diff --quiet`：后者看不到**未跟踪文件**，
// 而「用户在 Duet 关着的时候新建了一个文件」正是最典型的场景。
//
// ★ 未跟踪也算脏。它们不会被 git 操作冲掉，但恢复之后 AI 会在同一个目录里
// 干活，很可能覆盖同名文件——先告诉用户一声的成本远低于事后。
func IsDirty(ctx context.Context, path string) (bool, error) {
	out, err := run(ctx, path, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("查工作区状态 %s: %w", path, err)
	}
	return strings.TrimSpace(out) != "", nil
}
