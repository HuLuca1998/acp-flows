// Package runtime 检测本机装了哪些 ACP Runtime、能不能用。
//
// 这个包只做一件事：**看**。它跑 `--version`、查登录态，绝不发起任何
// 会话或模型调用——那会产生真实费用，而用户只是打开了一下设置页。
//
// ★ 上层拿到 Result 后按 Status 分支，**绝不按 Name 分支**。
// 「codex 要敲什么命令」这类知识只存在于 Spec 里，是数据不是代码；
// 加第三个 runtime 只加一条注册表记录，不改任何 if。
// 见 docs/rules/design-principles.md §4.4。
package runtime

import (
	"context"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Status 是一次检测的结论。
type Status string

const (
	// StatusReady 装好了、登录了，可以直接用。
	StatusReady Status = "ready"
	// StatusNotInstalled PATH 上找不到可执行文件。
	StatusNotInstalled Status = "not_installed"
	// StatusNotAuthenticated 装是装了，但没登录。
	StatusNotAuthenticated Status = "not_authenticated"
	// StatusProbeFailed 检测本身没跑成——超时、崩了、权限不对。
	//
	// ★ 这一态不能省。没有它就只能把「检测失败」并进 not_installed，
	// 于是界面会对着一个装好的 runtime 说「请先安装」，用户照做还装不上。
	// 不知道就说不知道。
	StatusProbeFailed Status = "probe_failed"
)

// Spec 描述一个 Runtime 怎么检测、没配好时该敲什么命令。
//
// 这是**数据**。新增 Runtime 时在注册表里加一条，不碰 Detect 一行。
type Spec struct {
	// Name 是 Runtime 标识，会原样出现在 API 响应里。
	Name string
	// Bin 是可执行文件名，去 PATH 上找。
	Bin string
	// VersionArgs 取版本号，要求零副作用、不联网。
	VersionArgs []string
	// AuthArgs 查登录态。退出码 0 视为已登录。
	//
	// 留空表示这个 Runtime 没有独立的登录概念——那么只要装上就是 ready。
	AuthArgs []string
	// InstallCommand 是没装时给用户的命令，必须能直接复制去终端敲。
	InstallCommand string
	// LoginCommand 是没登录时给用户的命令，同上。
	LoginCommand string
}

// Result 是一次检测的结论。可比较，方便测试直接用 == 断言幂等。
type Result struct {
	Name    string
	Status  Status
	Version string
	Path    string
	// Remedy 是用户可以直接敲的一整条命令。ready 与 probe_failed 时为空——
	// 前者不需要修，后者没法确定要修什么。
	//
	// ★ 命令由这里给出，前端只负责显示。前端一旦开始按 Name 拼命令，
	// 加第三个 Runtime 就要改两处，且迟早漂移。
	Remedy string
	// Detail 是 probe_failed 时的原因原文，给排查用，不直接展示给用户。
	Detail string
}

// Detect 检测单个 Runtime。timeout 是**每条子命令**的上限。
//
// 不返回 error：检测不出来本身就是一种结论（StatusProbeFailed），
// 用 error 表达会让调用方在「查不到」和「没装」之间做二选一的猜测。
func Detect(ctx context.Context, spec Spec, timeout time.Duration) Result {
	res := Result{Name: spec.Name}

	path, err := exec.LookPath(spec.Bin)
	if err != nil {
		// 找不到、或找到了但没有执行位——对用户是同一件事：装一下。
		res.Status = StatusNotInstalled
		res.Remedy = spec.InstallCommand
		res.Detail = err.Error()
		return res
	}
	res.Path = path

	out, err := run(ctx, path, spec.VersionArgs, timeout)
	if err != nil {
		// 能找到文件却跑不起来：坏了、卡住了、或被安全策略拦了。
		// 这时说「没装」是错的，说「就绪」更错。
		res.Status = StatusProbeFailed
		res.Detail = err.Error()
		return res
	}
	res.Version = out

	if len(spec.AuthArgs) == 0 {
		res.Status = StatusReady
		return res
	}

	if _, err := run(ctx, path, spec.AuthArgs, timeout); err != nil {
		// ★ 这里**不**区分「退出码非 0」和「超时」。
		// 登录态查不出来时保守地当成没登录：给出的修复命令是 `xxx login`，
		// 用户敲一下最坏是发现自己本来就登录着。反过来把没登录报成 ready，
		// 用户要到真正提需求时才撞墙，那时错误信息离原因已经很远了。
		res.Status = StatusNotAuthenticated
		res.Remedy = spec.LoginCommand
		res.Detail = err.Error()
		return res
	}

	res.Status = StatusReady
	return res
}

// DetectAll 并发检测一批 Runtime，每个都会有结论。
//
// ★ 必须并发：串行的话装了 5 个 Runtime 且有一个卡住，用户要等 5 倍超时
// 才能看到设置页。也必须**互不影响**：一个卡住不能让其余的结果丢失。
func DetectAll(ctx context.Context, specs []Spec, timeout time.Duration) []Result {
	results := make([]Result, len(specs))
	var wg sync.WaitGroup
	for i, spec := range specs {
		wg.Go(func() { results[i] = Detect(ctx, spec, timeout) })
	}
	wg.Wait()
	return results
}

// run 跑一条子命令，返回 stdout 的首行。
func run(ctx context.Context, path string, args []string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, args...)
	// ★ 显式断开 stdin。ACP Runtime 的正常形态是从 stdin 读 JSON-RPC，
	// 万一某个子命令不认识参数就进了交互模式，继承的 stdin 会让它
	// 一直等下去——`--version` 就此挂死整个后端。
	cmd.Stdin = nil
	// ★★ 没有这一行，上面的超时形同虚设。
	//
	// ctx 到期时 CommandContext 只 kill **直接**子进程。子进程要是自己
	// fork 过（shell 包装脚本、node 启动器——真实的 CLI 几乎都是这样），
	// 孙子进程会继承着 stdout 管道活下去，而 Output() 要等管道关闭才返回。
	// 于是超时被架空，一直等到孙子进程自然结束。
	//
	// 真踩过：`/bin/sleep 30` 配 1.5 秒超时，实测跑满 30 秒。
	// WaitDelay 让 exec 在 kill 之后到点强行断开管道。
	cmd.WaitDelay = 200 * time.Millisecond

	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	line, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
	return line, nil
}
