// Package memory 读写记忆的**正文**——一条记忆的 md 文件。
//
// ★★ **INV-MEM-8：正文只在 md 文件里，不进数据库。**
//
// 用户要能用任何编辑器打开它、改它、用 git 管它。塞进数据库的话，
// 那条记忆就只能通过 Duet 的界面看——而记忆是他自己的资产，
// 不该被一个应用扣住。数据库只存索引（id / 类型 / 状态 / 时间）。
package memory

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	// ErrBodyMissing 表示索引里有这条记忆，但 md 文件不在。
	//
	// ★ **不当成空正文**：空正文看起来像「这条记忆没内容」，
	// 而真相是「文件丢了」——两件事的处理方式完全不同。
	ErrBodyMissing = errors.New("fsstore/memory: 记忆正文文件不存在")

	// ErrBadID 表示记忆 id 不能安全地当文件名用。
	//
	// ★★ 这是**唯一**挡住「写到记忆库外面去」的那道检查：带 `/` `\` `..`
	// 的 id 一律拒绝。记忆库之外的东西是用户的其它文件——写坏了他找不回来。
	ErrBadID = errors.New("fsstore/memory: 记忆 id 不能当作文件名")
)

// Store 是一个记忆库目录下的正文读写。
type Store struct {
	root string
}

// NewStore 建一个正文读写器。root 是记忆库目录（`~/.acpflows/memories`）。
func NewStore(root string) *Store { return &Store{root: root} }

// Body 是一条记忆的正文。
type Body struct {
	// Title 取自 frontmatter 的 title；缺了就是空串。
	Title string
	// Text 是 frontmatter 之后的正文。
	//
	// ★ **解析失败时这里是整个文件的原文**：结构化字段拿不到没关系，
	// 但用户写的字一个都不能丢——那是他自己敲进去的东西。
	Text string
	// Raw 是文件的完整原文，含 frontmatter。
	Raw string
	// Path 是这条记忆的 md 文件路径。
	Path string
	// Malformed 标明 frontmatter 没解析成功（此时 Title 为空、Text 是原文）。
	Malformed bool
}

// Read 读一条记忆的正文。
//
// ★★ **每次都从磁盘读**，不缓存：用户可能刚用编辑器改过它，
// 而我们给他看的必须是他刚写下的那一版。
func (s *Store) Read(id string) (Body, error) {
	path, err := s.pathFor(id)
	if err != nil {
		return Body{}, err
	}

	raw, err := os.ReadFile(path) //nolint:gosec // 路径由 pathFor 限定在 root 内
	if err != nil {
		if os.IsNotExist(err) {
			// ★ 带上路径：用户要能自己去那个位置看一眼文件到底在不在
			return Body{}, fmt.Errorf("%w: %s", ErrBodyMissing, path)
		}
		return Body{}, fmt.Errorf("fsstore/memory: 读 %s: %w", path, err)
	}

	body := parse(string(raw))
	body.Path = path
	return body, nil
}

// Write 写一条记忆的正文。
//
// ★ **只写记忆库目录**。id 带上 `../` 时直接拒绝，不去猜他想干什么。
func (s *Store) Write(id string, b Body) error {
	path, err := s.pathFor(id)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("fsstore/memory: 建目录: %w", err)
	}

	content := b.Raw
	if content == "" {
		content = render(b)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return fmt.Errorf("fsstore/memory: 写 %s: %w", path, err)
	}
	return nil
}

// pathFor 把 id 变成 md 路径，并挡住跑出 root 的写法。
func (s *Store) pathFor(id string) (string, error) {
	if id == "" || strings.ContainsAny(id, `/\`) || strings.Contains(id, "..") {
		return "", fmt.Errorf("%w: %q", ErrBadID, id)
	}
	// ★ 这里**只有上面那一道检查**在挡越界。
	//
	// 原来还跟着一段 `filepath.Rel` 的「双保险」，但它永远进不去：能走到
	// 这一行的 id 已经不含 `/` `\` `..`，Join 的结果必然落在 root 下。
	// 留着的话，下一个人会以为有两层防御——而实际只有一层，
	// 他可能因此放心地把上面那道放宽。删掉比留着安全。
	return filepath.Join(s.root, id+".md"), nil
}

// parse 拆 frontmatter 与正文。
//
// ★★ **解析失败不丢内容**：Malformed 置位，Text 给整个原文。
// 一条 frontmatter 少了个引号就把用户写的三百字吞掉，那是最糟的处理方式。
func parse(raw string) Body {
	body := Body{Raw: raw, Text: raw}

	const fence = "---\n"
	if !strings.HasPrefix(raw, fence) {
		body.Malformed = true
		return body
	}
	rest := raw[len(fence):]
	end := strings.Index(rest, "\n"+fence)
	if end < 0 {
		// 起了个头没收尾——同样保留原文
		body.Malformed = true
		return body
	}

	for _, line := range strings.Split(rest[:end], "\n") {
		key, value, ok := strings.Cut(line, ":")
		if ok && strings.TrimSpace(key) == "title" {
			body.Title = strings.TrimSpace(value)
		}
	}
	body.Text = strings.TrimPrefix(rest[end+len("\n")+len(fence):], "\n")
	return body
}

// render 从字段拼一份 md（只在 Raw 为空时用）。
func render(b Body) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString("title: " + b.Title + "\n")
	sb.WriteString("---\n\n")
	sb.WriteString(b.Text)
	if !strings.HasSuffix(b.Text, "\n") {
		sb.WriteString("\n")
	}
	return sb.String()
}
