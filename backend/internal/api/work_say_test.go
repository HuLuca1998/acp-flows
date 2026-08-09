package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/api"
	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/internal/app/work"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// M5 U5.1.3 · POST /v1/works/{id}/messages
//
// ★★ 这是「连着说三句，AI 记得前两句」的那条路：同一个工作、同一条会话。

type sayStub struct {
	mu    sync.Mutex
	calls []sayCall
	err   error
}

type sayCall struct {
	workID string
	text   string
}

func (s *sayStub) Say(_ context.Context, workID, text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, sayCall{workID: workID, text: text})
	return s.err
}

func (s *sayStub) Start(context.Context, string, string, string) (work.View, error) {
	return work.View{}, nil
}
func (s *sayStub) List(context.Context) ([]work.View, error) { return nil, nil }
func (s *sayStub) Cancel(context.Context, string) error      { return nil }
func (s *sayStub) RequirementOf(context.Context, string) (work.RequirementView, error) {
	return work.RequirementView{}, nil
}
func (s *sayStub) FreezeRequirement(context.Context, string) error { return nil }
func (s *sayStub) PlanOf(context.Context, string) (work.PlanView, error) {
	return work.PlanView{}, nil
}
func (s *sayStub) PlanHistoryOf(context.Context, string) ([]work.PlanView, error) {
	return nil, nil
}
func (s *sayStub) StartPlanning(context.Context, string) error { return nil }
func (s *sayStub) ContractOf(context.Context, string) (work.ContractView, error) {
	return work.ContractView{}, nil
}
func (s *sayStub) DesignContract(context.Context, string, string) error { return nil }
func (s *sayStub) FreezeContract(context.Context, string, string) error { return nil }
func (s *sayStub) StartUnit(context.Context, string, string) error      { return nil }
func (s *sayStub) AcceptanceOf(context.Context, string, string) (work.AcceptanceView, error) {
	return work.AcceptanceView{}, nil
}
func (s *sayStub) CollectDiffEvidence(
	context.Context, string, string, []string,
) (model.Evidence, error) {
	return model.Evidence{}, nil
}
func (s *sayStub) Prepare(context.Context, string) (port.RepoStatus, error) {
	return port.RepoStatus{}, nil
}
func (s *sayStub) WorktreeOf(context.Context, string) (port.WorktreeState, error) {
	return port.WorktreeState{}, nil
}

func (s *sayStub) snapshot() []sayCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]sayCall(nil), s.calls...)
}

func postSay(t *testing.T, svc api.Config, workID, body string) *httptest.ResponseRecorder {
	t.Helper()
	svc.Token = testToken
	h, err := api.NewRouter(svc)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost,
		"/v1/works/"+workID+"/messages", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+testToken)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// ★ 返回 **202** 而不是 200：一轮要好几分钟，同步等的话请求早超时了。
func TestSayInWork_HappyPathIs202(t *testing.T) {
	svc := &sayStub{}
	rec := postSay(t, api.Config{Works: svc}, "work-01",
		`{"text":"取消后现场证据要保留"}`)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("状态码 = %d, 想要 202（响应体 %s）", rec.Code, rec.Body)
	}
	calls := svc.snapshot()
	if len(calls) != 1 {
		t.Fatalf("转调了 %d 次, 想要 1", len(calls))
	}
	// ★ 原话**一个字不少**地转下去
	if calls[0].workID != "work-01" || calls[0].text != "取消后现场证据要保留" {
		t.Errorf("转调参数 = %+v", calls[0])
	}
}

// ★★ 终态工作翻成 **409**，不是 500。
//
// 500 会让界面提示「服务器出错，再试一次」，而用户一试还是同样的结果。
func TestSayInWork_TerminalIs409(t *testing.T) {
	svc := &sayStub{err: work.ErrNotAcceptingMessages}
	rec := postSay(t, api.Config{Works: svc}, "work-01", `{"text":"再试一次好吗"}`)

	if rec.Code != http.StatusConflict {
		t.Errorf("状态码 = %d, 想要 409", rec.Code)
	}
	var problem struct {
		Type string `json:"type"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &problem)
	if problem.Type != "work_not_accepting_messages" {
		t.Errorf("错误码 = %q——界面按它查 i18n 词条", problem.Type)
	}
}

// 工作不存在时 404，不是 500。
func TestSayInWork_UnknownIs404(t *testing.T) {
	svc := &sayStub{err: model.ErrNotFound}
	rec := postSay(t, api.Config{Works: svc}, "work-nope", `{"text":"在吗"}`)

	if rec.Code != http.StatusNotFound {
		t.Errorf("状态码 = %d, 想要 404", rec.Code)
	}
}

// 空话与坏 JSON 一律 400，且**不转调**。
func TestSayInWork_RejectsBadInput(t *testing.T) {
	for _, body := range []string{`{"text":""}`, `{"text":"   "}`, `{}`, `not json`} {
		svc := &sayStub{}
		rec := postSay(t, api.Config{Works: svc}, "work-01", body)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("body=%s 的状态码 = %d, 想要 400", body, rec.Code)
		}
		if n := len(svc.snapshot()); n != 0 {
			t.Errorf("body=%s 却转调了 %d 次", body, n)
		}
	}
}

// 没装配时 503——不是 500，也不是静静成功。
func TestSayInWork_UnconfiguredSaysSo(t *testing.T) {
	rec := postSay(t, api.Config{}, "work-01", `{"text":"在吗"}`)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("状态码 = %d, 想要 503", rec.Code)
	}
}

// 没带 token 一律 401，且**不转调**。
func TestSayInWork_RequiresToken(t *testing.T) {
	svc := &sayStub{}
	h, err := api.NewRouter(api.Config{Token: testToken, Works: svc})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/works/work-01/messages",
		strings.NewReader(`{"text":"在吗"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("状态码 = %d, 想要 401", rec.Code)
	}
	if n := len(svc.snapshot()); n != 0 {
		t.Errorf("没带 token 却转调了 %d 次", n)
	}
}
