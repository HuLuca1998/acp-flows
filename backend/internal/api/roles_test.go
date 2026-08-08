package api_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/acp/runtime"
	"github.com/HuLuca1998/acp-flows/backend/internal/api"
	"github.com/HuLuca1998/acp-flows/backend/internal/app/role"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// M2 U2.4.1 · 角色表端点
//
// ★ 用**真的** app 服务 + 真的 adapter，不 mock：这条链路的价值就在于
// 「domain 的角色 + adapter 的绑定拼得对不对」，mock 掉任何一头都测不出来。

// roleBody 是响应里的一个角色。★ 用结构体接，不用 map——
// map 的字段名拼错编译器不报错，会写出「打了字段名但永远读不到值」的假断言。
type roleResp struct {
	Roles []struct {
		ID               string   `json:"id"`
		DisplayName      string   `json:"display_name"`
		Operations       []string `json:"operations"`
		Duty             string   `json:"duty"`
		Boundary         string   `json:"boundary"`
		SessionMode      string   `json:"session_mode"`
		ModeName         string   `json:"mode_name"`
		PermissionPolicy string   `json:"permission_policy"`
		RuntimeName      string   `json:"runtime_name"`
		IsPreset         bool     `json:"is_preset"`
		Problem          string   `json:"problem"`
	} `json:"roles"`
}

func getRoles(t *testing.T, cfg api.Config) (*httptest.ResponseRecorder, roleResp) {
	t.Helper()
	cfg.Token = "t"
	h, err := api.NewRouter(cfg)
	if err != nil {
		t.Fatalf("建路由: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/roles", nil)
	req.Header.Set("Authorization", "Bearer t")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var body roleResp
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("解响应: %v\n原文：%s", err, rec.Body.String())
		}
	}
	return rec, body
}

// 八个角色都出来了，字段接得上。
func TestListRoles_ReturnsEightPresets(t *testing.T) {
	rec, body := getRoles(t, api.Config{Roles: role.New(runtime.Bindings{})})
	if rec.Code != http.StatusOK {
		t.Fatalf("状态码 %d：%s", rec.Code, rec.Body.String())
	}
	if len(body.Roles) != 8 {
		t.Fatalf("返回 %d 个角色，想要 8 个", len(body.Roles))
	}

	first := body.Roles[0]
	if first.ID != "requirement_analyst" {
		t.Errorf("第一个是 %q——顺序就是设计稿角色表的行序", first.ID)
	}
	if first.DisplayName == "" || first.Duty == "" || first.Boundary == "" {
		t.Errorf("四要素有空的：%+v", first)
	}
	if len(first.Operations) == 0 {
		t.Error("承担的操作是空的")
	}
	if !first.IsPreset {
		t.Error("预置角色没标 is_preset")
	}
}

// ★★ 档位是**语义**，同时给出翻译好的档名。
//
// 直接返回档名的话，前端就得认识 `plan` / `read-only` 这些品牌相关的取值，
// 而那正是分层要挡住的东西。
func TestListRoles_ExposesSemanticModeAndTranslatedName(t *testing.T) {
	_, body := getRoles(t, api.Config{Roles: role.New(runtime.Bindings{})})

	byID := map[string]string{}
	modes := map[string]string{}
	for _, r := range body.Roles {
		byID[r.ID] = r.ModeName
		modes[r.ID] = r.SessionMode
	}

	// 实现工程师：受控写，绑 codex → agent
	if modes["implementer"] != "guarded_write" {
		t.Errorf("实现工程师的语义档 = %q", modes["implementer"])
	}
	if byID["implementer"] != "agent" {
		t.Errorf("实现工程师在 codex 上的档名 = %q，想要 agent", byID["implementer"])
	}

	// 审查员：只读，绑 claude → plan
	if modes["unit_reviewer"] != "read_only" {
		t.Errorf("审查员的语义档 = %q", modes["unit_reviewer"])
	}
	if byID["unit_reviewer"] != "plan" {
		t.Errorf("审查员在 claude 上的档名 = %q，想要 plan", byID["unit_reviewer"])
	}

	// ★ 同一个「只读」在两端档名不同——这正是要分两层的理由
	if byID["test_runner"] != "read-only" {
		t.Errorf("测试执行者在 codex 上的档名 = %q，想要 read-only", byID["test_runner"])
	}
}

// ★★ 没装配时**不返回 200 空列表**。
//
// 八个预置角色是内置的，用户看到空表只会以为应用坏了——
// 而真正的原因是装配漏了一根线。
func TestListRoles_UnconfiguredIsNotAnEmptyList(t *testing.T) {
	rec, _ := getRoles(t, api.Config{})
	if rec.Code == http.StatusOK {
		t.Fatalf("没装配却回了 200：%s", rec.Body.String())
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("状态码 %d，想要 503", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "roles_unavailable") {
		t.Errorf("没给出可查的错误码：%s", rec.Body.String())
	}
}

// ★ 绑定坏掉的角色**照样列出来**，带着原因。
//
// 跳过的话，用户在界面上看到七个角色，而他不知道少了哪一个、为什么少。
func TestListRoles_BrokenBindingStillListsTheRole(t *testing.T) {
	_, body := getRoles(t, api.Config{Roles: role.New(brokenBindings{})})
	if len(body.Roles) != 8 {
		t.Fatalf("返回 %d 个角色，坏的也要列出来", len(body.Roles))
	}
	for _, r := range body.Roles {
		if r.Problem == "" {
			t.Errorf("%s 的绑定坏了却没说原因", r.ID)
		}
		if r.DisplayName == "" {
			t.Errorf("%s 连名字都没有，用户认不出是哪个角色坏了", r.ID)
		}
	}
}

// brokenBindings 模拟「绑定表配坏了」。
type brokenBindings struct{}

var errBindingBroken = errors.New("绑定表读不出来")

func (brokenBindings) RuntimeFor(string) (string, error) {
	return "", errBindingBroken
}

func (brokenBindings) ModeNameOn(string, model.SessionMode) (string, error) {
	return "", errBindingBroken
}
