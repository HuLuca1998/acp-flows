package runtime_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/acp/runtime"
)

// 这些测试**不 mock exec**。检测的全部内容就是「去 PATH 上找一个可执行文件
// 并跑它」——把 exec 换成接口再塞个假实现，测的就只剩「假实现会返回我塞进去的值」。
//
// 所以这里造的是真文件：真的 shell 脚本、真的执行位、真的 PATH、真的退出码。
// 唯一被替换的是 PATH 指向哪个目录，而那正是这个功能在生产里唯一的输入。

// fakeBin 在 dir 下造一个真的可执行脚本。
//
// body 里可以用 $ARGLOG 追加记录自己收到的参数——用来证明检测没干别的事（R3）。
func fakeBin(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$ARGLOG\"\n" + body
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("造假 %s 失败: %v", name, err)
	}
}

// binDir 返回一个只有本测试可见的 PATH 目录，并把 PATH 指过去。
//
// ★ 必须把 PATH **整个替换**而不是追加：追加的话，开发机上真装了 codex
// 的人跑这个测试会命中真的 codex——测试结果取决于谁的机器，那就不是测试了。
//
// 代价是脚本里不能再靠 PATH 找 `sleep` 这类外部命令，得写 `/bin/sleep`。
func binDir(t *testing.T) (dir, arglog string) {
	t.Helper()
	dir = t.TempDir()
	arglog = filepath.Join(dir, "argv.log")
	t.Setenv("PATH", dir)
	t.Setenv("ARGLOG", arglog)
	return dir, arglog
}

// codexSpec 是测试用的探测规格。字段值与生产注册表无关——
// 这里测的是 Detect 的行为，不是某个具体 runtime 的配置。
func codexSpec() runtime.Spec {
	return runtime.Spec{
		Name:           "codex",
		Bin:            "codex",
		VersionArgs:    []string{"--version"},
		AuthArgs:       []string{"login", "status"},
		InstallCommand: "npm i -g @openai/codex",
		LoginCommand:   "codex login",
	}
}

