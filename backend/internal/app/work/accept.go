package work

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
)

// ErrNoCommitter 表示没装配提交能力。
//
// ★ 明确报错而不是「验收通过但什么都没提交」：后者会让用户以为改动已经
// 落进分支，而它其实还散在工作区里——下一次 `git checkout` 就没了。
var ErrNoCommitter = errors.New("work: 没有配置提交能力")

// ErrNothingAccepted 表示这个单元一条标准都没有证据。
//
// ★★ 允许「零证据通过」的话，「验收」这个动作就没有内容了——
// 用户点通过时以为自己核对过什么，而实际上什么都没有。
var ErrNothingAccepted = errors.New("work: 一条验收标准都没有证据，不能通过")

// SetCommitter 装上提交能力。
func (s *Service) SetCommitter(c port.Committer) { s.committer = c }

// AcceptUnit 验收一个单元：提交改动 + 落检查点。
//
// ★★ **通过由用户点，不由 AI 判断**（M8 第 5 条）。这个方法只在他点了
// 之后跑——没有任何路径能自动走到这里。
func (s *Service) AcceptUnit(ctx context.Context, workID, unitID string) (string, error) {
	if s.committer == nil {
		return "", fmt.Errorf("%w: %s", ErrNoCommitter, workID)
	}

	// ★ 先看有没有证据。一条都没有就不该通过——
	// 那时「验收」这个动作没有内容，用户以为自己核对过什么。
	view, err := s.AcceptanceOf(ctx, workID, unitID)
	if err != nil {
		return "", err
	}
	if !anyCovered(view) {
		return "", fmt.Errorf("%w: %s", ErrNothingAccepted, unitID)
	}

	w, err := s.repo.FindWork(ctx, workID)
	if err != nil {
		return "", fmt.Errorf("查工作 %s: %w", workID, err)
	}

	// ★★ 提交发生在**工作自己的 worktree** 上——用户的分支一字不动。
	sha, err := s.committer.Commit(ctx, w.WorktreePath(), acceptMessage(unitID, view))
	if err != nil {
		return "", fmt.Errorf("提交单元 %s: %w", unitID, err)
	}

	// ★ 落检查点：用户回头想接着干，得有东西告诉他「上次做到哪」。
	// 绑着那个 commit——不绑的话「恢复到哪」没有答案。
	s.emit(ctx, workID, "checkpoint", map[string]any{
		"reason": "unit_accepted", "unit": unitID, "commit": sha,
	})
	return sha, nil
}

// anyCovered 报告有没有**至少一条**标准拿到了证据。
func anyCovered(view AcceptanceView) bool {
	for _, c := range view.Criteria {
		if len(c.EvidenceIDs) > 0 {
			return true
		}
	}
	return false
}

// acceptMessage 造提交信息。
//
// ★★ 带上**单元 id 与已覆盖的标准数**：用户日后 `git log` 要能看懂
// 那一次提交是哪个单元、凭什么算通过的。只写「验收通过」的话，
// 三个月后那条 commit 什么都说明不了。
func acceptMessage(unitID string, view AcceptanceView) string {
	var covered int
	for _, c := range view.Criteria {
		if len(c.EvidenceIDs) > 0 {
			covered++
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "feat(%s): 验收通过\n\n", unitID)
	fmt.Fprintf(&b, "验收标准 %d 条，其中 %d 条有证据。\n", len(view.Criteria), covered)
	for _, c := range view.Criteria {
		mark := "○"
		if len(c.EvidenceIDs) > 0 {
			mark = "✓ " + strings.Join(c.EvidenceIDs, " ")
		}
		fmt.Fprintf(&b, "- [%s] %s %s\n", c.ID, c.Text, mark)
	}
	return b.String()
}
