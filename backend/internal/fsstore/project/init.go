// Package project 把一个本地目录初始化成 Duet 管得住的项目。
//
// ★★ **这个包的全部设计都围绕一件事：用户点确认之前就知道我们要动什么。**
//
// 他交出来的是自己的代码仓库。所以「预演」不是一个体贴的附加功能，
// 而是这个包的主结构：`Plan()` 算出要做什么，`Preview()` 把它讲出来，
// `Apply()` 照单执行——**三者用的是同一份 Plan**。
//
// 预演与执行各写一套的话，它们必然漂移，而漂移的方向永远是
// 「预演里没说的那件事被做了」。
package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	// ErrNotADirectory 表示目标路径不是一个已存在的目录。
	ErrNotADirectory = errors.New("fsstore/project: 目标不是一个已存在的目录")

	// ErrPathNotAbsolute 表示传进来的是相对路径。
	//
	// ★ 相对路径的失败信息很难懂（它相对谁？相对 duetd 的启动目录），
	// 所以在最外层就拦掉。
	ErrPathNotAbsolute = errors.New("fsstore/project: 项目路径必须是绝对路径")
)

// ActionKind 是一步初始化动作的类型。
type ActionKind string

const (
	// ActionCreateDir 建目录。
	ActionCreateDir ActionKind = "create_dir"
	// ActionCreateFile 建文件。
	ActionCreateFile ActionKind = "create_file"
	// ActionAppendLines 往已有文件**追加**若干行。
	//
	// ★ 是追加不是覆盖：`.gitignore` 是用户自己维护的文件，
	// 覆盖掉等于把他的规则删了，而他不会立刻发现。
	ActionAppendLines ActionKind = "append_lines"
)

// Action 是一步要做的事。
//
// ★ 它同时是**给用户看的说明**和**给执行器的指令**——
// 两者分开写的话，界面上说的和实际做的会各走各的。
type Action struct {
	Kind ActionKind
	// Path 是绝对路径。
	Path string
	// Lines 只在 ActionCreateFile / ActionAppendLines 时有值。
	Lines []string
	// Reason 是给用户看的一句话：为什么要做这一步。
	Reason string
	// AlreadyThere 为真表示这一步**不用做了**（目录已存在、忽略规则已经在了）。
	//
	// ★ 已经在了的条目仍然列出来而不是悄悄跳过：用户要看到的是
	// 「最终会变成什么样」，不是「这次改了几个字节」。
	AlreadyThere bool
}

// Plan 是一次初始化的完整计划。
type Plan struct {
	// Root 是项目目录。
	Root string
	// Actions 按执行顺序排列。
	Actions []Action
	// IsGitRepo 报告这是不是一个 git 仓库。
	//
	// ★ 不是的话**如实报告并继续**，绝不擅自 `git init`——
	// 在别人的目录里建一个 git 仓库是不可逆的，而他可能有自己的打算。
	IsGitRepo bool
}

// DuetDirName 是 Duet 在项目里唯一会创建的目录。
const DuetDirName = ".acpflows"

// ignoreLine 是要追加进 .gitignore 的那一行。
//
// ★ 只忽略 `runs/`：skills 与 memory 是**用户会想入 git 的东西**
// （他的项目约定、他团队的共享记忆），忽略掉等于替他做了决定。
const ignoreLine = ".acpflows/runs/"

// MakePlan 算出把 root 初始化成 Duet 项目要做哪些事。
//
// ★★ **只读**：它 stat 目录、读 `.gitignore`，但一个字节都不写。
// 有测试对着全目录指纹守这条。
func MakePlan(root string) (*Plan, error) {
	if !filepath.IsAbs(root) {
		return nil, fmt.Errorf("%w: %q", ErrPathNotAbsolute, root)
	}
	st, err := os.Stat(root)
	if err != nil || !st.IsDir() {
		return nil, fmt.Errorf("%w: %s", ErrNotADirectory, root)
	}

	duet := filepath.Join(root, DuetDirName)
	plan := &Plan{
		Root:      root,
		IsGitRepo: isGitRepo(root),
	}

	for _, d := range []struct{ path, reason string }{
		{duet, "Duet 在这个项目里唯一会碰的目录"},
		{filepath.Join(duet, "skills"), "项目级 Skill 放这里，可以入 git 和团队共享"},
		{filepath.Join(duet, "memory"), "项目记忆一条一个 md 文件，人可读可改"},
		{filepath.Join(duet, "runs"), "每次执行的过程记录，**不入 git**"},
	} {
		plan.Actions = append(plan.Actions, Action{
			Kind:         ActionCreateDir,
			Path:         d.path,
			Reason:       d.reason,
			AlreadyThere: dirExists(d.path),
		})
	}

	cfg := filepath.Join(duet, "project.yaml")
	plan.Actions = append(plan.Actions, Action{
		Kind:         ActionCreateFile,
		Path:         cfg,
		Lines:        defaultConfigLines(filepath.Base(root)),
		Reason:       "项目配置，人可读可改",
		AlreadyThere: fileExists(cfg),
	})

	// ★ 只有 git 仓库才谈得上 .gitignore。不是仓库还去写它，
	// 等于在用户目录里凭空造一个他没要的文件。
	if plan.IsGitRepo {
		gitignore := filepath.Join(root, ".gitignore")
		plan.Actions = append(plan.Actions, Action{
			Kind:         ActionAppendLines,
			Path:         gitignore,
			Lines:        []string{ignoreLine},
			Reason:       "执行过程记录不该进版本库",
			AlreadyThere: hasIgnoreLine(gitignore),
		})
	}

	return plan, nil
}

