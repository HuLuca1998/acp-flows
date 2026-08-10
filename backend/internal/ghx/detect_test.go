package ghx_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/ghx"
)

// M3 U3.1.3 R5 · gh 检测（Q41）
//
// ★★ **Duet 从头到尾不碰令牌**。这里只问两个问题：装了吗、登录了吗。
// 没有令牌可泄漏，是比「小心保管令牌」强得多的性质。

// 真实输出（照 `gh --version` / `gh auth status` 抄的）。
const (
	versionOut = "gh version 2.62.0 (2024-11-14)\nhttps://github.com/cli/cli/releases/tag/v2.62.0\n"

	loggedInOut = `github.com
  ✓ Logged in to github.com account HuLuca1998 (keyring)
  - Active account: true
  - Git operations protocol: https
  - Token: gho_************************************
`

	notLoggedInOut = `You are not logged into any GitHub hosts. To log in, run: gh auth login
`
)

// runnerFor 造一个按命令返回不同输出的 Runner。
func runnerFor(version, auth string, authErr error) ghx.Runner {
	return func(_ context.Context, _ string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "auth" {
			return auth, authErr
		}
		return version, nil
	}
}

// 装了且登录了。
func TestDetect_ReadyReadsVersionAndAccount(t *testing.T) {
	got := ghx.Detect(context.Background(), runnerFor(versionOut, loggedInOut, nil))

	if got.Status != ghx.StatusReady {
		t.Fatalf("状态 = %q，想要 ready（detail=%s）", got.Status, got.Detail)
	}
	if got.Version != "2.62.0" {
		t.Errorf("版本 = %q", got.Version)
	}
	if got.Account != "HuLuca1998" {
		t.Errorf("账号 = %q", got.Account)
	}
	// ★ ready 时不给修复命令：没什么要修的
	if got.Remedy != "" {
		t.Errorf("ready 却给了修复命令 %q", got.Remedy)
	}
}

// ★★ 装了但没登录 → 给 `gh auth login`。
//
// `gh auth status` 在没登录时以非 0 退出——**那是正常结论不是故障**。
// 报成 probe_failed 的话，用户拿不到那句「去登录」。
func TestDetect_NotAuthenticatedGivesLoginCommand(t *testing.T) {
	got := ghx.Detect(context.Background(),
		runnerFor(versionOut, notLoggedInOut, errors.New("exit status 1")))

	if got.Status != ghx.StatusNotAuthenticated {
		t.Fatalf("状态 = %q，想要 not_authenticated（detail=%s）", got.Status, got.Detail)
	}
	if got.Remedy != "gh auth login" {
		t.Errorf("修复命令 = %q，想要 gh auth login", got.Remedy)
	}
	// 版本还是读得出来的——它装着，只是没登录
	if got.Version != "2.62.0" {
		t.Errorf("版本 = %q", got.Version)
	}
}

// ★★ 认不出的失败**当成检测失败**，不当成「没登录」。
//
// 给出一句「请运行 gh auth login」而实际问题是别的（配置文件坏了、
// 网络不通），会让用户照着做一遍然后发现没用。
func TestDetect_UnknownAuthFailureIsProbeFailedNotUnauthenticated(t *testing.T) {
	got := ghx.Detect(context.Background(),
		runnerFor(versionOut, "error connecting to api.github.com", errors.New("exit status 1")))

	if got.Status == ghx.StatusNotAuthenticated {
		t.Error("把一个说不清的失败报成了「没登录」——" +
			"用户会照着 gh auth login 做一遍然后发现没用")
	}
	if got.Status != ghx.StatusProbeFailed {
		t.Errorf("状态 = %q，想要 probe_failed", got.Status)
	}
	// ★ 说不清要修什么就不给命令
	if got.Remedy != "" {
		t.Errorf("检测失败却给了修复命令 %q", got.Remedy)
	}
	if got.Detail == "" {
		t.Error("检测失败却没留下原因，没法排查")
	}
}

