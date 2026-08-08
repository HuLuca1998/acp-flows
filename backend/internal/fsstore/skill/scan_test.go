package skill_test

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
	"github.com/HuLuca1998/acp-flows/backend/internal/fsstore/skill"
)

// 契约来源：docs/spec/domain-model.md §15（INV-SKL-1/2/6）
//           design/INVENTORY.md §十（Skill 页）+ §五（创建项目时扫已有 skill）
//
// ★ 用真的目录和真的文件（t.TempDir），不 mock 文件系统——
// 「符号链接不跟出去」「扫完一个字节没改」这两条，mock 一个 fs 是测不出来的。

// writeSkill 在 root 下造一个真的 skill 目录。
func writeSkill(t *testing.T, root, dir, content string) string {
	t.Helper()
	path := filepath.Join(root, dir)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if content != "" {
		if err := os.WriteFile(filepath.Join(path, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

// 真实的 SKILL.md 样子（照本仓库自己的 skill 写的）。
const rustTestFirst = `---
name: rust-test-first
description: 先写测试再写实现，测试要能证明契约
version: "2.1"
compatibility: cargo >= 1.80
---

# rust-test-first

## 何时使用
`

const missingDescription = `---
name: git-worktree-guard
version: "0.4"
---

# git-worktree-guard
`

func scanTemp(t *testing.T, root string) []skill.Entry {
	t.Helper()
	got, err := skill.Scan(skill.Options{
		Root: root, Scope: model.SkillScopeGlobal, Source: ".acpflows/skills",
	})
	if err != nil {
		t.Fatalf("扫描 %s: %v", root, err)
	}
	return got
}

// R1 · 扫得到 frontmatter 的各个字段。
func TestScan_R1_ParsesFrontmatter(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "rust-test-first", rustTestFirst)

	got := scanTemp(t, root)
	if len(got) != 1 {
		t.Fatalf("扫到 %d 个，想要 1 个", len(got))
	}
	e := got[0]
	if e.Name != "rust-test-first" {
		t.Errorf("name = %q", e.Name)
	}
	if e.Version != "2.1" {
		t.Errorf("version = %q，想要 2.1（引号要剥掉）", e.Version)
	}
	if e.Description != "先写测试再写实现，测试要能证明契约" {
		t.Errorf("description = %q", e.Description)
	}
	if e.Compatibility != "cargo >= 1.80" {
		t.Errorf("compatibility = %q——值里有冒号后的空格和 >=，别在第一个冒号之后再切", e.Compatibility)
	}
	if !e.Validation.OK {
		t.Errorf("齐全的 skill 却没通过校验：%s", e.Validation.Reason)
	}
	// ★ INV-SKL-1：扫出来的一律是 draft。扫盘就直接 active 的话，
	// 用户往目录里丢个文件就等于让它进了注入清单——而他可能只是在草稿。
	if e.Status != model.SkillDraft {
		t.Errorf("状态 = %q，扫出来的一律该是 draft（INV-SKL-1）", e.Status)
	}
	if e.Source != ".acpflows/skills" {
		t.Errorf("source = %q——不标来源的话用户不知道 Duet 翻了他哪些目录", e.Source)
	}
}

// R2 · 缺 description 判 draft 并说明原因（INV-SKL-2）。
func TestScan_R2_MissingDescriptionExplained(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "git-worktree-guard", missingDescription)

	got := scanTemp(t, root)
	if len(got) != 1 {
		t.Fatalf("扫到 %d 个", len(got))
	}
	e := got[0]
	if e.Validation.OK {
		t.Fatal("缺 description 却通过了校验")
	}
	if !strings.Contains(e.Validation.Reason, "description") {
		t.Errorf("原因 = %q，应该点名 description——设计稿上就是这么显示的", e.Validation.Reason)
	}
	// 名字还认得出来，用户才知道去改哪一个
	if e.Name != "git-worktree-guard" {
		t.Errorf("name = %q", e.Name)
	}
}

// R3 · 一条坏的**不影响**其余条目。
//
// ★★ 整批失败的后果最狠：一个 frontmatter 写坏的 skill 让整个库列不出来，
// 用户连修它的入口都找不到。
func TestScan_R3_OneBrokenDoesNotHideOthers(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "rust-test-first", rustTestFirst)
	writeSkill(t, root, "broken-yaml", "---\n这行不是 key: value 也不是分隔符\n没有结束分隔符\n")
	writeSkill(t, root, "no-skill-md", "") // 空目录，连 SKILL.md 都没有

	got := scanTemp(t, root)
	if len(got) != 3 {
		t.Fatalf("扫到 %d 个，想要 3 个——坏的也要列出来，否则用户找不到它", len(got))
	}

	byDir := map[string]skill.Entry{}
	for _, e := range got {
		byDir[e.Dir] = e
	}
	if !byDir["rust-test-first"].Validation.OK {
		t.Error("好的那个被坏邻居连累了")
	}
	if byDir["broken-yaml"].Validation.OK {
		t.Error("frontmatter 坏的那个通过了校验")
	}
	// ★ 连 SKILL.md 都没有时，原因要指向缺文件，不是缺字段
	if r := byDir["no-skill-md"].Validation.Reason; !strings.Contains(r, "SKILL.md") {
		t.Errorf("没有 SKILL.md 的原因 = %q，应该指出缺的是文件", r)
	}
}

// R5 · **扫完用户的文件一字未动**（红线 3 / INV-SKL-6）。
//
// ★★ 判据是全目录的内容哈希 + 文件清单，不是「我们没写 open 的 O_WRONLY」。
// 前者管得住「顺手补个默认 frontmatter」这种好意。
func TestScan_R5_DoesNotTouchUserFiles(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "rust-test-first", rustTestFirst)
	writeSkill(t, root, "git-worktree-guard", missingDescription)
	writeSkill(t, root, "no-skill-md", "")

	before := fingerprint(t, root)
	scanTemp(t, root)
	after := fingerprint(t, root)

	if before != after {
		t.Errorf("扫描改动了用户的文件（红线 3）：\n扫描前 %s\n扫描后 %s", before, after)
	}
}

