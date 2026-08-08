// Package skill 扫描磁盘上的 Skill 目录，解析出 Duet 认识的条目。
//
// ★★ **这个包只读，一个字节都不写。**
//
// 用户的 skill 是他自己的产物（红线 3：不改用户项目的 skill）。我们扫它、
// 认它、在界面上把它列出来——但绝不改名、绝不补默认值、绝不「顺手修一下
// frontmatter」。有测试对着扫描前后的内容哈希守这条。
package skill

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// ErrScanRoot 表示扫描根目录本身有问题（不存在的目录不算问题，见 Scan 说明）。
var ErrScanRoot = errors.New("fsstore/skill: 扫描根目录不可用")

// Entry 是扫到的一个 Skill。
type Entry struct {
	// Name 取自 frontmatter 的 name；缺失时退回目录名，
	// 这样界面上至少认得出是哪个目录出的问题。
	Name string
	// Dir 是这个 Skill 的目录名。
	Dir string
	// Path 是它的绝对路径。
	Path string
	// Version 是 frontmatter 里的版本号原文（可能为空或非法，原样保留）。
	Version string
	// Description 取自 frontmatter。
	Description string
	// Compatibility 例 `cargo >= 1.80`。
	Compatibility string
	// Scope 是项目级还是全局。
	Scope model.SkillScope
	// Source 是这个 Skill 来自哪个目录约定（`.acpflows/skills` / `.claude/skills`）。
	//
	// ★ 创建项目时要告诉用户「发现已有 Skill 目录 · N」并列出来源——
	// 不标来源的话，他不知道 Duet 翻了他哪些目录。
	Source string
	// Status 是校验后的状态：通过为 draft（等发布），不通过也是 draft。
	//
	// ★ INV-SKL-1：**扫出来的一律是 draft**，校验通过只是「可以发布」，
	// 不是「已经发布」。扫盘就直接 active 的话，用户往目录里丢一个文件
	// 就等于让它进了注入清单——而他可能只是在草稿。
	Status model.SkillStatus
	// Validation 是校验结论，不通过时 Reason 直接可显示。
	Validation model.SkillValidation
}

// Options 是一次扫描的参数。
type Options struct {
	// Root 是要扫的目录（`~/.acpflows/skills` 或 `<项目>/.claude/skills`）。
	Root string
	// Scope 标明扫的是项目级还是全局。
	Scope model.SkillScope
	// Source 是来源标记，会原样带进每个 Entry。
	Source string
}

// Scan 扫一个 skills 目录。
//
// ★ **目录不存在不是错误**，返回空列表。绝大多数项目没有 `.claude/skills`，
// 把「没有」当成错误的话，创建项目的预演会因为一个正常状态而失败。
//
// ★ 单个条目坏掉**不影响其余条目**：一个 frontmatter 写坏的 skill
// 不该让整个库都列不出来——那会让用户完全失去修复它的入口。
func Scan(opts Options) ([]Entry, error) {
	if opts.Root == "" {
		return nil, fmt.Errorf("%w: 根目录为空", ErrScanRoot)
	}

	dirents, err := os.ReadDir(opts.Root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil // 没有这个目录 = 一个 skill 都没有，不是错误
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrScanRoot, opts.Root, err)
	}

	var out []Entry
	for _, de := range dirents {
		entry, ok := scanOne(opts, de)
		if !ok {
			continue
		}
		out = append(out, entry)
	}

	// 按目录名排序：`os.ReadDir` 已经排过，但显式排一次让顺序与文件系统无关。
	sort.Slice(out, func(i, j int) bool { return out[i].Dir < out[j].Dir })
	return out, nil
}

func scanOne(opts Options, de os.DirEntry) (Entry, bool) {
	// ★★ **符号链接一律不跟**，这里有**两道**，各自都独立挡得住：
	//
	//   ① 显式判 ModeSymlink
	//   ② `os.ReadDir` 的 DirEntry 是 Lstat 语义——指向目录的链接
	//      `IsDir()` 返回 false
	//
	// 造负例验过：单独拆掉任何一道，另一道仍然挡住。留着 ① 是因为它写的是
	// **意图**，而 ② 只是 Lstat 语义的副作用——哪天有人把 ② 换成
	// `os.Stat`（跟随链接）时，① 是唯一还站着的那道。
	//
	// 跟出去的后果：项目里一个指向 `~` 的链接就能让扫描漫游整个家目录，
	// 而用户以为他只是让 Duet 看一眼这个项目。
	if de.Type()&os.ModeSymlink != 0 {
		return Entry{}, false
	}
	if !de.IsDir() {
		return Entry{}, false
	}

	dir := de.Name()
	path := filepath.Join(opts.Root, dir)
	entry := Entry{
		Name:   dir, // 先用目录名兜底，读到 frontmatter 的 name 再覆盖
		Dir:    dir,
		Path:   path,
		Scope:  opts.Scope,
		Source: opts.Source,
		Status: model.SkillDraft,
	}

	raw, err := os.ReadFile(filepath.Join(path, "SKILL.md"))
	hasSkillMD := err == nil

	var fm model.SkillFrontmatter
	if hasSkillMD {
		fm = parseFrontmatter(string(raw))
		if fm.Name != "" {
			entry.Name = fm.Name
		}
		entry.Version = fm.Version
		entry.Description = fm.Description
		entry.Compatibility = fm.Compatibility
	}

	entry.Validation = model.ValidateSkill(hasSkillMD, fm)
	return entry, true
}

// parseFrontmatter 解出 SKILL.md 头部的 YAML frontmatter。
//
// ★ **只认我们要的四个键，不引 YAML 库。**
//
// frontmatter 是 `key: value` 的平铺结构，用一个 YAML 库来读它，换来的是
// 一个能执行任意锚点展开的解析器去读用户提供的文件。四个字符串键不值这个代价。
//
// ★ 解不出来**不报错**，返回空值——上层的 ValidateSkill 会把「缺什么」
// 说清楚。在这里报错的话，错误信息只能是「YAML 解析失败」，
// 而用户想知道的是「缺 description」。
func parseFrontmatter(content string) model.SkillFrontmatter {
	lines, ok := frontmatterLines(content)
	if !ok {
		return model.SkillFrontmatter{}
	}

	var fm model.SkillFrontmatter
	for _, line := range lines {
		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		switch strings.TrimSpace(key) {
		case "name":
			fm.Name = value
		case "description":
			fm.Description = value
		case "version":
			fm.Version = value
		case "compatibility":
			fm.Compatibility = value
		}
	}
	return fm
}

// frontmatterLines 取出 `---` 与 `---` 之间的行。
func frontmatterLines(content string) ([]string, bool) {
	// ★ 按 \n 切，靠 **TrimSpace** 吃掉 Windows 换行残留的 `\r`。
	//
	// 原本这里还有一行 `TrimRight(line, "\r")`——造负例时发现**拆了它测试照样绿**，
	// 因为下面每处比较都走了 TrimSpace。测不到的代码要么删掉要么说清它守什么，
	// 这一行属于前者。
	//
	// 读不到 frontmatter 的症状是「缺 name、description」，
	// 会把人引向一个根本没写错的文件——所以 CRLF 那条测试要留着。
	all := strings.Split(content, "\n")
	if len(all) == 0 || strings.TrimSpace(all[0]) != "---" {
		return nil, false
	}
	for i := 1; i < len(all); i++ {
		if strings.TrimSpace(all[i]) == "---" {
			return all[1:i], true
		}
	}
	return nil, false // 只有开头没有结尾：不是合法的 frontmatter
}
