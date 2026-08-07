package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/api"
	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
)

// stubDetector 只做一件事：把预置的结论原样交出来。
//
// 这里塞桩是**恰当**的——检测逻辑本身在 internal/acp/runtime 里用真脚本、
// 真 PATH、真退出码测过了。本层要验的只有一件事：那些结论怎么变成 JSON。
// 在这儿再跑一次真检测，测出的是本机装了什么，不是代码对不对。
type stubDetector struct {
	results []port.RuntimeStatus
	// calls 记录被调了几次，用来证明 handler 没重复探测
	calls int
}

func (s *stubDetector) DetectAll(context.Context) []port.RuntimeStatus {
	s.calls++
	return s.results
}

// runtimeBody 对应 openapi 的 Runtime。字段与契约一一对应——
// 这个结构体本身就是断言：多一个字段、少一个字段都会在解析时暴露。
type runtimeBody struct {
	Name          string `json:"name"`
	Status        string `json:"status"`
	Installed     bool   `json:"installed"`
	Authenticated bool   `json:"authenticated"`
	ActiveVersion string `json:"active_version"`
	Path          string `json:"path"`
	Remedy        *struct {
		Command string `json:"command"`
	} `json:"remedy"`
}

type runtimesResponse struct {
	Runtimes []runtimeBody `json:"runtimes"`
}

func newRuntimeServer(t *testing.T, d *stubDetector) http.Handler {
	t.Helper()
	h, err := api.NewRouter(api.Config{Token: testToken, Version: "1.4.2", Runtimes: d})
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	return h
}

// 四种状态各自映射成什么，逐条锁死。
//
// installed / authenticated 这两个布尔是从 status **推导**出来的，不是各存一份——
// 两份真源必然漂移，出现 status=ready 而 installed=false 这种自相矛盾的响应。
func TestListRuntimes_MapsEveryStatus(t *testing.T) {
	d := &stubDetector{results: []port.RuntimeStatus{
		{Name: "claude", Status: "ready", Version: "0.63.0", Path: "/usr/local/bin/claude-agent-acp"},
		{Name: "codex", Status: "not_installed", Remedy: "npm i -g @agentclientprotocol/codex-acp"},
		{Name: "gemini", Status: "not_authenticated", Version: "1.0.0", Remedy: "gemini login"},
		{Name: "broken", Status: "probe_failed", Detail: "signal: killed"},
	}}

	rec := do(t, newRuntimeServer(t, d), http.MethodGet, "/v1/runtimes", testToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}

	var got runtimesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("解析响应失败: %v (%s)", err, rec.Body.String())
	}
	if len(got.Runtimes) != 4 {
		t.Fatalf("返回 %d 个 runtime，想要 4 个", len(got.Runtimes))
	}

	want := []struct {
		status        string
		installed     bool
		authenticated bool
		remedy        string
	}{
		{"ready", true, true, ""},
		{"not_installed", false, false, "npm i -g @agentclientprotocol/codex-acp"},
		{"not_authenticated", true, false, "gemini login"},
		// 探测失败时**两个布尔都是 false**：不知道装没装，就别说装了
		{"probe_failed", false, false, ""},
	}
	for i, w := range want {
		r := got.Runtimes[i]
		if r.Status != w.status {
			t.Errorf("[%d] status = %q, 想要 %q", i, r.Status, w.status)
		}
		if r.Installed != w.installed {
			t.Errorf("[%d %s] installed = %v, 想要 %v", i, r.Status, r.Installed, w.installed)
		}
		if r.Authenticated != w.authenticated {
			t.Errorf("[%d %s] authenticated = %v, 想要 %v", i, r.Status, r.Authenticated, w.authenticated)
		}
		gotRemedy := ""
		if r.Remedy != nil {
			gotRemedy = r.Remedy.Command
		}
		if gotRemedy != w.remedy {
			t.Errorf("[%d %s] remedy = %q, 想要 %q", i, r.Status, gotRemedy, w.remedy)
		}
	}
}

// ★ runtimes 必须是**空数组不是 null**。
//
// 与 update/prepare 那两个字段同一个道理：前端对 null 调 .map() 会白屏，
// 而「一个 runtime 都没检测到」恰恰是新用户第一次打开设置页时的常态——
// 最需要看到修复提示的人，看到的会是一片空白。
func TestListRuntimes_EmptyIsArrayNotNull(t *testing.T) {
	rec := do(t, newRuntimeServer(t, &stubDetector{}), http.MethodGet, "/v1/runtimes", testToken)

	if body := rec.Body.String(); !strings.Contains(body, `"runtimes":[]`) {
		t.Errorf("空结果必须序列化成 []，实际响应体：%s", body)
	}
}

// R4：检测不可用时，其余端点照常工作。
//
// 设置页上「环境检测」和「版本」是两块独立的区域。检测挂了只该让那一块
// 显示不出来，不该让整个设置页打不开。
func TestListRuntimes_MissingDetectorDoesNotBreakOtherEndpoints(t *testing.T) {
	h, err := api.NewRouter(api.Config{Token: testToken, Version: "1.4.2"}) // 不配 Runtimes
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}

	rec := do(t, h, http.MethodGet, "/v1/runtimes", testToken)
	if rec.Code == http.StatusOK {
		t.Errorf("没配检测器却回了 200——界面会把「检测不了」显示成「一个都没装」")
	}

	// 同一个 router 上的其他端点不受影响
	rec = do(t, h, http.MethodGet, "/v1/system/version", testToken)
	if rec.Code != http.StatusOK {
		t.Errorf("/v1/system/version 被连累了：%d (%s)", rec.Code, rec.Body.String())
	}
}

// 一次请求只探一轮。探测要拉起子进程，重复探测会让打开设置页明显变慢。
func TestListRuntimes_DetectsOncePerRequest(t *testing.T) {
	d := &stubDetector{results: []port.RuntimeStatus{{Name: "claude", Status: "ready"}}}
	do(t, newRuntimeServer(t, d), http.MethodGet, "/v1/runtimes", testToken)

	if d.calls != 1 {
		t.Errorf("探测了 %d 轮，想要 1 轮", d.calls)
	}
}
