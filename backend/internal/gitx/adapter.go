package gitx

import (
	"context"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
)

// RemoteProber 实现 port.RemoteProbe。
type RemoteProber struct{}

// ProbeRemote 读出 origin。★ 不碰凭据、不发网络请求（Q41）。
func (RemoteProber) ProbeRemote(ctx context.Context, path string) (port.GitRemoteInfo, error) {
	r, err := ProbeRemote(ctx, path)
	if err != nil {
		return port.GitRemoteInfo{}, err
	}
	return port.GitRemoteInfo{
		URL:      r.URL,
		Host:     r.Host,
		Slug:     r.Slug(),
		IsGitHub: r.IsGitHub(),
	}, nil
}
