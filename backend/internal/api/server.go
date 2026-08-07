// Package api 是 HTTP 传输层。
//
// 它只做协议翻译：解析请求 → 调 app 用例 → 序列化响应。**不写业务逻辑。**
//
// 目标形态是 handler 接口由 api/openapi.yaml 生成（铁律 2），当前是 M0 的
// 最小骨架，只有 /v1/system/version，供 make dev-web 冒烟。
// 生成器接入见 docs/plan/roadmap.md 的「已经就绪的地基」（原单元 U0.10.1，编号已废弃）。
package api

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"runtime"
	"strings"
)

// Config 是构造 Router 需要的东西。
type Config struct {
	// Token 是一次性 bearer token，由 duetd 启动时生成。
	Token string
	// Version 是应用版本，取自构建时注入。
	Version string
	// Commit 是构建时的 commit hash。
	Commit string
	// Update 是更新用例。为 nil 时更新端点返回 update_not_configured
	// 而不是 panic —— 纯 Web 部署可以不接发布源。
	Update updateService
	// Runtimes 检测本机装了哪些 ACP Runtime。为 nil 时该端点返回
	// runtime_detection_unavailable，其余端点不受影响。
	Runtimes runtimeDetector
	// Projects 是本地项目用例。为 nil 时相关端点返回
	// project_service_unavailable，其余端点不受影响。
	Projects projectService
}

// ErrNoToken 表示配置里没有 token —— 那等于关掉鉴权。
var ErrNoToken = errors.New("api: token must not be empty")

// NewRouter 组装全部路由与中间件。
func NewRouter(cfg Config) (http.Handler, error) {
	if strings.TrimSpace(cfg.Token) == "" {
		return nil, ErrNoToken
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/system/version", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, versionInfo{
			Version:  cfg.Version,
			Platform: runtime.GOOS,
			Arch:     runtime.GOARCH,
			Commit:   cfg.Commit,
		})
	})

	// 更新链路：只检查、只判断能不能更新。**下载与安装是 Tauri updater 的事**，
	// duetd 会在更新时被替换掉，让它管自己的替换过程是错的（docs/adr/0002）。
	mux.HandleFunc("POST /v1/system/update/check", handleCheckUpdate(cfg.Update))
	mux.HandleFunc("POST /v1/system/update/prepare", handlePrepareUpdate(cfg.Update))

	// 环境检测：只看，不碰用户的 ~/.claude 与 ~/.codex，也不发起任何
	// 会产生费用的模型调用。
	mux.HandleFunc("GET /v1/runtimes", handleListRuntimes(cfg.Runtimes))

	// 项目：添加只登记路径，**往用户的项目目录里写零个字节**。
	mux.HandleFunc("GET /v1/projects", handleListProjects(cfg.Projects))
	mux.HandleFunc("POST /v1/projects", handleAddProject(cfg.Projects))
	mux.HandleFunc("DELETE /v1/projects/{id}", handleRemoveProject(cfg.Projects))

	// 未匹配到任何路由时返回 RFC 9457 的 Problem，而不是 Go 默认的纯文本 404。
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		writeProblem(w, http.StatusNotFound, "not_found", "Resource not found")
	})

	return withAuth(cfg.Token, mux), nil
}

type versionInfo struct {
	Version  string `json:"version"`
	Platform string `json:"platform"`
	Arch     string `json:"arch"`
	Commit   string `json:"commit,omitempty"`
}

// problem 是 RFC 9457 的错误对象。
//
// Type 是**机器可读的错误码**（snake_case），前端据此查 i18n 词条；
// Title 只给开发者看，界面不展示。见 docs/rules/i18n.md §3。
type problem struct {
	Type   string         `json:"type"`
	Title  string         `json:"title"`
	Status int            `json:"status"`
	Detail string         `json:"detail,omitempty"`
	Params map[string]any `json:"params,omitempty"`
}

// withAuth 校验 bearer token。
//
// duetd 只监听 127.0.0.1，但回环上的**任何**本机进程都能连——
// 没有 token 就等于任何程序都能静默驱动 Agent 写用户的代码。
func withAuth(token string, next http.Handler) http.Handler {
	want := []byte("Bearer " + token)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := []byte(r.Header.Get("Authorization"))
		// 定长比较，避免用比较耗时反推 token
		if subtle.ConstantTimeCompare(got, want) != 1 {
			// 401 响应里不带任何内部信息：不回显 token、不回显版本
			writeProblem(w, http.StatusUnauthorized, "unauthorized", "Missing or invalid token")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		// 响应头已经发出去了，改不了状态码。记日志，调用方会看到截断的响应。
		slog.Error("序列化响应失败", "err", err)
	}
}

func writeProblem(w http.ResponseWriter, status int, code, title string) {
	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problem{Type: code, Title: title, Status: status})
}
