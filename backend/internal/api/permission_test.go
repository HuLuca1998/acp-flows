package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/api"
	"github.com/HuLuca1998/acp-flows/backend/internal/app/permission"
)

// U3.1.4 · 应答端点（验收点 V8）
//
// ★ 这一层薄：校验入参、转调 Broker、把错误翻成 HTTP 状态码。
// 断言集中在「翻错了会怎样」——把 409 翻成 500 的话，
// 界面会提示「服务器出错，再试一次」，而用户一试就又发一条应答。

// answerRecorder 记下收到的应答，并可以模拟失败。
type answerRecorder struct {
	mu    sync.Mutex
	calls [][3]string
	err   error
}

func (a *answerRecorder) Answer(workID, askID, optionID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls = append(a.calls, [3]string{workID, askID, optionID})
	return a.err
}

func (a *answerRecorder) snapshot() [][3]string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([][3]string(nil), a.calls...)
}

func permissionRouter(t *testing.T, answers *answerRecorder) http.Handler {
	t.Helper()
	h, err := api.NewRouter(api.Config{Token: testToken, Permissions: answers})
	if err != nil {
		t.Fatalf("组装路由失败: %v", err)
	}
	return h
}

func postAnswer(t *testing.T, h http.Handler, workID string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/works/"+workID+"/permission", bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// ★★ R2：用户选的 optionId **原样送到 Broker**。
//
// 这一层做任何加工都是错的——它不知道 Agent 定义了什么。
func TestAnswerPermission_PassesOptionIDVerbatim(t *testing.T) {
	answers := &answerRecorder{}
	h := permissionRouter(t, answers)

	rec := postAnswer(t, h, "work-01", map[string]string{
		"ask_id": "ask-01", "option_id": "opt-deny",
	})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("状态码 = %d, 想要 204（响应体 %s）", rec.Code, rec.Body)
	}
	got := answers.snapshot()
	if len(got) != 1 {
		t.Fatalf("转调了 %d 次, 想要 1", len(got))
	}
	if got[0] != [3]string{"work-01", "ask-01", "opt-deny"} {
		t.Errorf("转调参数 = %v——这一层做了加工，而它不知道 Agent 定义了什么", got[0])
	}
}

// ★★ R4：重复应答要翻成 **409**，不是 500。
//
// 翻成 500 的话，界面提示「服务器出错，再试一次」，而用户一试就又发一条——
// 无限循环。409 对应的是「这条已经处理过了」，界面该做的是刷新。
func TestAnswerPermission_R4_DuplicateIs409(t *testing.T) {
	answers := &answerRecorder{err: permission.ErrNotPending}
	h := permissionRouter(t, answers)

	rec := postAnswer(t, h, "work-01", map[string]string{
		"ask_id": "ask-01", "option_id": "opt-allow",
	})

	if rec.Code != http.StatusConflict {
		t.Errorf("状态码 = %d, 想要 409——翻成 500 的话界面会提示「再试一次」，"+
			"而用户一试就又发一条应答", rec.Code)
	}
	var problem struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("响应体不是 Problem: %v (%s)", err, rec.Body)
	}
	if problem.Type != "permission_not_pending" {
		t.Errorf("错误码 = %q——界面按它查 i18n 词条", problem.Type)
	}
}

// ★ 缺字段要拒，而且**不能转调 Broker**。
//
// 空的 option_id 转下去的话，Agent 收到一个它不认识的选项，
// 这一轮会以一种没人预料的方式收场。
func TestAnswerPermission_RejectsMissingFields(t *testing.T) {
	tests := []struct {
		name string
		body map[string]string
		code string
	}{
		{"缺 ask_id", map[string]string{"option_id": "opt-allow"}, "permission_ask_required"},
		{"缺 option_id", map[string]string{"ask_id": "ask-01"}, "permission_option_required"},
		{"option_id 是空串", map[string]string{"ask_id": "ask-01", "option_id": ""}, "permission_option_required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			answers := &answerRecorder{}
			rec := postAnswer(t, permissionRouter(t, answers), "work-01", tt.body)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("状态码 = %d, 想要 400", rec.Code)
			}
			if n := len(answers.snapshot()); n != 0 {
				t.Errorf("转调了 %d 次——空的 option_id 送到 Agent，"+
					"这一轮会以一种没人预料的方式收场", n)
			}
			var problem struct {
				Type string `json:"type"`
			}
			_ = json.Unmarshal(rec.Body.Bytes(), &problem)
			if problem.Type != tt.code {
				t.Errorf("错误码 = %q, 想要 %q", problem.Type, tt.code)
			}
		})
	}
}

// 没配 Broker 时明确回 503，不是 404。
//
// 404 会让人以为是路径写错了，而真正的原因是装配漏了一根线。
func TestAnswerPermission_UnconfiguredIs503(t *testing.T) {
	h, err := api.NewRouter(api.Config{Token: testToken})
	if err != nil {
		t.Fatal(err)
	}
	rec := postAnswer(t, h, "work-01", map[string]string{
		"ask_id": "ask-01", "option_id": "opt-allow",
	})

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("状态码 = %d, 想要 503——404 会让人以为是路径写错了", rec.Code)
	}
}

// 没带 token 一律 401，且**不转调**。
func TestAnswerPermission_RequiresToken(t *testing.T) {
	answers := &answerRecorder{}
	h := permissionRouter(t, answers)

	req := httptest.NewRequest(http.MethodPost, "/v1/works/work-01/permission",
		bytes.NewReader([]byte(`{"ask_id":"a","option_id":"o"}`)))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("状态码 = %d, 想要 401", rec.Code)
	}
	if n := len(answers.snapshot()); n != 0 {
		t.Errorf("没带 token 却转调了 %d 次——回环上任何进程都能替用户点「允许」", n)
	}
}
