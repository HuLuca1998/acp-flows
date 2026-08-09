package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/api"
	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/internal/app/work"
	"github.com/HuLuca1998/acp-flows/backend/internal/gitx"
)

// M4 U4.1.1 · 开工前的仓库状态端点
//
// ★★ 用户还没决定开不开工——这一步**只看不动**。

type prepStub struct {
	status      port.RepoStatus
	err         error
	started     []string
	worktree    port.WorktreeState
	worktreeErr error
}

func (s *prepStub) Start(_ context.Context, project, prompt, baseRef string) (work.View, error) {
	s.started = append(s.started, project+"|"+prompt+"|"+baseRef)
	return work.View{ID: "work-01", Project: project, Prompt: prompt}, nil
}
func (s *prepStub) List(context.Context) ([]work.View, error) { return nil, nil }
func (s *prepStub) Cancel(context.Context, string) error      { return nil }
func (s *prepStub) Prepare(context.Context, string) (port.RepoStatus, error) {
	return s.status, s.err
}
func (s *prepStub) WorktreeOf(context.Context, string) (port.WorktreeState, error) {
	return s.worktree, s.worktreeErr
}

func callWorks(t *testing.T, cfg api.Config, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	cfg.Token = "t"
	h, err := api.NewRouter(cfg)
	if err != nil {
		t.Fatalf("建路由: %v", err)
	}
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer t")
	r.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

type prepResp struct {
	CurrentBranch string   `json:"current_branch"`
	Branches      []string `json:"branches"`
	HeadCommit    string   `json:"head_commit"`
	TrackedDirty  int      `json:"tracked_dirty"`
	Untracked     int      `json:"untracked"`
}

// ★★ 已跟踪与未跟踪**分开报**。
//
// 合成一条的话，「我只是新建了几个还没 add 的文件」和
// 「我改了正在跟踪的代码」会长得一模一样——而对用户是两件完全不同的事。
func TestPrepareWork_SeparatesTrackedFromUntracked(t *testing.T) {
	svc := &prepStub{status: port.RepoStatus{
		CurrentBranch: "main",
		Branches:      []string{"main", "develop"},
		HeadCommit:    "7c1de98",
		TrackedDirty:  2,
		Untracked:     5,
	}}

	rec := callWorks(t, api.Config{Works: svc}, "/v1/works/prepare", `{"project":"/tmp/p"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("状态码 %d：%s", rec.Code, rec.Body.String())
	}

	var got prepResp
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("解响应: %v", err)
	}
	if got.TrackedDirty != 2 || got.Untracked != 5 {
		t.Errorf("tracked=%d untracked=%d，想要 2 / 5——两个数不能合并",
			got.TrackedDirty, got.Untracked)
	}
	if len(got.Branches) != 2 {
		t.Errorf("分支 = %v——用户要从里面选基线", got.Branches)
	}
	if got.HeadCommit == "" {
		t.Error("没给 HEAD——弹层要显示「将从哪个 commit 开始」")
	}
}

// ★★ 探测**不开工**。
//
// 判据是 Start 一次都没被调用——用户还没点确认。
func TestPrepareWork_DoesNotStartAnything(t *testing.T) {
	svc := &prepStub{}
	callWorks(t, api.Config{Works: svc}, "/v1/works/prepare", `{"project":"/tmp/p"}`)

	if len(svc.started) != 0 {
		t.Errorf("★ 只是探测却开了工：%v", svc.started)
	}
}

// ★★ rebase / merge 中途**如实报出去**。
//
// 含糊成一句「准备失败」的话，用户不知道自己该做什么——
// 而他真正要做的是先把那次 merge 收尾。
func TestPrepareWork_MidOperationIsReported(t *testing.T) {
	svc := &prepStub{err: gitx.ErrMidOperation}

	rec := callWorks(t, api.Config{Works: svc}, "/v1/works/prepare", `{"project":"/tmp/p"}`)
	if rec.Code == http.StatusOK {
		t.Fatalf("中途态却回了 200：%s", rec.Body.String())
	}
	// ★ 单独的错误码，不是笼统的 work_operation_failed——
	// 界面据它给出「先把那次 merge 收尾」这句话
	if !strings.Contains(rec.Body.String(), "work_repo_mid_operation") {
		t.Errorf("错误码不够具体：%s", rec.Body.String())
	}
	if rec.Code != http.StatusConflict {
		t.Errorf("状态码 %d，想要 409——那是仓库的状态问题，不是我们坏了", rec.Code)
	}
}

// 空仓库如实报出去。
func TestPrepareWork_EmptyRepoIsReported(t *testing.T) {
	svc := &prepStub{err: gitx.ErrNoCommits}
	rec := callWorks(t, api.Config{Works: svc}, "/v1/works/prepare", `{"project":"/tmp/p"}`)
	if rec.Code == http.StatusOK {
		t.Fatalf("空仓库却回了 200：%s", rec.Body.String())
	}
	// ★ 要说清是「还没提交过」而不是「分支不存在」——
	// 后者会让用户去建分支，而他真正要做的是先提交一次
	if !strings.Contains(rec.Body.String(), "work_repo_no_commits") {
		t.Errorf("错误码不够具体：%s", rec.Body.String())
	}
}

// ★ 基线传得下去。
//
// 传不下去的话，用户在弹层里选了 `develop`，而工作还是从当前分支开的——
// 而当前分支上可能正躺着他没提交完的东西。
func TestStartWork_PassesBaseRefThrough(t *testing.T) {
	svc := &prepStub{}
	rec := callWorks(t, api.Config{Works: svc}, "/v1/works",
		`{"project":"/tmp/p","prompt":"做点事","base_ref":"develop"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("状态码 %d：%s", rec.Code, rec.Body.String())
	}

	if len(svc.started) != 1 {
		t.Fatalf("开工次数 = %d", len(svc.started))
	}
	if !strings.HasSuffix(svc.started[0], "|develop") {
		t.Errorf("基线没传下去：%q——用户选了 develop，工作却从别处开的", svc.started[0])
	}
}

// 不传基线时是空串（后端据此用当前 HEAD）。
func TestStartWork_EmptyBaseRefIsFine(t *testing.T) {
	svc := &prepStub{}
	callWorks(t, api.Config{Works: svc}, "/v1/works", `{"project":"/tmp/p","prompt":"做点事"}`)

	if len(svc.started) != 1 || !strings.HasSuffix(svc.started[0], "|") {
		t.Errorf("started = %v", svc.started)
	}
}

func TestPrepareWork_RejectsBadInput(t *testing.T) {
	for _, body := range []string{`{"project":""}`, `{"project":"   "}`, `{`} {
		rec := callWorks(t, api.Config{Works: &prepStub{}}, "/v1/works/prepare", body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("body=%q 的状态码 = %d，想要 400", body, rec.Code)
		}
	}
}

func TestPrepareWork_UnconfiguredSaysSo(t *testing.T) {
	rec := callWorks(t, api.Config{}, "/v1/works/prepare", `{"project":"/tmp/p"}`)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("状态码 %d，想要 503", rec.Code)
	}
}

// ★ 空分支列表序列化成 `[]` 不是 null——前端会崩在 `.map` 上。
func TestPrepareWork_EmptyBranchesIsArray(t *testing.T) {
	svc := &prepStub{status: port.RepoStatus{HeadCommit: "abc"}}
	rec := callWorks(t, api.Config{Works: svc}, "/v1/works/prepare", `{"project":"/tmp/p"}`)
	if strings.Contains(rec.Body.String(), `"branches":null`) {
		t.Errorf("空分支列表序列化成了 null：%s", rec.Body.String())
	}
}

var _ = errors.Is
