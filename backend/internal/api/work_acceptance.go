package api

import (
	"net/http"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/work"
)

// evidenceBody 对应 openapi 的 Evidence。
type evidenceBody struct {
	ID      string `json:"id"`
	UnitID  string `json:"unit_id"`
	Kind    string `json:"kind"`
	Source  string `json:"source"`
	Summary string `json:"summary"`
	Body    string `json:"body,omitempty"`
	// Trustworthy 说这条是不是应用直接采集的。界面据此把 AI 转述的标出来。
	Trustworthy bool `json:"trustworthy"`
	// ★ 空集合序列化成 `[]` 不是 null——前端会崩在 `.map` 上
	Criteria []string `json:"criteria"`
}

// criterionCoverageBody 对应 openapi 的 Acceptance.criteria 元素。
type criterionCoverageBody struct {
	ID   string `json:"id"`
	Text string `json:"text"`
	// EvidenceIDs 空表示**无证据**，不表示通过。
	EvidenceIDs []string `json:"evidence_ids"`
}

// acceptanceBody 对应 openapi 的 Acceptance。
type acceptanceBody struct {
	UnitID   string                  `json:"unit_id"`
	Criteria []criterionCoverageBody `json:"criteria"`
	Evidence []evidenceBody          `json:"evidence"`
}

// handleGetUnitAcceptance 处理 GET /v1/works/{id}/units/{unitId}/acceptance。**只读。**
func handleGetUnitAcceptance(svc workService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workID, unitID, ok := requireUnitID(w, r, svc)
		if !ok {
			return
		}
		view, err := svc.AcceptanceOf(r.Context(), workID, unitID)
		if err != nil {
			writeContractProblem(w, err)
			return
		}
		writeJSON(w, http.StatusOK, toAcceptanceBody(view))
	}
}

// handleCollectUnitEvidence 处理 POST /v1/works/{id}/units/{unitId}/acceptance。
//
// ★★ **应用自己去读 git，不问 AI。**
func handleCollectUnitEvidence(svc workService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workID, unitID, ok := requireUnitID(w, r, svc)
		if !ok {
			return
		}
		if _, err := svc.CollectDiffEvidence(r.Context(), workID, unitID, nil); err != nil {
			writeContractProblem(w, err)
			return
		}
		// ★ 采完回读一次再返回：界面要立刻显示新的对照表，
		// 而不是自己往列表里塞一条猜出来的
		view, err := svc.AcceptanceOf(r.Context(), workID, unitID)
		if err != nil {
			writeContractProblem(w, err)
			return
		}
		writeJSON(w, http.StatusOK, toAcceptanceBody(view))
	}
}

func toAcceptanceBody(v work.AcceptanceView) acceptanceBody {
	body := acceptanceBody{
		UnitID:   v.UnitID,
		Criteria: make([]criterionCoverageBody, 0, len(v.Criteria)),
		Evidence: make([]evidenceBody, 0, len(v.Evidence)),
	}
	for _, c := range v.Criteria {
		ids := c.EvidenceIDs
		if ids == nil {
			ids = []string{}
		}
		body.Criteria = append(body.Criteria, criterionCoverageBody{
			ID: c.ID, Text: c.Text, EvidenceIDs: ids,
		})
	}
	for _, e := range v.Evidence {
		criteria := e.Criteria
		if criteria == nil {
			criteria = []string{}
		}
		body.Evidence = append(body.Evidence, evidenceBody{
			ID: e.ID, UnitID: e.UnitID, Kind: e.Kind, Source: e.Source,
			Summary: e.Summary, Body: e.Body,
			Trustworthy: e.Trustworthy, Criteria: criteria,
		})
	}
	return body
}
