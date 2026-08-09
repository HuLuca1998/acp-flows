package work

import (
	"context"
	"errors"
	"fmt"
	"github.com/HuLuca1998/acp-flows/backend/internal/app/work/reply"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// MemoryBodies 写记忆正文（INV-MEM-8：正文只在 md 文件里）。
type MemoryBodies interface {
	WriteBody(id, title, text string) error
	// TitleOf 读一条记忆的标题；读不到时返回空串。
	// 去重要用它——正文在文件里，索引表里没有标题可比。
	TitleOf(id string) string
}

// SetMemories 装上记忆存储。为 nil 时候选照解析但不落库——
// 那时用户重开应用候选就没了，所以装配必须给它。
func (s *Service) SetMemories(repo port.MemoryRepo, bodies MemoryBodies) {
	s.memories = repo
	s.memoryBodies = bodies
}

// captureMemory 把 AI 这一轮回复里的记忆候选收下来。
//
// ★★ **收下的一律是 candidate**，不是 active。
//
// AI 自己写 active 的话，一条它误解的「经验」会一直影响后面每一轮，
// 而用户从没同意过。这里做不到 active——`model.ProposeCandidate` 只造
// candidate，压根没有别的入口。
//
// ★ 每一轮都跑，绝大多数轮次会安静地什么都不做（回复里没有围栏）。
func (s *Service) captureMemory(ctx context.Context, workID, unitID, agentSay, projectScope string) {
	draft, err := reply.ParseMemoryReply(agentSay)
	if err != nil {
		if errors.Is(err, reply.ErrNoMemoryInReply) && !reply.HasFence(agentSay, reply.MemoryFence) {
			// 没写围栏 = 这一轮没经验可记，是常态
			return
		}
		// ★★ 写了围栏但解析不出来：**不静默**。
		// 咽下去的话，AI 想记的那条经验消失了而没有任何人知道。
		s.emit(ctx, workID, "memory_candidate", map[string]any{
			"error": err.Error(),
		})
		return
	}
	if s.memories == nil {
		return
	}

	// ★ 同一条正文重复冒出来时不重复建：AI 在一个单元里跑好几轮，
	// 常会把上一轮那条经验再讲一遍。每轮建一条的话，
	// 用户的待审核列表里会出现五条一模一样的。
	if id, dup := s.existingCandidate(ctx, draft, projectScope); dup {
		s.emit(ctx, workID, "memory_candidate", map[string]any{
			"memory_id": id, "duplicate": true, "title": draft.Title,
		})
		return
	}

	scope := model.MemoryScope(projectScope)
	if draft.CrossProject {
		scope = model.CrossProjectScope
	}
	// ★ 出处：AI 给的 source_refs 可能是空的，补上当前单元——
	// 没有出处的记忆过一个月谁也说不清它当初为什么成立。
	refs := draft.SourceRefs
	if len(refs) == 0 && unitID != "" {
		refs = []string{unitID}
	}
	if len(refs) == 0 {
		refs = []string{workID}
	}

	id := s.ids.NextID("mem")
	m, err := model.ProposeCandidate(id, draft.Kind, scope, refs, "agent")
	if err != nil {
		s.emit(ctx, workID, "memory_candidate", map[string]any{"error": err.Error()})
		return
	}

	// ★★ 正文先落盘再落库：反过来的话，索引里有这条而 md 不在，
	// 用户点开看到的是「正文文件不存在」——比没有这条记忆更糟。
	if s.memoryBodies != nil {
		if err := s.memoryBodies.WriteBody(id, draft.Title, draft.Text); err != nil {
			s.emit(ctx, workID, "memory_candidate", map[string]any{
				"error": fmt.Sprintf("正文写入失败: %v", err),
			})
			return
		}
	}
	if err := s.memories.SaveMemory(ctx, m); err != nil {
		s.emit(ctx, workID, "memory_candidate", map[string]any{"error": err.Error()})
		return
	}

	// ★ 事件里带 memory_id：审核那一步要靠它，不带的话
	// 用户在时间线上看到一条候选却点不动。
	s.emit(ctx, workID, "memory_candidate", map[string]any{
		"memory_id": id,
		"kind":      string(draft.Kind),
		"scope":     string(scope),
		"title":     draft.Title,
		"text":      draft.Text,
		// 状态一并发出去，让界面自己也能看出「这是候选，还没生效」
		"status": string(model.MemoryCandidate),
	})
}

// existingCandidate 找有没有标题一样的候选。
//
// ★ 判据是**标题**这个粗判据，不是语义相似度：后者要嵌入模型，
// 而那是另一个量级的东西。粗判据挡住的是最常见的那种重复
// （AI 把同一条经验原样再讲一遍）。
func (s *Service) existingCandidate(
	ctx context.Context, draft reply.MemoryDraft, projectScope string,
) (string, bool) {
	if s.memoryBodies == nil || draft.Title == "" {
		return "", false
	}
	scope := model.MemoryScope(projectScope)
	if draft.CrossProject {
		scope = model.CrossProjectScope
	}
	list, err := s.memories.ListMemories(ctx, port.MemoryFilter{
		Scope:  string(scope),
		Status: string(model.MemoryCandidate),
	})
	if err != nil {
		return "", false
	}
	for _, m := range list {
		if s.memoryBodies.TitleOf(m.ID()) == draft.Title {
			return m.ID(), true
		}
	}
	return "", false
}

// withMemoryCapture 把记忆提取套在一轮的 onReply 外面。
//
// ★★ **这是记忆候选唯一的入口。** 不套的话 `reply.ParseMemoryReply` 是一段
// 永远不会被调用的代码——单测全绿，而真实路径上一条候选都不会出现。
func (s *Service) withMemoryCapture(
	ctx context.Context, workID string, inner func(string),
) func(string) {
	return func(agentSay string) {
		// ★ 内层先跑：计划/契约的解析不该因为记忆这边出岔子而受影响。
		if inner != nil {
			inner(agentSay)
		}
		unitID := ""
		scope := workID
		if w, err := s.repo.FindWork(ctx, workID); err == nil && w != nil {
			unitID = w.CurrentUnitID()
			// ★ 记忆的归属是**项目**而不是工作：同一个项目下的第二个工作
			// 也该看得到第一个工作里学到的东西。
			if p := w.ProjectPath(); p != "" {
				scope = p
			}
		}
		s.captureMemory(ctx, workID, unitID, agentSay, scope)
	}
}
