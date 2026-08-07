package api_test

// M1 · 一键更新的 HTTP 端点
//
// 契约在 api/openapi.yaml 的 /system/update/check 与 /system/update/prepare。
// 这一层只做协议翻译，业务判断在 app/system —— 所以这里断言的是
// **响应形状与契约一致**，不是「有没有新版本」那类业务结论。

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/api"
	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/internal/app/system"
	"github.com/HuLuca1998/acp-flows/backend/internal/constant"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

type stubSource struct {
	release port.Release
	err     error
}

func (s stubSource) Latest(context.Context) (port.Release, error) { return s.release, s.err }

type stubWorks struct {
	works []*model.Work
	err   error
}

func (s stubWorks) ListWorks(context.Context) ([]*model.Work, error) { return s.works, s.err }

// checkResponse 对应 openapi 的 UpdateStatus。
//
// 用结构体而不是 map：字段名拼错时 map 不报错，会写出「打了字段名却永远读不到值」的假断言。
type checkResponse struct {
	State          string `json:"state"`
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	Notes          string `json:"notes"`
	SizeBytes      int64  `json:"size_bytes"`
	PublishedAt    string `json:"published_at"`
}

// prepareResponse 对应 openapi 的 UpdatePrepareResult。
type prepareResponse struct {
	Status  string `json:"status"`
	Blocked []struct {
		WorkID string `json:"work_id"`
		Reason string `json:"reason"`
	} `json:"blocked"`
	Prepared []struct {
		WorkID       string `json:"work_id"`
		CheckpointID string `json:"checkpoint_id"`
	} `json:"prepared"`
}

func newUpdateServer(t *testing.T, src port.ReleaseSource, works port.WorkLister) http.Handler {
	t.Helper()
	svc, err := system.NewUpdateService(system.UpdateConfig{
		CurrentVersion:   "1.4.2",
		UpdaterAvailable: true,
		Source:           src,
		Works:            works,
	})
	if err != nil {
		t.Fatalf("构造用例失败: %v", err)
	}
	h, err := api.NewRouter(api.Config{
		Token:   testToken,
		Version: "1.4.2",
		Commit:  "abc1234",
		Update:  svc,
	})
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	return h
}

func TestCheckUpdate_MatchesContract(t *testing.T) {
	h := newUpdateServer(t,
		stubSource{release: port.Release{
			Version:   "1.5.0",
			Notes:     "修复取消超时",
			SizeBytes: 18_432_000,
		}},
		stubWorks{})

	rec := do(t, h, http.MethodPost, "/v1/system/update/check", testToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}

	var got checkResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("解析响应失败: %v (%s)", err, rec.Body.String())
	}
	if got.State != "available" {
		t.Errorf("state: want %q, got %q (%+v)", "available", got.State, got)
	}
	if got.CurrentVersion != "1.4.2" {
		t.Errorf("current_version: want %q, got %q", "1.4.2", got.CurrentVersion)
	}
	if got.LatestVersion != "1.5.0" {
		t.Errorf("latest_version: want %q, got %q", "1.5.0", got.LatestVersion)
	}
	if got.Notes != "修复取消超时" {
		t.Errorf("notes: got %q", got.Notes)
	}
}

// 发布源挂了要回 Problem，不能回 200 +「已是最新」。
//
// ★ 回 200 的话，用户界面显示「已是最新版本」，而真相是网络断了。
// 这类静默故障没有任何症状，直到有人发现自己半年没收到更新。
func TestCheckUpdate_SourceFailureReturnsProblem(t *testing.T) {
	h := newUpdateServer(t, stubSource{err: errNetwork}, stubWorks{})

	rec := do(t, h, http.MethodPost, "/v1/system/update/check", testToken)
	if rec.Code == http.StatusOK {
		t.Fatalf("发布源失败时不能回 200，body=%s", rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json; charset=utf-8" {
		t.Errorf("Content-Type: want problem+json, got %q", ct)
	}

	var p struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("解析 Problem 失败: %v", err)
	}
	// type 是机器可读错误码，前端据此查 i18n 词条，不能是中文文案
	if p.Type != "update_check_failed" {
		t.Errorf("problem type: want %q, got %q", "update_check_failed", p.Type)
	}
}

func TestPrepareUpdate_ReadyWhenNoActiveWork(t *testing.T) {
	h := newUpdateServer(t, stubSource{}, stubWorks{})

	rec := do(t, h, http.MethodPost, "/v1/system/update/prepare", testToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}

	var got prepareResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if got.Status != "ready" {
		t.Errorf("status: want %q, got %q", "ready", got.Status)
	}
	// openapi 里 prepared 与 blocked 都是 required，必须是数组而不是 null——
	// 前端 .map() 一个 null 会白屏
	if got.Blocked == nil {
		t.Error("blocked 必须是数组（哪怕为空），不能是 null")
	}
	if got.Prepared == nil {
		t.Error("prepared 必须是数组（哪怕为空），不能是 null")
	}
}

// ★ 有工作在跑时回 blocked，且**仍然是 200**。
//
// blocked 不是错误，是一个正常的业务结论：前端要拿着这个列表告诉用户
// 「这几个工作还在跑，先处理完再更新」。回 4xx 的话前端会当成请求出错。
func TestPrepareUpdate_BlockedIsStillTwoHundred(t *testing.T) {
	h := newUpdateServer(t, stubSource{}, stubWorks{works: []*model.Work{
		model.NewWorkAt("work-08", constant.WorkStateExecuting),
	}})

	rec := do(t, h, http.MethodPost, "/v1/system/update/prepare", testToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("blocked 是业务结论不是错误，status 应为 200, got %d", rec.Code)
	}

	var got prepareResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if got.Status != "blocked" {
		t.Fatalf("status: want %q, got %q", "blocked", got.Status)
	}
	if len(got.Blocked) != 1 || got.Blocked[0].WorkID != "work-08" {
		t.Fatalf("blocked 列表不对: %+v", got.Blocked)
	}
	if got.Blocked[0].Reason != "work_in_progress" {
		t.Errorf("reason: want %q, got %q", "work_in_progress", got.Blocked[0].Reason)
	}
}

// 未配置更新服务时（比如纯 Web 部署没接发布源）端点仍要能响应，不能 panic。
func TestUpdateEndpoints_WithoutServiceDoNotPanic(t *testing.T) {
	h := newServer(t) // Config.Update 为 nil

	for _, path := range []string{"/v1/system/update/check", "/v1/system/update/prepare"} {
		rec := do(t, h, http.MethodPost, path, testToken)
		if rec.Code == 0 || rec.Code == http.StatusOK {
			t.Errorf("%s 未配置更新服务时应返回错误而不是成功, got %d", path, rec.Code)
		}
	}
}

var errNetwork = &netErr{}

type netErr struct{}

func (*netErr) Error() string { return "dial tcp: lookup github.com: no such host" }
