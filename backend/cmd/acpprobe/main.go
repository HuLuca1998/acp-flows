// Command acpprobe 是 ACP Runtime 的**只读**真机探针。
//
// M0 U0.3.1。它存在的理由：Fake Runtime 要模仿的是**真实行为**，
// 凭 docs/notes/acp-field-notes.md 里标着「待验证」的假设去写 Fake，
// 等于把猜测固化成夹具，所有依赖它的上层测试都会是假的。
//
// 三条硬约束：
//   - **零模型开销**：只做 initialize + session/new，绝不发 session/prompt
//   - **只读**：不写目标目录，`~/.claude` 与 `~/.codex` 一个字节不碰
//   - **可对账**：输出是稳定的 JSON，同一 runtime 连跑两次除时间戳外逐字节相同
//
// 用法：
//
//	go run ./cmd/acpprobe codex
//	go run ./cmd/acpprobe claude --out report.json
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/acp/jsonrpc"
)

// runtimeSpec 描述怎么拉起一个 ACP Runtime。
type runtimeSpec struct {
	name string
	cmd  string
	args []string
	// envRemove 是必须从环境里删掉的变量。
	//
	// ★ 对本项目必踩：Claude Code 会给子进程注入 CLAUDECODE 等标记，
	// 继承下去传给 claude-agent-acp，它会误判自己跑在另一个 agent 内部而**拒绝服务**。
	// 见 docs/notes/acp-field-notes.md §5 坑 1。
	envRemove []string
	npmPkg    string
}

var specs = map[string]runtimeSpec{
	"claude": {
		name:      "claude",
		cmd:       "claude-agent-acp",
		envRemove: []string{"CLAUDECODE", "CLAUDE_CODE_ENTRYPOINT", "CLAUDE_CODE_SSE_PORT"},
		npmPkg:    "@agentclientprotocol/claude-agent-acp",
	},
	"codex": {
		name:      "codex",
		cmd:       "codex-acp",
		envRemove: []string{"CODEX_SANDBOX", "CODEX_SANDBOX_NETWORK_DISABLED"},
		npmPkg:    "@agentclientprotocol/codex-acp",
	},
}

// report 是探针的产出。字段顺序固定，便于 diff。
type report struct {
	Runtime         string          `json:"runtime"`
	Command         string          `json:"command"`
	ProbedAt        string          `json:"probed_at"`
	ProtocolVersion any             `json:"protocol_version"`
	AgentInfo       any             `json:"agent_info,omitempty"`
	Capabilities    any             `json:"agent_capabilities"`
	AuthMethods     any             `json:"auth_methods,omitempty"`
	SessionNew      json.RawMessage `json:"session_new,omitempty"`
	Findings        []string        `json:"findings"`
	Stderr          []string        `json:"stderr,omitempty"`
}

func main() {
	out := flag.String("out", "", "把报告写到文件，默认输出到 stdout")
	timeout := flag.Duration("timeout", 60*time.Second, "整体超时")
	flag.Parse()

	if flag.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "用法: acpprobe <claude|codex> [--out 文件] [--timeout 60s]\n")
		os.Exit(2)
	}
	spec, ok := specs[flag.Arg(0)]
	if !ok {
		fmt.Fprintf(os.Stderr, "未知 runtime %q，可选: claude codex\n", flag.Arg(0))
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	rep, err := probe(ctx, spec)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ 探测 %s 失败: %v\n", spec.name, err)
		if rep != nil && len(rep.Stderr) > 0 {
			// agent 崩溃、认证失败、版本错配的信息**只在 stderr 里**，
			// ACP 消息流里什么都没有。不带出来排查基本靠猜。
			fmt.Fprintf(os.Stderr, "\nruntime stderr:\n")
			for _, l := range rep.Stderr {
				fmt.Fprintf(os.Stderr, "  %s\n", l)
			}
		}
		os.Exit(1)
	}

	body, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "序列化报告失败: %v\n", err)
		os.Exit(1)
	}
	if *out == "" {
		fmt.Println(string(body))
		return
	}
	if err := os.WriteFile(*out, append(body, '\n'), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "写报告失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "✓ 报告已写入 %s\n", *out)
}

