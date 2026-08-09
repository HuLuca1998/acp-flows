package model

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

var (
	// ErrUnitRoleRequired 表示单元没写由谁做（裁定三）。
	//
	// ★★ 不写的后果是**到执行时才发现没人认领**——而那时用户已经等了几分钟。
	// 更糟的是随手派一个，让实现方审查自己的产出（INV-ATT-8 明令禁止）。
	ErrUnitRoleRequired = errors.New("model: 单元必须写明由哪个角色做")

	// ErrUnknownDependency 表示依赖指向了一个不存在的单元。
	//
	// ★ 静静忽略的话，那条依赖永远不生效：一个本该等着的单元会提前开工，
	// 而它依赖的东西还没做——表现是 AI 对着一个不存在的接口写代码。
	ErrUnknownDependency = errors.New("model: 依赖指向了计划里没有的单元")

	// ErrDependencyCycle 表示依赖成环。
	//
	// ★★ 有环就没有「先做哪个」的答案。不拦的话，调度会挑一个下手，
	// 而那个选择每次运行都可能不同——同一份计划跑两次结果不一样。
	ErrDependencyCycle = errors.New("model: 单元依赖成环")

	// ErrDuplicateUnitID 表示同一版计划里有两个同名单元。
	ErrDuplicateUnitID = errors.New("model: 同一版计划里有重复的单元标识")
)

// Unit 是计划里的一个开发单元。
//
// ★ **不可变**：字段全私有，只有值接收者的读方法。计划改了就出新版本。
type Unit struct {
	id    string
	title string
	// roleID 是**由哪个角色做**（裁定三）。必填，且必须在角色库里。
	roleID string
	// dependsOn 是它要等的那些单元。
	dependsOn []string
	// contractFrozen 说它的契约冻结了没有——设计稿上那句
	// 「依赖 unit-012 · 契约未冻结」的后半句。
	contractFrozen bool
	accepted       bool
}

// NewUnit 造一个单元。
//
// ★★ 角色**当场校验**，不留到执行时：留到那时的话，用户已经等了几分钟，
// 而错误信息会是「没人认领这个单元」——他看不出是计划写错了。
func NewUnit(id, title, roleID string, dependsOn []string) (Unit, error) {
	if strings.TrimSpace(id) == "" {
		return Unit{}, errors.New("model: 单元必须有标识")
	}
	if strings.TrimSpace(roleID) == "" {
		return Unit{}, fmt.Errorf("%w: %s", ErrUnitRoleRequired, id)
	}
	if _, err := RoleByID(roleID); err != nil {
		// ★ 错误里带上**角色 id 与单元 id**：只说「角色不存在」的话，
		// 用户要在几十个单元里挨个找是哪一个写错了。
		return Unit{}, fmt.Errorf("单元 %s 派给了 %q: %w", id, roleID, err)
	}
	return Unit{
		id: id, title: strings.TrimSpace(title), roleID: roleID,
		dependsOn: trimAll(dependsOn),
	}, nil
}

// RestoreUnit 从持久化状态重建，供 store 层用。
//
// ★ 与 NewUnit 分开：后者要校验角色，前者是把存过的读回来——
// 角色库将来删掉一个角色时，不该让历史计划读不出来。
func RestoreUnit(
	id, title, roleID string, dependsOn []string, contractFrozen, accepted bool,
) Unit {
	return Unit{
		id: id, title: title, roleID: roleID,
		dependsOn:      append([]string(nil), dependsOn...),
		contractFrozen: contractFrozen,
		accepted:       accepted,
	}
}

// ID 返回单元标识（形如 unit-012）。
func (u Unit) ID() string { return u.id }

// Title 返回标题。
func (u Unit) Title() string { return u.title }

// RoleID 返回由哪个角色做。
func (u Unit) RoleID() string { return u.roleID }

// DependsOn 返回依赖的单元。★ 副本：调用方改它不该动到计划。
func (u Unit) DependsOn() []string { return append([]string(nil), u.dependsOn...) }

// ContractFrozen 报告它的契约冻结了没有。
func (u Unit) ContractFrozen() bool { return u.contractFrozen }

// Accepted 报告它验收过没有。
func (u Unit) Accepted() bool { return u.accepted }

