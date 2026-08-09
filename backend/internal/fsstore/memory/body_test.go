package memory_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HuLuca1998/acp-flows/backend/internal/fsstore/memory"
)

// M10 U10.1.1 · 记忆正文的 md 读写
//
// ★★ INV-MEM-8：正文只在 md 文件里。用户要能用任何编辑器改它——
// 塞进数据库的话，那条记忆就只能通过 Duet 的界面看，
// 而记忆是他自己的资产。

const realNote = `---
title: 这个仓库的迁移必须手写 SQL
---

用 gorm AutoMigrate 会把 events 表的 role 列悄悄改成 NOT NULL，
而旧数据里那一列是空的——启动直接失败。
`

// R2 ★★ 用户手工改过 md 之后，读出来是**新内容**。
//
// 缓存的话，他改完打开 Duet 看到的还是旧的，会以为自己改错了地方。
func TestStore_Read_ReflectsUserEdits(t *testing.T) {
	root := t.TempDir()
	s := memory.NewStore(root)
	require.NoError(t, s.Write("mem-0042", memory.Body{Raw: realNote}))

	first, err := s.Read("mem-0042")
	require.NoError(t, err)
	require.Contains(t, first.Text, "gorm AutoMigrate")

	// 用户拿编辑器改了它
	edited := realNote + "\n补充：0008 那次就是这么炸的。\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "mem-0042.md"), []byte(edited), 0o600))

	again, err := s.Read("mem-0042")
	require.NoError(t, err)
	assert.Contains(t, again.Text, "0008 那次就是这么炸的", "读的是缓存，不是他刚改的那一版")
}

// R3 ★ md 文件不见了时**说清楚**，不当成空正文。
//
// 空正文看起来像「这条记忆没内容」，而真相是「文件丢了」。
func TestStore_Read_MissingFileIsAnErrorWithPath(t *testing.T) {
	s := memory.NewStore(t.TempDir())

	_, err := s.Read("mem-0042")

	require.ErrorIs(t, err, memory.ErrBodyMissing)
	assert.Contains(t, err.Error(), "mem-0042.md", "报错里没有路径，用户不知道该去哪找那个文件")
}

// R4 ★★ 写正文**只写记忆库目录**，不碰别的地方。
func TestStore_Write_RefusesToEscapeRoot(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(root, "..", "stolen.md")
	s := memory.NewStore(root)

	err := s.Write("../stolen", memory.Body{Text: "x"})

	// ★ 挡住它的是 ErrBadID 那道字符检查——**只有这一道**。
	// 断到具体的错误上，防的是有人把它放宽之后测试还绿着。
	require.ErrorIs(t, err, memory.ErrBadID)
	_, statErr := os.Stat(outside)
	assert.True(t, os.IsNotExist(statErr), "记忆库目录外面被写出了文件")
}

// R5 ★★ frontmatter 解析失败时**保留原文**。
//
// 一条 frontmatter 少了个引号就把用户写的三百字吞掉，是最糟的处理方式。
func TestStore_Read_MalformedFrontmatterKeepsText(t *testing.T) {
	root := t.TempDir()
	broken := "---\ntitle: 没有收尾的 frontmatter\n\n这三百字是用户自己敲的，一个都不能丢。\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "mem-0043.md"), []byte(broken), 0o600))

	body, err := memory.NewStore(root).Read("mem-0043")

	require.NoError(t, err, "解析不了 frontmatter 不该让整条记忆读不出来")
	assert.True(t, body.Malformed)
	assert.Contains(t, body.Text, "这三百字是用户自己敲的")
}

// 正常一条：frontmatter 与正文各归各位。
func TestStore_Read_SplitsTitleAndText(t *testing.T) {
	root := t.TempDir()
	s := memory.NewStore(root)
	require.NoError(t, s.Write("mem-0044", memory.Body{Raw: realNote}))

	body, err := s.Read("mem-0044")

	require.NoError(t, err)
	assert.Equal(t, "这个仓库的迁移必须手写 SQL", body.Title)
	assert.False(t, body.Malformed)
	assert.NotContains(t, body.Text, "title:", "frontmatter 漏进正文了")
}

// 用户自己写的 md 一个字节都不该被改写。
func TestStore_Write_KeepsRawByteForByte(t *testing.T) {
	root := t.TempDir()
	s := memory.NewStore(root)

	require.NoError(t, s.Write("mem-0045", memory.Body{Raw: realNote}))

	onDisk, err := os.ReadFile(filepath.Join(root, "mem-0045.md"))
	require.NoError(t, err)
	assert.Equal(t, realNote, string(onDisk), "写的时候顺手改了用户的内容")
}
