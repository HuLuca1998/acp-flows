package model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrInvalidSkillVersion 表示 Skill 版本号不是 `主.次` 形态。
var ErrInvalidSkillVersion = errors.New("model: 非法的 Skill 版本号")

// SkillStatus 是 Skill 的三态（domain-model.md §15.3）。
//
// ★ 取值会出现在界面词条 key 与注入记录里，改名等于破坏已有数据。
type SkillStatus string

const (
	// SkillDraft 草稿：新建或导入的一律先是这个，校验通过才能发布（INV-SKL-1）。
	SkillDraft SkillStatus = "draft"
	// SkillActive 生效：可以被注入。
	SkillActive SkillStatus = "active"
	// SkillDeprecated 已弃用：不进**新**的注入清单，但历史注入记录仍可解析
	// （INV-SKL-5）——不然翻旧账时会看到「引用了一个不存在的 skill」。
	SkillDeprecated SkillStatus = "deprecated"
)

var allSkillStatuses = [...]SkillStatus{SkillDraft, SkillActive, SkillDeprecated}

// AllSkillStatuses 返回状态全集。界面的筛选器照它渲染。
func AllSkillStatuses() []SkillStatus {
	out := make([]SkillStatus, len(allSkillStatuses))
	copy(out, allSkillStatuses[:])
	return out
}

// IsValid 报告取值是否在全集内。
func (s SkillStatus) IsValid() bool {
	for _, v := range allSkillStatuses {
		if v == s {
			return true
		}
	}
	return false
}

// SkillScope 是 Skill 的归属：项目级还是全局。
type SkillScope string

const (
	// SkillScopeProject 项目级：`<项目>/.acpflows/skills/`。
	SkillScopeProject SkillScope = "project"
	// SkillScopeGlobal 全局：`~/.acpflows/skills/`。
	SkillScopeGlobal SkillScope = "global"
)

// SkillVersion 是 `主.次` 形态的版本号（设计稿里是 `v2.1` `v1.4` `v0.4`）。
//
// ★ 和 `Version`（应用版本）**不是一回事**：那个是三段语义化版本，
// 这个是两段。混用的话「2.1 和 2.1.0 是同一版吗」会变成一个真问题。
type SkillVersion struct {
	Major int
	Minor int
}

// ParseSkillVersion 解析版本号，接受可选的 `v` 前缀。
func ParseSkillVersion(s string) (SkillVersion, error) {
	raw := strings.TrimPrefix(s, "v")
	major, minor, ok := strings.Cut(raw, ".")
	if !ok {
		return SkillVersion{}, fmt.Errorf("%w: %q 应为 主.次 两段", ErrInvalidSkillVersion, s)
	}
	ma, err := parseSkillVersionPart(major)
	if err != nil {
		return SkillVersion{}, fmt.Errorf("%w: %q 的主版本: %w", ErrInvalidSkillVersion, s, err)
	}
	mi, err := parseSkillVersionPart(minor)
	if err != nil {
		return SkillVersion{}, fmt.Errorf("%w: %q 的次版本: %w", ErrInvalidSkillVersion, s, err)
	}
	return SkillVersion{Major: ma, Minor: mi}, nil
}

func parseSkillVersionPart(p string) (int, error) {
	if p == "" {
		return 0, errors.New("为空")
	}
	// ★ 不接受前导零与正负号：`01` 与 `1` 会被排序成两个不同的版本，
	// 而它们在界面上长得几乎一样。
	if len(p) > 1 && p[0] == '0' {
		return 0, fmt.Errorf("%q 有前导零", p)
	}
	n, err := strconv.Atoi(p)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("%q 不是非负整数", p)
	}
	return n, nil
}

// String 返回 `2.1` 形态（不带 v 前缀）。
func (v SkillVersion) String() string {
	return strconv.Itoa(v.Major) + "." + strconv.Itoa(v.Minor)
}

// Compare 比较两个版本：v < other 返回 -1，相等 0，大于 1。
func (v SkillVersion) Compare(other SkillVersion) int {
	if v.Major != other.Major {
		if v.Major < other.Major {
			return -1
		}
		return 1
	}
	if v.Minor != other.Minor {
		if v.Minor < other.Minor {
			return -1
		}
		return 1
	}
	return 0
}

// SkillFrontmatter 是 SKILL.md 头部声明的字段。
//
// ★ 这是**已经解析好的**结构，怎么从 YAML 解出来是 fsstore 的事——
// domain 只负责判断它合不合格。
type SkillFrontmatter struct {
	Name          string
	Description   string
	Version       string
	Compatibility string
}

// SkillValidation 是一次校验的结论。
type SkillValidation struct {
	// OK 为真表示可以发布成 active。
	OK bool
	// Reason 在不通过时说明**为什么**，且要能直接显示给用户看
	// （设计稿原文：「校验未通过：frontmatter 缺 `description`」）。
	Reason string
}

// ValidateSkill 判断一个 Skill 能不能发布。
//
// ★ INV-SKL-2：不通过**必须给出可读原因**，不得静默拒绝。
// 静默拒绝的后果是用户看到一个 draft 状态的 skill 却不知道该改什么——
// 他唯一能做的事是删了重建，而重建出来还是 draft。
//
// ★ 顺序是刻意的：先报缺 SKILL.md，再报缺字段。反过来的话，
// 一个根本没有 SKILL.md 的目录会被报成「缺 description」，把人引向错误的方向。
func ValidateSkill(hasSkillMD bool, fm SkillFrontmatter) SkillValidation {
	if !hasSkillMD {
		return SkillValidation{Reason: "校验未通过：缺 SKILL.md"}
	}
	var missing []string
	if strings.TrimSpace(fm.Name) == "" {
		missing = append(missing, "name")
	}
	if strings.TrimSpace(fm.Description) == "" {
		missing = append(missing, "description")
	}
	if len(missing) > 0 {
		return SkillValidation{
			Reason: "校验未通过：frontmatter 缺 " + strings.Join(missing, "、"),
		}
	}
	if fm.Version != "" {
		if _, err := ParseSkillVersion(fm.Version); err != nil {
			return SkillValidation{Reason: "校验未通过：版本号 " + fm.Version + " 不是 主.次 形态"}
		}
	}
	return SkillValidation{OK: true}
}
