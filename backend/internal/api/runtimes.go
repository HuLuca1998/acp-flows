package api

import (
	"context"
	"net/http"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
)

// runtimeDetector 是本层需要的检测能力。接口定义在使用方（Go 的惯例），
// 这样 api 包不必认识 acp/runtime 的具体实现——分层由 depguard 强制。
type runtimeDetector interface {
	DetectAll(ctx context.Context) []port.RuntimeStatus
}

// remedyBody 对应 openapi 的 Runtime.remedy。
type remedyBody struct {
	Command string `json:"command"`
}

// runtimeBody 对应 openapi 的 Runtime。
type runtimeBody struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	// ★ installed / authenticated 是从 Status **推导**出来的，不是各存一份。
	// 两份真源必然漂移，早晚出现 status=ready 而 installed=false 这种
	// 自相矛盾的响应，而界面会同时相信两个。
	Installed     bool        `json:"installed"`
	Authenticated bool        `json:"authenticated"`
	ActiveVersion string      `json:"active_version,omitempty"`
	Path          string      `json:"path,omitempty"`
	Remedy        *remedyBody `json:"remedy,omitempty"`
}

type runtimesBody struct {
	// ★ 必须序列化成数组而不是 null。「一个都没检测到」正是新用户第一次
	// 打开设置页时的常态——最需要看到修复提示的人不该看到白屏。
	Runtimes []runtimeBody `json:"runtimes"`
}

// handleListRuntimes 处理 GET /v1/runtimes。
func handleListRuntimes(d runtimeDetector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if d == nil {
			// 不回 200 空列表：那会让界面把「检测不了」显示成「一个都没装」，
			// 并建议用户去安装已经装好的东西。
			writeProblem(w, http.StatusServiceUnavailable,
				"runtime_detection_unavailable", "Runtime detection is not configured")
			return
		}

		results := d.DetectAll(r.Context())
		body := runtimesBody{Runtimes: make([]runtimeBody, 0, len(results))}
		for _, res := range results {
			item := runtimeBody{
				Name:          res.Name,
				Status:        res.Status,
				Installed:     installedFor(res.Status),
				Authenticated: res.Status == string(statusReady),
				ActiveVersion: res.Version,
				Path:          res.Path,
			}
			if res.Remedy != "" {
				item.Remedy = &remedyBody{Command: res.Remedy}
			}
			body.Runtimes = append(body.Runtimes, item)
		}
		writeJSON(w, http.StatusOK, body)
	}
}

// runtimeStatus 是契约里 Runtime.status 的取值。
//
// 与 acp/runtime 的常量刻意分开：那边是检测实现，这边是线格式。
// 合并成一份会让 api 包 import 基础设施包，分层就破了。
type runtimeStatus string

const (
	statusReady        runtimeStatus = "ready"
	statusNotInstalled runtimeStatus = "not_installed"
	statusNotAuthed    runtimeStatus = "not_authenticated"
	statusProbeFailed  runtimeStatus = "probe_failed"
)

// installedFor 从状态推导「装没装」。
//
// ★ probe_failed 判为 false 而不是 true：检测失败时我们不知道装没装，
// 而 installed=true 会让界面显示成「已安装」——那是在替用户下一个
// 我们并没有依据的结论。
func installedFor(status string) bool {
	switch runtimeStatus(status) {
	case statusReady, statusNotAuthed:
		return true
	case statusNotInstalled, statusProbeFailed:
		return false
	default:
		// 认不出来的状态一律保守处理。新增状态时这里不会漏——
		// 漏了的表现是「显示成没装」，不是「显示成装好了」。
		return false
	}
}