// ★ 文件在但跑不起来（权限、架构不对、装坏了）→ **不报成「没装」**。
//
// 报成没装的话，用户会去 brew install 一个已经在那儿的东西。
func TestDetect_BrokenBinaryIsNotReportedAsMissing(t *testing.T) {
	if !ghInstalled() {
		t.Skip("本机没装 gh，这条测的是「装了但跑不起来」")
	}
	got := ghx.Detect(context.Background(),
		func(context.Context, string, ...string) (string, error) {
			return "", errors.New("bad CPU type in executable")
		})

	if got.Status == ghx.StatusNotInstalled {
		t.Error("跑不起来被报成了「没装」——用户会去 brew install 一个已经在的东西")
	}
	if got.Status != ghx.StatusProbeFailed {
		t.Errorf("状态 = %q，想要 probe_failed", got.Status)
	}
	if got.Path == "" {
		t.Error("明明找到了可执行文件却没记下路径")
	}
}

// ★ 账号名取不到就留空，**不猜**。
//
// 显示一个错的账号名比不显示糟得多——用户会以为自己登在另一个账号上。
func TestDetect_UnparseableAccountIsEmptyNotGuessed(t *testing.T) {
	got := ghx.Detect(context.Background(),
		runnerFor(versionOut, "✓ Logged in to github.com (oauth_token)\n", nil))

	if got.Status != ghx.StatusReady {
		t.Fatalf("状态 = %q", got.Status)
	}
	if got.Account != "" {
		t.Errorf("账号名解不出却编了一个：%q", got.Account)
	}
}

// ★★ 全程不碰令牌：结果里**任何字段都不能带出 token**。
//
// `gh auth status` 的输出里就有一行 `Token: gho_***`，
// 把它整个塞进 Detail 或 Account 的话，令牌会进日志、进界面。
func TestDetect_NeverLeaksToken(t *testing.T) {
	withToken := strings.Replace(loggedInOut,
		"gho_************************************", "gho_REALSECRETVALUE12345", 1)

	got := ghx.Detect(context.Background(), runnerFor(versionOut, withToken, nil))

	for name, v := range map[string]string{
		"Account": got.Account, "Detail": got.Detail,
		"Version": got.Version, "Remedy": got.Remedy, "Path": got.Path,
	} {
		if strings.Contains(v, "gho_") {
			t.Errorf("★★ %s 里带出了令牌：%q——Q41 的全部意义就是不碰它", name, v)
		}
	}
}

// 版本号解不出时留空，不影响状态判定。
func TestDetect_UnparseableVersionStillWorks(t *testing.T) {
	got := ghx.Detect(context.Background(), runnerFor("某种没见过的输出\n", loggedInOut, nil))

	if got.Status != ghx.StatusReady {
		t.Errorf("状态 = %q——版本号读不出不该影响「装了且登录了」这个结论", got.Status)
	}
	if got.Version != "" {
		t.Errorf("版本 = %q", got.Version)
	}
}

// 没装 gh 时给安装命令。
//
// ★ 这条依赖本机环境：装了 gh 就跳过（那时走的是别的分支）。
func TestDetect_NotInstalledGivesInstallCommand(t *testing.T) {
	if ghInstalled() {
		t.Skip("本机装了 gh，这条测的是没装的情况")
	}
	got := ghx.Detect(context.Background(), nil)

	if got.Status != ghx.StatusNotInstalled {
		t.Fatalf("状态 = %q", got.Status)
	}
	if got.Remedy != "brew install gh" {
		t.Errorf("修复命令 = %q", got.Remedy)
	}
}

func ghInstalled() bool {
	got := ghx.Detect(context.Background(),
		func(context.Context, string, ...string) (string, error) { return versionOut, nil })
	return got.Status != ghx.StatusNotInstalled
}
