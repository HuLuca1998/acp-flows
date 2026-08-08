package model

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrInvalidMemoryKind 表示记忆类型不在全集内。
	ErrInvalidMemoryKind = errors.New("model: 非法的记忆类型")

	// ErrMemoryTransition 表示这条状态迁移不被允许（INV-MEM-4）。
	ErrMemoryTransition = errors.New("model: 不允许的记忆状态迁移")

	// ErrMemoryNeedsUserAction 表示晋升为 active 缺少用户确认动作。
	//
	// ★★ **绝不自动写入**（INV-MEM-2）。这条在三处被写死：
	// `AGENTS.md` §9 的反例、设计规范事件表的「绝不自动写入」、
	// `architecture.md` 的 `memory_candidate` 事件。
	//
	// 自动晋升的后果不是「多了一条记忆」，而是**AI 把自己的一次臆断
	// 变成了以后每一轮的前提**——而用户从没看过那句话。
	ErrMemoryNeedsUserAction = errors.New("model: candidate → active 必须有用户确认动作")

	// ErrMemoryNoSourceRefs 表示记忆没有证据支撑（INV-MEM-3）。
	ErrMemoryNoSourceRefs = errors.New("model: 记忆必须有 source_refs（指向 Evidence 或 Unit）")

	// ErrMemoryNeedsReason 表示废弃没给理由（INV-MEM-7）。
	ErrMemoryNeedsReason = errors.New("model: 废弃记忆必须说明理由")

	// ErrMemorySelfSupersede 表示 supersedes 指向了自己（INV-MEM-7）。
	ErrMemorySelfSupersede = errors.New("model: supersedes 不能指向自身")
)

// MemoryKind 是记忆的类型（domain-model.md §14.3）。
type MemoryKind string

const (
	// MemoryConstraint 约束：这个项目**不许**怎么做。
	MemoryConstraint MemoryKind = "constraint"
	// MemoryExperience 经验：这样做过，管用。
	MemoryExperience MemoryKind = "experience"
	// MemoryFact 事实：这个项目就是这样的。
	MemoryFact MemoryKind = "fact"
)

var allMemoryKinds = [...]MemoryKind{MemoryConstraint, MemoryExperience, MemoryFact}

// AllMemoryKinds 返回类型全集。界面上的筛选器照它渲染。
func AllMemoryKinds() []MemoryKind {
	out := make([]MemoryKind, len(allMemoryKinds))
	copy(out, allMemoryKinds[:])
	return out
}

// IsValid 报告取值是否在全集内。
func (k MemoryKind) IsValid() bool {
	for _, v := range allMemoryKinds {
		if v == k {
			return true
		}
	}
	return false
}

// MemoryStatus 是记忆的状态（domain-model.md §14.4）。
//
// ★ **五态，不是设计稿上那三个。** 设计稿的筛选器只有
// `active` / `候选` / `已失效` 三档——那是**界面的分组**：
// 「已失效」这一档同时装着 `invalid` 与 `obsolete`。
// 两者对用户长得一样，对系统不一样（废弃要带理由、可指向 supersedes）。
type MemoryStatus string

const (
	// MemoryCandidate 候选：AI 提的，**等人拍板**。
	MemoryCandidate MemoryStatus = "candidate"
	// MemoryActive 生效：会被注入。
	MemoryActive MemoryStatus = "active"
	// MemoryDiscarded 已否决：用户看过，不要。
	MemoryDiscarded MemoryStatus = "discarded"
	// MemoryInvalid 已失效：不再注入，但历史运行仍可追溯当时用过它。
	MemoryInvalid MemoryStatus = "invalid"
	// MemoryObsolete 已废弃：必须给理由，可指向 supersedes。
	MemoryObsolete MemoryStatus = "obsolete"
)

var allMemoryStatuses = [...]MemoryStatus{
	MemoryCandidate, MemoryActive, MemoryDiscarded, MemoryInvalid, MemoryObsolete,
}

// AllMemoryStatuses 返回状态全集。
func AllMemoryStatuses() []MemoryStatus {
	out := make([]MemoryStatus, len(allMemoryStatuses))
	copy(out, allMemoryStatuses[:])
	return out
}

