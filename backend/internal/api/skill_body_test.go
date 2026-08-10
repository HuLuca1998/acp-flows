package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/api"
	skillstore "github.com/HuLuca1998/acp-flows/backend/internal/fsstore/skill"
)

// M10 U10.7.2 · Skill 正文端点
//
// ★ 与列表同一套真目录真文件（homeWithSkills），不 mock 读取器：
// 这条链路的价值在于「盘上的 SKILL.md 能不能原样到详情栏」。

type skillBodyResp struct {
	Dir         string `json:"dir"`
	Frontmatter string `json:"frontmatter"`
	Text        string `json:"text"`
}

func getSkillBody(t *testing.T, cfg api.Config, dir string) (*httptest.ResponseRecorder, skillBodyResp) {
	t.Helper()
	cfg.Token = "t"
	h, err := api.NewRouter(cfg)
	if err != nil {
		t.Fatalf("建路由: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/skills/"+dir+"/body", nil)
	req.Header.Set("Authorization", "Bearer t")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var body skillBodyResp
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("解响应: %v\n原文：%s", err, rec.Body.String())
		}
	}
	return rec, body
}

const bodyEndpointSkill = `---
name: rust-test-first
version: "2.1"
---

先写失败测试，再实现。
`

// R1/R2：盘上的 SKILL.md 原样到详情——frontmatter 与正文分开给。
func TestGetSkillBody_ReadsFromDisk(t *testing.T) {
	home := homeWithSkills(t, map[string]string{"rust-test-first": bodyEndpointSkill})
	store := skillstore.Store{Home: home}

	rec, body := getSkillBody(t, api.Config{Skills: store, SkillBodies: store}, "rust-test-first")
	if rec.Code != http.StatusOK {
		t.Fatalf("状态码 %d，想要 200：%s", rec.Code, rec.Body.String())
	}
	if body.Dir != "rust-test-first" {
		t.Fatalf("dir=%q，想要 rust-test-first", body.Dir)
	}
	if !strings.Contains(body.Frontmatter, "name: rust-test-first") {
		t.Fatalf("frontmatter 里应有 name 行，得到 %q", body.Frontmatter)
	}
	if !strings.Contains(body.Text, "先写失败测试，再实现。") {
		t.Fatalf("正文丢了：%q", body.Text)
	}
	if strings.Contains(body.Text, "name: rust-test-first") {
		t.Fatalf("正文里不该混着 frontmatter：%q", body.Text)
	}
}

// R4：文件不在 → 404，且 detail 里带路径——空正文与文件丢了是两回事。
func TestGetSkillBody_MissingCarriesPath(t *testing.T) {
	home := homeWithSkills(t, map[string]string{})
	store := skillstore.Store{Home: home}

	rec, _ := getSkillBody(t, api.Config{Skills: store, SkillBodies: store}, "no-such-skill")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("状态码 %d，想要 404：%s", rec.Code, rec.Body.String())
	}
	wantPath := filepath.Join(home, "skills", "no-such-skill", "SKILL.md")
	if !strings.Contains(rec.Body.String(), wantPath) {
		t.Fatalf("响应里应带路径 %q，得到 %s", wantPath, rec.Body.String())
	}
}

// 没装配读取器 → 503 明说，不装作「这条 Skill 没正文」。
func TestGetSkillBody_UnconfiguredIsNotEmpty(t *testing.T) {
	home := homeWithSkills(t, map[string]string{"rust-test-first": bodyEndpointSkill})

	rec, _ := getSkillBody(t, api.Config{Skills: skillstore.Store{Home: home}}, "rust-test-first")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("状态码 %d，想要 503：%s", rec.Code, rec.Body.String())
	}
}
