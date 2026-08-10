package gitx_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/gitx"
)

// M3 U3.1.3 · GitHub remote 识别
//
// ★★ **不碰凭据**（Q41）：`gh` 自己把令牌存在 keychain 里，
// Duet 从头到尾不碰明文。这里跑的 `git remote get-url` 不联网、不需要认证。

// ★★ R1 · https 与 ssh 两种写法得到**同一个** owner/repo。
//
// 认不出的话，用户会看到「有 remote 但不知道是哪个仓库」，
// 而那正是他打开这个对话框想确认的事。
func TestParseRemoteURL_R1_BothFormsGiveTheSameSlug(t *testing.T) {
	cases := []struct {
		name, url         string
		host, owner, repo string
	}{
		{"https 带 .git", "https://github.com/HuLuca1998/acp-flows.git", "github.com", "HuLuca1998", "acp-flows"},
		{"https 不带 .git", "https://github.com/HuLuca1998/acp-flows", "github.com", "HuLuca1998", "acp-flows"},
		{"scp 写法", "git@github.com:HuLuca1998/acp-flows.git", "github.com", "HuLuca1998", "acp-flows"},
		{"ssh:// 写法", "ssh://git@github.com/HuLuca1998/acp-flows.git", "github.com", "HuLuca1998", "acp-flows"},
		{"带端口", "ssh://git@github.com:22/HuLuca1998/acp-flows.git", "github.com", "HuLuca1998", "acp-flows"},
		{"结尾有斜杠", "https://github.com/HuLuca1998/acp-flows/", "github.com", "HuLuca1998", "acp-flows"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := gitx.ParseRemoteURL(c.url)
			if got.Host != c.host {
				t.Errorf("host = %q，想要 %q", got.Host, c.host)
			}
			if got.Owner != c.owner || got.Repo != c.repo {
				t.Errorf("owner/repo = %q/%q，想要 %q/%q", got.Owner, got.Repo, c.owner, c.repo)
			}
			if got.Slug() != c.owner+"/"+c.repo {
				t.Errorf("slug = %q", got.Slug())
			}
			if !got.IsGitHub() {
				t.Error("没认出这是 GitHub")
			}
			// ★ URL 原文一定要留着：解析只是锦上添花
			if got.URL != c.url {
				t.Errorf("URL 原文被改了：%q", got.URL)
			}
		})
	}
}

// ★★ R4 · 非 GitHub 的 remote **照常显示**，不被丢弃。
//
// 丢掉的话，用 GitLab 的用户会看到「没有 remote」，而他明明配了一个。
func TestParseRemoteURL_R4_NonGitHubIsKept(t *testing.T) {
	cases := []struct {
		url, host, slug string
	}{
		{"https://gitlab.com/group/repo.git", "gitlab.com", "group/repo"},
		// ★ GitLab 允许多级 group：取**最后两段**，否则 `group/sub` 会被当成 owner/repo
		{"https://gitlab.com/group/sub/repo.git", "gitlab.com", "sub/repo"},
		{"git@git.mycompany.internal:team/thing.git", "git.mycompany.internal", "team/thing"},
	}

	for _, c := range cases {
		got := gitx.ParseRemoteURL(c.url)
		if got.URL == "" {
			t.Errorf("%q 的 URL 被丢了——用 GitLab 的用户会看到「没有 remote」", c.url)
		}
		if got.Host != c.host {
			t.Errorf("%q 的 host = %q，想要 %q", c.url, got.Host, c.host)
		}
		if got.Slug() != c.slug {
			t.Errorf("%q 的 slug = %q，想要 %q", c.url, got.Slug(), c.slug)
		}
		if got.IsGitHub() {
			t.Errorf("%q 被当成了 GitHub", c.url)
		}
	}
}

// ★★ URL 里夹着的密码**绝不能带出来**。
//
// 有些人的 remote 就是 `https://user:token@github.com/...` 这种写法，
// 而这个字段会显示在界面上、写进日志。
func TestParseRemoteURL_StripsCredentials(t *testing.T) {
	got := gitx.ParseRemoteURL("https://someone:ghp_secrettoken@github.com/owner/repo.git")

	if strings.Contains(got.Host, "ghp_secrettoken") || strings.Contains(got.Host, "someone") {
		t.Errorf("host 里带出了凭据：%q", got.Host)
	}
	if got.Host != "github.com" {
		t.Errorf("host = %q", got.Host)
	}
	if got.Slug() != "owner/repo" {
		t.Errorf("slug = %q", got.Slug())
	}
}

