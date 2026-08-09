package gitx

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrNothingToCommit 表示工作区没有可提交的改动。
//
// ★★ **不造空提交**：一个「验收通过」却什么都没改的单元，说明该被质疑的
// 是那次验收，而不是往历史里塞一个空 commit 把问题盖过去。
var ErrNothingToCommit = errors.New("gitx: 没有可提交的改动")

// Commit 把工作区里**全部**改动提交到当前分支。
//
// ★★ 只在**工作自己的 worktree** 里跑（调用方保证 path 是它）——
// 用户的主工作区一字不动，那是 M4 定下的规矩。
//
// ★ `git add -A` 会带上未跟踪的新文件：AI 干活时新建文件是常态，
// 不带的话提交里少了一半东西，而 diff 证据里明明有它们。
func Commit(ctx context.Context, path, message string) (string, error) {
	if strings.TrimSpace(message) == "" {
		return "", errors.New("gitx: 提交信息不能为空")
	}

	// ★ 先看有没有东西可提交。`git commit` 在没有改动时会失败，
	// 而那条错误里没有「为什么」——我们自己判一次，给得出人话。
	dirty, err := IsDirty(ctx, path)
	if err != nil {
		return "", err
	}
	if !dirty {
		return "", fmt.Errorf("%w: %s", ErrNothingToCommit, path)
	}

	if _, err := run(ctx, path, "add", "-A"); err != nil {
		return "", fmt.Errorf("gitx: 暂存改动: %w", err)
	}
	// ★ `--no-verify` **不加**：用户的 pre-commit 钩子该照常跑。
	// 绕过它等于替他决定「这次不用检查」——而他装那个钩子正是为了每次都检查。
	if _, err := run(ctx, path, "commit", "-m", message); err != nil {
		return "", fmt.Errorf("gitx: 提交: %w", err)
	}

	out, err := run(ctx, path, "rev-parse", "--short", "HEAD")
	if err != nil {
		// ★ 提交已经成功了，只是读不出 sha——**不返回错误**：
		// 返回错误会让调用方以为没提交，从而重试一次，
		// 而那一次会造出一个空提交或者第二个 commit。
		return "", nil
	}
	return strings.TrimSpace(out), nil
}
