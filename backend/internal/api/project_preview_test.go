package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/api"
)

// M3 U3.2.1 · 创建项目的预演端点
//
// ★★ 这一族守的是**先说再做**：用户交出来的是他自己的代码仓库，
// 「预演里没说的那件事被做了」是最不该发生的。

type previewResp struct {
	Path      string `json:"path"`
	Name      string `json:"name"`
	IsGitRepo bool   `json:"is_git_repo"`
	Actions   []struct {
		Kind         string   `json:"kind"`
		Path         string   `json:"path"`
		Reason       string   `json:"reason"`
		AlreadyThere bool     `json:"already_there"`
		Lines        []string `json:"lines"`
	} `json:"actions"`
	Skills []struct {
		Name             string `json:"name"`
		Source           string `json:"source"`
		ValidationOK     bool   `json:"validation_ok"`
		ValidationReason string `json:"validation_reason"`
	} `json:"skills"`
	Remote *struct {
		URL      string `json:"url"`
		Slug     string `json:"slug"`
		IsGitHub bool   `json:"is_github"`
	} `json:"remote"`
	Gh *struct {
		Status  string `json:"status"`
		Version string `json:"version"`
		Account string `json:"account"`
		Remedy  string `json:"remedy"`
	} `json:"gh"`
}

func callProjects(t *testing.T, cfg api.Config, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	cfg.Token = "t"
	h, err := api.NewRouter(cfg)
	if err != nil {
		t.Fatalf("建路由: %v", err)
	}
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer t")
	r.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

// R1 · 四块都在：将做什么 · 已有 Skill · remote · gh。
func TestPreviewProject_ReturnsAllFourBlocks(t *testing.T) {
	rec := callProjects(t, api.Config{Projects: &stubProjects{}},
		http.MethodPost, "/v1/projects/preview", `{"path":"/tmp/demo"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("状态码 %d：%s", rec.Code, rec.Body.String())
	}

	var got previewResp
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("解响应: %v\n%s", err, rec.Body.String())
	}

	if len(got.Actions) == 0 {
		t.Error("没有「将做什么」——那用户对着一个空对话框点确认")
	}
	// ★ 每一步都要说得出为什么，不然用户凭什么点确认
	for _, a := range got.Actions {
		if a.Reason == "" {
			t.Errorf("%s 这一步没写为什么", a.Path)
		}
	}
	if len(got.Skills) != 1 {
		t.Fatalf("已有 Skill 返回 %d 条", len(got.Skills))
	}
	// ★ 校验没过的要带原因，且**来源**要标出来——用户要能照着去找
	if got.Skills[0].ValidationReason == "" {
		t.Error("校验没过却不说为什么")
	}
	if got.Skills[0].Source == "" {
		t.Error("没标来源——用户不知道 Duet 翻了他哪些目录")
	}
	if got.Remote == nil || got.Remote.Slug != "o/r" {
		t.Errorf("remote = %+v", got.Remote)
	}
	if got.Gh == nil || got.Gh.Status != "ready" {
		t.Errorf("gh = %+v", got.Gh)
	}
}

// ★★ 预演**不初始化**。
//
// 判据是「初始化器一次都没被调用」——先看后做是这一步的全部意义。
func TestPreviewProject_DoesNotInitialize(t *testing.T) {
	svc := &stubProjects{}
	callProjects(t, api.Config{Projects: svc},
		http.MethodPost, "/v1/projects/preview", `{"path":"/tmp/demo"}`)

	if len(svc.initialized) != 0 {
		t.Errorf("★ 预演却动了手：%v——用户还没点确认", svc.initialized)
	}
}

// ★★ 加项目**默认不初始化**。
//
// 静默往用户的仓库里写东西是最快失去信任的方式。
func TestAddProject_DoesNotInitializeByDefault(t *testing.T) {
	svc := &stubProjects{}
	rec := callProjects(t, api.Config{Projects: svc},
		http.MethodPost, "/v1/projects", `{"path":"/tmp/demo"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("状态码 %d：%s", rec.Code, rec.Body.String())
	}

	if len(svc.initialized) != 0 {
		t.Errorf("★ 没要求初始化却动了手：%v", svc.initialized)
	}
}

// 显式要求了才初始化。
func TestAddProject_InitializesWhenAsked(t *testing.T) {
	svc := &stubProjects{}
	rec := callProjects(t, api.Config{Projects: svc},
		http.MethodPost, "/v1/projects", `{"path":"/tmp/demo","initialize":true}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("状态码 %d：%s", rec.Code, rec.Body.String())
	}

	if len(svc.initialized) != 1 {
		t.Fatalf("要求初始化却没做：%v", svc.initialized)
	}
}

// ★ 初始化失败要说清楚，且**登记不回滚**。
//
// 连登记一起撤的话，用户点了「创建」却什么都没发生，而错误信息一闪而过。
// 留着登记他至少能在列表里看到它、能重试。
func TestAddProject_InitFailureIsReportedNotSilent(t *testing.T) {
	svc := &stubProjects{initErr: errBindingBroken}
	rec := callProjects(t, api.Config{Projects: svc},
		http.MethodPost, "/v1/projects", `{"path":"/tmp/demo","initialize":true}`)

	if rec.Code == http.StatusCreated {
		t.Fatalf("初始化失败却回了 201：%s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "project_init_failed") {
		t.Errorf("没给出可查的错误码：%s", rec.Body.String())
	}
}

// 空路径与坏 JSON 一律拒。
func TestPreviewProject_RejectsBadInput(t *testing.T) {
	for _, body := range []string{`{"path":""}`, `{"path":"   "}`, `{`} {
		rec := callProjects(t, api.Config{Projects: &stubProjects{}},
			http.MethodPost, "/v1/projects/preview", body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("body=%q 的状态码 = %d，想要 400", body, rec.Code)
		}
	}
}

// 没装配时说清楚。
func TestPreviewProject_UnconfiguredSaysSo(t *testing.T) {
	rec := callProjects(t, api.Config{},
		http.MethodPost, "/v1/projects/preview", `{"path":"/tmp/demo"}`)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("状态码 %d，想要 503", rec.Code)
	}
}

// ★ 空集合序列化成 `[]` 不是 null——前端会崩在 `.map` 上。
func TestPreviewProject_EmptyCollectionsAreArrays(t *testing.T) {
	rec := callProjects(t, api.Config{Projects: &stubProjects{}},
		http.MethodPost, "/v1/projects/preview", `{"path":"/tmp/demo"}`)
	body := rec.Body.String()
	if strings.Contains(body, `"actions":null`) || strings.Contains(body, `"skills":null`) {
		t.Errorf("空集合序列化成了 null：%s", body)
	}
}
