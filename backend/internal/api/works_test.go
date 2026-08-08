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
	"github.com/HuLuca1998/acp-flows/backend/internal/app/work"
	"github.com/HuLuca1998/acp-flows/backend/internal/constant"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
	"github.com/HuLuca1998/acp-flows/backend/internal/gitx"
)

// U2.4.1 · /v1/works（验收点 V5）
//
// 这一层验的是「用例的结论怎么变成 HTTP」。**不在这里再切一次真 worktree**——
// 「不往用户项目里写」在 app 层用真 git 仓库验过了，重复一遍只会让 api
// 测试依赖本机的 git。

type stubWorkSvc struct {
	items    []work.View
	startErr error
}

func (s *stubWorkSvc) Start(_ context.Context, project, prompt string) (work.View, error) {
	if s.startErr != nil {
		return work.View{}, s.startErr
	}
	v := work.View{
		ID: "work-01", State: constant.WorkStateClarifying,
		Project: project, Worktree: "/tmp/wt/work-01", Prompt: prompt,
	}
	s.items = append(s.items, v)
	return v, nil
}

func (s *stubWorkSvc) List(context.Context) ([]work.View, error) { return s.items, nil }

func (s *stubWorkSvc) Cancel(context.Context, string) error { return nil }

type workBody struct {
	ID       string `json:"id"`
	State    string `json:"state"`
	Project  string `json:"project"`
	Worktree string `json:"worktree"`
	Prompt   string `json:"prompt"`
}

func newWorkServer(t *testing.T, svc *stubWorkSvc) http.Handler {
	t.Helper()
	h, err := api.NewRouter(api.Config{Token: testToken, Version: "1.4.2", Works: svc})
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	return h
}

