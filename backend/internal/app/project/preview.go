package project

import (
	"context"
	"fmt"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// Preview 是创建项目前给用户看的东西。
//
// 四块正好对应设计稿弹层的四个区块：
// 将做什么 · 发现的已有 Skill · GitHub remote · `gh` 状态。
type Preview struct {
	Path      string
	Name      string
	IsGitRepo bool
	Actions   []port.InitAction
	Skills    []port.SkillEntry
	Remote    port.GitRemoteInfo
	Gh        port.GhInfo
}

// PreviewInit 算出「把这个目录交给 Duet 会发生什么」。
//
// ★★ **一个字节都不写。**
//
// ★ 四块里任何一块出问题都**不让整次预演失败**：扫不到 skill、
// 读不出 remote、`gh` 没装——这些都是很常见的正常状态。
// 整体失败的话，用户会因为「他没装 gh」而根本创建不了项目。
func (s *Service) PreviewInit(ctx context.Context, path string) (*Preview, error) {
	// 先规整路径并校验：相对路径这类输入错误不该先去碰文件系统。
	probe, err := model.NewProject(idPrefix+"-probe", path)
	if err != nil {
		return nil, err
	}
	root := probe.Path()

	if s.init == nil {
		return nil, fmt.Errorf("project: 没有配置初始化器")
	}
	plan, err := s.init.PlanInit(root)
	if err != nil {
		// ★ 这一块失败**才**是真失败：算不出计划就没什么可给用户看的，
		// 而让他对着一个空对话框点「创建」是最坏的。
		return nil, fmt.Errorf("算初始化计划 %s: %w", root, err)
	}

	out := &Preview{
		Path:      root,
		Name:      probe.Name(),
		IsGitRepo: plan.IsGitRepo,
		Actions:   plan.Actions,
		// ★ 空切片而不是 nil：nil 会序列化成 null，前端崩在 .map 上
		Skills: []port.SkillEntry{},
	}

	if s.skills != nil {
		if found, scanErr := s.skills.DiscoverInProject(root); scanErr == nil {
			out.Skills = found
		}
		// 扫不动就是没扫到——绝大多数项目本来就没有 skill 目录
	}

	if s.remote != nil && plan.IsGitRepo {
		if r, remoteErr := s.remote.ProbeRemote(ctx, root); remoteErr == nil {
			out.Remote = r
		}
	}

	if s.gh != nil {
		out.Gh = s.gh.DetectGh(ctx)
	}

	return out, nil
}

// InitializeAt 照预演给出的同一份计划执行。
//
// ★★ **重新算一次计划而不是把上次那份传回来**：
// 用户看完弹层到点确认之间可能过了几分钟，期间目录变了。
// 拿旧计划去执行的话，「已经在了」的判断是过期的——
// 而那正是回滚会误删用户文件的入口。
//
// 一致性靠的是**同一个 PlanInit**，不是同一个 Plan 实例。
func (s *Service) InitializeAt(path string) error {
	if s.init == nil {
		return fmt.Errorf("project: 没有配置初始化器")
	}
	plan, err := s.init.PlanInit(path)
	if err != nil {
		return fmt.Errorf("算初始化计划 %s: %w", path, err)
	}
	return s.init.ApplyInit(plan)
}
