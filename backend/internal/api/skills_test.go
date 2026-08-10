package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/api"
	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	skillstore "github.com/HuLuca1998/acp-flows/backend/internal/fsstore/skill"
)

// M2 U2.4.1 · Skill 库端点
//
// ★ 用**真的**目录与真的文件（`t.TempDir`），不 mock 扫描器：
// 这条链路的价值在于「盘上的东西能不能原样到界面」。

type skillResp struct {
	Skills []struct {
		Name             string `json:"name"`
		Dir              string `json:"dir"`
		Version          string `json:"version"`
		Description      string `json:"description"`
		Scope            string `json:"scope"`
		Source           string `json:"source"`
		Status           string `json:"status"`
		ValidationOK     bool   `json:"validation_ok"`
		ValidationReason string `json:"validation_reason"`
	} `json:"skills"`
}

func getSkills(t *testing.T, cfg api.Config, query string) (*httptest.ResponseRecorder, skillResp) {
	t.Helper()
	cfg.Token = "t"
	h, err := api.NewRouter(cfg)
	if err != nil {
		t.Fatalf("建路由: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/skills"+query, nil)
	req.Header.Set("Authorization", "Bearer t")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var body skillResp
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("解响应: %v\n原文：%s", err, rec.Body.String())
		}
	}
	return rec, body
}

// homeWithSkills 造一个真的 `~/.acpflows` 样子的目录。
func homeWithSkills(t *testing.T, skills map[string]string) string {
	t.Helper()
	home := t.TempDir()
	for dir, content := range skills {
		path := filepath.Join(home, "skills", dir)
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		if content == "" {
			continue
		}
		if err := os.WriteFile(filepath.Join(path, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

const goodSkill = `---
name: rust-test-first
description: 先写测试再写实现
version: "2.1"
---
`

const draftSkill = `---
name: git-worktree-guard
version: "0.4"
---
`

// 盘上的东西原样到了界面，包括校验没过的那条与它的原因。
func TestListSkills_CarriesValidationReasonToTheUI(t *testing.T) {
	home := homeWithSkills(t, map[string]string{
		"rust-test-first":    goodSkill,
		"git-worktree-guard": draftSkill,
	})

	rec, body := getSkills(t, api.Config{Skills: skillstore.Store{Home: home}}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("状态码 %d：%s", rec.Code, rec.Body.String())
	}
	if len(body.Skills) != 2 {
		t.Fatalf("返回 %d 条，想要 2 条", len(body.Skills))
	}

	byDir := map[string]int{}
	for i, s := range body.Skills {
		byDir[s.Dir] = i
	}

	good := body.Skills[byDir["rust-test-first"]]
	if !good.ValidationOK || good.Version != "2.1" || good.Description == "" {
		t.Errorf("好的那条没接上：%+v", good)
	}
	// ★ 扫出来的一律是 draft（INV-SKL-1）——扫盘就直接 active 的话，
	// 用户往目录里丢个文件就等于让它进了注入清单
	if good.Status != "draft" {
		t.Errorf("状态 = %q，扫出来的一律该是 draft", good.Status)
	}
	if good.Source == "" {
		t.Error("没标来源——用户不知道 Duet 翻了他哪些目录")
	}

	bad := body.Skills[byDir["git-worktree-guard"]]
	if bad.ValidationOK {
		t.Error("缺 description 的那条通过了校验")
	}
	if !strings.Contains(bad.ValidationReason, "description") {
		t.Errorf("原因 = %q，应该点名 description——设计稿上就是这么显示的", bad.ValidationReason)
	}
}

// ★ 一条都没有时是**空数组**，不是 null。
//
// null 会让前端崩在 `.map` 上，而「一个 skill 都没有」正是新用户的常态。
func TestListSkills_EmptyIsArrayNotNull(t *testing.T) {
	rec, _ := getSkills(t, api.Config{Skills: skillstore.Store{Home: t.TempDir()}}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("状态码 %d：%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"skills":null`) {
		t.Errorf("空集合序列化成了 null：%s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"skills":[]`) {
		t.Errorf("响应里没有空数组：%s", rec.Body.String())
	}
}

// 项目级 Skill：给项目路径就扫那个项目（U3.1.2 被用户提前要了）。
//
// ★ 用**真的**项目目录（`.claude/skills` 约定），不 mock 扫描器。
func TestListSkills_ProjectScopeListsProjectSkills(t *testing.T) {
	project := t.TempDir()
	dir := filepath.Join(project, ".claude", "skills", "repo-conventions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	skillMD := "---\nname: repo-conventions\ndescription: 本仓库的提交规范\nversion: \"1.0\"\n---\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skillMD), 0o644); err != nil {
		t.Fatal(err)
	}

	rec, body := getSkills(t, api.Config{Skills: skillstore.Store{Home: t.TempDir()}},
		"?scope=project&project="+project)
	if rec.Code != http.StatusOK {
		t.Fatalf("状态码 %d，想要 200：%s", rec.Code, rec.Body.String())
	}
	if len(body.Skills) != 1 {
		t.Fatalf("想要 1 条项目级 Skill，得到 %d：%s", len(body.Skills), rec.Body.String())
	}
	got := body.Skills[0]
	if got.Name != "repo-conventions" || got.Scope != "project" {
		t.Fatalf("条目不对：%+v", got)
	}
}

// ★ `scope=project` 不给 project 路径 → 明确报错——
// 回一个永远的空列表的话，用户以为自己的项目 skill 没被认出来。
func TestListSkills_ProjectScopeNeedsPath(t *testing.T) {
	rec, _ := getSkills(t, api.Config{Skills: skillstore.Store{Home: t.TempDir()}}, "?scope=project")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("状态码 %d，想要 400：%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "project_path_required") {
		t.Errorf("没给出可查的错误码：%s", rec.Body.String())
	}
}

// 没装配时说清楚，不装作「一个都没有」。
func TestListSkills_UnconfiguredIsNotAnEmptyList(t *testing.T) {
	rec, _ := getSkills(t, api.Config{}, "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("状态码 %d，想要 503：%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "skills_unavailable") {
		t.Errorf("没给出可查的错误码：%s", rec.Body.String())
	}
}

// ★ 扫不动要说出来，不装作「一个都没有」。
//
// 装作没有的话，用户以为自己的 skill 丢了，而实际是目录读不了。
func TestListSkills_ScanFailureIsReported(t *testing.T) {
	rec, _ := getSkills(t, api.Config{Skills: failingScanner{}}, "")
	if rec.Code == http.StatusOK {
		t.Fatalf("扫描失败却回了 200：%s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "skill_scan_failed") {
		t.Errorf("没给出可查的错误码：%s", rec.Body.String())
	}
}

type failingScanner struct{}

func (failingScanner) ScanGlobal() ([]port.SkillEntry, error) {
	return nil, errBindingBroken
}

func (failingScanner) DiscoverInProject(string) ([]port.SkillEntry, error) {
	return nil, errBindingBroken
}
