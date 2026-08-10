package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/internal/app/work"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
	"github.com/HuLuca1998/acp-flows/backend/internal/gitx"
)

// workService 是本层需要的工作用例。接口定义在使用方。
type workService interface {
	// Start 开一个工作。baseRef 留空时用仓库当前 HEAD。
	Start(ctx context.Context, project, prompt, baseRef string) (work.View, error)
	List(ctx context.Context) ([]work.View, error)
	// Cancel 停掉一个工作正在跑的那一轮。
	Cancel(ctx context.Context, workID string) error
	// Say 在一个已有的工作里接着说一句——同一个工作、同一条会话。
	Say(ctx context.Context, workID, text string, refs ...string) error
	// RequirementOf 读出这个工作当前的需求快照。没有时返回 model.ErrNotFound。
	RequirementOf(ctx context.Context, workID string) (work.RequirementView, error)
	// FreezeRequirement 冻结当前这一版需求。**由用户点，不由 AI 判断。**
	FreezeRequirement(ctx context.Context, workID string) error
	// PlanOf 读出当前计划。没有时返回 model.ErrNotFound。
	PlanOf(ctx context.Context, workID string) (work.PlanView, error)
	// PlanHistoryOf 列出全部计划版本，从新到旧。
	PlanHistoryOf(ctx context.Context, workID string) ([]work.PlanView, error)
	// StartPlanning 让计划架构师产出一版计划。需求没冻结时拒绝。
	StartPlanning(ctx context.Context, workID string) error
	// ContractOf 读出一个单元的当前契约。
	ContractOf(ctx context.Context, unitID string) (work.ContractView, error)
	// DesignContract 让单元设计师产出一版契约。
	DesignContract(ctx context.Context, workID, unitID string) error
	// FreezeContract 冻结当前这一版契约。**由用户点。**
	FreezeContract(ctx context.Context, workID, unitID string) error
	// StartUnit 让一个单元开工。契约没冻结时拒绝。
	StartUnit(ctx context.Context, workID, unitID string) error
	// AcceptanceOf 把契约的标准与采到的证据对上。
	AcceptanceOf(ctx context.Context, workID, unitID string) (work.AcceptanceView, error)
	// CollectDiffEvidence 采集 diff 证据。**应用自己去读 git，不问 AI。**
	CollectDiffEvidence(
		ctx context.Context, workID, unitID string, criteria []string,
	) (model.Evidence, error)
	// AcceptUnit 验收通过：提交改动并落检查点。**由用户点。**
	AcceptUnit(ctx context.Context, workID, unitID string) (string, error)
	// PendingDecisionsOf 列出还没答的决策。左栏那个亮蓝点靠它。
	PendingDecisionsOf(ctx context.Context, workID string) ([]work.DecisionView, error)
	// AnswerDecision 记下用户的选择。**只能答一次。**
	AnswerDecision(ctx context.Context, workID, decisionID, optionID string) error
	// Prepare 返回开工前的仓库状态。**一个字节都不写。**
	Prepare(ctx context.Context, project string) (port.RepoStatus, error)
	// WorktreeOf 返回一个工作的 git 现场，右栏照它渲染。
	WorktreeOf(ctx context.Context, workID string) (port.WorktreeState, error)
}

// workBody 对应 openapi 的 Work。
type workBody struct {
	ID       string `json:"id"`
	State    string `json:"state"`
	Project  string `json:"project,omitempty"`
	Worktree string `json:"worktree,omitempty"`
	Prompt   string `json:"prompt,omitempty"`
	// Title 是列表里显示的名字，取自用户提的那句需求。
	Title string `json:"title,omitempty"`
}

type worksBody struct {
	// ★ 必须序列化成数组而不是 null。「一个工作都没有」正是新用户
	// 第一次打开时的状态，前端对 null 调 .map() 会白屏。
	Works []workBody `json:"works"`
}

type startWorkRequest struct {
	Project string `json:"project"`
	Prompt  string `json:"prompt"`
	// BaseRef 是用户选的基线，留空时用仓库当前 HEAD。
	BaseRef string `json:"base_ref"`
}

// workProblems 把领域错误映射成机器可读的错误码。
//
// ★ **能让用户自己解决的错误要单独一个码。** 非 git 仓库落到通用错误的话，
// 界面上只有一句「操作失败」——而他其实只需要跑一次 `git init`。
var workProblems = []struct {
	err    error
	code   string
	status int
}{
	{gitx.ErrNotARepo, "work_project_not_a_repo", http.StatusBadRequest},
	// ★★ 这两条要**单独的错误码**，不能落进笼统的 work_operation_failed：
	// 用户看到「准备失败」不知道自己该做什么，而他真正要做的是
	// 「先把那次 merge 收尾」或「先提交一次」。
	{gitx.ErrMidOperation, "work_repo_mid_operation", http.StatusConflict},
	{gitx.ErrNoCommits, "work_repo_no_commits", http.StatusBadRequest},
	{work.ErrNoStatusProbe, "work_status_probe_unavailable", http.StatusServiceUnavailable},
	{gitx.ErrNotADirectory, "work_project_not_found", http.StatusBadRequest},
	{model.ErrProjectPathNotAbsolute, "project_path_not_absolute", http.StatusBadRequest},
	{model.ErrNotFound, "work_not_found", http.StatusNotFound},
}

