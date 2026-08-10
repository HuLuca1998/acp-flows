package skill_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/fsstore/skill"
)

// 契约来源：M10 U10.7.2（Skill 页详情栏）。
//
// ★ 正文**从磁盘现读**，不留副本——Skill 是用户的产物，他随时可能用
// 编辑器改它。所以这里的契约是「读到的就是盘上的」，而不是「读到的
// 是某次扫描时的快照」。

const bodySkill = `---
name: rust-test-first
version: "2.1"
---

先写失败测试，再实现。

## 步骤

1. 按验收标准逐条写失败测试
`

// writeSkillFile 在 home 下造一个真的全局 Skill。
func writeSkillFile(t *testing.T, home, dir, content string) string {
	t.Helper()
	path := filepath.Join(home, "skills", dir)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(path, "SKILL.md")
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return file
}

// R2：读回来的就是盘上那份——frontmatter 是围栏内的原文，正文是围栏之后的全部。
func TestStore_ReadGlobalBody_R2_MatchesDiskFile(t *testing.T) {
	home := t.TempDir()
	writeSkillFile(t, home, "rust-test-first", bodySkill)

	s := skill.Store{Home: home}
	fm, text, err := s.ReadGlobalBody("rust-test-first")
	if err != nil {
		t.Fatalf("读正文: %v", err)
	}

	wantFM := "name: rust-test-first\nversion: \"2.1\""
	if fm != wantFM {
		t.Fatalf("frontmatter 与盘上不一致：\n得到 %q\n想要 %q", fm, wantFM)
	}
	wantText := "\n先写失败测试，再实现。\n\n## 步骤\n\n1. 按验收标准逐条写失败测试\n"
	if text != wantText {
		t.Fatalf("正文与盘上不一致：\n得到 %q\n想要 %q", text, wantText)
	}
}

// 没有 frontmatter 的文件：frontmatter 为空串，正文是整个文件——
// 不能因为头上没有围栏就把人家的正文丢一截。
func TestStore_ReadGlobalBody_NoFrontmatterKeepsWholeText(t *testing.T) {
	home := t.TempDir()
	raw := "# 没头的文件\n\n照样是合法的 SKILL.md 正文。\n"
	writeSkillFile(t, home, "headless", raw)

	s := skill.Store{Home: home}
	fm, text, err := s.ReadGlobalBody("headless")
	if err != nil {
		t.Fatalf("读正文: %v", err)
	}
	if fm != "" {
		t.Fatalf("没有 frontmatter 时应为空串，得到 %q", fm)
	}
	if text != raw {
		t.Fatalf("正文应是整个文件：\n得到 %q\n想要 %q", text, raw)
	}
}

// R4：文件不在要**带路径**说清楚——「空正文」和「文件丢了」是两回事，
// 前者用户会当成没内容，后者他该去看磁盘。
func TestStore_ReadGlobalBody_R4_MissingSaysPath(t *testing.T) {
	home := t.TempDir()

	s := skill.Store{Home: home}
	_, _, err := s.ReadGlobalBody("no-such-skill")
	if err == nil {
		t.Fatal("不存在的 Skill 应报错，得到 nil")
	}
	wantPath := filepath.Join(home, "skills", "no-such-skill", "SKILL.md")
	if !strings.Contains(err.Error(), wantPath) {
		t.Fatalf("错误里应带路径 %q，得到 %q", wantPath, err.Error())
	}
}

// 目录名不许逃出 skills 根——一个带 `..` 的 dir 就能读到家目录里任何文件。
func TestStore_ReadGlobalBody_RejectsPathEscape(t *testing.T) {
	home := t.TempDir()
	// 在 skills 根**之外**放一个诱饵：逃逸成功的话会读到它。
	if err := os.MkdirAll(filepath.Join(home, "evil"), 0o755); err != nil {
		t.Fatal(err)
	}
	bait := filepath.Join(home, "evil", "SKILL.md")
	if err := os.WriteFile(bait, []byte("逃逸成功"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := skill.Store{Home: home}
	for _, dir := range []string{"../evil", "a/b", `a\b`, "..", ""} {
		fm, text, err := s.ReadGlobalBody(dir)
		if err == nil {
			t.Fatalf("dir=%q 应被拒绝，得到 fm=%q text=%q", dir, fm, text)
		}
		if strings.Contains(text, "逃逸成功") {
			t.Fatalf("dir=%q 读到了 skills 根之外的文件", dir)
		}
	}
}
