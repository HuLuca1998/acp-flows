package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/permission"
)

// permissionAnswerer 是本层需要的应答能力。接口定义在使用方。
type permissionAnswerer interface {
	Answer(workID, askID, optionID string) error
}

type answerPermissionRequest struct {
	AskID    string `json:"ask_id"`
	OptionID string `json:"option_id"`
}

// handleAnswerPermission 处理 POST /v1/works/{id}/permission。
//
// ★ 这一层**不做任何加工**：`option_id` 原样往下传。它不知道 Agent
// 定义了什么，任何「顺手规整一下」都可能把用户的「拒绝」变成「允许」。
func handleAnswerPermission(answers permissionAnswerer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if answers == nil {
			// 503 而不是 404：404 会让人以为是路径写错了，
			// 而真正的原因是装配漏了一根线。
			writeProblem(w, http.StatusServiceUnavailable,
				"permission_service_unavailable", "Permission service is not configured")
			return
		}

		workID := r.PathValue("id")
		if strings.TrimSpace(workID) == "" {
			writeProblem(w, http.StatusBadRequest, "work_id_required", "work id is required")
			return
		}

		var body answerPermissionRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeProblem(w, http.StatusBadRequest, "invalid_request_body", "malformed JSON")
			return
		}
		if strings.TrimSpace(body.AskID) == "" {
			writeProblem(w, http.StatusBadRequest,
				"permission_ask_required", "ask_id is required")
			return
		}
		// ★ 空的 option_id 绝不往下传：Agent 收到一个它不认识的选项，
		// 这一轮会以一种没人预料的方式收场。
		if strings.TrimSpace(body.OptionID) == "" {
			writeProblem(w, http.StatusBadRequest,
				"permission_option_required", "option_id is required")
			return
		}

		if err := answers.Answer(workID, body.AskID, body.OptionID); err != nil {
			// ★ 409 而不是 500：500 会让界面提示「服务器出错，再试一次」，
			// 而用户一试就又发一条应答——无限循环。
			// 409 对应的是「这条已经处理过了」，界面该做的是刷新。
			if errors.Is(err, permission.ErrNotPending) {
				writeProblem(w, http.StatusConflict,
					"permission_not_pending", "this request is no longer waiting")
				return
			}
			writeProblem(w, http.StatusInternalServerError,
				"permission_answer_failed", "could not deliver the answer")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
