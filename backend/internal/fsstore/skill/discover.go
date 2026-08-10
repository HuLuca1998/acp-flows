package skill

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// skipDirs 是扫描时整棵跳过的目录。
//
// ★ 设计稿原文：「扫描 `**/skills`（跳过 node_modules 与 target）」。
// 不跳的话，一个装了依赖的前端项目要走十几万个目录——
// 而创建项目的弹层是**用户点了在等**的界面。
//
// `.git` 与 `.acpflows/runs` 也跳：前者是仓库内部结构，
// 后者是我们自己的执行记录，都不可能有用户的 skill。
var skipDirs = map[string]bool{
	"node_modules": true,
	"target":       true,
	".git":         true,
	"dist":         true,
	"build":        true,
	"vendor":       true,
	".venv":        true,
	"__pycache__":  true,
}

// maxDiscoverDepth 是相对项目根的最大下探层数。
//
// ★ 有上限而不是无限深：真实项目里的 skill 目录都在浅层
// （`.claude/skills`、`tools/agent/skills`），而深层遍历的代价
// 全部由等在弹层前面的用户承担。
//
// 探不到的话用户还能手动指定——比让他盯着一个转圈的对话框强。
const maxDiscoverDepth = 5

// Discover 在项目里找出所有像 skill 库的目录。
//
// ★★ **只读**，且**不跟符号链接**（同 Scan）：项目里一个指向 `~` 的链接
// 就能让扫描漫游整个家目录，而用户以为他只是让 Duet 看一眼这个项目。
//
// 找到的每个 `skills/` 目录都用 `Scan` 解析——**复用同一套解析与校验**。
// 另起一套的话，同一个坏 frontmatter 在两条路径下会给出不同的原因，
// 而用户不知道该信哪个。
func Discover(root string) ([]Entry, error) {
	if root == "" {
		return nil, nil
	}
	dirs, err := findSkillDirs(root)
	if err != nil {
		return nil, err
	}

	var out []Entry
	for _, dir := range dirs {
		rel, relErr := filepath.Rel(root, dir)
		if relErr != nil {
			rel = dir
		}
		entries, scanErr := Scan(Options{
			Root:  dir,
			Scope: model.SkillScopeProject,
			// ★ 来源是**项目内的相对路径**：用户要能照着去找。
			// 报绝对路径的话，界面上一长串前缀全是噪声。
			Source: rel,
		})
		if scanErr != nil {
			// ★ 一个目录读不了**不影响其余**：整批失败的话，
			// 一个权限不对的子目录会让用户以为项目里一个 skill 都没有。
			continue
		}
		out = append(out, entries...)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		return out[i].Dir < out[j].Dir
	})
	return out, nil
}

// findSkillDirs 找出项目里所有名叫 `skills` 的目录。
func findSkillDirs(root string) ([]string, error) {
	var found []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// ★ 某个子目录读不了就跳过它，别中断整次扫描——
			// 用户的项目里有个权限不对的目录是很常见的事。
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if path == root {
			return nil
		}

		name := d.Name()
		if skipDirs[name] {
			return filepath.SkipDir
		}
		// ★ 符号链接指向的目录，`WalkDir` 的 DirEntry 是 Lstat 语义，
		// `IsDir()` 为 false，所以走不进去——这里再显式挡一道，
		// 理由同 `scanOne`：② 是语义副作用，① 写的才是意图。
		if d.Type()&os.ModeSymlink != 0 {
			return filepath.SkipDir
		}

		if depth(root, path) > maxDiscoverDepth {
			return filepath.SkipDir
		}
		if name == "skills" {
			found = append(found, path)
			// ★ 找到就不再往下：`skills/<名字>/` 里面不会再嵌一层 skill 库，
			// 继续下探只会把每个 skill 的子目录也当成候选。
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return found, nil
}

// depth 返回 path 相对 root 的层数（root 本身是 0）。
func depth(root, path string) int {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return maxDiscoverDepth + 1
	}
	if rel == "." {
		return 0
	}
	return strings.Count(rel, string(filepath.Separator)) + 1
}
