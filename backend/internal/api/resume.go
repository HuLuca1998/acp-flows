package api

import (
	"context"
	"net/http"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/checkpoint"
)

// checkpointService 是本层需要的检查点用例。接口定义在使用方。
type checkpointService interface {
	ListResumable(ctx context.Context) ([]checkpoint.Resumable, error)
}

// resumableBody 对应 openapi 的 ResumableWork。
type resumableBody struct {
	WorkID       string `json:"work_id"`
	CheckpointID string `json:"checkpoint_id"`
	UnitID       string `json:"unit_id,omitempty"`
	PausedAt     string `json:"paused_at,omitempty"`
}

type resumableListBody struct {
	// ★ 必须序列化成数组而不是 null。「一个可恢复的都没有」正是绝大多数
	// 用户每次打开应用时的状态，而前端对 null 调 .map() 会白屏。
	Resumable []resumableBody `json:"resumable"`
}

// handleListResumable 处理 GET /v1/system/resume。
func handleListResumable(svc checkpointService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			// 503 而不是 404：404 会让人以为是路径写错了
			writeProblem(w, http.StatusServiceUnavailable,
				"checkpoint_service_unavailable", "Checkpoint service is not configured")
			return
		}

		items, err := svc.ListResumable(r.Context())
		if err != nil {
			// ★ **绝不降级成空列表**：那会让用户以为「没有可恢复的」，
			// 而实际是查不了——他会以为自己的工作丢了。
			writeProblem(w, http.StatusInternalServerError,
				"checkpoint_list_failed", "could not list resumable works")
			return
		}

		body := resumableListBody{Resumable: make([]resumableBody, 0, len(items))}
		for _, it := range items {
			row := resumableBody{
				WorkID: it.WorkID,
				// 检查点标识暂用工作标识——一个工作当前只有一个检查点。
				// 契约里它是必填，留空的话前端拿到一个没法引用的条目。
				CheckpointID: it.WorkID,
			}
			if !it.PausedAt.IsZero() {
				row.PausedAt = it.PausedAt.UTC().Format(time.RFC3339)
			}
			body.Resumable = append(body.Resumable, row)
		}
		writeJSON(w, http.StatusOK, body)
	}
}
