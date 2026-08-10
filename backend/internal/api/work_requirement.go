package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/work"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// requirementBody 对应 openapi 的 Requirement。
type requirementBody struct {
	Version int      `json:"version"`
	Items   []string `json:"items"`
	// ★ 空集合序列化成 `[]` 不是 null——前端会崩在 `.map` 上
	OpenFacts []string `json:"open_facts"`
	Frozen    bool     `json:"frozen"`
}

// handleGetWorkRequirement 处理 GET /v1/works/{id}/requirement。**只读。**
func handleGetWorkRequirement(svc workService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := requireWorkID(w, r, svc)
		if !ok {
			return
		}

		view, err := svc.RequirementOf(r.Context(), id)
		if err != nil {
			writeRequirementProblem(w, err)
			return
		}
		writeJSON(w, http.StatusOK, toRequirementBody(view))
	}
}

// handleFreezeWorkRequirement 处理 POST /v1/works/{id}/requirement。
//
// ★★ **由用户点，不由 AI 判断**。AI 说「我觉得问清楚了」和用户说「就这样」
// 是两件事——而冻结之后这一版就进了计划与契约，改不动了。
func handleFreezeWorkRequirement(svc workService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := requireWorkID(w, r, svc)
		if !ok {
			return
		}

		if err := svc.FreezeRequirement(r.Context(), id); err != nil {
			writeRequirementProblem(w, err)
			return
		}
		// ★ 冻结完**回读一次**再返回：界面要立刻显示「v2 已冻结」，
		// 而不是自己猜一个状态出来——猜的那个与库里不一致时没人会发现。
		view, err := svc.RequirementOf(r.Context(), id)
		if err != nil {
			writeRequirementProblem(w, err)
			return
		}
		writeJSON(w, http.StatusOK, toRequirementBody(view))
	}
}

func toRequirementBody(v work.RequirementView) requirementBody {
	body := requirementBody{
		Version: v.Version,
		Frozen:  v.Frozen,
		Items:   make([]string, 0, len(v.Items)),
		// ★ 空切片而不是 nil：nil 序列化成 null，前端崩在 .map 上
		OpenFacts: make([]string, 0, len(v.OpenFacts)),
	}
	body.Items = append(body.Items, v.Items...)
	body.OpenFacts = append(body.OpenFacts, v.OpenFacts...)
	return body
}

// requireWorkID 取出并校验路径里的工作 id，顺带挡住没装配的情形。
func requireWorkID(w http.ResponseWriter, r *http.Request, svc workService) (string, bool) {
	if svc == nil {
		writeProblem(w, http.StatusServiceUnavailable,
			"work_service_unavailable", "Work service is not configured")
		return "", false
	}
	id := r.PathValue("id")
	if strings.TrimSpace(id) == "" {
		writeProblem(w, http.StatusBadRequest, "work_id_required", "work id is required")
		return "", false
	}
	return id, true
}

// writeRequirementProblem 把需求相关的错误翻成 HTTP 状态码。
func writeRequirementProblem(w http.ResponseWriter, err error) {
	switch {
	// ★★ 还有待确认的事实是 **409** 不是 500：这不是故障，
	// 是「现在还不能冻」。界面该做的是把还剩哪几条列出来让用户确认。
	case errors.Is(err, model.ErrOpenFactsRemain):
		writeProblem(w, http.StatusConflict, "requirement_open_facts_remain",
			"there are still open facts to confirm")
	case errors.Is(err, model.ErrRequirementFrozen):
		writeProblem(w, http.StatusConflict, "requirement_already_frozen",
			"this version is already frozen")
	// ★ 还没有需求是 404 —— 那是**新工作的常态**，
	// 界面据此不显示标签，而不是显示一个「v0」。
	case errors.Is(err, model.ErrNotFound):
		writeProblem(w, http.StatusNotFound, "requirement_not_found",
			"this work has no requirement yet")
	case errors.Is(err, work.ErrRequirementsUnavailable):
		writeProblem(w, http.StatusServiceUnavailable, "requirement_store_unavailable",
			"requirement storage is not configured")
	default:
		writeProblem(w, http.StatusInternalServerError, "work_operation_failed",
			"could not read the requirement")
	}
}