// ★ 解析不出 owner/repo 时**保留 URL 原文**。
//
// 自建 git 服务的路径结构千奇百怪，显示原文比显示「无」诚实。
func TestParseRemoteURL_KeepsUrlWhenUnparseable(t *testing.T) {
	for _, url := range []string{
		"https://example.com/onlyonepart",
		"some-weird-thing",
		"/srv/git/bare-repo.git",
	} {
		got := gitx.ParseRemoteURL(url)
		if got.URL != url {
			t.Errorf("%q 的 URL 原文没了：%q", url, got.URL)
		}
		if got.Slug() != "" {
			t.Errorf("%q 编出了一个 slug：%q", url, got.Slug())
		}
	}
}

// ★★ R2 · 没有 remote 时如实说「无」，**不是错误**。
//
// 本地仓库、还没推过的项目都很常见——当成错误的话，
// 创建项目的预演会因为一个完全正常的状态而失败。
func TestProbeRemote_R2_NoRemoteIsEmptyNotError(t *testing.T) {
	root := t.TempDir()
	initRepo(t, root)

	got, err := gitx.ProbeRemote(context.Background(), root)
	if err != nil {
		t.Fatalf("没有 remote 却报了错：%v——本地仓库很常见", err)
	}
	if got.URL != "" {
		t.Errorf("编出了一个 remote：%q", got.URL)
	}
	if got.Slug() != "" {
		t.Errorf("编出了一个 slug：%q", got.Slug())
	}
}

// 配了 remote 就读得出来（真 git 仓库，不 mock）。
func TestProbeRemote_ReadsConfiguredOrigin(t *testing.T) {
	root := t.TempDir()
	initRepo(t, root)
	gitRun(t, root, "remote", "add", "origin", "https://github.com/HuLuca1998/acp-flows.git")

	got, err := gitx.ProbeRemote(context.Background(), root)
	if err != nil {
		t.Fatalf("读 remote: %v", err)
	}
	if got.Slug() != "HuLuca1998/acp-flows" {
		t.Errorf("slug = %q", got.Slug())
	}
	if !got.IsGitHub() {
		t.Error("没认出这是 GitHub")
	}
}

// ★ 非 git 目录不报错，返回空。
func TestProbeRemote_NonRepoIsEmpty(t *testing.T) {
	got, err := gitx.ProbeRemote(context.Background(), t.TempDir())
	if err != nil {
		t.Errorf("非仓库报了错：%v", err)
	}
	if got.URL != "" {
		t.Errorf("非仓库却读出了 %q", got.URL)
	}
}

// ★★ R3 · **只读**：读 remote 不改仓库。
func TestProbeRemote_R3_DoesNotModifyRepo(t *testing.T) {
	root := t.TempDir()
	initRepo(t, root)
	gitRun(t, root, "remote", "add", "origin", "git@github.com:o/r.git")

	before := gitStatus(t, root)
	if _, err := gitx.ProbeRemote(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	if after := gitStatus(t, root); before != after {
		t.Errorf("读 remote 改了仓库状态：\n之前 %q\n之后 %q", before, after)
	}
}

func initRepo(t *testing.T, dir string) {
	t.Helper()
	gitRun(t, dir, "init", "-q")
	gitRun(t, dir, "config", "user.email", "t@example.com")
	gitRun(t, dir, "config", "user.name", "t")
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	if err := gitx.RunForTest(context.Background(), dir, args...); err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
}

func gitStatus(t *testing.T, dir string) string {
	t.Helper()
	// 用文件系统指纹代替 git status：连 .git 里的配置改动也算进去
	var b strings.Builder
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil //nolint:nilerr // 走不进去的目录跳过即可
		}
		rel, _ := filepath.Rel(dir, path)
		b.WriteString(rel)
		b.WriteString(":")
		content, _ := os.ReadFile(path)
		b.Write(content)
		b.WriteString("\n")
		return nil
	})
	return b.String()
}