func probe(ctx context.Context, spec runtimeSpec) (*report, error) {
	bin, err := exec.LookPath(spec.cmd)
	if err != nil {
		return nil, fmt.Errorf("找不到可执行文件 %s。安装：npm i -g %s", spec.cmd, spec.npmPkg)
	}

	cmd := exec.CommandContext(ctx, bin, spec.args...)
	cmd.Env = cleanEnv(spec.envRemove)
	// ctx 到期时只有直接子进程会被 kill。真实的 ACP Runtime 是 node 启动器
	// 再 fork 出实际进程，孙子进程继承着下面这三个管道活下去，Wait() 就一直
	// 等不到 EOF——`make probe` 表现为"卡住不动"，而不是干脆地超时报错。
	cmd.WaitDelay = time.Second

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	rep := &report{
		Runtime:  spec.name,
		Command:  bin,
		ProbedAt: time.Now().UTC().Format(time.RFC3339),
		Findings: []string{},
	}

	if err := cmd.Start(); err != nil {
		return rep, fmt.Errorf("启动 %s: %w", bin, err)
	}
	defer func() {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	// stderr 必须采集（field-notes §5 坑 5）
	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		buf := make([]byte, 8192)
		for {
			n, rerr := stderrPipe.Read(buf)
			if n > 0 {
				for _, l := range strings.Split(strings.TrimRight(string(buf[:n]), "\n"), "\n") {
					if l != "" {
						rep.Stderr = append(rep.Stderr, l)
					}
				}
			}
			if rerr != nil {
				return
			}
		}
	}()

	conn := jsonrpc.New(stdout, stdin, nil)
	go func() { _ = conn.Serve(ctx) }()

	// ── initialize ───────────────────────────────────────────
	var init map[string]any
	err = conn.CallInto(ctx, "initialize", map[string]any{
		"protocolVersion": 1,
		"clientCapabilities": map[string]any{
			"fs": map[string]bool{"readTextFile": false, "writeTextFile": false},
		},
		"clientInfo": map[string]string{"name": "acpprobe", "version": "0.1.0"},
	}, &init)
	if err != nil {
		return rep, fmt.Errorf("initialize: %w", err)
	}

	rep.ProtocolVersion = init["protocolVersion"]
	rep.Capabilities = init["agentCapabilities"]
	rep.AgentInfo = init["agentInfo"]
	rep.AuthMethods = init["authMethods"]

	if pv, ok := init["protocolVersion"].(float64); ok && pv != 1 {
		rep.Findings = append(rep.Findings,
			fmt.Sprintf("⚠ protocolVersion = %v，不是笔记里记录的 1", pv))
	}

	// ── session/new ──────────────────────────────────────────
	// cwd 必须是**已存在的绝对路径**。用临时目录，绝不碰用户的仓库。
	cwd, err := os.MkdirTemp("", "acpprobe")
	if err != nil {
		return rep, fmt.Errorf("建临时 cwd: %w", err)
	}
	defer func() { _ = os.RemoveAll(cwd) }()
	abs, _ := filepath.Abs(cwd)

	var sess json.RawMessage
	sess, err = conn.Call(ctx, "session/new", map[string]any{
		"cwd":        abs,
		"mcpServers": []any{}, // codex 必须传空数组，非空会覆盖 thread config 的 mcp_servers
	})
	if err != nil {
		var rpcErr *jsonrpc.Error
		if errors.As(err, &rpcErr) && rpcErr.Code == jsonrpc.CodeAuthRequired {
			return rep, fmt.Errorf("需要先登录：%s login（或设 CODEX_API_KEY）", spec.name)
		}
		return rep, fmt.Errorf("session/new: %w", err)
	}
	rep.SessionNew = redactSessionID(sess)
	rep.Findings = append(rep.Findings, analyze(spec.name, sess)...)

	return rep, nil
}

// analyze 对着 docs/notes/acp-field-notes.md 的待验证项逐条核对。
func analyze(runtime string, sess json.RawMessage) []string {
	var s struct {
		Modes struct {
			AvailableModes []struct {
				ID string `json:"id"`
			} `json:"availableModes"`
			CurrentModeID string `json:"currentModeId"`
		} `json:"modes"`
		Models        *json.RawMessage `json:"models"`
		ConfigOptions []struct {
			ID       string `json:"id"`
			Category string `json:"category"`
		} `json:"configOptions"`
	}
	if err := json.Unmarshal(sess, &s); err != nil {
		return []string{"⚠ session/new 响应结构与预期不符，无法自动核对：" + err.Error()}
	}

	var out []string

	// 待验证 R2：档位取值与默认档
	ids := make([]string, 0, len(s.Modes.AvailableModes))
	for _, m := range s.Modes.AvailableModes {
		ids = append(ids, m.ID)
	}
	out = append(out, fmt.Sprintf("modes = [%s]，默认 = %q",
		strings.Join(ids, " "), s.Modes.CurrentModeID))

	// 待验证 R4：claude 的 session/new 顶层是否**没有** models
	if runtime == "claude" && s.Models != nil {
		out = append(out, "⚠ claude 的 session/new 顶层出现了 models —— 与笔记记录的『没有』不符")
	}
	if runtime == "codex" && s.Models == nil {
		out = append(out, "⚠ codex 的 session/new 顶层没有 models —— 与笔记记录的『有 25 个』不符")
	}

	// 待验证 R1：configOptions 是否有 category，推理强度是否为 thought_level
	hasCategory := false
	for _, o := range s.ConfigOptions {
		if o.Category != "" {
			hasCategory = true
		}
		if o.Category == "thought_level" {
			out = append(out, fmt.Sprintf("✓ 推理强度的 id=%q，category=thought_level（按 category 取可跨端）", o.ID))
		}
	}
	if len(s.ConfigOptions) > 0 && !hasCategory {
		out = append(out, "⚠ configOptions 全部没有 category —— 『按 category 取而不按 id 取』这条不成立了")
	}
	for _, o := range s.ConfigOptions {
		out = append(out, fmt.Sprintf("  configOption id=%q category=%q", o.ID, o.Category))
	}

	return out
}

// redactSessionID 把 sessionId 换成占位符，让同一 runtime 两次运行的报告可 diff。
func redactSessionID(raw json.RawMessage) json.RawMessage {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return raw
	}
	if _, ok := m["sessionId"]; ok {
		m["sessionId"] = "<redacted>"
	}
	out, err := json.Marshal(m)
	if err != nil {
		return raw
	}
	return out
}

// cleanEnv 返回删掉嵌套会话标记后的环境。
func cleanEnv(remove []string) []string {
	drop := make(map[string]bool, len(remove))
	for _, k := range remove {
		drop[k] = true
	}
	out := make([]string, 0, len(os.Environ()))
	for _, kv := range os.Environ() {
		if k, _, ok := strings.Cut(kv, "="); ok && drop[k] {
			continue
		}
		out = append(out, kv)
	}
	return out
}
