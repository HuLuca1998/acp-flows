package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/api"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// U2.1.1 · /v1/projects（验收点 V4）
//
// 这一层验的是「用例的结论怎么变成 HTTP」。**不在这里再跑一次真文件系统**——
// 「添加不写用户目录」在 app 层用真 git 仓库验过了，在这儿重复一遍
// 只会让 api 测试依赖本机环境。

type stubProjects struct {
	items   []*model.Project
	addErr  error
	removed []string
}

func (s *stubProjects) Add(_ context.Context, path string) (*model.Project, error) {
	if s.addErr != nil {
		return nil, s.addErr
	}
	p, err := model.NewProject("proj-01", path)
	if err != nil {
		return nil, err
	}
	s.items = append(s.items, p)
	return p, nil
}

func (s *stubProjects) List(context.Context) ([]*model.Project, error) { return s.items, nil }

func (s *stubProjects) Remove(_ context.Context, id string) error {
	s.removed = append(s.removed, id)
	return nil
}

func (s *stubProjects) Remedy(p *model.Project) string {
	if !p.IsGitRepo() {
		return "git init"
	}
	return ""
}

type projectBody struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Path          string `json:"path"`
	IsGitRepo     bool   `json:"is_git_repo"`
	DefaultBranch string `json:"default_branch"`
	Remedy        *struct {
		Command string `json:"command"`
	} `json:"remedy"`
}

func newProjectServer(t *testing.T, svc *stubProjects) http.Handler {
	t.Helper()
	h, err := api.NewRouter(api.Config{Token: testToken, Version: "1.4.2", Projects: svc})
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	return h
}

func postJSON(t *testing.T, h http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1"+path, bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestAddProject_ReturnsCreated(t *testing.T) {
	svc := &stubProjects{}
	rec := postJSON(t, newProjectServer(t, svc), "/projects", `{"path":"/Users/me/work/my-app"}`)

	// 201 而不是 200：新建了一个资源，语义要对得上契约
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, 想要 201 (%s)", rec.Code, rec.Body.String())
	}

	var got projectBody
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("解析响应: %v (%s)", err, rec.Body.String())
	}
	if got.Path != "/Users/me/work/my-app" {
		t.Errorf("path = %q", got.Path)
	}
	if got.Name != "my-app" {
		t.Errorf("name = %q, 想要目录名", got.Name)
	}
}

// R3：非 git 目录要带上**能直接敲的命令**，而不是一句「请检查配置」。
func TestAddProject_NonGitCarriesRemedy(t *testing.T) {
	svc := &stubProjects{}
	rec := postJSON(t, newProjectServer(t, svc), "/projects", `{"path":"/Users/me/notes"}`)

	var got projectBody
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Remedy == nil || got.Remedy.Command != "git init" {
		t.Errorf("remedy = %+v, 想要 git init", got.Remedy)
	}
}

// 领域错误要翻成机器可读的错误码，前端据此查 i18n 词条（i18n.md §3）。
// 直接把 Go 的 error 文本吐给前端的话，那串英文会漏进界面。
func TestAddProject_DomainErrorBecomesProblem(t *testing.T) {
	svc := &stubProjects{addErr: model.ErrProjectPathNotAbsolute}
	rec := postJSON(t, newProjectServer(t, svc), "/projects", `{"path":"work/app"}`)

	if rec.Code == http.StatusCreated {
		t.Fatal("相对路径却回了 201")
	}
	var p struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.Type != "project_path_not_absolute" {
		t.Errorf("type = %q, 想要 project_path_not_absolute", p.Type)
	}
}

func TestAddProject_RejectsBadRequest(t *testing.T) {
	for _, body := range []string{`{}`, `{"path":""}`, `not json`} {
		rec := postJSON(t, newProjectServer(t, &stubProjects{}), "/projects", body)
		if rec.Code == http.StatusCreated {
			t.Errorf("body %q 却回了 201", body)
		}
	}
}

// ★ 空列表必须是 `[]` 不是 `null`——前端对 null 调 .map() 会白屏，
// 而「一个项目都没有」正是新用户第一次打开时的状态。
func TestListProjects_EmptyIsArrayNotNull(t *testing.T) {
	rec := do(t, newProjectServer(t, &stubProjects{}), http.MethodGet, "/v1/projects", testToken)

	if body := rec.Body.String(); !strings.Contains(body, `"projects":[]`) {
		t.Errorf("空结果必须是 []，实际：%s", body)
	}
}

func TestRemoveProject_ReturnsNoContent(t *testing.T) {
	svc := &stubProjects{}
	h := newProjectServer(t, svc)

	req := httptest.NewRequest(http.MethodDelete, "/v1/projects/proj-01", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, 想要 204 (%s)", rec.Code, rec.Body.String())
	}
	if len(svc.removed) != 1 || svc.removed[0] != "proj-01" {
		t.Errorf("removed = %v", svc.removed)
	}
}

// 没配用例时不 panic，也不假装成功。
func TestProjects_WithoutServiceDoNotPanic(t *testing.T) {
	h, err := api.NewRouter(api.Config{Token: testToken, Version: "1.4.2"})
	if err != nil {
		t.Fatal(err)
	}

	rec := do(t, h, http.MethodGet, "/v1/projects", testToken)
	if rec.Code == http.StatusOK {
		t.Error("没配用例却回了 200")
	}
}
