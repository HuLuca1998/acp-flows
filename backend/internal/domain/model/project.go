package model

import (
	"errors"
	"path/filepath"
	"strings"
)

// 项目相关的领域错误。api 层把它们映射成 Problem 的机器可读错误码。
var (
	// ErrProjectIDRequired 表示没给标识。
	ErrProjectIDRequired = errors.New("project id is required")
	// ErrProjectPathNotAbsolute 表示路径不是绝对路径。
	//
	// ★ 相对路径会在 duetd 的工作目录下解析，而那是用户完全不知道的位置：
	// 今天能用，换个启动方式（Tauri sidecar / launchd / 直接跑）就指到别处。
	// 最坏的情况是 Duet 在一个用户没预期的目录里开工作区并动文件。
	ErrProjectPathNotAbsolute = errors.New("project path must be absolute")
	// ErrProjectNameRequired 表示显示名是空的。
	ErrProjectNameRequired = errors.New("project name must not be blank")
)

// Project 是用户加进来的一个本地代码目录。
//
// ★ **这个聚合不持有任何「Duet 要往里写的路径」。**
// 添加项目往用户目录里写零个字节：worktree 在 `~/.acpflows/worktrees`
// （open-questions Q30），记忆与 Skill 要到用户主动创建第一条时才写
// `<project>/.acpflows/`。
//
// 加进来这个动作本身如果让用户的 `git status` 多出东西，
// 那是最快失去信任的方式——他刚把自己的仓库交给你。
type Project struct {
	id   string
	name string
	path string
	// isGitRepo 与 defaultBranch 由基础设施探测后填入。
	// 领域层不做 IO，所以它们是被 setter 注入的，不是构造时算出来的。
	isGitRepo     bool
	defaultBranch string
}

// NewProject 构造一个项目。path 必须是绝对路径。
//
// 显示名默认取目录名，用户之后可以改（Rename），改名不影响 path。
func NewProject(id, path string) (*Project, error) {
	if strings.TrimSpace(id) == "" {
		return nil, ErrProjectIDRequired
	}
	if !filepath.IsAbs(path) {
		return nil, ErrProjectPathNotAbsolute
	}

	// ★ 规整不是洁癖：`/a/b`、`/a/b/`、`/a/./b`、`/a/c/../b` 是同一个目录，
	// 不规整的话用户从 Finder 拖两次就会看到两条一模一样的记录。
	clean := filepath.Clean(path)

	return &Project{id: id, name: displayName(clean), path: clean}, nil
}

// displayName 取目录名作为默认显示名。
//
// 根目录没有目录名可取，退回路径本身——返回空字符串的话，
// 界面上这一行会显示成一片空白，看起来像记录丢了。
func displayName(cleanPath string) string {
	base := filepath.Base(cleanPath)
	if base == "" || base == "." || base == string(filepath.Separator) {
		return cleanPath
	}
	return base
}

// ID 返回项目标识。
func (p *Project) ID() string { return p.id }

// Name 返回显示名。
func (p *Project) Name() string { return p.name }

// Path 返回规整后的绝对路径。
func (p *Project) Path() string { return p.path }

// IsGitRepo 报告这个目录是不是 git 仓库。
func (p *Project) IsGitRepo() bool { return p.isGitRepo }

// DefaultBranch 返回默认分支；非 git 仓库时为空。
func (p *Project) DefaultBranch() string { return p.defaultBranch }

// SetGitInfo 填入探测结果。
//
// ★ 不是 git 仓库**也允许添加**，只是标出来。直接拒绝的话，用户得先去命令行
// `git init` 才能用这个产品——而他很可能正是不想碰命令行才来用的。
func (p *Project) SetGitInfo(isRepo bool, defaultBranch string) {
	p.isGitRepo = isRepo
	p.defaultBranch = defaultBranch
}

// Rename 改显示名。**不动 path**——两者跟着一起变的话，
// Duet 会去操作一个不存在的目录。
func (p *Project) Rename(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ErrProjectNameRequired
	}
	p.name = trimmed
	return nil
}

// Restore 从持久化状态重建聚合，供 store 层的 mapper 使用。
//
// 与 NewProject 分开是有意的：NewProject 是「用户新加一个项目」，
// 要校验输入；Restore 是「把已经存过的东西读回来」，不该因为校验规则变严
// 就让用户列表里的老项目突然读不出来。
func Restore(id, name, path string, isGitRepo bool, defaultBranch string) *Project {
	return &Project{
		id: id, name: name, path: path,
		isGitRepo: isGitRepo, defaultBranch: defaultBranch,
	}
}