// IsValid 报告取值是否在全集内。
func (s MemoryStatus) IsValid() bool {
	for _, v := range allMemoryStatuses {
		if v == s {
			return true
		}
	}
	return false
}

// allowedMemoryTransitions 是 INV-MEM-4 的全部四条迁移。
//
// ★ 没有 `active → candidate`（退回候选）：那等于让一条已经被人确认过的
// 记忆重新变成待审，而它可能已经被注入过几十轮了。要撤就走 invalid。
//
// ★ 没有任何指向 `candidate` 的边——candidate 只能是**创建时**的状态。
var allowedMemoryTransitions = map[MemoryStatus][]MemoryStatus{
	MemoryCandidate: {MemoryActive, MemoryDiscarded},
	MemoryActive:    {MemoryInvalid, MemoryObsolete},
	MemoryDiscarded: {},
	MemoryInvalid:   {},
	MemoryObsolete:  {},
}

// AllowedMemoryTransitionsFrom 返回某个状态能去哪。
//
// 用于穷举测试：加了新状态却没接进状态机时，测试会红。
func AllowedMemoryTransitionsFrom(s MemoryStatus) []MemoryStatus {
	out := make([]MemoryStatus, len(allowedMemoryTransitions[s]))
	copy(out, allowedMemoryTransitions[s])
	return out
}

// MemoryScope 是记忆的归属。
//
// ★ L2（项目）用项目名，L3（跨项目）用 CrossProjectScope。
// 用同一个字段而不是加个 bool：加 bool 的话，「scope=acp-engine 且
// isCrossProject=true」这种自相矛盾的状态是表达得出来的。
type MemoryScope string

// CrossProjectScope 是 L3 跨项目记忆的 scope 取值。
const CrossProjectScope MemoryScope = "*"

// IsCrossProject 报告这是不是跨项目记忆。
func (s MemoryScope) IsCrossProject() bool { return s == CrossProjectScope }

// Memory 是一条记忆的**索引与状态**。
//
// ★★ **不含正文**（INV-MEM-8）。正文只存在于 md 文件里
// （`<project>/.acpflows/memory/<id>.md`），人可读可编辑、可入 git。
// 把正文放进来的话，md 与 DB 会各存一份，而它们迟早对不上——
// 到时候「哪一份是真的」没有答案。有反射测试守这条。
//
// ★★ **没有 Delete**（INV-MEM-6）：失效 ≠ 删除。
// 删掉的话，半年前那次运行「当时用的是哪条记忆」就永远查不到了。
type Memory struct {
	id          string
	kind        MemoryKind
	scope       MemoryScope
	status      MemoryStatus
	sourceRefs  []string
	createdBy   string
	reason      string
	supersedes  string
	historyLen  int
	confirmedBy string
}

// ProposeCandidate 提一条候选记忆。
//
// ★★ **这是创建记忆的唯一入口，且它只造得出 candidate**（INV-MEM-2）。
// 没有第二个构造函数能直接造出 active 的——有的话，那就是「自动写入」
// 的后门，而 INV-MEM-2 在三份文档里被写死过。
//
// ★ `sourceRefs` 必填（INV-MEM-3）：记忆必须由证据支撑，
// 聊天原文与一次成功经验都不算。空着的话，AI 的一句臆断就能变成
// 以后每一轮的前提。
func ProposeCandidate(id string, kind MemoryKind, scope MemoryScope, sourceRefs []string, createdBy string) (*Memory, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("model: 记忆 id 不能为空")
	}
	if !kind.IsValid() {
		return nil, fmt.Errorf("%w: %q", ErrInvalidMemoryKind, kind)
	}
	if strings.TrimSpace(string(scope)) == "" {
		return nil, errors.New("model: 记忆 scope 不能为空")
	}
	if len(sourceRefs) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrMemoryNoSourceRefs, id)
	}
	refs := make([]string, len(sourceRefs))
	copy(refs, sourceRefs)

	return &Memory{
		id:         id,
		kind:       kind,
		scope:      scope,
		status:     MemoryCandidate,
		sourceRefs: refs,
		createdBy:  createdBy,
		historyLen: 1, // 创建本身就是一条变更历史（INV-MEM-10）
	}, nil
}

