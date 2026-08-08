package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/internal/app/project"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// projectService 是本层需要的项目用例。接口定义在使用方（Go 的惯例）。
type projectService interface {
	Add(ctx context.Context, path string) (*model.Project, error)
	List(ctx context.Context) ([]*model.Project, error)
	Remove(ctx context.Context, id string) error
	// Remedy 返回用户需要敲的命令；不需要做什么时为空。
	Remedy(p *model.Project) string
	// PreviewInit 算出「把这个目录交给 Duet 会发生什么」。**一个字节都不写。**
	PreviewInit(ctx context.Context, path string) (*project.Preview, error)
	// InitializeAt 照预演给出的同一份计划执行。
	InitializeAt(path string) error
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
	// Initialize 为真时照 `/projects/preview` 的计划创建 `.acpflows/`。
	//
	// ★ **默认 false**：静默往用户的仓库里写东西是最快失去信任的方式。
	// 前端必须先调 preview 把要做的事讲给他听。
	Initialize bool `json:"initialize"`
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

		if req.Initialize {
			// ★ 登记成功之后才初始化，且**初始化失败不回滚登记**：
			// 项目已经在列表里了，用户能看到它、能重试初始化。
			// 连登记一起撤的话，他点了「创建」却什么都没发生，
			// 而错误信息一闪而过。
			if initErr := svc.InitializeAt(p.Path()); initErr != nil {
				writeProblem(w, http.StatusInternalServerError,
					"project_init_failed", initErr.Error())
				return
			}
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

// previewRequest 是 POST /v1/projects/preview 的请求体。
type previewRequest struct {
	Path string `json:"path"`
}

// projectActionBody 对应 openapi 的 ProjectAction。
type projectActionBody struct {
	Kind         string   `json:"kind"`
	Path         string   `json:"path"`
	Reason       string   `json:"reason"`
	AlreadyThere bool     `json:"already_there"`
	Lines        []string `json:"lines,omitempty"`
}

// gitRemoteBody 对应 openapi 的 GitRemote。
type gitRemoteBody struct {
	URL      string `json:"url,omitempty"`
	Host     string `json:"host,omitempty"`
	Slug     string `json:"slug,omitempty"`
	IsGitHub bool   `json:"is_github"`
}

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
