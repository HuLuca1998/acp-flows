// Package testutil 提供测试专用的夹具与辅助。
//
// ★ 与 internal/util 是两回事，不要混：
//   - internal/util  生产代码的纯工具函数，生产代码可以 import
//   - tests/testutil 只有测试能 import；生产代码 import 它会被 depguard 拦下
package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 铁律 6：测试禁止读写用户真实数据。这些前缀一律拦下。
//
// 相对于用户家目录。新增受保护路径时同时补 guard_test.go 的用例。
var forbiddenUnderHome = []string{
	".acpflows",     // 全局数据目录：DB、凭据、runtime 注册表
	".duet",         // worktree 根目录
	".claude",       // Claude Code 的机器级配置与会话历史
	".codex",        // codex 的机器级配置
	".duet-updater", // minisign 私钥备份（adr/0007）——丢了就再也推不了更新
}

// UserHomeForTest 返回用户家目录，取不到时跳过当前测试。
//
// 单独抽出来是为了让守卫的测试能拿到同一个基准，不必各自 os.UserHomeDir()。
func UserHomeForTest(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("拿不到用户家目录，跳过：%v", err)
	}
	return home
}

// CheckPathAllowed 报告测试是否可以访问 path。
//
// 命中用户真实数据目录时返回错误，错误信息指向铁律 6 并带上被拦的路径——
// 排查时能立刻知道是哪一条规则、哪一个路径。
//
// 注意：临时目录下出现同名子目录（例如 <tmp>/.acpflows）是允许的，
// 判定只看是否落在真实家目录下。
func CheckPathAllowed(path string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		// 拿不到家目录时无从判定，放行而不是误伤。
		return nil
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("解析路径 %s 失败: %w", path, err)
	}
	abs = filepath.Clean(abs)

	for _, name := range forbiddenUnderHome {
		root := filepath.Join(home, name)
		if abs == root || strings.HasPrefix(abs, root+string(filepath.Separator)) {
			return fmt.Errorf(
				"违反铁律 6：测试试图访问用户真实数据 %s（受保护目录 %s）；"+
					"请改用 testutil.TempPaths(t) 或 t.TempDir()",
				path, root)
		}
	}
	return nil
}

// GuardPath 在路径不被允许时直接终止测试。
//
// 供夹具在真正打开文件前调用——比返回错误更早地把问题暴露在调用栈上。
func GuardPath(t *testing.T, path string) {
	t.Helper()
	if err := CheckPathAllowed(path); err != nil {
		t.Fatal(err)
	}
}
