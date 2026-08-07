package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// projectService 是本层需要的项目用例。接口定义在使用方（Go 的惯例）。
type projectService interface {
	Add(ctx context.Context, path string) (*model.Project, error)
	List(ctx context.Context) ([]*model.Project, error)
	Remove(ctx context.Context, id string) error
	// Remedy 返回用户需要敲的命令；不需要做什么时为空。
	Remedy(p *model.Project) string
}

// projectBody 对应 openapi 的 Project。
type projectBody struct {
	ID            string      `json:"id"`
	Name          string      `json:"name"`
	Path          string      `json:"path"`
	IsGitRepo     bool        `json:"is_git_repo"`
	DefaultBranch string      `json:"default_branch,omitempty"`
	Remedy        *remedyBody `json:"remedy,omitempty"`
}

type projectsBody struct {
	// ★ 必须序列化成数组而不是 null。「一个项目都没有」正是新用户
	// 第一次打开时的状态，前端对 null 调 .map() 会白屏。
	Projects []projectBody `json:"projects"`
}

type addProjectRequest struct {
	Path string `json:"path"`
}

// projectProblems 把领域错误映射成机器可读的错误码。
//
// ★ 前端据这个码查 i18n 词条（docs/rules/i18n.md §3）。
// 直接把 Go 的 error 文本吐出去的话，那串英文会漏进中文界面。
var projectProblems = []struct {
	err    error
	code   string
	status int
}{
	{model.ErrProjectPathNotAbsolute, "project_path_not_absolute", http.StatusBadRequest},
	// 路径打错是用户能自己解决的，要和「探测崩了」分开——
	// 显示成同一句通用错误的话，他不知道该改路径还是该找人
	{port.ErrPathNotFound, "project_path_not_found", http.StatusBadRequest},
	{model.ErrProjectIDRequired, "project_id_required", http.StatusBadRequest},
	{model.ErrProjectNameRequired, "project_name_required", http.StatusBadRequest},
	{model.ErrAlreadyExists, "project_already_exists", http.StatusConflict},
	{model.ErrNotFound, "project_not_found", http.StatusNotFound},
}

func writeProjectProblem(w http.ResponseWriter, op string, err error) {
	for _, m := range projectProblems {
		if errors.Is(err, m.err) {
			writeProblem(w, m.status, m.code, op)
			return
		}
	}
	// 认不出来的错误统一成一个码，**不把原文交给前端**——
	// 里面可能有本机路径这类不该出现在界面上的东西。
	writeProblem(w, http.StatusInternalServerError, "project_operation_failed", op)
}

// handleListProjects 处理 GET /v1/projects。
func handleListProjects(svc projectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			writeProblem(w, http.StatusServiceUnavailable,
				"project_service_unavailable", "Project service is not configured")
			return
		}

		items, err := svc.List(r.Context())
		if err != nil {
			writeProjectProblem(w, "list projects", err)
			return
		}

		body := projectsBody{Projects: make([]projectBody, 0, len(items))}
		for _, p := range items {
			body.Projects = append(body.Projects, toProjectBody(svc, p))
		}
		writeJSON(w, http.StatusOK, body)
	}
}

// handleAddProject 处理 POST /v1/projects。
func handleAddProject(svc projectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			writeProblem(w, http.StatusServiceUnavailable,
				"project_service_unavailable", "Project service is not configured")
			return
		}

		var req addProjectRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeProblem(w, http.StatusBadRequest, "invalid_request_body", "malformed JSON")
			return
		}
		if strings.TrimSpace(req.Path) == "" {
			writeProblem(w, http.StatusBadRequest, "project_path_required", "path is required")
			return
		}

		p, err := svc.Add(r.Context(), req.Path)
		if err != nil {
			writeProjectProblem(w, "add project", err)
			return
		}

		// 201 而不是 200：新建了一个资源。
		writeJSON(w, http.StatusCreated, toProjectBody(svc, p))
	}
}

// handleRemoveProject 处理 DELETE /v1/projects/{id}。
//
// **只取消登记，不删用户的任何文件**——这是「移除」与「删除」的全部区别。
func handleRemoveProject(svc projectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			writeProblem(w, http.StatusServiceUnavailable,
				"project_service_unavailable", "Project service is not configured")
			return
		}

		id := r.PathValue("id")
		if id == "" {
			writeProblem(w, http.StatusBadRequest, "project_id_required", "id is required")
			return
		}

		if err := svc.Remove(r.Context(), id); err != nil {
			writeProjectProblem(w, "remove project "+id, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func toProjectBody(svc projectService, p *model.Project) projectBody {
	body := projectBody{
		ID:            p.ID(),
		Name:          p.Name(),
		Path:          p.Path(),
		IsGitRepo:     p.IsGitRepo(),
		DefaultBranch: p.DefaultBranch(),
	}
	// 修复命令由后端给，前端只负责显示——与 Runtime 检测同一套做法。
	// 前端一旦开始自己拼命令，加第三种情形就要改两处。
	if cmd := svc.Remedy(p); cmd != "" {
		body.Remedy = &remedyBody{Command: cmd}
	}
	return body
}
