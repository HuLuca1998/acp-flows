package skill_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/fsstore/skill"
)

// M3 U3.1.2 · 扫项目里已有的 Skill
//
// ★ 设计稿原文：「扫描 `**/skills`（跳过 node_modules 与 target）」——
// 不是只看两个固定目录。真实项目里它可能在 `.claude/skills`、
// `tools/agent/skills`、`.acpflows/skills` 任何一处。

// project 造一棵真的项目目录树。
func project(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func discover(t *testing.T, root string) []skill.Entry {
	t.Helper()
	got, err := skill.Discover(root)
	if err != nil {
		t.Fatalf("扫描 %s: %v", root, err)
	}
	return got
}

// ★★ R1 · 三个来源都扫得到，且**标出来源**。
//
// 不标来源的话，用户不知道 Duet 翻了他哪些目录——
// 而他交出来的是自己的仓库。
func TestDiscover_R1_FindsEverySkillsDirAndTagsSource(t *testing.T) {
	root := project(t, map[string]string{
		".claude/skills/rust-test-first/SKILL.md": rustTestFirst,
		".acpflows/skills/duet-own/SKILL.md":      rustTestFirst,
		"tools/agent/skills/deep-one/SKILL.md":    rustTestFirst,
		"README.md":                               "# 项目",
	})

	got := discover(t, root)
	if len(got) != 3 {
		t.Fatalf("扫到 %d 条，想要 3 条：%+v", len(got), sources(got))
	}

	bySource := map[string]bool{}
	for _, e := range got {
		bySource[e.Source] = true
		if e.Source == "" {
			t.Errorf("%s 没标来源——用户不知道 Duet 翻了他哪些目录", e.Dir)
		}
		if filepath.IsAbs(e.Source) {
			t.Errorf("来源报了绝对路径 %q——界面上一长串前缀全是噪声", e.Source)
		}
	}
	for _, want := range []string{
		filepath.Join(".claude", "skills"),
		filepath.Join(".acpflows", "skills"),
		filepath.Join("tools", "agent", "skills"),
	} {
		if !bySource[want] {
			t.Errorf("没扫到来源 %q，实际扫到 %v", want, sources(got))
		}
	}
}

// ★★ R2 · 校验复用 `U2.2.1` 那一套——同一个坏 frontmatter，
// 两条路径给出**同样的原因文本**。
//
// 另起一套的话，用户会看到两种说法而不知道该信哪个。
func TestDiscover_R2_ReusesTheSameValidator(t *testing.T) {
	root := project(t, map[string]string{
		".claude/skills/broken/SKILL.md": missingDescription,
	})

	viaDiscover := discover(t, root)
	if len(viaDiscover) != 1 {
		t.Fatalf("扫到 %d 条", len(viaDiscover))
	}

	viaScan, err := skill.Scan(skill.Options{Root: filepath.Join(root, ".claude", "skills")})
	if err != nil {
		t.Fatal(err)
	}
	if len(viaScan) != 1 {
		t.Fatalf("直接 Scan 得到 %d 条", len(viaScan))
	}

	if viaDiscover[0].Validation.Reason != viaScan[0].Validation.Reason {
		t.Errorf("两条路径的原因不一样：\nDiscover: %q\nScan:     %q\n"+
			"——用户会看到两种说法而不知道该信哪个",
			viaDiscover[0].Validation.Reason, viaScan[0].Validation.Reason)
	}
	if viaDiscover[0].Validation.OK {
		t.Error("缺 description 却通过了校验")
	}
}

// ★★ R3 · **只读**：扫完用户的文件一字未动。
func TestDiscover_R3_DoesNotTouchUserFiles(t *testing.T) {
	root := project(t, map[string]string{
		".claude/skills/a/SKILL.md":       rustTestFirst,
		"tools/agent/skills/b/SKILL.md":   missingDescription,
		"node_modules/pkg/skills/c/x.txt": "别碰我",
	})

	before := fingerprint(t, root)
	discover(t, root)
	if after := fingerprint(t, root); before != after {
		t.Errorf("扫描改动了用户的文件（红线 3）：\n之前 %s\n之后 %s", before, after)
	}
}

// ★★ R4 · 符号链接不跟出项目外。
//
// 项目里一个指向 `~` 的链接就能让扫描漫游整个家目录，
// 而用户以为他只是让 Duet 看一眼这个项目。
func TestDiscover_R4_DoesNotFollowSymlinksOutOfProject(t *testing.T) {
	outside := project(t, map[string]string{"skills/secret/SKILL.md": rustTestFirst})

	root := project(t, map[string]string{".claude/skills/mine/SKILL.md": rustTestFirst})
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Skipf("这个环境建不了符号链接: %v", err)
	}

	got := discover(t, root)
	for _, e := range got {
		if strings.Contains(e.Path, outside) {
			t.Errorf("跟着符号链接扫到了项目外的 %s——"+
				"一个指向 ~ 的链接就能让它漫游整个家目录", e.Path)
		}
	}
	if len(got) != 1 {
		t.Errorf("扫到 %d 条，想要 1 条（只有项目内那个）：%v", len(got), sources(got))
	}
}