// Subplan 是一组单元。
//
// ★ **进度不单独存**：`3/3` 由单元的验收状态算出来。存一个字段的话，
// 它会和单元的真实状态漂移——而用户看到「3/3」时以为全做完了。
type Subplan struct {
	id    string
	title string
	units []Unit
}

// NewSubplan 造一个子计划。
func NewSubplan(id, title string, units []Unit) (Subplan, error) {
	if strings.TrimSpace(id) == "" {
		return Subplan{}, errors.New("model: 子计划必须有标识")
	}
	return Subplan{
		id: id, title: strings.TrimSpace(title),
		units: append([]Unit(nil), units...),
	}, nil
}

// ID 返回子计划标识（形如 subplan-01）。
func (s Subplan) ID() string { return s.id }

// Title 返回标题。
func (s Subplan) Title() string { return s.title }

// Units 返回单元。★ 副本。
func (s Subplan) Units() []Unit { return append([]Unit(nil), s.units...) }

// Progress 返回「已验收 / 总数」。
//
// ★★ **算出来的，不是存的**。存一个字段的话它会和单元的真实状态漂移，
// 而用户看到「3/3」时以为全做完了。
func (s Subplan) Progress() (done, total int) {
	for _, u := range s.units {
		if u.accepted {
			done++
		}
	}
	return done, len(s.units)
}

// Status 返回子计划的状态。
//
// ★ 由单元推导：全验收 = accepted，一个都没动 = pending，其余 = in_progress。
// 单独存一个状态字段的话，它与进度会各说各话。
func (s Subplan) Status() string {
	done, total := s.Progress()
	switch {
	case total == 0:
		return "empty"
	case done == total:
		return "accepted"
	case done == 0:
		return "pending"
	default:
		return "in_progress"
	}
}

// ValidateSubplans 校验一整版计划的子计划集合。
//
// 三件事，缺一不可：
//   - 单元标识不重复
//   - 依赖指向的单元**存在**（跨子计划也算）
//   - 依赖**不成环**
func ValidateSubplans(subplans []Subplan) error {
	known := map[string]bool{}
	for _, sp := range subplans {
		for _, u := range sp.units {
			if known[u.id] {
				return fmt.Errorf("%w: %s", ErrDuplicateUnitID, u.id)
			}
			known[u.id] = true
		}
	}

	// ★ 依赖可以跨子计划——设计稿里 unit-013 依赖 unit-012 就是这样。
	// 只在本子计划里找的话，跨子计划的依赖会被误报成「不存在」。
	deps := map[string][]string{}
	for _, sp := range subplans {
		for _, u := range sp.units {
			for _, d := range u.dependsOn {
				if !known[d] {
					return fmt.Errorf("%w: 单元 %s 依赖 %s", ErrUnknownDependency, u.id, d)
				}
			}
			deps[u.id] = u.dependsOn
		}
	}

	if cycle := findCycle(deps); len(cycle) > 0 {
		// ★ 错误里**把环列出来**：只说「成环了」的话，用户要在几十个单元里
		// 自己找那一圈，而那正是他最找不出来的东西。
		return fmt.Errorf("%w: %s", ErrDependencyCycle, strings.Join(cycle, " → "))
	}
	return nil
}

// findCycle 找一个环并按顺序返回它；没有环返回 nil。
func findCycle(deps map[string][]string) []string {
	const (
		white = 0 // 没访问过
		gray  = 1 // 正在访问（在当前这条路径上）
		black = 2 // 访问完了
	)
	color := map[string]int{}
	var path, cycle []string

	// ★ 按标识排序再走：map 的遍历顺序每次都不同，
	// 而「报哪个环」不稳定的话，同一份计划两次运行给出两条不同的错误信息。
	ids := make([]string, 0, len(deps))
	for id := range deps {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var visit func(string) bool
	visit = func(id string) bool {
		color[id] = gray
		path = append(path, id)

		for _, next := range deps[id] {
			switch color[next] {
			case gray:
				// 找到了：从 path 里 next 出现的位置截到末尾，再接回 next
				for i, p := range path {
					if p == next {
						cycle = append(append([]string(nil), path[i:]...), next)
						return true
					}
				}
				return true
			case white:
				if visit(next) {
					return true
				}
			}
		}

		path = path[:len(path)-1]
		color[id] = black
		return false
	}

	for _, id := range ids {
		if color[id] == white && visit(id) {
			return cycle
		}
	}
	return nil
}
