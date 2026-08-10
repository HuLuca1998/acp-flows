package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReadGlobalBody 读一个全局 Skill 的 SKILL.md，frontmatter 与正文分开给。
//
// ★ **从磁盘现读，不留副本**——Skill 是用户的产物，他随时可能用编辑器
// 改它。列表接口给的是扫描时刻的样子，这里给的是「现在盘上的样子」。
//
// ★ frontmatter 原样返回（不含 `---` 围栏）：解析失败的文件，
// 用户要看得到自己到底写了什么才改得动。
func (s Store) ReadGlobalBody(dir string) (frontmatter, text string, err error) {
	// ★★ 目录名不许逃出 skills 根：一个带 `..` 的 dir 就能读到
	// 家目录里任何文件。列表接口给的 dir 永远是单段目录名，
	// 带分隔符的只能是构造出来的请求。
	if dir == "" || dir == "." || dir == ".." ||
		strings.ContainsAny(dir, `/\`) {
		return "", "", fmt.Errorf("fsstore/skill: 非法的目录名 %q", dir)
	}

	path := filepath.Join(s.Home, "skills", dir, "SKILL.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		// ★ 带路径明说——「空正文」与「文件丢了」是两回事，
		// 后者用户该去看磁盘上出了什么事。
		return "", "", fmt.Errorf("fsstore/skill: 读不到 %s: %w", path, err)
	}
	frontmatter, text = splitFrontmatter(string(raw))
	return frontmatter, text, nil
}

// splitFrontmatter 把文件切成 frontmatter 原文与正文。
// 没有合法围栏时 frontmatter 为空串、正文是整个文件——
// 不能因为头上没有围栏就把人家的正文丢一截。
func splitFrontmatter(content string) (frontmatter, text string) {
	all := strings.Split(content, "\n")
	if len(all) == 0 || strings.TrimSpace(all[0]) != "---" {
		return "", content
	}
	for i := 1; i < len(all); i++ {
		if strings.TrimSpace(all[i]) == "---" {
			return strings.Join(all[1:i], "\n"), strings.Join(all[i+1:], "\n")
		}
	}
	return "", content // 只有开头没有结尾：不是合法的 frontmatter
}