// ★ 跳过 node_modules 与 target。
//
// 不跳的话，一个装了依赖的前端项目要走十几万个目录——
// 而创建项目的弹层是**用户点了在等**的界面。
func TestDiscover_SkipsHeavyDirs(t *testing.T) {
	root := project(t, map[string]string{
		".claude/skills/mine/SKILL.md":            rustTestFirst,
		"node_modules/some-pkg/skills/x/SKILL.md": rustTestFirst,
		"target/debug/skills/y/SKILL.md":          rustTestFirst,
		"frontend/dist/skills/z/SKILL.md":         rustTestFirst,
		".git/hooks/skills/w/SKILL.md":            rustTestFirst,
	})

	got := discover(t, root)
	for _, e := range got {
		for _, heavy := range []string{"node_modules", "target", "dist", ".git"} {
			if strings.Contains(e.Source, heavy) {
				t.Errorf("扫进了 %s：%s——那里面的不是用户的 skill，"+
					"而遍历它的代价由等在弹层前面的用户承担", heavy, e.Source)
			}
		}
	}
	if len(got) != 1 {
		t.Errorf("扫到 %d 条，想要 1 条：%v", len(got), sources(got))
	}
}

// ★ 找到 `skills/` 就不再往下。
//
// 继续下探的话，每个 skill 自己的子目录（`scripts/` `references/`）
// 都会被当成候选，同一条 skill 被重复收录。
func TestDiscover_DoesNotDescendIntoSkillDirs(t *testing.T) {
	root := project(t, map[string]string{
		".claude/skills/deep/SKILL.md":               rustTestFirst,
		".claude/skills/deep/skills/nested/SKILL.md": rustTestFirst,
	})

	got := discover(t, root)
	if len(got) != 1 {
		t.Errorf("扫到 %d 条，想要 1 条——找到 skills/ 就该停，"+
			"否则每个 skill 的子目录都会被当成候选：%v", len(got), sources(got))
	}
}

// ★ R5 · 一个都没有时是**空**不是错。
//
// 绝大多数项目就是没有 skill 目录。当成错误的话，
// 创建项目的预演会因为一个完全正常的状态而失败。
func TestDiscover_R5_NoSkillsIsEmptyNotError(t *testing.T) {
	root := project(t, map[string]string{"README.md": "# 空项目"})

	got, err := skill.Discover(root)
	if err != nil {
		t.Errorf("没有 skill 却报了错：%v", err)
	}
	if len(got) != 0 {
		t.Errorf("扫到 %d 条", len(got))
	}
}

func TestDiscover_EmptyRootIsEmpty(t *testing.T) {
	got, err := skill.Discover("")
	if err != nil {
		t.Errorf("空路径报了错：%v", err)
	}
	if len(got) != 0 {
		t.Errorf("空路径扫到 %d 条", len(got))
	}
}

// 结果按来源与目录名排序，不受文件系统返回顺序影响。
func TestDiscover_SortedBySourceThenDir(t *testing.T) {
	root := project(t, map[string]string{
		"zeta/skills/b/SKILL.md":  rustTestFirst,
		"zeta/skills/a/SKILL.md":  rustTestFirst,
		"alpha/skills/c/SKILL.md": rustTestFirst,
	})

	got := discover(t, root)
	if len(got) != 3 {
		t.Fatalf("扫到 %d 条", len(got))
	}
	keys := make([]string, len(got))
	for i, e := range got {
		keys[i] = e.Source + "/" + e.Dir
	}
	for i := 1; i < len(keys); i++ {
		if keys[i-1] > keys[i] {
			t.Errorf("顺序不对：%v", keys)
			break
		}
	}
}

func sources(entries []skill.Entry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Source + "/" + e.Dir
	}
	return out
}