func TestStartWork_ReturnsCreated(t *testing.T) {
	svc := &stubWorkSvc{}
	rec := postJSON(t, newWorkServer(t, svc), "/works",
		`{"project":"/Users/me/work/app","prompt":"帮我加个功能"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, 想要 201 (%s)", rec.Code, rec.Body.String())
	}

	var got workBody
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("解析响应: %v (%s)", err, rec.Body.String())
	}
	if got.State != "clarifying" {
		t.Errorf("state = %q", got.State)
	}
	// worktree 要带出来：界面上「在哪干活」是用户会问的第一个问题
	if got.Worktree == "" {
		t.Error("响应里没有 worktree 路径")
	}
}

// ★ 非 git 仓库要翻成**能让用户自己解决**的错误码。
//
// 落到通用错误的话，界面上只有一句「操作失败」——而他其实只需要跑一次
// `git init`。
func TestStartWork_NonRepoBecomesActionableProblem(t *testing.T) {
	svc := &stubWorkSvc{startErr: gitx.ErrNotARepo}
	rec := postJSON(t, newWorkServer(t, svc), "/works",
		`{"project":"/Users/me/notes","prompt":"做点事"}`)

	if rec.Code == http.StatusCreated {
		t.Fatal("非 git 目录却回了 201")
	}
	var p struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.Type != "work_project_not_a_repo" {
		t.Errorf("type = %q, 想要 work_project_not_a_repo——"+
			"落到通用错误的话，用户只看到「操作失败」而他其实只需要 git init", p.Type)
	}
}

func TestStartWork_RejectsBadRequest(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`{"project":"/a"}`,            // 缺 prompt
		`{"prompt":"做点事"}`,            // 缺 project
		`{"project":"","prompt":"x"}`, // 空 project
		`not json`,
	} {
		rec := postJSON(t, newWorkServer(t, &stubWorkSvc{}), "/works", body)
		if rec.Code == http.StatusCreated {
			t.Errorf("body %q 却回了 201", body)
		}
	}
}

// ★ 空列表是 `[]` 不是 `null`——「一个工作都没有」正是新用户第一次打开的状态。
func TestListWorks_EmptyIsArrayNotNull(t *testing.T) {
	rec := do(t, newWorkServer(t, &stubWorkSvc{}), http.MethodGet, "/v1/works", testToken)

	if body := rec.Body.String(); !strings.Contains(body, `"works":[]`) {
		t.Errorf("空结果必须是 []，实际：%s", body)
	}
}

func TestWorks_WithoutServiceDoNotPanic(t *testing.T) {
	h, err := api.NewRouter(api.Config{Token: testToken, Version: "1.4.2"})
	if err != nil {
		t.Fatal(err)
	}

	rec := do(t, h, http.MethodGet, "/v1/works", testToken)
	if rec.Code == http.StatusOK {
		t.Error("没配用例却回了 200")
	}
}

// 无 token 401 —— 工作能看到用户项目里正在发生的一切。
func TestWorks_RequiresToken(t *testing.T) {
	h := newWorkServer(t, &stubWorkSvc{})

	req := httptest.NewRequest(http.MethodGet, "/v1/works", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("无 token 却回了 %d", rec.Code)
	}
}

// ── U3.2.3 · 取消端点 ──────────────────────────────────────

type cancelStub struct {
	mu    sync.Mutex
	calls []string
	err   error
}

func (c *cancelStub) Cancel(_ context.Context, workID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, workID)
	return c.err
}

func (c *cancelStub) Start(context.Context, string, string) (work.View, error) {
	return work.View{}, nil
}
func (c *cancelStub) List(context.Context) ([]work.View, error) { return nil, nil }

func (c *cancelStub) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.calls)
}

func postCancel(t *testing.T, svc *cancelStub, workID string) *httptest.ResponseRecorder {
	t.Helper()
	h, err := api.NewRouter(api.Config{Token: testToken, Works: svc})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/works/"+workID+"/cancel", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestCancelWork_HappyPathIs204(t *testing.T) {
	svc := &cancelStub{}
	rec := postCancel(t, svc, "work-01")

	if rec.Code != http.StatusNoContent {
		t.Fatalf("状态码 = %d, 想要 204（响应体 %s）", rec.Code, rec.Body)
	}
	if svc.count() != 1 {
		t.Errorf("转调了 %d 次, 想要 1", svc.count())
	}
}

// ★★ 状态不允许取消时翻成 **409**，不是 500。
//
// 500 会让界面提示「服务器出错，再试一次」，而用户一试还是同样的结果——
// 409 对应的是「现在不能停」，界面该做的是说清楚为什么。
func TestCancelWork_NotAllowedIs409(t *testing.T) {
	svc := &cancelStub{err: model.ErrCancelNotAllowed}
	rec := postCancel(t, svc, "work-01")

	if rec.Code != http.StatusConflict {
		t.Errorf("状态码 = %d, 想要 409——500 会让界面提示「再试一次」，"+
			"而用户一试还是同样的结果", rec.Code)
	}
	var problem struct {
		Type string `json:"type"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &problem)
	if problem.Type != "work_cancel_not_allowed" {
		t.Errorf("错误码 = %q——界面按它查 i18n 词条", problem.Type)
	}
}

// 工作不存在时 404，不是 500。
func TestCancelWork_UnknownIs404(t *testing.T) {
	svc := &cancelStub{err: model.ErrNotFound}
	rec := postCancel(t, svc, "work-nope")

	if rec.Code != http.StatusNotFound {
		t.Errorf("状态码 = %d, 想要 404", rec.Code)
	}
}

// 没带 token 一律 401，且**不转调**。
func TestCancelWork_RequiresToken(t *testing.T) {
	svc := &cancelStub{}
	h, err := api.NewRouter(api.Config{Token: testToken, Works: svc})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/works/work-01/cancel", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("状态码 = %d, 想要 401", rec.Code)
	}
	if svc.count() != 0 {
		t.Errorf("没带 token 却转调了 %d 次——回环上任何进程都能停掉用户的工作", svc.count())
	}
}
