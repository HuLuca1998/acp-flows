package model

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrRequirementFrozen 表示试图改一份已经冻结的需求。
	//
	// ★★ 冻结之后**一个字都不能改**（INV-REQ-2）。要改就出新版本。
	//
	// 允许改的后果是：计划、契约、单元全都是照着某一版需求做的，
	// 而那一版**已经不存在了**——事后没人说得清「当时到底要做什么」。
	ErrRequirementFrozen = errors.New("model: 需求已冻结，改它只能出新版本")

	// ErrOpenFactsRemain 表示还有待确认的事实，不能冻结（INV-REQ-1）。
	//
	// ★ 带着没问清的问题往下走，AI 会自己替用户做决定——
	// 而那些决定会一路固化进计划与契约，等他发现时已经改了几十个文件。
	ErrOpenFactsRemain = errors.New("model: 还有待确认的事实，不能冻结")

	// ErrNoRequirementItems 表示一条需求条目都没有。
	ErrNoRequirementItems = errors.New("model: 需求至少要有一条可验证的条目")

	// ErrRequirementVersionNotNext 表示版本号跳号或回退。
	ErrRequirementVersionNotNext = errors.New("model: 需求版本号必须严格递增且不跳号")
)

// RequirementSnapshot 是某一版需求快照。
//
// ★★ **不可变**：字段全私有，只有值接收者的读方法。要改就用 `Revise`
// 造一个新版本——这与 `PlanVersion` 是同一条规矩（INV-REQ-2）。
//
// ★ 做成**独立聚合**而不是 Work 的值对象（规格 OPEN-2 未裁定，这里选前者）：
// 它自己有版本链、有冻结状态、要被计划与证据引用。
// 塞进 Work 的话，每次改需求都要写整个 Work，而版本链会寄生在
// 一个本来只管状态机的聚合里。
//
// 有反射测试守着「没有指针接收者的导出 mutator」。
type RequirementSnapshot struct {
	workID  string
	version int
	items   []string
	// openFacts 是待确认的事实清单；非空时不得冻结。
	openFacts []string
	frozen    bool
}

// NewRequirement 造第一版需求（v1）。
//
// ★ 新建出来**不是冻结的**：需求分析师要先追问清楚。
func NewRequirement(workID string, items, openFacts []string) (*RequirementSnapshot, error) {
	if strings.TrimSpace(workID) == "" {
		return nil, errors.New("model: 需求必须属于某个工作")
	}
	kept := trimAll(items)
	if len(kept) == 0 {
		return nil, ErrNoRequirementItems
	}
	return &RequirementSnapshot{
		workID:    workID,
		version:   1,
		items:     kept,
		openFacts: trimAll(openFacts),
	}, nil
}

// RestoreRequirement 从持久化状态重建，供 store 层用。
//
// ★ 与 NewRequirement 分开：后者是「用户新提一份需求」要校验，
// 前者是「把存过的读回来」——校验规则变严时不该让老数据读不出来。
func RestoreRequirement(
	workID string, version int, items, openFacts []string, frozen bool,
) *RequirementSnapshot {
	return &RequirementSnapshot{
		workID:    workID,
		version:   version,
		items:     append([]string(nil), items...),
		openFacts: append([]string(nil), openFacts...),
		frozen:    frozen,
	}
}

// WorkID 返回所属工作。
func (r *RequirementSnapshot) WorkID() string { return r.workID }

// Version 返回版本号，从 1 开始。
func (r *RequirementSnapshot) Version() int { return r.version }

// Frozen 报告这一版冻结了没有。
func (r *RequirementSnapshot) Frozen() bool { return r.frozen }

// Items 返回需求条目。★ 副本：调用方改它不该动到快照本身。
func (r *RequirementSnapshot) Items() []string {
	return append([]string(nil), r.items...)
}

// OpenFacts 返回待确认的事实清单。★ 副本，理由同上。
func (r *RequirementSnapshot) OpenFacts() []string {
	return append([]string(nil), r.openFacts...)
}

// Freeze 冻结这一版。
//
// ★★ 还有待确认的事实时**拒绝冻结**（INV-REQ-1）：
// 带着没问清的问题往下走，AI 会自己替用户做决定——
// 而那些决定会一路固化进计划与契约。
func (r *RequirementSnapshot) Freeze() error {
	if r.frozen {
		// 幂等：重复冻结不报错。用户手快点两下是常态。
		return nil
	}
	if len(r.openFacts) > 0 {
		return fmt.Errorf("%w: 还剩 %d 条（%s）",
			ErrOpenFactsRemain, len(r.openFacts), strings.Join(r.openFacts, " · "))
	}
	r.frozen = true
	return nil
}

// ResolveFact 划掉一条待确认的事实。
//
// ★ 冻结之后不能再动（INV-REQ-2）——那时清单本身也是历史的一部分。
func (r *RequirementSnapshot) ResolveFact(fact string) error {
	if r.frozen {
		return fmt.Errorf("%w: v%d", ErrRequirementFrozen, r.version)
	}
	fact = strings.TrimSpace(fact)
	out := make([]string, 0, len(r.openFacts))
	var found bool
	for _, f := range r.openFacts {
		if f == fact {
			found = true
			continue
		}
		out = append(out, f)
	}
	if !found {
		return fmt.Errorf("model: 待确认清单里没有 %q", fact)
	}
	r.openFacts = out
	return nil
}

// Revise 基于这一版造下一版。
//
// ★★ **旧版本原样留着**（INV-REQ-2）：用户要能回答
// 「上周那版需求说的是什么」。改写旧版的话，那个问题永远没有答案。
//
// ★ 未冻结的版本也能修订——那只是「还在追问的过程中改了想法」。
func (r *RequirementSnapshot) Revise(items, openFacts []string) (*RequirementSnapshot, error) {
	kept := trimAll(items)
	if len(kept) == 0 {
		return nil, ErrNoRequirementItems
	}
	return &RequirementSnapshot{
		workID:    r.workID,
		version:   r.version + 1,
		items:     kept,
		openFacts: trimAll(openFacts),
	}, nil
}

// IsNextOf 报告这一版是不是 prev 的直接下一版。
//
// ★ 版本链只增不跳号：跳号的话，中间那一版去哪了没人说得清，
// 而「为什么变成现在这样」正是版本链存在的理由。
func (r *RequirementSnapshot) IsNextOf(prev *RequirementSnapshot) error {
	if prev == nil {
		if r.version != 1 {
			return fmt.Errorf("%w: 第一版必须是 v1，拿到 v%d",
				ErrRequirementVersionNotNext, r.version)
		}
		return nil
	}
	if r.workID != prev.workID {
		return fmt.Errorf("model: 需求版本跨了工作：%s → %s", prev.workID, r.workID)
	}
	if r.version != prev.version+1 {
		return fmt.Errorf("%w: v%d 之后应该是 v%d，拿到 v%d",
			ErrRequirementVersionNotNext, prev.version, prev.version+1, r.version)
	}
	return nil
}

// CanStartPlanning 报告能不能开始规划。
//
// ★★ 只有**冻结过的**需求才能进计划（INV-REQ-1）：
// 需求还在变的时候做出来的计划，做完也对不上。
func (r *RequirementSnapshot) CanStartPlanning() bool { return r.frozen }

// trimAll 去掉首尾空白并丢掉空串。
//
// ★ 丢空串不是洁癖：一条空的需求条目会让「需求 6 · 已映射 6」这种统计
// 变成假的——那个 6 里有一条根本没内容。
func trimAll(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}
