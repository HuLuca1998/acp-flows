package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// memoryBody 对应 openapi 的 Memory。
//
// ★★ **没有正文字段**（INV-MEM-8）：正文只在 md 文件里。
type memoryBody struct {
	ID          string   `json:"id"`
	Kind        string   `json:"kind"`
	Scope       string   `json:"scope"`
	Status      string   `json:"status"`
	SourceRefs  []string `json:"source_refs"`
	CreatedBy   string   `json:"created_by,omitempty"`
	ConfirmedBy string   `json:"confirmed_by,omitempty"`
	Reason      string   `json:"reason,omitempty"`
	Supersedes  string   `json:"supersedes,omitempty"`
	Injectable  bool     `json:"injectable"`
	HistoryLen  int      `json:"history_len,omitempty"`
}

type memoriesBody struct {
	Memories []memoryBody `json:"memories"`
}

// memoryService 是本层需要的能力。接口定义在使用方。
type memoryService interface {
	List(ctx context.Context, f port.MemoryFilter) ([]*model.Memory, error)
	Confirm(ctx context.Context, id, actor string) (*model.Memory, error)
	Reject(ctx context.Context, id, actor string) (*model.Memory, error)
}

func toMemoryBody(m *model.Memory) memoryBody {
	refs := m.SourceRefs()
	if refs == nil {
		// ★ 空依据也要序列化成 `[]` 而不是 null——前端会崩在 `.map` 上。
		refs = []string{}
	}
	return memoryBody{
		ID:          m.ID(),
		Kind:        string(m.Kind()),
		Scope:       string(m.Scope()),
		Status:      string(m.Status()),
		SourceRefs:  refs,
		CreatedBy:   m.CreatedBy(),
		ConfirmedBy: m.ConfirmedBy(),
		Reason:      m.Reason(),
		Supersedes:  m.Supersedes(),
		Injectable:  m.Injectable(),
		HistoryLen:  m.HistoryLen(),
	}
}

// handleListMemories 处理 GET /v1/memories。
func handleListMemories(s memoryService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s == nil {
			writeProblem(w, http.StatusServiceUnavailable,
				"memory_service_unavailable", "Memory service is not configured")
			return
		}

		q := r.URL.Query()
		items, err := s.List(r.Context(), port.MemoryFilter{
			Scope:  q.Get("scope"),
			Status: q.Get("status"),
		})
		if err != nil {
			// ★ 查不动要说出来，不装作「一条都没有」——
			// 装作没有的话，用户以为 Duet 把记忆忘光了。
			writeProblem(w, http.StatusInternalServerError, "memory_list_failed", err.Error())
			return
		}

		body := memoriesBody{Memories: make([]memoryBody, 0, len(items))}
		for _, m := range items {
			body.Memories = append(body.Memories, toMemoryBody(m))
		}
		writeJSON(w, http.StatusOK, body)
	}
}

type reviewRequest struct {
	Decision string `json:"decision"`
	Actor    string `json:"actor"`
}

// handleReviewMemory 处理 POST /v1/memories/{id}/review。
//
// ★★ 这是 `candidate → active` 的**唯一入口**（INV-MEM-2）。
// AI 没有任何路径能自己把候选变成生效。
func handleReviewMemory(s memoryService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s == nil {
			writeProblem(w, http.StatusServiceUnavailable,
				"memory_service_unavailable", "Memory service is not configured")
			return
		}

		id := r.PathValue("id")
		if id == "" {
			writeProblem(w, http.StatusBadRequest, "memory_id_required", "Memory id is required")
			return
		}

		var req reviewRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeProblem(w, http.StatusBadRequest, "invalid_body", err.Error())
			return
		}
		// ★ actor 空值一律拒绝——这就是「用户确认动作」在 HTTP 层的落点。
		// 放行的话，一个后台任务就能把 AI 提的候选变成长期记忆。
		if req.Actor == "" {
			writeProblem(w, http.StatusBadRequest,
				"memory_actor_required", "Reviewing a memory requires an actor")
			return
		}

		var (
			m   *model.Memory
			err error
		)
		switch req.Decision {
		case "confirm":
			m, err = s.Confirm(r.Context(), id, req.Actor)
		case "reject":
			m, err = s.Reject(r.Context(), id, req.Actor)
		default:
			writeProblem(w, http.StatusBadRequest,
				"memory_decision_invalid", "decision must be confirm or reject")
			return
		}

		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				writeProblem(w, http.StatusNotFound, "memory_not_found", err.Error())
				return
			}
			// 状态机拒绝（比如对一条已生效的记忆再点确认）是 409 而不是 500——
			// 那是用户操作的结果，不是我们坏了。
			if errors.Is(err, model.ErrMemoryTransition) {
				writeProblem(w, http.StatusConflict, "memory_transition_rejected", err.Error())
				return
			}
			writeProblem(w, http.StatusBadRequest, "memory_review_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, toMemoryBody(m))
	}
}
