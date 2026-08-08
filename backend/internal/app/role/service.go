// Package role 把角色定义（domain）与 Runtime 绑定（adapter）拼成
// 界面要显示的那张表。
package role

import (
	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// View 是一个角色在界面上的样子。
//
// ★ 它比 `model.Role` 多两个字段（`RuntimeName` / `ModeName`），
// 因为那两个不是角色自身的属性——设计稿的原则是「角色先定义、再绑定 Runtime」。
type View struct {
	Role model.Role
	// RuntimeName 是当前绑定的 Runtime。
	RuntimeName string
	// ModeName 是**翻译好的**档名，只用于展示。
	//
	// ★ 让后端翻译而不是前端：前端翻译就得认识 `plan` / `read-only`
	// 这些品牌相关的取值，那正是分层要挡住的东西。
	ModeName string
	// Problem 在绑定查不到或档位翻译不出来时说明原因。
	//
	// ★ 出问题的角色**照样列出来**，只是带着原因。整批失败的话，
	// 一个角色的绑定坏了会让整页空白——而用户连是哪个角色坏了都不知道。
	Problem string
}

// Service 提供角色查询。
type Service struct {
	bindings port.RoleBindings
}

// New 造一个服务。
func New(bindings port.RoleBindings) *Service {
	return &Service{bindings: bindings}
}

// List 返回八个预置角色，顺序就是设计稿角色表的行序。
//
// ★ 绑定查不到时**不跳过那个角色**：跳过的话，用户在界面上看到七个角色，
// 而他根本不知道少了哪一个、为什么少。
func (s *Service) List() []View {
	roles := model.PresetRoles()
	out := make([]View, 0, len(roles))

	for _, r := range roles {
		v := View{Role: r}
		if s.bindings == nil {
			v.Problem = "没有配置 Runtime 绑定"
			out = append(out, v)
			continue
		}

		rt, err := s.bindings.RuntimeFor(r.ID())
		if err != nil {
			v.Problem = err.Error()
			out = append(out, v)
			continue
		}
		v.RuntimeName = rt

		mode, err := s.bindings.ModeNameOn(rt, r.SessionMode())
		if err != nil {
			v.Problem = err.Error()
			out = append(out, v)
			continue
		}
		v.ModeName = mode
		out = append(out, v)
	}
	return out
}
