package gitx

import (
	"context"
	"strings"
)

// Remote 是 `origin` 的识别结果。
//
// ★ 没有 remote 时全部字段为空——**不编造**。
// 猜一个出来的话，用户会看到一个他从没配过的仓库名。
type Remote struct {
	// URL 是原样的 remote 地址。
	//
	// ★ 非 GitHub 的（GitLab / 自建）也照样带出来：
	// 丢掉的话，用 GitLab 的用户会看到「没有 remote」，
	// 而他明明配了一个。
	URL string
	// Host 是 `github.com` / `gitlab.com` / 自建域名。
	Host string
	// Owner 与 Repo 只在解析得出时有值。
	Owner string
	Repo  string
}

// IsGitHub 报告这个 remote 在 github.com 上。
func (r Remote) IsGitHub() bool { return r.Host == "github.com" }

// Slug 返回 `owner/repo`，解析不出时为空。
func (r Remote) Slug() string {
	if r.Owner == "" || r.Repo == "" {
		return ""
	}
	return r.Owner + "/" + r.Repo
}

// ProbeRemote 读出 `origin` 的地址并尽量解析成 owner/repo。
//
// ★★ **只读，且不碰任何凭据**（Q41）：它跑的是 `git remote get-url`，
// 那条命令不联网、不需要认证。**绝不为了识别账号去发网络请求**——
// 那既慢又要凭据，而用户只是想确认「Duet 认出的是不是我这个仓库」。
func ProbeRemote(ctx context.Context, path string) (Remote, error) {
	out, err := run(ctx, path, "remote", "get-url", "origin")
	if err != nil {
		// ★ 没有 origin 不是错误：本地仓库、还没推过的项目都很常见。
		return Remote{}, nil
	}
	url := strings.TrimSpace(out)
	if url == "" {
		return Remote{}, nil
	}
	return ParseRemoteURL(url), nil
}

// ParseRemoteURL 把 remote 地址解析成结构。
//
// 三种真实写法都要认（`git remote -v` 里都见得到）：
//
//	https://github.com/owner/repo.git
//	git@github.com:owner/repo.git
//	ssh://git@github.com/owner/repo.git
//
// ★ 解析不出 owner/repo 时**保留 URL 原文**——
// 那是自建 git 服务或不常见路径结构的情况，界面显示原文比显示「无」诚实。
func ParseRemoteURL(url string) Remote {
	r := Remote{URL: url}

	rest := url
	switch {
	case strings.HasPrefix(rest, "ssh://"):
		rest = strings.TrimPrefix(rest, "ssh://")
		rest = stripUserInfo(rest)
		host, path, ok := strings.Cut(rest, "/")
		if !ok {
			return r
		}
		r.Host = host
		fillOwnerRepo(&r, path)

	case strings.HasPrefix(rest, "https://"), strings.HasPrefix(rest, "http://"):
		rest = strings.TrimPrefix(strings.TrimPrefix(rest, "https://"), "http://")
		rest = stripUserInfo(rest)
		host, path, ok := strings.Cut(rest, "/")
		if !ok {
			return r
		}
		r.Host = host
		fillOwnerRepo(&r, path)

	default:
		// scp 写法：`git@github.com:owner/repo.git`
		//
		// ★ 必须在冒号处切而不是在 `/`：这种写法里主机名后面跟的是冒号，
		// 按 `/` 切会把 `git@github.com:owner` 整个当成主机。
		userHost, path, ok := strings.Cut(rest, ":")
		if !ok {
			return r
		}
		r.Host = stripUserInfo(userHost)
		fillOwnerRepo(&r, path)
	}

	// 端口号不属于主机标识（`github.com:22`）
	if h, _, ok := strings.Cut(r.Host, ":"); ok {
		r.Host = h
	}
	return r
}

// stripUserInfo 去掉 `git@` / `user:pass@` 前缀。
//
// ★ 顺带把可能夹在 URL 里的密码摘掉——**它绝不该出现在界面上或日志里**。
// 有些人的 remote 就是 `https://user:token@github.com/...` 这种写法。
func stripUserInfo(s string) string {
	if i := strings.LastIndex(s, "@"); i >= 0 {
		return s[i+1:]
	}
	return s
}

// fillOwnerRepo 从路径部分填出 owner/repo。
func fillOwnerRepo(r *Remote, path string) {
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, ".git")
	path = strings.TrimSuffix(path, "/")

	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return
	}
	// ★ 取**最后两段**：GitLab 允许多级 group（`group/sub/repo`），
	// 取前两段会把 `group/sub` 当成 owner/repo。
	owner, repo := parts[len(parts)-2], parts[len(parts)-1]
	if owner == "" || repo == "" {
		return
	}
	r.Owner, r.Repo = owner, repo
}
