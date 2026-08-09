package api

import (
	"encoding/json"
	"net/http"
	"strings"
)

// ghStatusBody 对应 openapi 的 GhStatus。
//
// ★★ **没有 token 字段**，也永远不会有（Q41）。
type ghStatusBody struct {
	Status  string `json:"status"`
	Version string `json:"version,omitempty"`
	Account string `json:"account,omitempty"`
	Remedy  string `json:"remedy,omitempty"`
}

// projectPreviewBody 对应 openapi 的 ProjectPreview。
type projectPreviewBody struct {
	Path      string              `json:"path"`
	Name      string              `json:"name,omitempty"`
	IsGitRepo bool                `json:"is_git_repo"`
	Actions   []projectActionBody `json:"actions"`
	Skills    []skillBody         `json:"skills"`
	Remote    *gitRemoteBody      `json:"remote,omitempty"`
	Gh        *ghStatusBody       `json:"gh,omitempty"`
}

// handlePreviewProject 处理 POST /v1/projects/preview。
//
// ★★ **一个字节都不写。** 用户交出来的是他自己的代码仓库——
// 「先说再做」是这一步的全部意义。
func handlePreviewProject(svc projectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			writeProblem(w, http.StatusServiceUnavailable,
				"project_service_unavailable", "Project service is not configured")
			return
		}

		var req previewRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeProblem(w, http.StatusBadRequest, "invalid_request_body", "malformed JSON")
			return
		}
		if strings.TrimSpace(req.Path) == "" {
			writeProblem(w, http.StatusBadRequest, "project_path_required", "path is required")
			return
		}

		pv, err := svc.PreviewInit(r.Context(), req.Path)
		if err != nil {
			writeProjectProblem(w, "preview project", err)
			return
		}

		body := projectPreviewBody{
			Path:      pv.Path,
			Name:      pv.Name,
			IsGitRepo: pv.IsGitRepo,
			// ★ 空切片而不是 nil：nil 序列化成 null，前端崩在 .map 上
			Actions: make([]projectActionBody, 0, len(pv.Actions)),
			Skills:  make([]skillBody, 0, len(pv.Skills)),
		}
		for _, a := range pv.Actions {
			body.Actions = append(body.Actions, projectActionBody{
				Kind:         a.Kind,
				Path:         a.Path,
				Reason:       a.Reason,
				AlreadyThere: a.AlreadyThere,
				Lines:        a.Lines,
			})
		}
		for _, s := range pv.Skills {
			body.Skills = append(body.Skills, skillBody{
				Name:             s.Name,
				Dir:              s.Dir,
				Version:          s.Version,
				Description:      s.Description,
				Compatibility:    s.Compatibility,
				Scope:            s.Scope,
				Source:           s.Source,
				Status:           s.Status,
				ValidationOK:     s.ValidationOK,
				ValidationReason: s.ValidationReason,
			})
		}
		// ★ 没有 remote 时**整块省掉**而不是给一堆空字段：
		// 界面据此显示「这个项目还没有 remote」，而不是一行空白。
		if pv.Remote.URL != "" {
			body.Remote = &gitRemoteBody{
				URL: pv.Remote.URL, Host: pv.Remote.Host,
				Slug: pv.Remote.Slug, IsGitHub: pv.Remote.IsGitHub,
			}
		}
		if pv.Gh.Status != "" {
			body.Gh = &ghStatusBody{
				Status: pv.Gh.Status, Version: pv.Gh.Version,
				Account: pv.Gh.Account, Remedy: pv.Gh.Remedy,
			}
		}
		writeJSON(w, http.StatusOK, body)
	}
}
