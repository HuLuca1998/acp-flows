package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/work"
)

// sayRequest 是 POST /v1/works/{id}/messages 的请求体。
type sayRequest struct {
	Text string `json:"text"`
}

// handleSayInWork 处理 POST /v1/works/{id}/messages。
//
// ★★ 这是「连着说三句，AI 记得前两句」的那条路：同一个工作、同一条会话。
// 前端不走这里而是再 POST /v1/works 的话，用户说第二句时开的是一个新工作——
// 新 worktree、新会话、新时间线，前一句彻底不在上下文里。
//
// ★ 返回 **202** 而不是 200：一轮要好几分钟，同步等的话请求早超时了。
// 结果通过事件流回到界面。
func handleSayInWork(svc workService) http.HandlerFunc {
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

		var req sayRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeProblem(w, http.StatusBadRequest, "invalid_body", "request body must be JSON")
			return
		}
		if strings.TrimSpace(req.Text) == "" {
			writeProblem(w, http.StatusBadRequest, "work_message_required", "text is required")
			return
		}

		if err := svc.Say(r.Context(), workID, req.Text); err != nil {
			writeSayProblem(w, err)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}
}

// writeSayProblem 把「接着说」的错误翻成 HTTP 状态码。
//
// ★ 「这个工作已经结束」是 **409** 不是 500：500 会让界面提示
// 「服务器出错，再试一次」，而用户一试还是同样的结果。
func writeSayProblem(w http.ResponseWriter, err error) {
	code := work.ErrorCode(err)
	switch code {
	case "work_not_accepting_messages":
		writeProblem(w, http.StatusConflict, code, "this work no longer accepts messages")
	case "work_not_found":
		writeProblem(w, http.StatusNotFound, code, "work not found")
	default:
		writeProblem(w, http.StatusInternalServerError, code, "could not deliver this message")
	}
}
