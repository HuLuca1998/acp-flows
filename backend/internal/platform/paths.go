// Package platform 收拢与运行环境打交道的东西：路径、时间、ID、进程、凭据。
//
// 它存在的理由是把**全部不确定性的入口收在一处**，这样测试才可能是确定的。
// domain 与 app 里出现裸 time.Now() / rand / os.UserHomeDir() 时，
// 测试就变成薛定谔的——由 lint 拦。
package platform

import (
	"fmt"
	"os"
	"path/filepath"
)

// 数据目录名。改这些会影响用户已有数据，属于破坏性变更。
const (
	dataDirName     = ".acpflows"
	worktreeDirName = ".duet"
)

// osPaths 是生产环境的路径实现，全部落在用户家目录下。
type osPaths struct {
	home string
}

// NewPaths 返回生产环境的 port.Paths 实现。
//
// 家目录取自 os.UserHomeDir() 而非 os.Getenv("HOME")：
// 后者在某些启动方式下是空的——Tauri 拉起 sidecar 时尤其要注意。
func NewPaths() (*osPaths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home dir: %w", err)
	}
	return &osPaths{home: home}, nil
}

// NewPathsAt 把数据目录指向 root，用于 dev-web 模式（DUET_DATA_DIR）。
//
// 开发态默认落在 ~/.duet-dev，与用户真实数据隔离。
func NewPathsAt(root string) *osPaths { return &osPaths{home: root} }

func (p *osPaths) DataDir() string { return filepath.Join(p.home, dataDirName) }
func (p *osPaths) DBPath() string  { return filepath.Join(p.DataDir(), "duet.db") }
func (p *osPaths) RuntimeSession() string {
	return filepath.Join(p.DataDir(), "runtime", "session.json")
}
func (p *osPaths) RuntimesDir() string     { return filepath.Join(p.DataDir(), "runtimes") }
func (p *osPaths) CredentialsPath() string { return filepath.Join(p.DataDir(), "credentials") }
func (p *osPaths) WorktreeRoot() string {
	return filepath.Join(p.home, worktreeDirName, "worktrees")
}

// EnsureDirs 建出全部需要的目录。
//
// 凭据目录用 0700：里面是加密后的 GitHub 令牌，不该对同机其他用户可读。
func (p *osPaths) EnsureDirs() error {
	for _, dir := range []string{
		p.DataDir(),
		filepath.Dir(p.RuntimeSession()),
		p.RuntimesDir(),
		p.WorktreeRoot(),
	} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}
	return nil
}