func TestDetect(t *testing.T) {
	tests := []struct {
		name string
		// setup 造出这次要检测的环境；返回空字符串表示不造任何可执行文件
		setup func(t *testing.T, dir string)
		// spec 微调这次用的规格；nil 表示用 codexSpec() 原样
		spec       func(runtime.Spec) runtime.Spec
		timeout    time.Duration
		wantStatus runtime.Status
		wantRemedy string
		// wantVersion 为空表示不检查
		wantVersion string
	}{
		{
			name: "装好且已登录 → ready，不给修复命令",
			setup: func(t *testing.T, dir string) {
				// 两条子命令都成功；--version 打印版本号
				fakeBin(t, dir, "codex", `
case "$1" in
  --version) echo "codex-cli 0.63.0" ;;
  login)     exit 0 ;;
esac
`)
			},
			wantStatus:  runtime.StatusReady,
			wantRemedy:  "",
			wantVersion: "0.63.0", // 从 "codex-cli 0.63.0" 里抽出来的
		},
		{
			name:       "PATH 上根本没有 → not_installed，给安装命令",
			setup:      func(*testing.T, string) {}, // 空目录
			wantStatus: runtime.StatusNotInstalled,
			wantRemedy: "npm i -g @openai/codex",
		},
		{
			name: "装了但没登录 → not_authenticated，给的是登录命令",
			setup: func(t *testing.T, dir string) {
				fakeBin(t, dir, "codex", `
case "$1" in
  --version) echo "codex-cli 0.63.0" ;;
  login)     echo "Not logged in" >&2; exit 1 ;;
esac
`)
			},
			wantStatus: runtime.StatusNotAuthenticated,
			// R2：提示里必须有用户能直接敲的命令，不是「请检查配置」
			wantRemedy: "codex login",
		},
		{
			name: "探测挂住 → probe_failed，而不是谎称没装",
			setup: func(t *testing.T, dir string) {
				// 绝对路径：PATH 已被清空，`sleep` 是找不到的。
				// 写成 `sleep 30` 的话脚本会立刻以「命令不存在」失败，
				// 这条用例就会因为**错误的原因**变绿，再也测不到超时。
				fakeBin(t, dir, "codex", `/bin/sleep 30`)
			},
			// ★ 别把这个超时设得太小。macOS 首次执行一个新建的可执行文件要过
			// 代码签名检查，实测能花掉 100ms 以上——超时设成 80ms 的话，
			// **所有**用例都会 probe_failed，包括本该 ready 的那些。
			timeout:    time.Second,
			wantStatus: runtime.StatusProbeFailed,
			// 卡住时给不出确定结论，也就给不出确定的修复命令
			wantRemedy: "",
		},
		{
			// ★ 真实的坑：`claude auth status` **未登录时也返回 exit 0**，
			// 结论藏在 JSON 的 loggedIn 字段里。只看退出码就会把没登录的
			// claude 报成 ready，用户要到真正提需求时才撞墙。
			name: "auth 命令退出码是 0，但输出说没登录 → not_authenticated",
			setup: func(t *testing.T, dir string) {
				fakeBin(t, dir, "codex", `
case "$1" in
  --version) echo "codex-cli 0.63.0" ;;
  login)     echo '{"loggedIn": false}'; exit 0 ;;
esac
`)
			},
			spec: func(s runtime.Spec) runtime.Spec {
				s.AuthOKSubstring = `"loggedIn": true`
				return s
			},
			wantStatus: runtime.StatusNotAuthenticated,
			wantRemedy: "codex login",
		},
		{
			// codex 把登录结论写在 **stderr**（实测：`Logged in using an API key`）。
			// 只读 stdout 的话这条子串永远匹配不上，已登录会被报成没登录。
			name: "登录结论写在 stderr 也要认",
			setup: func(t *testing.T, dir string) {
				fakeBin(t, dir, "codex", `
case "$1" in
  --version) echo "codex-cli 0.63.0" ;;
  login)     echo "Logged in using an API key" >&2; exit 0 ;;
esac
`)
			},
			spec: func(s runtime.Spec) runtime.Spec {
				s.AuthOKSubstring = "Logged in"
				return s
			},
			wantStatus: runtime.StatusReady,
		},
		{
			// ★ acp-field-notes §5 坑 1：Claude Code 给子进程注入 CLAUDECODE
			// 等标记，claude-agent-acp 继承到之后会误判自己跑在另一个 agent
			// 内部而**拒绝服务**。开发时 duetd 正是由 Claude Code 拉起的。
			name: "EnvRemove 里的变量必须对子进程不可见",
			setup: func(t *testing.T, dir string) {
				t.Setenv("CLAUDECODE", "1")
				fakeBin(t, dir, "codex", `
case "$1" in
  --version)
    if [ -n "$CLAUDECODE" ]; then echo "refusing to run inside an agent" >&2; exit 1; fi
    echo "codex-cli 0.63.0" ;;
  login) exit 0 ;;
esac
`)
			},
			spec: func(s runtime.Spec) runtime.Spec {
				s.EnvRemove = []string{"CLAUDECODE"}
				return s
			},
			wantStatus:  runtime.StatusReady,
			wantVersion: "0.63.0", // 从 "codex-cli 0.63.0" 里抽出来的
		},
		{
			// ★ 适配器与 CLI 是两个可执行文件：能不能跑看 claude-agent-acp，
			// 登没登录归 claude 管。AuthBin 就是为这件事存在的。
			name: "登录态查的是 AuthBin，不是 Bin",
			setup: func(t *testing.T, dir string) {
				fakeBin(t, dir, "codex", `
case "$1" in
  --version) echo "codex-cli 0.63.0" ;;
  # 适配器自己没有 login 子命令：真被拿去查登录态就会走到这里
  *) echo "unknown command" >&2; exit 127 ;;
esac
`)
				fakeBin(t, dir, "codex-cli", `echo "Logged in"; exit 0`)
			},
			spec: func(s runtime.Spec) runtime.Spec {
				s.AuthBin = "codex-cli"
				s.AuthOKSubstring = "Logged in"
				return s
			},
			wantStatus: runtime.StatusReady,
		},
		{
			// 装了适配器却没装 CLI：跑得起来但一定没登录。
			// 保守判成 not_authenticated 并给出登录命令，而不是 ready。
			name: "AuthBin 没装 → not_authenticated",
			setup: func(t *testing.T, dir string) {
				fakeBin(t, dir, "codex", `
case "$1" in
  --version) echo "codex-cli 0.63.0" ;;
esac
`)
			},
			spec: func(s runtime.Spec) runtime.Spec {
				s.AuthBin = "codex-cli" // 不造这个文件
				return s
			},
			wantStatus: runtime.StatusNotAuthenticated,
			wantRemedy: "codex login",
		},
		{
			name: "文件在但不可执行 → probe_failed",
			setup: func(t *testing.T, dir string) {
				// 造一个没有执行位的同名文件：装了一半 / 权限被改坏的真实情形
				p := filepath.Join(dir, "codex")
				if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			// LookPath 会因为没有执行位而找不到它，等同于没装——
			// 这条断言锁住的是「不许把它当成 ready」
			wantStatus: runtime.StatusNotInstalled,
			wantRemedy: "npm i -g @openai/codex",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, _ := binDir(t)
			tt.setup(t, dir)

			timeout := tt.timeout
			if timeout == 0 {
				timeout = 5 * time.Second
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			spec := codexSpec()
			if tt.spec != nil {
				spec = tt.spec(spec)
			}
			got := runtime.Detect(ctx, spec, timeout)

			if got.Status != tt.wantStatus {
				t.Errorf("Status = %q, 想要 %q（detail: %s）", got.Status, tt.wantStatus, got.Detail)
			}
			if got.Remedy != tt.wantRemedy {
				t.Errorf("Remedy = %q, 想要 %q", got.Remedy, tt.wantRemedy)
			}
			if tt.wantVersion != "" && got.Version != tt.wantVersion {
				t.Errorf("Version = %q, 想要 %q", got.Version, tt.wantVersion)
			}
			if got.Name != "codex" {
				t.Errorf("Name = %q, 想要 codex", got.Name)
			}
		})
	}
}

// 版本号要从各式各样的 --version 输出里抽出来。
//
// ★ 四种格式都是真机实测的原文。不抽的话界面上会显示成
// 「版本 @agentclientprotocol/codex-acp 1.1.7」——用户看到的是一串包名。
func TestDetectExtractsVersionNumber(t *testing.T) {
	tests := []struct {
		raw  string // --version 的真实输出
		want string
	}{
		{"0.63.0", "0.63.0"},                              // claude-agent-acp
		{"@agentclientprotocol/codex-acp 1.1.7", "1.1.7"}, // codex-acp
		{"codex-cli 0.145.0", "0.145.0"},                  // codex
		{"2.1.224 (Claude Code)", "2.1.224"},              // claude
		{"v1.2.3-beta.1", "1.2.3-beta.1"},                 // 带前缀与预发布号
		{"没有版本号的一行字", "没有版本号的一行字"},                        // 抽不出就原样保留，不弄丢信息
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			dir, _ := binDir(t)
			// raw 里没有单引号，用单引号包住最省事
			fakeBin(t, dir, "codex", "case \"$1\" in\n"+
				"  --version) echo '"+tt.raw+"' ;;\n"+
				"  login)     exit 0 ;;\n"+
				"esac\n")
			got := runtime.Detect(context.Background(), codexSpec(), 5*time.Second)
			if got.Version != tt.want {
				t.Errorf("Version = %q, 想要 %q", got.Version, tt.want)
			}
		})
	}
}

