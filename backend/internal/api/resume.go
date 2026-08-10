package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/checkpoint"
)

// checkpointService 是本层需要的检查点用例。接口定义在使用方。
type checkpointService interface {
	ListResumable(ctx context.Context) ([]checkpoint.Resumable, error)
	// Resume 工作区脏时返回 ErrWorktreeDirty，**不改任何状态**。
	Resume(ctx context.Context, workID string) (checkpoint.View, error)
	// ResumeForce 在用户确认之后恢复，跳过脏检查。
	//
	// ★ 「跳过检查」不等于「覆盖改动」——我们本来就不动他的文件。
	ResumeForce(ctx context.Context, workID string) (checkpoint.View, error)
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
				// ★ 停在哪个单元上。没开始做单元时为空——
				// `omitempty` 会把它省掉，界面那一行就不显示，
				// 而不是显示一个空的「单元：」。
				UnitID: it.UnitID,
			}
			if !it.PausedAt.IsZero() {
				row.PausedAt = it.PausedAt.UTC().Format(time.RFC3339)
			}
			body.Resumable = append(body.Resumable, row)
		}
		writeJSON(w, http.StatusOK, body)
	}
}

// resumeResultBody 是一次恢复的结果。
type resumeResultBody struct {
	WorkID string `json:"work_id"`
	State  string `json:"state"`
}

// handleResumeWork 处理 POST /v1/system/resume/{id}。
//
// ★★ **工作区脏时先告知**，不静默恢复：他手工改过那个 worktree，
// 而我们把状态推回可跑之后 AI 会接着往上写。先问一句，
// 他才有机会去看看自己改了什么。
func handleResumeWork(svc checkpointService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			writeProblem(w, http.StatusServiceUnavailable,
				"checkpoint_service_unavailable", "Checkpoint service is not configured")
			return
		}
		workID := r.PathValue("id")
		// ★ force 由用户显式带上——默认永远是「先检查」。
		force := r.URL.Query().Get("force") == "true"

		var (
			view checkpoint.View
			err  error
		)
		if force {
			view, err = svc.ResumeForce(r.Context(), workID)
		} else {
			view, err = svc.Resume(r.Context(), workID)
		}
		if err != nil {
			// ★★ 脏是一个**用户能处理的状态**，不是服务器错误：
			// 409 让前端知道「问他一句再来」，而 500 只会显示成「出错了」。
			if errors.Is(err, checkpoint.ErrWorktreeDirty) {
				writeProblem(w, http.StatusConflict, "worktree_dirty", err.Error())
				return
			}
			writeProblem(w, http.StatusInternalServerError, "resume_failed", err.Error())
			return
		}

		writeJSON(w, http.StatusOK, resumeResultBody{
			WorkID: view.WorkID, State: string(view.State),
		})
	}
}
