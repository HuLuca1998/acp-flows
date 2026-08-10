package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/work"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// decisionOptionBody 对应 openapi 的 Decision.options 元素。
type decisionOptionBody struct {
	ID   string `json:"id"`
	Text string `json:"text"`
	// Impact 是「选了它会怎样」。★ 必填——没有它用户在盲选。
	Impact string `json:"impact"`
	// Recommended **只是标记，不是预选**。
	Recommended bool `json:"recommended"`
}

// decisionBody 对应 openapi 的 Decision。
type decisionBody struct {
	ID           string               `json:"id"`
	UnitID       string               `json:"unit_id,omitempty"`
	Level        string               `json:"level"`
	Question     string               `json:"question"`
	Options      []decisionOptionBody `json:"options"`
	AnsweredWith string               `json:"answered_with,omitempty"`
}

type decisionsBody struct {
	// ★ 空集合序列化成 `[]` 不是 null——前端会崩在 `.map` 上
	Decisions []decisionBody `json:"decisions"`
}

type answerDecisionRequest struct {
	OptionID string `json:"option_id"`
}

// handleListPendingDecisions 处理 GET /v1/works/{id}/decisions。
//
// ★★ 左栏那个亮蓝点靠它。
func handleListPendingDecisions(svc workService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workID, ok := requireWorkID(w, r, svc)
		if !ok {
			return
		}
		views, err := svc.PendingDecisionsOf(r.Context(), workID)
		if err != nil {
			writeDecisionProblem(w, err)
			return
		}
		body := decisionsBody{Decisions: make([]decisionBody, 0, len(views))}
		for _, v := range views {
			body.Decisions = append(body.Decisions, toDecisionBody(v))
		}
		writeJSON(w, http.StatusOK, body)
	}
}

// handleAnswerDecision 处理 POST /v1/works/{id}/decisions/{decisionId}。
func handleAnswerDecision(svc workService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workID, ok := requireWorkID(w, r, svc)
		if !ok {
			return
		}
		decisionID := r.PathValue("decisionId")
		if strings.TrimSpace(decisionID) == "" {
			writeProblem(w, http.StatusBadRequest, "decision_id_required", "decision id is required")
			return
		}

		var req answerDecisionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeProblem(w, http.StatusBadRequest, "invalid_body", "request body must be JSON")
			return
		}
		if strings.TrimSpace(req.OptionID) == "" {
			writeProblem(w, http.StatusBadRequest, "option_id_required", "option_id is required")
			return
		}

		if err := svc.AnswerDecision(r.Context(), workID, decisionID, req.OptionID); err != nil {
			writeDecisionProblem(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func toDecisionBody(v work.DecisionView) decisionBody {
	body := decisionBody{
		ID: v.ID, UnitID: v.UnitID, Level: v.Level, Question: v.Question,
		AnsweredWith: v.AnsweredWith,
		Options:      make([]decisionOptionBody, 0, len(v.Options)),
	}
	for _, o := range v.Options {
		body.Options = append(body.Options, decisionOptionBody{
			ID: o.ID, Text: o.Text, Impact: o.Impact, Recommended: o.Recommended,
		})
	}
	return body
}

// writeDecisionProblem 把决策相关的错误翻成 HTTP 状态码。
func writeDecisionProblem(w http.ResponseWriter, err error) {
	switch {
	// ★★ 答过是 **409** 不是 500：这不是故障，是「已经答过了」。
	case errors.Is(err, model.ErrDecisionAnswered):
		writeProblem(w, http.StatusConflict, "decision_answered",
			"this decision has already been answered")
	case errors.Is(err, model.ErrNotFound):
		writeProblem(w, http.StatusNotFound, "decision_not_found", "decision not found")
	case errors.Is(err, work.ErrDecisionsUnavailable):
		writeProblem(w, http.StatusServiceUnavailable, "decision_store_unavailable",
			"decision storage is not configured")
	default:
		writeProblem(w, http.StatusInternalServerError, "work_operation_failed",
			"could not handle this decision")
	}
}
