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
	"bytes"
	"context"
	"os"
	"os/exec"
	"regexp"
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
	// AuthBin 是查登录态用的可执行文件。留空则用 Bin。
	//
	// ★ 适配器与 CLI 常常是**两个不同的可执行文件**：能不能跑取决于
	// claude-agent-acp，登没登录归 claude 管。
	AuthBin string
	// AuthArgs 查登录态。留空表示这个 Runtime 没有独立的登录概念，
	// 那么只要装上就是 ready。
	AuthArgs []string
	// AuthOKSubstring 是「已登录」的判据：合并后的输出里含这个子串才算登录。
	// 留空则只看退出码。
	//
	// ★ 不能一律只看退出码。实测 `claude auth status` **未登录时也返回 0**，
	// 结论藏在 JSON 的 `"loggedIn"` 字段里——只看退出码会把没登录的 claude
	// 报成 ready，用户要到真正提需求时才撞墙，那时错误离原因已经很远了。
	AuthOKSubstring string
	// EnvRemove 是必须从子进程环境里删掉的变量。
	//
	// ★ 必踩：Claude Code 会给子进程注入 CLAUDECODE 等标记，
	// claude-agent-acp 继承到之后会误判自己跑在另一个 agent 内部而**拒绝服务**
	// （docs/notes/acp-field-notes.md §5 坑 1）。开发时 duetd 正是被
	// Claude Code 拉起的，不清理的话检测结果全是假的。
	EnvRemove []string
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

	out, _, err := run(ctx, path, spec.VersionArgs, spec.EnvRemove, timeout)
	if err != nil {
		// 能找到文件却跑不起来：坏了、卡住了、或被安全策略拦了。
		// 这时说「没装」是错的，说「就绪」更错。
		res.Status = StatusProbeFailed
		res.Detail = err.Error()
		return res
	}
	res.Version = extractVersion(out)

	if len(spec.AuthArgs) == 0 {
		res.Status = StatusReady
		return res
	}

	authBin := path
	if spec.AuthBin != "" && spec.AuthBin != spec.Bin {
		authBin, err = exec.LookPath(spec.AuthBin)
		if err != nil {
			// 装了适配器却没装 CLI：跑得起来，但一定没登录。
			// 报 ready 会让用户在真正提需求时才发现问题。
			res.Status = StatusNotAuthenticated
			res.Remedy = spec.LoginCommand
			res.Detail = err.Error()
			return res
		}
	}

	_, combined, err := run(ctx, authBin, spec.AuthArgs, spec.EnvRemove, timeout)
	loggedIn := err == nil
	if loggedIn && spec.AuthOKSubstring != "" {
		loggedIn = strings.Contains(combined, spec.AuthOKSubstring)
	}
	if !loggedIn {
		// ★ 这里**不**区分「退出码非 0」和「超时」。
		// 登录态查不出来时保守地当成没登录：给出的修复命令是 `xxx login`，
		// 用户敲一下最坏是发现自己本来就登录着。反过来把没登录报成 ready，
		// 用户要到真正提需求时才撞墙，那时错误信息离原因已经很远了。
		res.Status = StatusNotAuthenticated
		res.Remedy = spec.LoginCommand
		if err != nil {
			res.Detail = err.Error()
		} else {
			res.Detail = combined
		}
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

// run 跑一条子命令，返回 (stdout 首行, stdout+stderr 全文, error)。
//
// ★ 两个返回值都要：版本号取 stdout 首行（stderr 上常有 npm 的告警噪音），
// 而登录态的结论**可能写在 stderr**——实测 `codex login status` 就是
// 把「Logged in using an API key」写到 stderr 的。只读 stdout 会把
// 已登录判成没登录。
func run(ctx context.Context, path string, args, envRemove []string, timeout time.Duration) (string, string, error) {
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
	if len(envRemove) > 0 {
		cmd.Env = cleanEnv(envRemove)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	combined := stdout.String() + stderr.String()
	line, _, _ := strings.Cut(strings.TrimSpace(stdout.String()), "\n")
	if err != nil {
		return "", combined, err
	}
	return line, combined, nil
}

// versionPattern 匹配语义化版本号，允许 `-beta.1` 这类预发布后缀。
var versionPattern = regexp.MustCompile(`\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?`)

// extractVersion 从 --version 的输出里抽出版本号。
//
// ★ 各家格式完全不统一，都是实测的原文：
//
//	claude-agent-acp  →  0.63.0
//	codex-acp         →  @agentclientprotocol/codex-acp 1.1.7
//	codex             →  codex-cli 0.145.0
//	claude            →  2.1.224 (Claude Code)
//
// 不抽的话，界面上会显示成「版本 @agentclientprotocol/codex-acp 1.1.7」——
// 用户看到的是一串包名。
//
// 抽不出来时**原样返回**而不是留空：显示得难看总好过显示不出来，
// 而且那点原文正是排查「这是个什么版本」的唯一线索。
func extractVersion(raw string) string {
	if m := versionPattern.FindString(raw); m != "" {
		return m
	}
	return raw
}

// cleanEnv 返回删掉指定变量后的环境。
func cleanEnv(remove []string) []string {
	drop := make(map[string]bool, len(remove))
	for _, k := range remove {
		drop[k] = true
	}
	env := os.Environ()
	kept := make([]string, 0, len(env))
	for _, kv := range env {
		if name, _, ok := strings.Cut(kv, "="); !ok || !drop[name] {
			kept = append(kept, kv)
		}
	}
	return kept
}