func writeWorkProblem(w http.ResponseWriter, op string, err error) {
	for _, m := range workProblems {
		if errors.Is(err, m.err) {
			writeProblem(w, m.status, m.code, op)
			return
		}
	}
	// 认不出来的统一成一个码，**不把原文交给前端**——
	// 里面可能有本机路径这类不该出现在界面上的东西。
	writeProblem(w, http.StatusInternalServerError, "work_operation_failed", op)
}

// handleListWorks 处理 GET /v1/works。
func handleListWorks(svc workService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			writeProblem(w, http.StatusServiceUnavailable,
				"work_service_unavailable", "Work service is not configured")
			return
		}

		views, err := svc.List(r.Context())
		if err != nil {
			writeWorkProblem(w, "list works", err)
			return
		}

		body := worksBody{Works: make([]workBody, 0, len(views))}
		for _, v := range views {
			body.Works = append(body.Works, toWorkBody(v))
		}
		writeJSON(w, http.StatusOK, body)
	}
}

// handleStartWork 处理 POST /v1/works。
func handleStartWork(svc workService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			writeProblem(w, http.StatusServiceUnavailable,
				"work_service_unavailable", "Work service is not configured")
			return
		}

		var req startWorkRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeProblem(w, http.StatusBadRequest, "invalid_request_body", "malformed JSON")
			return
		}
		if strings.TrimSpace(req.Project) == "" {
			writeProblem(w, http.StatusBadRequest, "work_project_required", "project is required")
			return
		}
		if strings.TrimSpace(req.Prompt) == "" {
			// 没有需求的工作没有意义，而它会占着一个 worktree
			writeProblem(w, http.StatusBadRequest, "work_prompt_required", "prompt is required")
			return
		}

		v, err := svc.Start(r.Context(), req.Project, req.Prompt, req.BaseRef)
		if err != nil {
			writeWorkProblem(w, "start work", err)
			return
		}

		writeJSON(w, http.StatusCreated, toWorkBody(v))
	}
}

func toWorkBody(v work.View) workBody {
	return workBody{
		ID: v.ID, State: string(v.State),
		Project: v.Project, Worktree: v.Worktree, Prompt: v.Prompt,
		Title: v.Title,
	}
}

// handleCancelWork 处理 POST /v1/works/{id}/cancel。
func handleCancelWork(svc workService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			writeProblem(w, http.StatusServiceUnavailable,
				"work_service_unavailable", "Work service is not configured")
			return
		}
		workID := r.PathValue("id")
		if strings.TrimSpace(workID) == "" {
			writeProblem(w, http.StatusBadRequest, "work_id_required", "work id is required")
			return
		}

		if err := svc.Cancel(r.Context(), workID); err != nil {
			writeCancelProblem(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// writeCancelProblem 把取消的错误翻成 HTTP 状态码。
//
// ★ 「现在不能停」是 **409** 不是 500：500 会让界面提示
// 「服务器出错，再试一次」，而用户一试还是同样的结果。
// 409 对应的是「现在不能停」，界面该做的是说清楚为什么。
func writeCancelProblem(w http.ResponseWriter, err error) {
	code := work.ErrorCode(err)
	switch code {
	case "work_cancel_not_allowed":
		writeProblem(w, http.StatusConflict, code, "this work cannot be cancelled right now")
	case "work_not_found":
		writeProblem(w, http.StatusNotFound, code, "work not found")
	case "work_cancel_unavailable":
		writeProblem(w, http.StatusServiceUnavailable, code, "cancel is not available")
	default:
		writeProblem(w, http.StatusInternalServerError, code, "could not cancel this work")
	}
}

// fileChangeBody 对应 openapi 的 FileChange。
type fileChangeBody struct {
	Path    string `json:"path"`
	Added   int    `json:"added"`
	Removed int    `json:"removed"`
}

// commitInfoBody 对应 openapi 的 CommitInfo。
type commitInfoBody struct {
	SHA     string `json:"sha"`
	Subject string `json:"subject"`
	When    string `json:"when"`
}

// worktreeStateBody 对应 openapi 的 WorktreeState。
type worktreeStateBody struct {
	Branch     string           `json:"branch"`
	BaseCommit string           `json:"base_commit,omitempty"`
	Ahead      int              `json:"ahead"`
	Changes    []fileChangeBody `json:"changes"`
	Commits    []commitInfoBody `json:"commits"`
}

// handleGetWorkWorktree 处理 GET /v1/works/{id}/worktree。**只读。**
func handleGetWorkWorktree(svc workService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			writeProblem(w, http.StatusServiceUnavailable,
				"work_service_unavailable", "Work service is not configured")
			return
		}
		id := r.PathValue("id")
		if id == "" {
			writeProblem(w, http.StatusBadRequest, "work_id_required", "work id is required")
			return
		}

		st, err := svc.WorktreeOf(r.Context(), id)
		if err != nil {
			writeWorkProblem(w, "read worktree", err)
			return
		}

		body := worktreeStateBody{
			Branch: st.Branch, BaseCommit: st.BaseCommit, Ahead: st.Ahead,
			// ★ 空集合序列化成 `[]` 不是 null——前端会崩在 `.map` 上
			Changes: make([]fileChangeBody, 0, len(st.Changes)),
			Commits: make([]commitInfoBody, 0, len(st.Commits)),
		}
		for _, c := range st.Changes {
			body.Changes = append(body.Changes, fileChangeBody{
				Path: c.Path, Added: c.Added, Removed: c.Removed,
			})
		}
		for _, c := range st.Commits {
			body.Commits = append(body.Commits, commitInfoBody{
				SHA: c.SHA, Subject: c.Subject, When: c.When,
			})
		}
		writeJSON(w, http.StatusOK, body)
	}
}
