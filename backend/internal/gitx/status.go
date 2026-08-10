package gitx

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var (
	// ErrMidOperation 表示仓库正处在 rebase / merge / cherry-pick 中途。
	//
	// ★★ **那时不能开工**：切 worktree 会把一个做了一半的操作丢在那儿，
	// 而用户很可能正在解冲突。他回来时会发现自己的现场不见了。
	ErrMidOperation = errors.New("gitx: 仓库正处在一次未完成的操作中")

	// ErrNoCommits 表示空仓库（刚 git init，还没有 commit）。
	//
	// ★ 单独一个错误而不是当成「没有分支」：worktree 需要一个基线 commit，
	// 而空仓库根本没有。含糊报「分支不存在」会让用户去建分支，
	// 而他真正要做的是先提交一次。
	ErrNoCommits = errors.New("gitx: 仓库还没有任何 commit")
)

// Status 是开工前的仓库状态。
type Status struct {
	// CurrentBranch 是 HEAD 所在的分支。
	CurrentBranch string
	// Branches 是本地分支，按名字排序。
	Branches []string
	// HeadCommit 是 HEAD 的短 SHA。
	HeadCommit string

	// TrackedDirty 是**已跟踪文件**里被改动的数量。
	//
	// ★★ 与未跟踪分开数（下一个字段）：合成一个「有 N 处改动」的话，
	// 「我只是新建了几个还没 add 的文件」和「我改了正在跟踪的代码」
	// 会长得一模一样，而对用户是两件完全不同的事。
	TrackedDirty int
	// Untracked 是**未跟踪文件**的数量。
	Untracked int
}

// IsDirty 报告工作区有没有未提交的改动（含未跟踪）。
func (s Status) IsDirty() bool { return s.TrackedDirty > 0 || s.Untracked > 0 }

// ProbeStatus 探测开工前的仓库状态。**只读，一个字节都不写。**
//
// ★ 中途态（rebase / merge / cherry-pick）**报错拒绝开工**，
// 不是「如实报告然后照样开」：那时切 worktree 会丢掉用户正在解的冲突。
func ProbeStatus(ctx context.Context, path string) (Status, error) {
	// 先确认是仓库。不是的话上层要给的是「这不是 git 仓库」，
	// 而不是一串 git 的报错。
	info, err := Probe(ctx, path)
	if err != nil {
		return Status{}, err
	}
	if !info.IsRepo {
		return Status{}, fmt.Errorf("%w: %s", ErrNotADirectory, path)
	}

	if op := midOperation(ctx, path); op != "" {
		return Status{}, fmt.Errorf("%w: %s", ErrMidOperation, op)
	}

	head, err := run(ctx, path, "rev-parse", "--short", "HEAD")
	if err != nil {
		// ★ 空仓库时 rev-parse HEAD 以非 0 退出。**这不是故障**——
		// 「新建文件夹 → git init → 加进 Duet」是真实路径。
		return Status{}, ErrNoCommits
	}

	st := Status{
		CurrentBranch: info.DefaultBranch,
		HeadCommit:    strings.TrimSpace(head),
	}

	branches, err := run(ctx, path, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return Status{}, fmt.Errorf("列本地分支: %w", err)
	}
	for _, b := range strings.Split(strings.TrimSpace(branches), "\n") {
		if b = strings.TrimSpace(b); b != "" {
			st.Branches = append(st.Branches, b)
		}
	}

	// `--porcelain` 的输出是稳定的机器格式；`-uall` 让未跟踪文件逐个列出
	// 而不是只报一个目录——用户要看的是「几个文件」。
	out, err := run(ctx, path, "status", "--porcelain", "-uall")
	if err != nil {
		return Status{}, fmt.Errorf("查工作区状态: %w", err)
	}
	st.TrackedDirty, st.Untracked = countDirty(out)

	return st, nil
}

