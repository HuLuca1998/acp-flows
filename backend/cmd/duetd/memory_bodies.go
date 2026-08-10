package main

import (
	"path/filepath"

	fsmemory "github.com/HuLuca1998/acp-flows/backend/internal/fsstore/memory"
)

// memoryBodies 把记忆正文写到 `<data>/memories/<id>.md`。
//
// ★★ INV-MEM-8：**正文只在 md 文件里**。用户要能用任何编辑器打开它、
// 改它、用 git 管它——塞进数据库的话，那条记忆就只能通过 Duet 的界面看。
type memoryBodies struct {
	store *fsmemory.Store
}

func newMemoryBodies(dataDir string) memoryBodies {
	return memoryBodies{store: fsmemory.NewStore(filepath.Join(dataDir, "memories"))}
}

func (b memoryBodies) WriteBody(id, title, text string) error {
	return b.store.Write(id, fsmemory.Body{Title: title, Text: text})
}

// TitleOf 读一条记忆的标题；读不到就返回空串。
//
// ★ 读不到**不报错**：调用方是去重，而「读不到标题」只意味着
// 「这条不是重复的」——为此打断一次正常的记忆收集不值得。
func (b memoryBodies) TitleOf(id string) string {
	body, err := b.store.Read(id)
	if err != nil {
		return ""
	}
	return body.Title
}

// ReadBody 读一条记忆的正文，供 API 用。
//
// ★★ 读不到时**报错**，不返回空正文：空正文看起来像「这条记忆没内容」，
// 用户会照着这个印象直接把它收下——而真相是文件丢了。
func (b memoryBodies) ReadBody(id string) (string, string, error) {
	body, err := b.store.Read(id)
	if err != nil {
		return "", "", err
	}
	return body.Title, body.Text, nil
}