// ID 返回记忆标识。
func (m *Memory) ID() string { return m.id }

// Kind 返回类型。
func (m *Memory) Kind() MemoryKind { return m.kind }

// Scope 返回归属。
func (m *Memory) Scope() MemoryScope { return m.scope }

// Status 返回当前状态。
func (m *Memory) Status() MemoryStatus { return m.status }

// SourceRefs 返回依据。★ 副本：调用方改它不该动到记忆本身。
func (m *Memory) SourceRefs() []string {
	out := make([]string, len(m.sourceRefs))
	copy(out, m.sourceRefs)
	return out
}

// CreatedBy 返回产出者角色。
func (m *Memory) CreatedBy() string { return m.createdBy }

// Reason 返回废弃理由（只有 obsolete 才有）。
func (m *Memory) Reason() string { return m.reason }

// Supersedes 返回被本条取代的记忆 id。
func (m *Memory) Supersedes() string { return m.supersedes }

// HistoryLen 返回变更历史的条数（INV-MEM-10：只增不减）。
func (m *Memory) HistoryLen() int { return m.historyLen }

// ConfirmedBy 返回是谁确认的。
func (m *Memory) ConfirmedBy() string { return m.confirmedBy }

// Confirm 把候选确认为生效。
//
// ★★ `actor` 必填且**必须是人**：这就是 INV-MEM-2 里那个「用户确认动作」。
// 允许空 actor 的话，调用方一句 `Confirm("")` 就绕过了整条规矩，
// 而代码读起来完全正常。
func (m *Memory) Confirm(actor string) error {
	if strings.TrimSpace(actor) == "" {
		return fmt.Errorf("%w: %s", ErrMemoryNeedsUserAction, m.id)
	}
	if err := m.transition(MemoryActive); err != nil {
		return err
	}
	m.confirmedBy = actor
	return nil
}

// Reject 否决一条候选。
func (m *Memory) Reject() error { return m.transition(MemoryDiscarded) }

// MarkInvalid 标记失效：不再注入，历史仍可追溯。
func (m *Memory) MarkInvalid() error { return m.transition(MemoryInvalid) }

// Deprecate 废弃：必须给理由，可指向取代它的记忆（INV-MEM-7）。
func (m *Memory) Deprecate(reason, supersedes string) error {
	if strings.TrimSpace(reason) == "" {
		return fmt.Errorf("%w: %s", ErrMemoryNeedsReason, m.id)
	}
	if supersedes == m.id {
		return fmt.Errorf("%w: %s", ErrMemorySelfSupersede, m.id)
	}
	if err := m.transition(MemoryObsolete); err != nil {
		return err
	}
	m.reason = reason
	m.supersedes = supersedes
	return nil
}

// transition 走状态机，成功时追加一条变更历史。
func (m *Memory) transition(to MemoryStatus) error {
	for _, allowed := range allowedMemoryTransitions[m.status] {
		if allowed == to {
			m.status = to
			m.historyLen++ // INV-MEM-10：每次变更追加一条，历史不可删不可改
			return nil
		}
	}
	return fmt.Errorf("%w: %s → %s", ErrMemoryTransition, m.status, to)
}

// Injectable 报告这条记忆能不能进**新的**注入清单。
//
// ★★ 只有 `active` 能进（INV-MEM-2 / INV-MEM-5）。
//
//   - `candidate` 不能：它还没人拍板，进了就等于自动写入
//   - `invalid` / `obsolete` 不能：但**历史注入记录仍要能解析出它们**，
//     所以这个方法只管「新的」，不影响回溯
func (m *Memory) Injectable() bool { return m.status == MemoryActive }

// VisibleIn 报告这条记忆在某个项目的检索范围内可见（INV-MEM-1）。
//
// ★★ 项目 P1 的记忆**永不**出现在 P2 的注入清单里。串项目的后果是
// 把 A 项目的约束当成 B 项目的前提——而两个项目的约定常常正好相反。
func (m *Memory) VisibleIn(project MemoryScope) bool {
	if m.scope.IsCrossProject() {
		return true // L3 对所有项目可见
	}
	return m.scope == project
}