// countDirty 数出已跟踪的改动与未跟踪的文件。
//
// `git status --porcelain` 每行前两列是状态码：
//
//	`?? path`  未跟踪
//	` M path`  已跟踪、工作区有改动
//	`M  path`  已跟踪、已暂存
//	`MM path`  两者都有
func countDirty(out string) (tracked, untracked int) {
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 2 {
			continue
		}
		if line[:2] == "??" {
			untracked++
			continue
		}
		tracked++
	}
	return tracked, untracked
}

// midOperation 报告仓库处在哪个未完成的操作里；没有则返回空串。
//
// ★ 用 `git rev-parse --git-path` 问 git 要路径，**不自己拼 `.git/…`**：
// worktree 与 submodule 里 `.git` 是文件，真正的目录在别处，
// 自己拼的话这些仓库永远检测不出中途态。
func midOperation(ctx context.Context, path string) string {
	for _, probe := range []struct{ marker, name string }{
		{"rebase-merge", "rebase"},
		{"rebase-apply", "rebase"},
		{"MERGE_HEAD", "merge"},
		{"CHERRY_PICK_HEAD", "cherry-pick"},
		{"REVERT_HEAD", "revert"},
		{"BISECT_LOG", "bisect"},
	} {
		p, err := run(ctx, path, "rev-parse", "--git-path", probe.marker)
		if err != nil {
			continue
		}
		if pathExistsIn(ctx, path, strings.TrimSpace(p)) {
			return probe.name
		}
	}
	return ""
}

// pathExistsIn 判断 git 给出的路径存不存在。
//
// ★ `--git-path` 返回的可能是相对仓库根的路径（普通仓库）
// 也可能是绝对路径（worktree）。两种都要认。
func pathExistsIn(_ context.Context, root, p string) bool {
	if p == "" {
		return false
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(root, p)
	}
	_, err := os.Stat(p)
	return err == nil
}

// WorktreeState 是一个工作区的 git 现场，右栏「工作区」照它渲染。
type WorktreeState struct {
	// Branch 是工作区所在的分支。
	Branch string
	// BaseCommit 是这个工作的起点（短 SHA）。
	BaseCommit string
	// Ahead 是相对基线领先几个 commit。
	Ahead int
	// Changes 是未提交的改动，逐个文件。
	Changes []FileChange
	// Commits 是**本次工作产生的** commit（基线之后的）。
	//
	// ★ 基线之前的不算：那些是用户自己的历史，混进来会让他
	// 以为 Duet 改了他早先的提交。
	Commits []CommitInfo
}

// FileChange 是一个文件的改动。
type FileChange struct {
	Path string
	// Added / Removed 是增删行数。
	//
	// ★ 设计稿要的是 `+64 −12` 这个形态——只说「改了 3 个文件」的话，
	// 用户判断不出这次改动有多大。
	Added   int
	Removed int
}

// CommitInfo 是一条 commit。
type CommitInfo struct {
	SHA     string
	Subject string
	// When 是相对时间的原文（`2 minutes ago`）。
	//
	// ★ 让 git 算相对时间而不是我们算：它处理了时区与本地化，
	// 而我们自己算会在跨时区时差一天。
	When string
}

