package work

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// ErrNoStatusProbe 表示没有配置仓库状态探测。
//
// ★ 报错而不是返回空状态：空状态会让弹层显示「仓库很干净」，
// 而实际我们根本没查过——用户据此开工，可能丢掉他没提交的改动。
var ErrNoStatusProbe = errors.New("work: 没有配置仓库状态探测")

// WithStatusProbe 装上仓库状态探测。
func (s *Service) WithStatusProbe(p port.RepoStatusProbe) *Service {
	s.status = p
	return s
}

// Prepare 返回开工前的仓库状态。**一个字节都不写。**
func (s *Service) Prepare(ctx context.Context, project string) (port.RepoStatus, error) {
	if !filepath.IsAbs(project) {
		return port.RepoStatus{}, fmt.Errorf("%w: %q", model.ErrProjectPathNotAbsolute, project)
	}
	if s.status == nil {
		return port.RepoStatus{}, ErrNoStatusProbe
	}
	return s.status.ProbeRepoStatus(ctx, project)
}

// WorktreeOf 返回一个工作的 git 现场。
//
// ★★ 基线从**工作记录里读**，不重新猜：猜的话，
// 「本次工作改了什么」会随着仓库变化而漂移，
// 而那正是用户拿来判断「AI 到底干了什么」的东西。
func (s *Service) WorktreeOf(ctx context.Context, workID string) (port.WorktreeState, error) {
	if s.status == nil {
		return port.WorktreeState{}, ErrNoStatusProbe
	}
	w, err := s.repo.FindWork(ctx, workID)
	if err != nil {
		return port.WorktreeState{}, err
	}
	path, base := w.WorktreePath(), w.BaseCommit()
	if path == "" {
		// ★ 还没切 worktree（initializing / initializing_failed）——
		// 这不是错误，界面据此显示「还没有工作区」而不是一片报错。
		return port.WorktreeState{}, nil
	}
	return s.status.ProbeWorktreeState(ctx, path, base)
}
