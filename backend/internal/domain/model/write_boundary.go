package model

import (
	"path"
	"strings"
)

// BoundaryVerdict 是「这次写入在不在边界内」的判定。
type BoundaryVerdict string

const (
	// BoundaryInside 在允许写入的范围内。
	BoundaryInside BoundaryVerdict = "in_boundary"
	// BoundaryOutside 越界了。
	//
	// ★ 越界**不等于拒绝**：裁决权始终在用户手里。这个判定只负责
	// 把「它想动的东西超出了说好的范围」摆到他眼前。
	BoundaryOutside BoundaryVerdict = "out_of_boundary"
	// BoundaryUnknown 说不清——没有契约，或契约里没写边界。
	//
	// ★★ **不是** `in_boundary`。把「不知道」当成「没问题」，
	// 等于在最该提醒的时候保持沉默：契约没冻结时用户正好最需要看清楚
	// AI 要动什么。
	BoundaryUnknown BoundaryVerdict = "unknown"
)

// WriteBoundary 是一个单元允许改动的范围。
//
// ★★ 用**路径前缀**而不是正则。正则能表达更多，但边界是用户唯一的防线，
// 而一条他自己都读不懂的正则不构成防线——他会直接点「允许」。
//
// 前缀以 `/` 结尾表示目录，否则表示单个文件。
type WriteBoundary struct {
	// Allowed 是允许改的前缀。**空表示什么都不许改**，不是什么都许。
	Allowed []string
	// Forbidden 是明确禁止的前缀，**压过 Allowed**。
	//
	// ★ 有它才能表达「这个包随便改，但别碰它的 schema」——
	// 而那正是最常见的边界形状。
	Forbidden []string
}

// IsEmpty 报告这条边界有没有说过任何事。
func (b WriteBoundary) IsEmpty() bool {
	return len(b.Allowed) == 0 && len(b.Forbidden) == 0
}

// Judge 判定一次写入在不在边界内。
//
// ★ `p` 是**相对工作区根**的路径。绝对路径先由调用方换算——
// 这一层不认识工作区在哪。
//
// ★ 不看文件存不存在：AI 要新建一个文件时，那个路径当然还不存在，
// 而那正是最需要判边界的时刻。
func (b WriteBoundary) Judge(p string) BoundaryVerdict {
	if b.IsEmpty() {
		return BoundaryUnknown
	}

	p = normalizeBoundaryPath(p)

	// ★★ **禁止项先判**：同时命中时判越界。
	// 反过来的话，「允许 internal/、禁止 internal/api/schema.go」
	// 会让那个文件被放行——而它正是被单独拎出来禁止的那一个。
	for _, f := range b.Forbidden {
		if matchesPrefix(p, f) {
			return BoundaryOutside
		}
	}
	for _, a := range b.Allowed {
		if matchesPrefix(p, a) {
			return BoundaryInside
		}
	}
	// ★ 有边界但没命中允许项 = 越界。「没说可以」就是不可以——
	// 安全默认是拒绝，不是放行。
	return BoundaryOutside
}

// matchesPrefix 判断 p 在不在前缀 prefix 覆盖的范围内。
//
// ★★ **按路径段比**，不是按字符串前缀。`internal/acp/` 不该匹配
// `internal/acpx/foo.go`——按字符串比的话它会匹配上，
// 而那是一个用户从没同意过的目录。
func matchesPrefix(p, prefix string) bool {
	prefix = normalizeBoundaryPath(prefix)
	if prefix == "" {
		return false
	}
	if p == prefix {
		return true
	}
	return strings.HasPrefix(p, prefix+"/")
}

// normalizeBoundaryPath 把路径规整成可比的形式。
//
// ★ 去掉首尾的 `/` 与 `./`，并折叠 `..`——不折叠的话
// `internal/acp/../../etc/passwd` 会被判成「在 internal/acp/ 里」。
func normalizeBoundaryPath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimSuffix(p, "/")
	p = strings.TrimPrefix(p, "./")
	if p == "" {
		return ""
	}
	// path.Clean 折叠 `..` 与重复的 `/`
	cleaned := path.Clean(p)
	return strings.TrimPrefix(cleaned, "/")
}
