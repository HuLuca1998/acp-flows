package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/work"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// criterionBody 对应 openapi 的 Contract.criteria 元素。
type criterionBody struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// writeBoundaryBody 对应 openapi 的 WriteBoundary。
type writeBoundaryBody struct {
	// ★ 空集合序列化成 `[]` 不是 null——前端会崩在 `.map` 上
	Allowed   []string `json:"allowed"`
	Forbidden []string `json:"forbidden"`
}

// contractBody 对应 openapi 的 Contract。
type contractBody struct {
	UnitID   string            `json:"unit_id"`
	Version  int               `json:"version"`
	Frozen   bool              `json:"frozen"`
	Criteria []criterionBody   `json:"criteria"`
	Boundary writeBoundaryBody `json:"boundary"`
}

// handleGetUnitContract 处理 GET /v1/works/{id}/units/{unitId}/contract。**只读。**
func handleGetUnitContract(svc workService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, unitID, ok := requireUnitID(w, r, svc)
		if !ok {
			return
		}
		view, err := svc.ContractOf(r.Context(), unitID)
		if err != nil {
			writeContractProblem(w, err)
			return
		}
		writeJSON(w, http.StatusOK, toContractBody(view))
	}
}

// handleDesignUnitContract 处理 POST /v1/works/{id}/units/{unitId}/contract。
//
// ★ 返回 **202**：设计要好几分钟，结果通过事件流回到界面。
func handleDesignUnitContract(svc workService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workID, unitID, ok := requireUnitID(w, r, svc)
		if !ok {
			return
		}
		if err := svc.DesignContract(r.Context(), workID, unitID); err != nil {
			writeContractProblem(w, err)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}
}

// handleFreezeUnitContract 处理 POST /v1/works/{id}/units/{unitId}/contract/freeze。
func handleFreezeUnitContract(svc workService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workID, unitID, ok := requireUnitID(w, r, svc)
		if !ok {
			return
		}
		if err := svc.FreezeContract(r.Context(), workID, unitID); err != nil {
			writeContractProblem(w, err)
			return
		}
		// ★ 冻完回读一次再返回：界面要立刻显示「已冻结」，
		// 而不是自己猜一个状态出来
		view, err := svc.ContractOf(r.Context(), unitID)
		if err != nil {
			writeContractProblem(w, err)
			return
		}
		writeJSON(w, http.StatusOK, toContractBody(view))
	}
}

// handleStartUnit 处理 POST /v1/works/{id}/units/{unitId}/start。
func handleStartUnit(svc workService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workID, unitID, ok := requireUnitID(w, r, svc)
		if !ok {
			return
		}
		if err := svc.StartUnit(r.Context(), workID, unitID); err != nil {
			writeContractProblem(w, err)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}
}

func toContractBody(v work.ContractView) contractBody {
	body := contractBody{
		UnitID: v.UnitID, Version: v.Version, Frozen: v.Frozen,
		Criteria: make([]criterionBody, 0, len(v.Criteria)),
		Boundary: writeBoundaryBody{
			// ★ 空切片而不是 nil：nil 序列化成 null，前端崩在 .map 上
			Allowed:   make([]string, 0, len(v.Allowed)),
			Forbidden: make([]string, 0, len(v.Forbidden)),
		},
	}
	for _, c := range v.Criteria {
		body.Criteria = append(body.Criteria, criterionBody{ID: c.ID, Text: c.Text})
	}
	body.Boundary.Allowed = append(body.Boundary.Allowed, v.Allowed...)
	body.Boundary.Forbidden = append(body.Boundary.Forbidden, v.Forbidden...)
	return body
}

// requireUnitID 取出并校验路径里的工作与单元 id。
func requireUnitID(w http.ResponseWriter, r *http.Request, svc workService) (string, string, bool) {
	workID, ok := requireWorkID(w, r, svc)
	if !ok {
		return "", "", false
	}
	unitID := r.PathValue("unitId")
	if strings.TrimSpace(unitID) == "" {
		writeProblem(w, http.StatusBadRequest, "unit_id_required", "unit id is required")
		return "", "", false
	}
	return workID, unitID, true
}

// writeContractProblem 把契约相关的错误翻成 HTTP 状态码。
func writeContractProblem(w http.ResponseWriter, err error) {
	switch {
	// ★★ 契约没冻结是 **409** 不是 500：这不是故障，是「现在还不能开工」。
	case errors.Is(err, work.ErrContractNotFrozen):
		writeProblem(w, http.StatusConflict, "contract_not_frozen",
			"freeze the contract before starting this unit")
	// ★ 空契约不许冻结：「做完了」得有判据
	case errors.Is(err, model.ErrContractEmpty):
		writeProblem(w, http.StatusConflict, "contract_empty",
			"a contract needs at least one acceptance criterion")
	case errors.Is(err, model.ErrContractFrozen):
		writeProblem(w, http.StatusConflict, "contract_already_frozen",
			"this version is already frozen")
	case errors.Is(err, work.ErrUnitNotInPlan):
		writeProblem(w, http.StatusNotFound, "unit_not_in_plan",
			"the current plan has no such unit")
	case errors.Is(err, model.ErrNotFound):
		writeProblem(w, http.StatusNotFound, "contract_not_found",
			"this unit has no contract yet")
	case errors.Is(err, work.ErrContractsUnavailable), errors.Is(err, work.ErrPlansUnavailable):
		writeProblem(w, http.StatusServiceUnavailable, "contract_store_unavailable",
			"contract storage is not configured")
	default:
		writeProblem(w, http.StatusInternalServerError, "work_operation_failed",
			"could not handle this contract")
	}
}
