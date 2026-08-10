// Package workspace 读 worktree 里的文件，给引用注入用（U10.7.3）。
//
// ★ **只读，一个字节都不写。** 引用是用户让 AI「看一眼这个文件」，
// 不是让我们动它。
package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Store 实现 app/work 的 WorkspaceFiles。
type Store struct{}

// ReadFile 读 root 下的一个相对路径文件。
//
// ★★ 路径校验在这里做，不指望调用方：绝对路径、`..`、空串一律拒——
// 一个带 `..` 的引用就能把家目录里任何文件塞进 prompt 发给模型。
func (Store) ReadFile(root, rel string) (string, error) {
	if rel == "" || filepath.IsAbs(rel) {
		return "", fmt.Errorf("fsstore/workspace: 引用路径不合法 %q", rel)
	}
	clean := filepath.Clean(rel)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("fsstore/workspace: 引用路径逃出了工作区 %q", rel)
	}

	path := filepath.Join(root, clean)
	raw, err := os.ReadFile(path)
	if err != nil {
		// ★ 带完整路径明说——用户要知道哪个引用坏了、去哪看。
		return "", fmt.Errorf("fsstore/workspace: 读不到引用文件 %s: %w", path, err)
	}
	return string(raw), nil
}
