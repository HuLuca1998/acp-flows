package testutil

import (
	"path/filepath"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
)

// tempPaths 把全部数据目录重定向到测试专属的临时目录。
type tempPaths struct{ root string }

// TempPaths 返回一个落在 t.TempDir() 下的 port.Paths。
//
// 每次调用给出**不同**的目录：测试之间不共享状态，否则一个测试写脏的数据
// 会让另一个测试莫名其妙地失败。目录随测试结束自动清理。
func TempPaths(t *testing.T) port.Paths {
	t.Helper()
	root := t.TempDir()
	p := &tempPaths{root: root}
	// 自证：夹具自己也要过守卫，避免将来改坏了却没人发现。
	GuardPath(t, p.DataDir())
	return p
}

func (p *tempPaths) DataDir() string { return filepath.Join(p.root, ".acpflows") }
func (p *tempPaths) DBPath() string  { return filepath.Join(p.DataDir(), "duet.db") }
func (p *tempPaths) RuntimeSession() string {
	return filepath.Join(p.DataDir(), "runtime", "session.json")
}
func (p *tempPaths) RuntimesDir() string     { return filepath.Join(p.DataDir(), "runtimes") }
func (p *tempPaths) CredentialsPath() string { return filepath.Join(p.DataDir(), "credentials") }
func (p *tempPaths) WorktreeRoot() string    { return filepath.Join(p.DataDir(), "worktrees") }
