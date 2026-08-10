// Package memory 提供记忆的查询与人工审核。
package memory

import (
	"context"
	"errors"
	"fmt"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// ErrNoRepo 表示没有配置仓储。
var ErrNoRepo = errors.New("memory: 没有配置仓储")

// ErrNoActor 表示审核动作没带人。
//
// ★★ 这就是 INV-MEM-2 里那个「用户确认动作」在应用层的落点。
// 允许匿名确认的话，一个后台任务就能把 AI 提的候选变成长期记忆——
// 而 AGENTS.md §9 把「自动把一次成功经验写成长期记忆」列为明令反例。
var ErrNoActor = errors.New("memory: 审核记忆必须带上是谁做的决定")

// Service 是记忆用例。
type Service struct {
	repo port.MemoryRepo
}

// New 造一个服务。
func New(repo port.MemoryRepo) *Service {
	return &Service{repo: repo}
}

// List 按条件列出记忆。
func (s *Service) List(ctx context.Context, f port.MemoryFilter) ([]*model.Memory, error) {
	if s.repo == nil {
		return nil, ErrNoRepo
	}
	return s.repo.ListMemories(ctx, f)
}

// Confirm 把一条候选确认为生效。★ actor 必填——见 ErrNoActor。
func (s *Service) Confirm(ctx context.Context, id, actor string) (*model.Memory, error) {
	return s.review(ctx, id, actor, func(m *model.Memory) error { return m.Confirm(actor) })
}

// Reject 否决一条候选。
//
// ★ 同样要求 actor：否决也是**人做的决定**，半年后要查得到是谁否的。
func (s *Service) Reject(ctx context.Context, id, actor string) (*model.Memory, error) {
	return s.review(ctx, id, actor, func(m *model.Memory) error { return m.Reject() })
}

func (s *Service) review(
	ctx context.Context, id, actor string, apply func(*model.Memory) error,
) (*model.Memory, error) {
	if s.repo == nil {
		return nil, ErrNoRepo
	}
	if actor == "" {
		return nil, fmt.Errorf("%w: %s", ErrNoActor, id)
	}

	m, err := s.repo.FindMemory(ctx, id)
	if err != nil {
		return nil, err
	}
	// ★ 状态机的守卫在 domain 里，这里不重复判断——
	// 判两遍必然漂移，而漂移的那一侧会静默放行。
	if err := apply(m); err != nil {
		return nil, err
	}
	if err := s.repo.SaveMemory(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}