// R3：检测**零模型开销**。
//
// 断言方式是让假 codex 把每次收到的 argv 记下来，然后逐条核对：
// 只允许出现 spec 里声明的那两组参数。真要有人往检测里塞一次 prompt
// 换取"顺便验证一下能不能用"，这里会立刻红。
func TestDetectNeverPrompts(t *testing.T) {
	dir, arglog := binDir(t)
	fakeBin(t, dir, "codex", `
case "$1" in
  --version) echo "codex-cli 0.63.0" ;;
  login)     exit 0 ;;
esac
`)

	got := runtime.Detect(context.Background(), codexSpec(), 5*time.Second)
	if got.Status != runtime.StatusReady {
		t.Fatalf("前置条件没成立：Status = %q", got.Status)
	}

	raw, err := os.ReadFile(arglog)
	if err != nil {
		t.Fatalf("假 codex 没记下任何调用: %v", err)
	}
	var calls []string
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if line != "" {
			calls = append(calls, line)
		}
	}

	allowed := map[string]bool{"--version": true, "login status": true}
	for _, c := range calls {
		if !allowed[c] {
			t.Errorf("检测过程中执行了未声明的调用 %q——"+
				"检测只许问版本和登录态，任何多余的子命令都可能产生费用", c)
		}
	}
	if len(calls) != 2 {
		t.Errorf("调用了 %d 次（%v），想要恰好 2 次——"+
			"重复探测会让打开设置页变慢", len(calls), calls)
	}
}

