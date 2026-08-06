package api_test

// M0 U0.10.1 · duetd serve + 本地回环鉴权
//
// 验收标准见 docs/milestones/M0-acp-foundation.md § S0.10 U0.10.1。
// 这些测试先于实现写就。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/api"
)

const testToken = "test-token-0123456789"

func newServer(t *testing.T) http.Handler {
	t.Helper()
	h, err := api.NewRouter(api.Config{
		Token:   testToken,
		Version: "0.1.0-test",
		Commit:  "abc1234",
	})
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	return h
}

func do(t *testing.T, h http.Handler, method, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// R3 ★ 无 Authorization 一律 401，且不泄漏任何信息。
//
// 这条防的是「本机其他程序静默驱动 Agent 写用户的代码」——
// 只监听回环还不够，回环上的任何进程都能连。
func TestAuth_R3_RejectsWithoutToken(t *testing.T) {
	h := newServer(t)

	tests := []struct {
		name  string
		token string
	}{
		{"完全没有 Authorization 头", ""},
		{"token 为空", " "},
		{"token 不对", "wrong-token"},
		{"token 是正确 token 的前缀", testToken[:5]},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, h, http.MethodGet, "/v1/system/version", tt.token)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("状态码 = %d, want 401", rec.Code)
			}
			// 不能泄漏正确的 token、也不能泄漏版本等内部信息
			body := rec.Body.String()
			if strings.Contains(body, testToken) {
				t.Error("401 响应里泄漏了正确的 token")
			}
			if strings.Contains(body, "0.1.0-test") {
				t.Error("401 响应里泄漏了版本信息")
			}
		})
	}
}

// 带正确 token 时放行。
func TestAuth_AcceptsValidToken(t *testing.T) {
	rec := do(t, newServer(t), http.MethodGet, "/v1/system/version", testToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
}

// R5 · 响应结构与 api/openapi.yaml 的 VersionInfo schema 一致。
func TestSystemVersion_R5_MatchesContract(t *testing.T) {
	rec := do(t, newServer(t), http.MethodGet, "/v1/system/version", testToken)

	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("响应不是合法 JSON: %v", err)
	}

	// openapi.yaml 的 VersionInfo required: [version, platform, arch]
	for _, key := range []string{"version", "platform", "arch"} {
		if _, ok := got[key]; !ok {
			t.Errorf("响应缺少必填字段 %q（见 api/openapi.yaml VersionInfo）", key)
		}
	}
	if got["version"] != "0.1.0-test" {
		t.Errorf("version = %v, want 0.1.0-test", got["version"])
	}
}

// 未知路径返回 404，且格式是 RFC 9457 的 Problem。
func TestNotFound_ReturnsProblem(t *testing.T) {
	rec := do(t, newServer(t), http.MethodGet, "/v1/does-not-exist", testToken)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("状态码 = %d, want 404", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
		t.Errorf("Content-Type = %q, want application/problem+json", ct)
	}

	var p map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("响应不是合法 JSON: %v", err)
	}
	// Problem required: [type, title, status]
	for _, key := range []string{"type", "title", "status"} {
		if _, ok := p[key]; !ok {
			t.Errorf("Problem 缺少必填字段 %q", key)
		}
	}
	// type 必须是机器可读的错误码（snake_case），前端据此查 i18n 词条
	if typ, _ := p["type"].(string); typ != "not_found" {
		t.Errorf("type = %q, want %q", typ, "not_found")
	}
}

// 空 token 的配置必须被拒绝——那等于没有鉴权。
func TestNewRouter_RejectsEmptyToken(t *testing.T) {
	if _, err := api.NewRouter(api.Config{Token: ""}); err == nil {
		t.Fatal("空 token 的配置被接受了，这等于关掉鉴权")
	}
}