// R4 · 符号链接不跟出项目外。
//
// ★ 跟出去的话，项目里一个指向 `~` 的链接就能让扫描漫游到整个家目录——
// 而用户以为他只是让 Duet 看一眼这个项目。
func TestScan_R4_DoesNotFollowSymlinks(t *testing.T) {
	outside := t.TempDir()
	writeSkill(t, outside, "secret-skill", rustTestFirst)

	root := t.TempDir()
	writeSkill(t, root, "rust-test-first", rustTestFirst)
	if err := os.Symlink(filepath.Join(outside, "secret-skill"), filepath.Join(root, "linked")); err != nil {
		t.Skipf("这个环境建不了符号链接: %v", err)
	}

	got := scanTemp(t, root)
	for _, e := range got {
		if e.Dir == "linked" {
			t.Errorf("跟着符号链接扫到了项目外的 %s——一个指向 ~ 的链接就能让它漫游整个家目录", e.Path)
		}
	}
	if len(got) != 1 {
		t.Errorf("扫到 %d 个，想要 1 个（只有真实目录那个）", len(got))
	}
}

// R6 · 一个都没有时是**空**不是错。
//
// ★ 绝大多数项目没有 `.claude/skills`。把「没有」当错误的话，
// 创建项目的预演会因为一个完全正常的状态而失败。
func TestScan_R6_MissingDirIsEmptyNotError(t *testing.T) {
	got, err := skill.Scan(skill.Options{
		Root:  filepath.Join(t.TempDir(), "根本不存在的目录"),
		Scope: model.SkillScopeProject,
	})
	if err != nil {
		t.Errorf("目录不存在报了错：%v——绝大多数项目就是没有这个目录", err)
	}
	if len(got) != 0 {
		t.Errorf("扫到 %d 个", len(got))
	}
}

func TestScan_R6b_EmptyDirIsEmpty(t *testing.T) {
	got := scanTemp(t, t.TempDir())
	if len(got) != 0 {
		t.Errorf("空目录扫到 %d 个", len(got))
	}
}

// 目录里的散装文件不算 skill——skill 是目录，不是文件。
func TestScan_IgnoresLooseFiles(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "rust-test-first", rustTestFirst)
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# 说明"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := scanTemp(t, root)
	if len(got) != 1 {
		t.Fatalf("扫到 %d 个，想要 1 个", len(got))
	}
}

// ★ Windows 换行的 SKILL.md 也要读得出来。
//
// 读不出来的症状是「缺 name、description」——把人引向一个根本没写错的文件，
// 而真正的原因是 `---\r` 匹配不上分隔符。
func TestScan_HandlesCRLF(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "crlf-skill", strings.ReplaceAll(rustTestFirst, "\n", "\r\n"))

	got := scanTemp(t, root)
	if len(got) != 1 {
		t.Fatalf("扫到 %d 个", len(got))
	}
	if !got[0].Validation.OK {
		t.Errorf("CRLF 换行的文件没通过校验：%s", got[0].Validation.Reason)
	}
	if got[0].Name != "rust-test-first" {
		t.Errorf("name = %q", got[0].Name)
	}
}

// 结果按目录名排序，不受文件系统返回顺序影响。
func TestScan_SortedByDir(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"zebra", "alpha", "middle"} {
		writeSkill(t, root, d, rustTestFirst)
	}
	got := scanTemp(t, root)
	dirs := make([]string, len(got))
	for i, e := range got {
		dirs[i] = e.Dir
	}
	if !sort.StringsAreSorted(dirs) {
		t.Errorf("顺序 = %v，应该按目录名排序", dirs)
	}
}

// fingerprint 算整棵目录树的指纹：路径 + 内容哈希。
func fingerprint(t *testing.T, root string) string {
	t.Helper()
	h := sha256.New()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		h.Write([]byte(rel + "\n"))
		if d.IsDir() {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		h.Write(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(h.Sum(nil))
}