// R4：一个 runtime 检测不出来，不许影响另一个。
//
// DetectAll 必须每个都返回结论。挂住的那个是 probe_failed，
// 正常的那个仍然是 ready——而不是整批一起失败。
func TestDetectAllIsolatesFailures(t *testing.T) {
	dir, _ := binDir(t)
	fakeBin(t, dir, "codex", `
case "$1" in
  --version) echo "codex-cli 0.63.0" ;;
  login)     exit 0 ;;
esac
`)
	fakeBin(t, dir, "claude", `/bin/sleep 30`)

	specs := []runtime.Spec{
		codexSpec(),
		{
			Name: "claude", Bin: "claude",
			VersionArgs: []string{"--version"}, AuthArgs: []string{"auth", "status"},
			InstallCommand: "npm i -g @anthropic-ai/claude-code",
			LoginCommand:   "claude login",
		},
	}

	const timeout = 1500 * time.Millisecond
	start := time.Now()
	results := runtime.DetectAll(context.Background(), specs, timeout)
	elapsed := time.Since(start)

	if len(results) != 2 {
		t.Fatalf("返回 %d 条结果，想要 2 条——每个 runtime 都必须有结论", len(results))
	}

	byName := map[string]runtime.Result{}
	for _, r := range results {
		byName[r.Name] = r
	}
	if got := byName["codex"].Status; got != runtime.StatusReady {
		t.Errorf("codex = %q, 想要 ready——另一个 runtime 卡住不该连累它", got)
	}
	if got := byName["claude"].Status; got != runtime.StatusProbeFailed {
		t.Errorf("claude = %q, 想要 probe_failed", got)
	}

	// 并发执行：串行的话 codex 要排在卡死的 claude 后面，总耗时会明显超过
	// 一个超时。留一倍余量，避免 CI 机器慢就误报。
	if elapsed > 2*timeout {
		t.Errorf("耗时 %v——检测必须并发，否则装了 5 个 runtime 就要等 5 倍超时", elapsed)
	}
}

// 幂等：连查两次结论一致。设置页会反复打开，每次结果不一样是不能接受的。
func TestDetectIsIdempotent(t *testing.T) {
	dir, _ := binDir(t)
	fakeBin(t, dir, "codex", `
case "$1" in
  --version) echo "codex-cli 0.63.0" ;;
  login)     exit 0 ;;
esac
`)

	first := runtime.Detect(context.Background(), codexSpec(), 5*time.Second)
	second := runtime.Detect(context.Background(), codexSpec(), 5*time.Second)

	if first != second {
		t.Errorf("两次结果不同：\n  1: %+v\n  2: %+v", first, second)
	}
}