// ProbeWorktree 读出一个工作区的现场。**只读。**
//
// ★ base 是这个工作的基线 commit。传空时只报分支与未提交改动——
// 没有基线就算不出「领先几个」与「本次工作的 commit」，
// 而**编一个 0 出来比不报更糟**。
func ProbeWorktree(ctx context.Context, path, base string) (WorktreeState, error) {
	info, err := Probe(ctx, path)
	if err != nil {
		return WorktreeState{}, err
	}
	if !info.IsRepo {
		return WorktreeState{}, fmt.Errorf("%w: %s", ErrNotARepo, path)
	}

	st := WorktreeState{Branch: info.DefaultBranch, BaseCommit: base}

	// 未提交改动：--numstat 给出每个文件的增删行数。
	// `HEAD` 比较的是工作区与最后一次提交。
	if out, err := run(ctx, path, "diff", "--numstat", "HEAD"); err == nil {
		st.Changes = parseNumstat(out)
	}

	// ★★ **未跟踪的新文件也算改动。**
	//
	// `git diff` 不认识它们——而 AI 干活时新建文件是常态。不算的话，
	// 一个新写了三个文件的单元，右栏与验收证据都显示「改了 0 个文件」，
	// 而用户会以为它什么都没做。
	//
	// ★ 增删行数按「全是新增」算：它对 git 来说还不存在，
	// 所以每一行都是新的。
	st.Changes = append(st.Changes, untrackedChanges(ctx, path)...)

	if base == "" {
		// ★ 早返回守的是「**不依赖 git 对空 ref 的解释**」。
		//
		// 造负例时发现拆掉它测试照样绿——因为当前的 git 对 `..HEAD`
		// 也是拒绝的，`err != nil` 让两个字段留在零值。
		// 但那是**它今天的行为**：某个版本把 `..HEAD` 解释成全部历史的话，
		// 我们会把用户自己的全部提交报成「本次工作的 commit」。
		//
		// 这一条留着，且这段注释就是它的说明——不是测不出来就该删。
		return st, nil
	}

	// 领先几个 commit + 本次工作的 commit 列表。
	if out, err := run(ctx, path, "rev-list", "--count", base+"..HEAD"); err == nil {
		st.Ahead = atoiSafe(strings.TrimSpace(out))
	}
	if out, err := run(ctx, path, "log", "--format=%h%x1f%s%x1f%cr", base+"..HEAD"); err == nil {
		st.Commits = parseCommits(out)
	}

	return st, nil
}

// parseNumstat 解析 `git diff --numstat` 的输出。
//
// 每行是 `<added>\t<removed>\t<path>`；二进制文件的增删是 `-`。
func parseNumstat(out string) []FileChange {
	var changes []FileChange
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		// ★ 二进制文件的 `-` 记成 0 而不是跳过：跳过的话，
		// 用户会看到「改了 2 个文件」而实际动了 3 个。
		changes = append(changes, FileChange{
			Path:    parts[2],
			Added:   atoiSafe(parts[0]),
			Removed: atoiSafe(parts[1]),
		})
	}
	return changes
}

// parseCommits 解析 `git log --format=%h\x1f%s\x1f%cr` 的输出。
//
// ★ 用 \x1f（单元分隔符）而不是常见的 `|` 或 `:`：
// commit 标题里出现竖线和冒号是很正常的事，而 \x1f 不可能出现在里面。
func parseCommits(out string) []CommitInfo {
	var commits []CommitInfo
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x1f", 3)
		if len(parts) != 3 {
			continue
		}
		commits = append(commits, CommitInfo{SHA: parts[0], Subject: parts[1], When: parts[2]})
	}
	return commits
}

// atoiSafe 把字符串转成整数，转不动返回 0。
func atoiSafe(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}

// untrackedChanges 把未跟踪的新文件也变成改动条目。
//
// ★ `--others --exclude-standard` 是「未跟踪且没被 .gitignore 忽略的」——
// 带上被忽略的话，`node_modules` 会把证据淹掉。
func untrackedChanges(ctx context.Context, path string) []FileChange {
	out, err := run(ctx, path, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil
	}

	var changes []FileChange
	for _, name := range strings.Split(strings.TrimSpace(out), "\n") {
		if name = strings.TrimSpace(name); name == "" {
			continue
		}
		changes = append(changes, FileChange{Path: name, Added: countLines(path, name)})
	}
	return changes
}

// countLines 数一个新文件有多少行。读不出来时返回 0——
// ★ 二进制文件、权限不足都会走到这里，而**报 0 行比报错更合适**：
// 那个文件确实在改动列表里，只是行数说不清。
func countLines(repo, name string) int {
	data, err := os.ReadFile(filepath.Join(repo, name))
	if err != nil {
		return 0
	}
	if len(data) == 0 {
		return 0
	}
	n := strings.Count(string(data), "\n")
	if !strings.HasSuffix(string(data), "\n") {
		n++ // 最后一行没有换行符
	}
	return n
}
