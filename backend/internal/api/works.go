package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/work"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
	"github.com/HuLuca1998/acp-flows/backend/internal/gitx"
)

// workService 是本层需要的工作用例。接口定义在使用方。
type workService interface {
	Start(ctx context.Context, project, prompt string) (work.View, error)
	List(ctx context.Context) ([]work.View, error)
	// Cancel 停掉一个工作正在跑的那一轮。
	Cancel(ctx context.Context, workID string) error
}

// workBody 对应 openapi 的 Work。
type workBody struct {
	ID       string `json:"id"`
	State    string `json:"state"`
	Project  string `json:"project,omitempty"`
	Worktree string `json:"worktree,omitempty"`
	Prompt   string `json:"prompt,omitempty"`
}

type worksBody struct {
	// ★ 必须序列化成数组而不是 null。「一个工作都没有」正是新用户
	// 第一次打开时的状态，前端对 null 调 .map() 会白屏。
	Works []workBody `json:"works"`
}

type startWorkRequest struct {
	Project string `json:"project"`
	Prompt  string `json:"prompt"`
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

		v, err := svc.Start(r.Context(), req.Project, req.Prompt)
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
