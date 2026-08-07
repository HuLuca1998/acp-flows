package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/system"
)

// updateService 是本层需要的更新用例。
//
// 接口定义在使用方（Go 的惯例），这样 api 包不必 import app 的具体实现，
// 测试也能塞一个桩进来。
type updateService interface {
	Check(ctx context.Context) (system.UpdateStatus, error)
	Prepare(ctx context.Context) (system.PrepareResult, error)
}

// updateStatusBody 对应 openapi 的 UpdateStatus。
type updateStatusBody struct {
	State          string `json:"state"`
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version,omitempty"`
	Notes          string `json:"notes,omitempty"`
	SizeBytes      int64  `json:"size_bytes,omitempty"`
	PublishedAt    string `json:"published_at,omitempty"`
}

// blockedWorkBody 对应 openapi 的 UpdatePrepareResult.blocked[]。
type blockedWorkBody struct {
	WorkID string `json:"work_id"`
	Reason string `json:"reason"`
}

// preparedWorkBody 对应 openapi 的 UpdatePrepareResult.prepared[]。
type preparedWorkBody struct {
	WorkID       string `json:"work_id"`
	CheckpointID string `json:"checkpoint_id"`
}

// updatePrepareBody 对应 openapi 的 UpdatePrepareResult。
type updatePrepareBody struct {
	Status string `json:"status"`
	// ★ prepared 与 blocked 在契约里是 required，**必须序列化成数组而不是 null**。
	// 前端对 null 调 .map() 会白屏。
	Prepared []preparedWorkBody `json:"prepared"`
	Blocked  []blockedWorkBody  `json:"blocked"`
}

// handleCheckUpdate 处理 POST /v1/system/update/check。
func handleCheckUpdate(svc updateService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			writeProblem(w, http.StatusServiceUnavailable,
				"update_not_configured", "Update service is not configured")
			return
		}

		status, err := svc.Check(r.Context())
		if err != nil {
			// ★ 绝不降级成 200 +「已是最新版本」：那样网络故障会伪装成
			// 「没有更新」，用户永远不会知道自己在用旧版本。
			slog.Error("检查更新失败", "err", err)
			writeProblem(w, http.StatusBadGateway,
				"update_check_failed", "Failed to query the release source")
			return
		}

		body := updateStatusBody{
			State:          string(status.State),
			CurrentVersion: status.CurrentVersion,
			LatestVersion:  status.LatestVersion,
			Notes:          status.Notes,
			SizeBytes:      status.SizeBytes,
		}
		if !status.PublishedAt.IsZero() {
			body.PublishedAt = status.PublishedAt.UTC().Format(time.RFC3339)
		}
		writeJSON(w, http.StatusOK, body)
	}
}

// handlePrepareUpdate 处理 POST /v1/system/update/prepare。
func handlePrepareUpdate(svc updateService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			writeProblem(w, http.StatusServiceUnavailable,
				"update_not_configured", "Update service is not configured")
			return
		}

		result, err := svc.Prepare(r.Context())
		if err != nil {
			slog.Error("更新前准备失败", "err", err)
			writeProblem(w, http.StatusInternalServerError,
				"update_prepare_failed", "Failed to determine whether it is safe to update")
			return
		}

		// 两个数组都初始化成空切片，保证序列化出 [] 而不是 null。
		body := updatePrepareBody{
			Status:   string(result.Status),
			Prepared: []preparedWorkBody{},
			Blocked:  []blockedWorkBody{},
		}
		for _, b := range result.Blocked {
			body.Blocked = append(body.Blocked, blockedWorkBody{WorkID: b.WorkID, Reason: b.Reason})
		}

		// ★ blocked 是**业务结论不是错误**，仍然回 200：
		// 前端要拿着这个列表告诉用户「这几个工作还在跑」。
		// 回 4xx 的话前端会当成请求出错，把列表丢掉。
		writeJSON(w, http.StatusOK, body)
	}
}
