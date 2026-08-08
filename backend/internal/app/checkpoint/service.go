// Package checkpoint 回答两个问题：「有哪些工作能接着做」「接着做之前要不要先提醒他」。
//
// ★ 最要紧的一条：**工作区被手工改动过时先告知，不静默覆盖**。
// 用户在 Duet 关着的时候自己改了几行，恢复时被冲掉——那是不可挽回的损失，
// 而他甚至不知道发生过。
package checkpoint

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/internal/constant"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// ErrWorktreeDirty 表示工作区有未提交的改动，恢复前要先让用户知道。
//
// ★ 拿到它的调用方**必须问过用户再走 ResumeForce**，不能自己决定继续。
var ErrWorktreeDirty = errors.New("checkpoint: 工作区有未提交的改动")

// ErrNotResumable 表示这个工作现在不能恢复（不是 paused）。
var ErrNotResumable = errors.New("checkpoint: 这个工作不是暂停状态")

// Resumable 是一条可恢复的工作。
type Resumable struct {
	WorkID string
	// PausedAt 是暂停时间。用户开着三四个工作时靠它认出「哪个是刚才那个」。
	PausedAt time.Time
}

// View 是恢复之后交给上层的视图。
type View struct {
	WorkID string
	State  constant.WorkState
}

// DirtyChecker 查一个工作区有没有未提交的改动。接口定义在使用方。
type DirtyChecker func(ctx context.Context, path string) (bool, error)

// Service 是检查点用例。
type Service struct {
	repo      port.WorkRepo
	worktrees port.WorktreeLocator
	isDirty   DirtyChecker
}

// New 组装。isDirty 留空时用 gitx 的实现——由 cmd 装配传进来。
func New(repo port.WorkRepo, wt port.WorktreeLocator, opts ...Option) *Service {
	s := &Service{repo: repo, worktrees: wt}
	for _, o := range opts {
		o(s)
	}
	if s.isDirty == nil {
		// ★ 默认**当成脏的**：查不了就先问用户，别替他决定。
		// 默认干净的话，一个查不动的工作区会被静默覆盖。
		s.isDirty = func(context.Context, string) (bool, error) {
			return true, errors.New("checkpoint: 没有配置工作区检查")
		}
	}
	return s
}

// Option 是构造选项。
type Option func(*Service)

// WithDirtyChecker 装上工作区脏净检查。
func WithDirtyChecker(f DirtyChecker) Option {
	return func(s *Service) { s.isDirty = f }
}

// ListResumable 列出能接着做的工作。
//
// ★ **只列 paused**。把跑完的、失败的也列出来的话，用户对着一串条目
// 不知道该点哪个——而点进去发现「没什么可恢复的」比列表里没有它更让人困惑。
func (s *Service) ListResumable(ctx context.Context) ([]Resumable, error) {
	works, err := s.repo.ListWorks(ctx)
	if err != nil {
		return nil, fmt.Errorf("列出工作: %w", err)
	}

	// 空结果返回空切片而不是 nil：api 层要序列化成 [] 而不是 null，
	// 前端对 null 调 .map() 会白屏
	out := make([]Resumable, 0)
	for _, w := range works {
		if w.State() != constant.WorkStatePaused {
			continue
		}
		out = append(out, Resumable{
			WorkID: w.ID(),
			// 暂停时间暂用当前时间——落库的时间戳是 U4.1.2 后续的事，
			// 但**字段现在就得在**，否则界面渲染不出来（列表里认不出哪个是哪个）
			PausedAt: time.Now(),
		})
	}
	return out, nil
}

// Resume 恢复一个工作。工作区脏时返回 ErrWorktreeDirty，**不改任何状态**。
func (s *Service) Resume(ctx context.Context, workID string) (View, error) {
	return s.resume(ctx, workID, false)
}

// ResumeForce 在用户确认之后恢复，跳过脏检查。
//
// ★ 「跳过检查」不等于「覆盖改动」——我们本来就不动用户的文件，
// 只是把工作的状态推回可跑。
func (s *Service) ResumeForce(ctx context.Context, workID string) (View, error) {
	return s.resume(ctx, workID, true)
}

func (s *Service) resume(ctx context.Context, workID string, force bool) (View, error) {
	w, err := s.repo.FindWork(ctx, workID)
	if err != nil {
		return View{}, fmt.Errorf("查工作 %s: %w", workID, err)
	}
	if w.State() != constant.WorkStatePaused {
		return View{}, fmt.Errorf("%w: 工作 %s 处于 %s", ErrNotResumable, workID, w.State())
	}

	// ★ 工作区找不到就报错，不当成「干净」放行——恢复会在一个不存在的
	// 目录上继续，AI 的第一个工具调用就炸，而错误离原因已经很远了。
	path, err := s.worktrees.WorktreePath(ctx, workID)
	if err != nil {
		return View{}, fmt.Errorf("找工作 %s 的工作区: %w", workID, err)
	}

	if !force {
		dirty, err := s.isDirty(ctx, path)
		if err != nil {
			return View{}, fmt.Errorf("查工作区 %s: %w", path, err)
		}
		if dirty {
			// ★ **一个字节都不改**。「告知」的意思是「还没恢复」——
			// 先斩后奏的话，用户点「取消」也来不及了。
			return View{}, fmt.Errorf("%w: %s", ErrWorktreeDirty, path)
		}
	}

	if err := w.Transition(constant.WorkStateExecuting); err != nil {
		return View{}, fmt.Errorf("工作 %s 状态迁移: %w", workID, err)
	}
	if err := s.repo.SaveWork(ctx, w); err != nil {
		return View{}, fmt.Errorf("保存工作 %s: %w", workID, err)
	}
	return View{WorkID: workID, State: w.State()}, nil
}

var _ = model.ErrNotFound // 让 model 的导入有意义：FindWork 的错误由它定义
