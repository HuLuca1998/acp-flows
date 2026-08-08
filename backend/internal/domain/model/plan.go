package model

import (
	"errors"
	"fmt"
)

// 计划相关的错误。
var (
	// ErrDispositionMissing 表示有已验收工作没给处置。
	//
	// ★ 漏掉一项的后果是：那项工作既没说「仍有效」也没说「要回滚」，
	// 用户不知道自己上周验收过的东西还算不算数。
	ErrDispositionMissing = errors.New("model: 有已验收工作没有声明处置")
	// ErrDispositionUnknownTarget 表示给了一个没验收过的目标。
	ErrDispositionUnknownTarget = errors.New("model: 处置指向了一个没验收过的目标")
	// ErrDispositionInvalid 表示处置取值不在全集里。
	ErrDispositionInvalid = errors.New("model: 处置取值不合法")
	// ErrPlanVersionNotNext 表示版本号不是「上一版 + 1」。
	ErrPlanVersionNotNext = errors.New("model: 计划版本号必须严格递增且不跳号")
)

// Disposition 是对一项已验收工作的处置。
//
// ★ 取值会原样出现在事件载荷与界面词条 key 里，改名等于破坏已有数据。
type Disposition string

// 四种处置，见 docs/spec/domain-model.md §6.3。
const (
	// DispositionStillValid 仍有效：已验收成果不受本次重规划影响。
	DispositionStillValid Disposition = "still_valid"
	// DispositionNeedsSupplement 需补充：成果保留，但要追加单元。
	DispositionNeedsSupplement Disposition = "needs_supplement"
	// DispositionNeedsRollback 需回滚：成果必须撤销。
	DispositionNeedsRollback Disposition = "needs_rollback"
	// DispositionObsolete 已废弃：成果作废但不删除历史。
	DispositionObsolete Disposition = "obsolete"
)

var allDispositions = [...]Disposition{
	DispositionStillValid,
	DispositionNeedsSupplement,
	DispositionNeedsRollback,
	DispositionObsolete,
}

// AllDispositions 返回处置全集。界面上的选择器照它渲染，加一种只加一处。
func AllDispositions() []Disposition {
	out := make([]Disposition, len(allDispositions))
	copy(out, allDispositions[:])
	return out
}

// IsValid 报告取值是否在全集内。
func (d Disposition) IsValid() bool {
	for _, v := range allDispositions {
		if v == d {
			return true
		}
	}
	return false
}

// PlanVersion 是计划的一版。
//
// ★ **不可变**：所有字段私有，只有值接收者的读方法。要新增内容就造一个
// 新版本（`Next`），别在旧版本上动手——这是 INV-PLAN-4 的实现方式。
//
// 改写的后果是用户看不到「为什么改」：他打开计划面板只看到当前这版，
// 而 AI 上周为什么推翻了原方案，答案不在任何地方。
type PlanVersion struct {
	version      int
	title        string
	dispositions map[string]Disposition
}

// NewPlanVersion 造第一版。第一版没有已验收工作，所以不需要处置。
func NewPlanVersion(version int, title string, dispositions map[string]Disposition) PlanVersion {
	return PlanVersion{
		version:      version,
		title:        title,
		dispositions: copyDispositions(dispositions),
	}
}

// NewReplan 造一个重规划版本，**必须**给出每一项已验收工作的处置。
func NewReplan(
	version int, title string, accepted []string, dispositions map[string]Disposition,
) (PlanVersion, error) {
	if err := validateDispositions(accepted, dispositions); err != nil {
		return PlanVersion{}, err
	}
	return NewPlanVersion(version, title, dispositions), nil
}

// Version 返回版本号。
func (v PlanVersion) Version() int { return v.version }

// Title 返回标题。
func (v PlanVersion) Title() string { return v.title }

// Dispositions 返回处置的副本——**返回内部 map 的话，调用方能改掉它**，
// 而「不可变」就名存实亡了。
func (v PlanVersion) Dispositions() map[string]Disposition {
	return copyDispositions(v.dispositions)
}

// Next 基于这一版造下一版。
//
// ★ 版本号必须是 `当前 + 1`。跳号的话，用户看到 v3 之后是 v6，
// 会以为自己漏看了两版——而实际上那两版根本不存在。
func (v PlanVersion) Next(
	version int, title string, accepted []string, dispositions map[string]Disposition,
) (PlanVersion, error) {
	if version != v.version+1 {
		return PlanVersion{}, fmt.Errorf("%w: 当前 v%d，下一版只能是 v%d，给的是 v%d",
			ErrPlanVersionNotNext, v.version, v.version+1, version)
	}
	return NewReplan(version, title, accepted, dispositions)
}

// validateDispositions 校验处置是否**恰好**覆盖已验收工作。
func validateDispositions(accepted []string, dispositions map[string]Disposition) error {
	for _, id := range accepted {
		d, ok := dispositions[id]
		if !ok {
			return fmt.Errorf("%w: %s", ErrDispositionMissing, id)
		}
		if !d.IsValid() {
			return fmt.Errorf("%w: %s 的处置是 %q", ErrDispositionInvalid, id, d)
		}
	}

	// 反过来也要校验：给了一个没验收过的目标，说明调用方的认知与事实不符——
	// 静静忽略的话，那条处置永远不会生效，而没人知道
	inAccepted := make(map[string]bool, len(accepted))
	for _, id := range accepted {
		inAccepted[id] = true
	}
	for id := range dispositions {
		if !inAccepted[id] {
			return fmt.Errorf("%w: %s", ErrDispositionUnknownTarget, id)
		}
	}
	return nil
}

func copyDispositions(in map[string]Disposition) map[string]Disposition {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]Disposition, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