// Pending 返回**真正要做的**那些步骤（跳过已经在了的）。
func (p *Plan) Pending() []Action {
	out := make([]Action, 0, len(p.Actions))
	for _, a := range p.Actions {
		if !a.AlreadyThere {
			out = append(out, a)
		}
	}
	return out
}

// Paths 返回计划涉及的全部路径，按 Actions 的顺序。
//
// ★ 预演与执行的一致性靠它比对：两边都从**同一个 Plan** 取路径，
// 所以「界面上说要建 A，实际建了 B」在结构上就不可能发生。
func (p *Plan) Paths() []string {
	out := make([]string, len(p.Actions))
	for i, a := range p.Actions {
		out[i] = a.Path
	}
	return out
}

// Apply 照计划执行。
//
// ★★ **中途失败要回滚到动手之前**（R5）：留半成品的话，
// 用户的项目里多了一个残缺的 `.acpflows/`，而他既不知道它在那儿、
// 也不知道它是不是完整的。重试时那个残缺目录还会让「已经在了」判断出错。
//
// ★ 只回滚**这次真的创建的东西**——已经存在的目录一个都不动。
func Apply(p *Plan) (err error) {
	var created []string // 逆序清理
	defer func() {
		if err == nil {
			return
		}
		for i := len(created) - 1; i >= 0; i-- {
			_ = os.RemoveAll(created[i])
		}
	}()

	for _, a := range p.Actions {
		if a.AlreadyThere {
			continue
		}
		// ★★ **动手前再看一眼它在不在。**
		//
		// 计划是算出来的，执行是后来发生的——中间那段时间里目录可能
		// 已经被别的东西建了（用户手快点了两次、另一个 Duet 实例、
		// 他自己 mkdir 的）。`MkdirAll` 对已存在的目录**成功返回**，
		// 于是我们会把它记成「这次创建的」，回滚时 `RemoveAll` 掉——
		// **连同里面用户已有的东西**。
		//
		// 计划里的 AlreadyThere 挡不住这个：它是算计划那一刻的快照。
		existed := pathExists(a.Path)

		switch a.Kind {
		case ActionCreateDir:
			if err = os.MkdirAll(a.Path, 0o755); err != nil {
				return fmt.Errorf("建目录 %s: %w", a.Path, err)
			}
			if !existed {
				created = append(created, a.Path)
			}

		case ActionCreateFile:
			if err = os.WriteFile(a.Path, []byte(strings.Join(a.Lines, "\n")+"\n"), 0o644); err != nil {
				return fmt.Errorf("写文件 %s: %w", a.Path, err)
			}
			if !existed {
				created = append(created, a.Path)
			}

		case ActionAppendLines:
			if err = appendLines(a.Path, a.Lines); err != nil {
				return fmt.Errorf("追加 %s: %w", a.Path, err)
			}
			// ★ 追加进的是**用户自己的文件**，回滚时绝不删它——
			// 删掉的话，一次失败的初始化会顺手清掉他的 .gitignore。

		default:
			return fmt.Errorf("不认识的动作类型 %q", a.Kind)
		}
	}
	return nil
}

// appendLines 往文件尾部追加若干行，文件不存在就建。
//
// ★ **保证前面有换行**：用户的 `.gitignore` 最后一行常常没有换行符，
// 直接追加会和他的最后一条规则粘成一行——那条规则就此失效，
// 而 git 不会报错。
func appendLines(path string, lines []string) error {
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	var b strings.Builder
	b.Write(existing)
	if len(existing) > 0 && !strings.HasSuffix(string(existing), "\n") {
		b.WriteString("\n")
	}
	for _, l := range lines {
		b.WriteString(l)
		b.WriteString("\n")
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func isGitRepo(root string) bool {
	// ★ 只看 `.git` 在不在，**不跑 git 命令**：这一步在预演阶段执行，
	// 而跑外部命令的副作用不受我们控制。
	//
	// `.git` 可能是目录（普通仓库）也可能是文件（worktree / submodule）。
	_, err := os.Stat(filepath.Join(root, ".git"))
	return err == nil
}

// pathExists 报告这个路径现在存不存在（目录或文件都算）。
func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// hasIgnoreLine 报告 .gitignore 里已经有那条规则了。
//
// ★ 逐行精确比对，不用 strings.Contains：`.acpflows/runs/` 是
// `#.acpflows/runs/`（被注释掉的）的子串，含糊匹配会让我们
// 以为规则已经生效，而实际它被注释着。
func hasIgnoreLine(path string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(line) == ignoreLine {
			return true
		}
	}
	return false
}

func defaultConfigLines(name string) []string {
	return []string{
		"# Duet 的项目配置。人可读可改，改完下次读取时生效。",
		"name: " + name,
		"",
		"# 这个项目的 Skill 与记忆放在同目录的 skills/ 与 memory/ 下。",
		"# runs/ 是执行过程记录，已加进 .gitignore。",
	}
}
