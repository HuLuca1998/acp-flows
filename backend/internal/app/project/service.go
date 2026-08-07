// Package project 是本地项目的用例：添加、列出、移除。
//
// ★ **添加项目往用户的目录里写零个字节。**
// 顺手初始化 `.acpflows/` 目录结构是很自然的想法，但用户刚把自己的仓库
// 加进来、`git status` 就多出一堆没见过的东西，是最快失去信任的方式。
//
// 记忆与 Skill 确实存在 `<project>/.acpflows/` 下，但那是 M3 的事，
// 且要等用户主动创建第一条时才写。worktree 则完全不在用户项目里——
// 它在 `~/.acpflows/worktrees`（open-questions Q30）。
package project

import (
	"context"
	"errors"
	"fmt"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// idPrefix 是项目标识的前缀，与 openapi 的 ProjectID 模式一致。
const idPrefix = "proj"

// gitInitCommand 是非 git 目录时给用户的命令。
//
// ★ 给的是**能直接复制去终端敲的一整条命令**，不是「请初始化仓库」。
// 与 Runtime 检测同一套做法：命令由后端给，前端只负责显示。
const gitInitCommand = "git init"

// Service 是项目用例。
type Service struct {
	repo port.ProjectRepo
	git  port.GitProbe
	ids  port.IDGen
}

// New 组装用例。
func New(repo port.ProjectRepo, git port.GitProbe, ids port.IDGen) *Service {
	return &Service{repo: repo, git: git, ids: ids}
}

// Add 把一个本地目录加进来。
//
// 幂等：同一个目录（含 `/a/b` 与 `/a/b/` 这类不同写法）重复添加返回已有记录，
// 不报错也不落第二条。用户从 Finder 拖两次是很常见的，列表里冒出两条一模一样的
// 项目会让人以为自己点错了。
func (s *Service) Add(ctx context.Context, path string) (*model.Project, error) {
	// 先构造再探测：相对路径这类输入错误不该先去碰文件系统。
	// 这里用一个临时 ID，确认要落库时才取真正的 ID——
	// 否则重复添加会白白消耗掉一个序号，列表里的 ID 出现跳号很难解释。
	probe, err := model.NewProject(idPrefix+"-probe", path)
	if err != nil {
		return nil, err
	}

	existing, err := s.repo.FindProjectByPath(ctx, probe.Path())
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, model.ErrNotFound) {
		return nil, fmt.Errorf("查项目 %s: %w", probe.Path(), err)
	}

	// ★ 探测只读。路径不存在或不是目录在这里报错——
	// 静默当成「普通目录」的话，打错的路径会一直躺在列表里，
	// 直到真正开工时才发现，那时错误离原因已经很远了。
	info, err := s.git.ProbeGit(ctx, probe.Path())
	if err != nil {
		return nil, fmt.Errorf("探测 %s: %w", probe.Path(), err)
	}

	p, err := model.NewProject(s.ids.NextID(idPrefix), probe.Path())
	if err != nil {
		return nil, err
	}
	p.SetGitInfo(info.IsRepo, info.DefaultBranch)

	if err := s.repo.SaveProject(ctx, p); err != nil {
		return nil, fmt.Errorf("保存项目 %s: %w", p.Path(), err)
	}
	return p, nil
}

// List 返回全部项目。
func (s *Service) List(ctx context.Context) ([]*model.Project, error) {
	return s.repo.ListProjects(ctx)
}

// Remove 取消登记。**不删用户的任何文件**——这是「移除」与「删除」的全部区别。
func (s *Service) Remove(ctx context.Context, id string) error {
	return s.repo.DeleteProject(ctx, id)
}

// Remedy 返回用户需要敲的命令；不需要做什么时为空。
//
// ★ 判断依据是**探测结论**，不是项目名字。加第三种「需要修」的情形时
// 只在这里加一个分支，界面一行不用改。
func (s *Service) Remedy(p *model.Project) string {
	if !p.IsGitRepo() {
		return gitInitCommand
	}
	return ""
}
