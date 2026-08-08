package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/api"
	"github.com/HuLuca1998/acp-flows/backend/internal/app/checkpoint"
)

// U4.1.2 · /v1/system/resume（验收点 V10）

type resumeStub struct {
	items []checkpoint.Resumable
	err   error
}

func (s *resumeStub) ListResumable(context.Context) ([]checkpoint.Resumable, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.items, nil
}

func (s *resumeStub) Resume(context.Context, string) (checkpoint.View, error) {
	return checkpoint.View{}, nil
}

func (s *resumeStub) ResumeForce(context.Context, string) (checkpoint.View, error) {
	return checkpoint.View{}, nil
}

func getResume(t *testing.T, svc api.Config) *httptest.ResponseRecorder {
	t.Helper()
	svc.Token = testToken
	h, err := api.NewRouter(svc)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/system/resume", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestListResumable_ReturnsItems(t *testing.T) {
	paused := time.Date(2026, 8, 8, 10, 0, 0, 0, time.UTC)
	rec := getResume(t, api.Config{Checkpoints: &resumeStub{
		items: []checkpoint.Resumable{{WorkID: "work-01", PausedAt: paused}},
	}})

	if rec.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, 想要 200（%s）", rec.Code, rec.Body)
	}
	var body struct {
		Resumable []struct {
			WorkID   string `json:"work_id"`
			PausedAt string `json:"paused_at"`
		} `json:"resumable"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("响应不是合法 JSON: %v (%s)", err, rec.Body)
	}
	if len(body.Resumable) != 1 || body.Resumable[0].WorkID != "work-01" {
		t.Fatalf("响应 = %+v", body.Resumable)
	}
	if body.Resumable[0].PausedAt == "" {
		t.Error("没带暂停时间——用户认不出哪个是刚才那个")
	}
}

// ★★ 一个都没有时必须是 `[]` 而不是 `null`。
//
// 前端对 null 调 .map() 会白屏——而「一个可恢复的都没有」正是绝大多数
// 用户每次打开应用时的状态。
func TestListResumable_EmptyIsJSONArray(t *testing.T) {
	rec := getResume(t, api.Config{Checkpoints: &resumeStub{}})

	var body struct {
		Resumable json.RawMessage `json:"resumable"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if string(body.Resumable) != "[]" {
		t.Errorf("resumable = %s, 想要 []——\n"+
			"前端对 null 调 .map() 会白屏，而「一个可恢复的都没有」"+
			"正是绝大多数用户每次打开应用时的状态", body.Resumable)
	}
}

// 没配服务时明确回 503，不是 404。
func TestListResumable_UnconfiguredIs503(t *testing.T) {
	rec := getResume(t, api.Config{})

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("状态码 = %d, 想要 503——404 会让人以为是路径写错了", rec.Code)
	}
}

// 查询失败时回 500 且带错误码，不是空列表。
//
// ★ 空列表会让用户以为「没有可恢复的」，而实际是查不了——
// 他会以为自己的工作丢了。
func TestListResumable_FailureIsNotAnEmptyList(t *testing.T) {
	rec := getResume(t, api.Config{Checkpoints: &resumeStub{err: errors.New("数据库锁住了")}})

	if rec.Code == http.StatusOK {
		t.Fatalf("查询失败却回了 200（%s）——\n"+
			"用户以为「没有可恢复的」，而实际是查不了，他会以为自己的工作丢了", rec.Body)
	}
	var problem struct {
		Type string `json:"type"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &problem)
	if problem.Type == "" {
		t.Error("没有机器可读的错误码")
	}
}

// 没带 token 一律 401。
func TestListResumable_RequiresToken(t *testing.T) {
	h, err := api.NewRouter(api.Config{Token: testToken, Checkpoints: &resumeStub{}})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/system/resume", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("状态码 = %d, 想要 401", rec.Code)
	}
}
