package api

import (
	"errors"
	"net/http"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/work"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// planUnitBody 对应 openapi 的 PlanUnit。
type planUnitBody struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	// RoleID 是**由哪个角色做**（裁定三）。
	RoleID string `json:"role_id"`
	// RoleDisplayName 认不出时**留空**，不编一个——编出来的名字与角色页
	// 那张表对不上，用户会以为有两个不同的角色。
	RoleDisplayName string `json:"role_display_name,omitempty"`
	// ★ 空集合序列化成 `[]` 不是 null——前端会崩在 `.map` 上
	DependsOn      []string `json:"depends_on"`
	ContractFrozen bool     `json:"contract_frozen"`
	Accepted       bool     `json:"accepted"`
}

// subplanBody 对应 openapi 的 Subplan。
type subplanBody struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	// Status / Done / Total 全是**算出来的**，不是存的字段。
	Status string         `json:"status"`
	Done   int            `json:"done"`
	Total  int            `json:"total"`
	Units  []planUnitBody `json:"units"`
}

// planBody 对应 openapi 的 Plan。
type planBody struct {
	Version int    `json:"version"`
	Title   string `json:"title"`
	// SubplanCount / UnitCount 就是设计稿的「N 子计划 · M 单元」。
	SubplanCount int               `json:"subplan_count"`
	UnitCount    int               `json:"unit_count"`
	Dispositions map[string]string `json:"dispositions,omitempty"`
	Subplans     []subplanBody     `json:"subplans"`
}

type planHistoryBody struct {
	Versions []planBody `json:"versions"`
}

// handleGetWorkPlan 处理 GET /v1/works/{id}/plan。**只读。**
func handleGetWorkPlan(svc workService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := requireWorkID(w, r, svc)
		if !ok {
			return
		}
		view, err := svc.PlanOf(r.Context(), id)
		if err != nil {
			writePlanProblem(w, err)
			return
		}
		writeJSON(w, http.StatusOK, toPlanBody(view))
	}
}

// handleGetWorkPlanHistory 处理 GET /v1/works/{id}/plan/history。
//
// ★ 设计稿计划面板的「变更历史」：改过几版、每次为什么改。
func handleGetWorkPlanHistory(svc workService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := requireWorkID(w, r, svc)
		if !ok {
			return
		}
		views, err := svc.PlanHistoryOf(r.Context(), id)
		if err != nil {
			writePlanProblem(w, err)
			return
		}
		// ★ 空集合序列化成 `[]` 不是 null
		body := planHistoryBody{Versions: make([]planBody, 0, len(views))}
		for _, v := range views {
			body.Versions = append(body.Versions, toPlanBody(v))
		}
		writeJSON(w, http.StatusOK, body)
	}
}

// handleStartWorkPlanning 处理 POST /v1/works/{id}/plan。
//
// ★ 返回 **202**：规划要好几分钟，结果通过事件流回到界面。
func handleStartWorkPlanning(svc workService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := requireWorkID(w, r, svc)
		if !ok {
			return
		}
		if err := svc.StartPlanning(r.Context(), id); err != nil {
			writePlanProblem(w, err)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}
}

func toPlanBody(v work.PlanView) planBody {
	body := planBody{
		Version: v.Version, Title: v.Title,
		SubplanCount: v.SubplanCount, UnitCount: v.UnitCount,
		Dispositions: v.Dispositions,
		Subplans:     make([]subplanBody, 0, len(v.Subplans)),
	}
	for _, sp := range v.Subplans {
		sb := subplanBody{
			ID: sp.ID, Title: sp.Title, Status: sp.Status,
			Done: sp.Done, Total: sp.Total,
			Units: make([]planUnitBody, 0, len(sp.Units)),
		}
		for _, u := range sp.Units {
			deps := u.DependsOn
			if deps == nil {
				deps = []string{}
			}
			sb.Units = append(sb.Units, planUnitBody{
				ID: u.ID, Title: u.Title, RoleID: u.RoleID,
				RoleDisplayName: u.RoleName, DependsOn: deps,
				ContractFrozen: u.ContractFrozen, Accepted: u.Accepted,
			})
		}
		body.Subplans = append(body.Subplans, sb)
	}
	return body
}

// writePlanProblem 把计划相关的错误翻成 HTTP 状态码。
func writePlanProblem(w http.ResponseWriter, err error) {
	switch {
	// ★★ 需求没冻结是 **409** 不是 500：这不是故障，是「现在还不能规划」。
	// 界面该做的是提示他先冻结需求。
	case errors.Is(err, work.ErrRequirementNotFrozen):
		writeProblem(w, http.StatusConflict, "requirement_not_frozen",
			"freeze the requirement before planning")
	// ★ 还没规划是 404 —— 新工作的常态，界面据此不显示计划面板
	case errors.Is(err, model.ErrNotFound):
		writeProblem(w, http.StatusNotFound, "plan_not_found",
			"this work has no plan yet")
	case errors.Is(err, model.ErrPlanVersionNotNext):
		writeProblem(w, http.StatusConflict, "plan_version_conflict",
			"this plan version already exists")
	case errors.Is(err, work.ErrPlansUnavailable),
		errors.Is(err, work.ErrRequirementsUnavailable):
		writeProblem(w, http.StatusServiceUnavailable, "plan_store_unavailable",
			"plan storage is not configured")
	default:
		writeProblem(w, http.StatusInternalServerError, "work_operation_failed",
			"could not read the plan")
	}
}
