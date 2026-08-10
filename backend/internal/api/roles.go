package api

import (
	"net/http"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/role"
)

// roleBody 对应 openapi 的 Role。
type roleBody struct {
	ID               string   `json:"id"`
	DisplayName      string   `json:"display_name"`
	Operations       []string `json:"operations"`
	Duty             string   `json:"duty,omitempty"`
	Personality      string   `json:"personality,omitempty"`
	Boundary         string   `json:"boundary,omitempty"`
	Output           string   `json:"output,omitempty"`
	SessionMode      string   `json:"session_mode"`
	ModeName         string   `json:"mode_name,omitempty"`
	PermissionPolicy string   `json:"permission_policy"`
	RuntimeName      string   `json:"runtime_name"`
	IsPreset         bool     `json:"is_preset"`
	// Problem 在绑定坏掉时说明原因。★ 出问题的角色照样列出来——
	// 跳过的话用户看到七个角色，而他不知道少了哪一个、为什么少。
	Problem string `json:"problem,omitempty"`
}

type rolesBody struct {
	// ★ 必须序列化成数组而不是 null（同 runtimes 那条）。
	Roles []roleBody `json:"roles"`
}

// roleLister 是本层需要的能力。接口定义在使用方，api 包不必认识 app/role 之外的东西。
type roleLister interface {
	List() []role.View
}

// handleListRoles 处理 GET /v1/roles。
func handleListRoles(l roleLister) http.HandlerFunc {
	// ★ 不接 *http.Request：角色表**没有任何过滤参数**——
	// 八个预置角色是全集，加 scope / status 之类的查询参数只会让人
	// 以为「筛掉的那些还在别处」。
	return func(w http.ResponseWriter, _ *http.Request) {
		if l == nil {
			// ★ 不回 200 空列表：那会让界面把「没装配」显示成「一个角色都没有」，
			// 而八个预置角色是**内置的**，用户看到空表只会以为应用坏了。
			writeProblem(w, http.StatusServiceUnavailable,
				"roles_unavailable", "Role service is not configured")
			return
		}

		views := l.List()
		body := rolesBody{Roles: make([]roleBody, 0, len(views))}
		for _, v := range views {
			ops := v.Role.Operations()
			opNames := make([]string, 0, len(ops))
			for _, op := range ops {
				opNames = append(opNames, string(op))
			}
			body.Roles = append(body.Roles, roleBody{
				ID:               v.Role.ID(),
				DisplayName:      v.Role.DisplayName(),
				Operations:       opNames,
				Duty:             v.Role.Duty(),
				Personality:      v.Role.Personality(),
				Boundary:         v.Role.Boundary(),
				Output:           v.Role.Output(),
				SessionMode:      string(v.Role.SessionMode()),
				ModeName:         v.ModeName,
				PermissionPolicy: string(v.Role.PermissionPolicy()),
				RuntimeName:      v.RuntimeName,
				IsPreset:         v.Role.IsPreset(),
				Problem:          v.Problem,
			})
		}
		writeJSON(w, http.StatusOK, body)
	}
}
