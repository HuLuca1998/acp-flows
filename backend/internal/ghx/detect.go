// Package ghx 检测本机的 GitHub CLI。
//
// ★★ **Duet 从头到尾不碰令牌**（open-questions.md Q41 裁定）。
//
// `gh` 自己把令牌存在 macOS keychain 里，我们只问它两个问题：
// 装了吗、登录了吗。不进数据库、不进日志、不进 git 历史、不进任何配置文件——
// **没有令牌可泄漏，是比「小心保管令牌」强得多的性质**。
//
// 代价说清楚：用户必须自己装 `gh` 并登录一次。这比「在应用里贴一个令牌」
// 多一步，但换来的是我们永远不需要为一个泄漏的令牌负责。
package ghx

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

// detectTimeout 是每条子命令的上限。
//
// ★ `gh auth status` 在网络不通时会卡住——而这条检测跑在
// **用户点了在等**的创建项目对话框里。
const detectTimeout = 5 * time.Second

// Status 是 gh 的四态。
//
// ★ 和 Runtime 检测同构，且**必须有第四态**：只用「装了/登录了」两个布尔
// 表达不了「检测本身失败了」，那时 `installed=false` 与「确实没装」
// 无法区分，界面会把一个可能是假的结论告诉用户，还附上一句「请先安装」。
type Status string

const (
	// StatusReady 装了且登录了。
	StatusReady Status = "ready"
	// StatusNotInstalled 没装。
	StatusNotInstalled Status = "not_installed"
	// StatusNotAuthenticated 装了但没登录。
	StatusNotAuthenticated Status = "not_authenticated"
	// StatusProbeFailed 检测本身失败了——**不猜**。
	StatusProbeFailed Status = "probe_failed"
)

// 修复命令。★ 由这里给出，前端只负责原样显示。
const (
	installCommand = "brew install gh"
	loginCommand   = "gh auth login"
)

// Result 是一次检测的结论。可比较，方便测试用 == 断言幂等。
type Result struct {
	Status Status
	// Version 形如 `2.62.0`，检测不到时为空。
	Version string
	Path    string
	// Remedy 是用户可以直接敲的一整条命令。ready 与 probe_failed 时为空——
	// 前者不需要修，后者没法确定要修什么。
	Remedy string
	// Account 是登录的 GitHub 账号名，只在 ready 时可能有值。
	//
	// ★ 它来自 `gh auth status` 的输出，**不是我们发网络请求查的**。
	Account string
	// Detail 是 probe_failed 时的原因原文，给排查用，不直接展示给用户。
	Detail string
}

// Runner 跑一条命令并返回合并后的输出。抽出来是为了测试能注入。
//
// ★ 定义在使用方，实现是本包的 execRunner——测试里换成脚本或桩，
// 而**不 mock `exec.Command` 本身**。
type Runner func(ctx context.Context, name string, args ...string) (string, error)

// Detect 检测本机的 gh。
//
// 不返回 error：检测不出来本身就是一种结论（StatusProbeFailed）。
// 用 error 表达会让调用方在「查不到」和「没装」之间做二选一的猜测。
func Detect(ctx context.Context, run Runner) Result {
	if run == nil {
		run = execRunner
	}

	path, err := exec.LookPath("gh")
	if err != nil {
		return Result{Status: StatusNotInstalled, Remedy: installCommand}
	}

	verCtx, cancel := context.WithTimeout(ctx, detectTimeout)
	defer cancel()
	out, err := run(verCtx, "gh", "--version")
	if err != nil {
		// ★ 文件在但跑不起来（权限、架构不对、装坏了）——
		// **不报成「没装」**：那会让用户去 `brew install` 一个已经在那儿的东西。
		return Result{
			Status: StatusProbeFailed,
			Path:   path,
			Detail: err.Error(),
		}
	}

	res := Result{Path: path, Version: parseVersion(out)}

	authCtx, cancelAuth := context.WithTimeout(ctx, detectTimeout)
	defer cancelAuth()
	authOut, authErr := run(authCtx, "gh", "auth", "status")
	if authErr != nil {
		// ★ 没登录时 `gh auth status` 以非 0 退出——**这是正常结论不是故障**。
		//
		// 但要和「命令本身跑不起来」分开：前者给「去登录」，后者给不出建议。
		if isNotLoggedIn(authOut) {
			res.Status = StatusNotAuthenticated
			res.Remedy = loginCommand
			return res
		}
		res.Status = StatusProbeFailed
		res.Detail = authErr.Error()
		return res
	}

	res.Status = StatusReady
	res.Account = parseAccount(authOut)
	return res
}

// isNotLoggedIn 判断这次失败是不是「就是没登录」。
//
// ★ 认不出时**当成检测失败**而不是「没登录」：给出一句
// 「请运行 gh auth login」而实际问题是别的（比如配置文件坏了），
// 会让用户照着做一遍然后发现没用。
func isNotLoggedIn(out string) bool {
	lower := strings.ToLower(out)
	for _, sign := range []string{
		"not logged in",
		"no accounts",
		"you are not logged into any github hosts",
		"gh auth login",
	} {
		if strings.Contains(lower, sign) {
			return true
		}
	}
	return false
}

// parseVersion 从 `gh version 2.62.0 (2024-11-14)` 里取出版本号。
func parseVersion(out string) string {
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		// 形如 gh version 2.62.0 (...)
		if len(fields) >= 3 && fields[0] == "gh" && fields[1] == "version" {
			return fields[2]
		}
	}
	return ""
}

// parseAccount 从 `gh auth status` 的输出里取出账号名。
//
// 真实输出形如：
//
//	github.com
//	  ✓ Logged in to github.com account HuLuca1998 (keyring)
//
// ★ 取不到就留空，**不猜**：显示一个错的账号名比不显示糟得多——
// 用户会以为自己登在另一个账号上。
func parseAccount(out string) string {
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		for i, f := range fields {
			if f == "account" && i+1 < len(fields) {
				return strings.TrimSpace(fields[i+1])
			}
		}
	}
	return ""
}

// execRunner 真的跑一条命令。
//
// ★ 合并 stdout 与 stderr：`gh auth status` 把结论写在 **stderr** 上，
// 只读 stdout 的话「没登录」这条永远判不出来。
func execRunner(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	// ★ **必须设 WaitDelay**：ctx 超时时 exec 只 kill 直接子进程，
	// 而它 fork 出去的孙子进程还攥着管道——`CombinedOutput` 会一直等读完，
	// 于是超时被架空。`gh` 会起子进程（keyring 助手、浏览器），这条必踩。
	cmd.WaitDelay = time.Second

	// ★ 顺带禁掉交互式提示：`gh` 在某些状态下会问「要不要现在登录」，
	// 而这条命令跑在一个没有终端的后台进程里——它会挂到超时。
	cmd.Stdin = nil

	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), errors.Join(err, ctx.Err())
	}
	return string(out), nil
}
