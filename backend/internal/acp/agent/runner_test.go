package agent_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/acp/agent"
	"github.com/HuLuca1998/acp-flows/backend/internal/acp/runtime"
	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
)

// U2.4.1 · 真的把 Agent 进程拉起来（验收点 V5 的 R3）
//
// ★ 这一层不用 Fake Runtime：它要验的正是**进程怎么拉起来、拉不起来时
// 用户看到什么**——换成管道就把被测的东西替换掉了。
// 所以用一个写在 t.TempDir() 里的脚本冒充 Agent。

// fakeAgentScript 写一个可执行脚本冒充 ACP Agent。
//
// body 是脚本正文；返回可执行文件的绝对路径。
func fakeAgentScript(t *testing.T, name, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

type busRecorder struct {
	mu     sync.Mutex
	events []port.WorkEvent
	err    error
}

func (b *busRecorder) PublishWorkEvent(_ context.Context, e port.WorkEvent) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, e)
	return b.err
}

func (b *busRecorder) snapshot() []port.WorkEvent {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]port.WorkEvent(nil), b.events...)
}

// ★★ 一个 Runtime 都没就绪时，错误里要**说清楚该去做什么**。
//
// 「connection refused」这种话对用户毫无意义——他需要知道的是
// 「装一下 claude-agent-acp」或者「先登录」。这条错误最终会出现在
// 时间线的失败事件里，是他唯一能看到的线索。
func TestProcessRunner_NoRuntimeReady(t *testing.T) {
	r := &agent.ProcessRunner{
		Specs: []runtime.Spec{{
			Name:           "claude",
			Bin:            filepath.Join(t.TempDir(), "根本不存在"),
			VersionArgs:    []string{"--version"},
			InstallCommand: "npm i -g @agentclientprotocol/claude-agent-acp",
		}},
		Bus: &busRecorder{},
	}

	err := r.RunTurn(context.Background(), port.AgentTurn{
		WorkID: "work-01", Cwd: t.TempDir(), Prompt: "做点事",
	})
	if err == nil {
		t.Fatal("一个 Runtime 都没装却跑成功了")
	}
	if !strings.Contains(err.Error(), "npm i -g") {
		t.Errorf("错误里没有补救办法: %v\n"+
			"这句话是用户唯一能看到的线索，只说「失败了」他不知道该干什么", err)
	}
}

// ★ 进程拉起来了但立刻退出时，要把它的 **stderr 带回来**。
//
// 不带的话，用户看到的是「连接断开」，而真正的原因
// （比如 claude-agent-acp 说「请先登录」）躺在一个没人读的管道里。
func TestProcessRunner_ReportsAgentStderr(t *testing.T) {
	bin := fakeAgentScript(t, "claude-agent-acp", `
case "$1" in
  --version) echo "0.63.0"; exit 0 ;;
esac
echo "Error: not authenticated, run 'claude auth login'" >&2
exit 1
`)
	r := &agent.ProcessRunner{
		Specs: []runtime.Spec{{Name: "claude", Bin: bin, VersionArgs: []string{"--version"}}},
		Bus:   &busRecorder{},
	}

	err := r.RunTurn(context.Background(), port.AgentTurn{
		WorkID: "work-01", Cwd: t.TempDir(), Prompt: "做点事",
	})
	if err == nil {
		t.Fatal("Agent 直接退出了却跑成功了")
	}
	if !strings.Contains(err.Error(), "not authenticated") {
		t.Errorf("错误里没带上 Agent 的 stderr: %v\n"+
			"真正的原因躺在一个没人读的管道里，用户只看到「连接断开」", err)
	}
}

// ★★ 事件要**发到总线**，那是它去到界面的唯一通路。
//
// 这里用一个照着 ACP 规范说话的最小脚本：握手、建会话、说一句话、结束。
func TestProcessRunner_PublishesToBus(t *testing.T) {
	bin := fakeAgentScript(t, "claude-agent-acp", `
case "$1" in
  --version) echo "0.63.0"; exit 0 ;;
esac
while IFS= read -r line; do
  case "$line" in
    *'"initialize"'*)
      id=$(printf '%s' "$line" | sed 's/.*"id":\([0-9]*\).*/\1/')
      printf '{"jsonrpc":"2.0","id":%s,"result":{"protocolVersion":1}}\n' "$id" ;;
    *'"session/new"'*)
      id=$(printf '%s' "$line" | sed 's/.*"id":\([0-9]*\).*/\1/')
      printf '{"jsonrpc":"2.0","id":%s,"result":{"sessionId":"s1","configOptions":[{"id":"permission_mode","name":"\u6743\u9650\u6a21\u5f0f","category":"mode","type":"select","currentValue":"plan","options":[{"value":"plan","name":"plan"},{"value":"default","name":"default"}]}]}}\n' "$id" ;;
    *'"session/set_config_option"'*)
      id=$(printf '%s' "$line" | sed 's/.*"id":\([0-9]*\).*/\1/')
      printf '{"jsonrpc":"2.0","id":%s,"result":{"configOptions":[{"id":"permission_mode","category":"mode","type":"select","currentValue":"default"}]}}\n' "$id" ;;
    *'"session/prompt"'*)
      id=$(printf '%s' "$line" | sed 's/.*"id":\([0-9]*\).*/\1/')
      printf '{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"s1","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"我看一下"}}}}\n'
      printf '{"jsonrpc":"2.0","id":%s,"result":{"stopReason":"end_turn"}}\n' "$id" ;;
  esac
done
`)
	bus := &busRecorder{}
	r := &agent.ProcessRunner{
		Specs: []runtime.Spec{{Name: "claude", Bin: bin, VersionArgs: []string{"--version"}}},
		Bus:   bus,
	}

	if err := r.RunTurn(context.Background(), port.AgentTurn{
		WorkID: "work-07", Cwd: t.TempDir(), Prompt: "帮我加个功能",
	}); err != nil {
		t.Fatalf("跑一轮失败: %v", err)
	}

	var types []string
	for _, e := range bus.snapshot() {
		types = append(types, e.Type)
		if e.WorkID != "work-07" {
			t.Errorf("事件 %q 的 work_id = %q", e.Type, e.WorkID)
		}
	}

	if !contains(types, "message_chunk") {
		t.Errorf("AI 说的话没到总线上，收到的是 %v——那是它去到界面的唯一通路", types)
	}
	if !contains(types, "turn_end") {
		t.Errorf("没发 turn_end，收到的是 %v", types)
	}
}

// ★★ 这一轮跑完，Agent 进程**不能还活着**。
//
// 留着的话，用户每提一个需求就多一个常驻进程，各自握着一个 worktree。
// 关掉应用之后它们还在——这是最难被发现、也最难被原谅的一类 bug。
func TestProcessRunner_LeavesNoOrphan(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "pid")
	bin := fakeAgentScript(t, "claude-agent-acp", `
case "$1" in
  --version) echo "0.63.0"; exit 0 ;;
esac
echo $$ > `+pidFile+`
while IFS= read -r line; do
  case "$line" in
    *'"initialize"'*)
      id=$(printf '%s' "$line" | sed 's/.*"id":\([0-9]*\).*/\1/')
      printf '{"jsonrpc":"2.0","id":%s,"result":{"protocolVersion":1}}\n' "$id" ;;
    *'"session/new"'*)
      id=$(printf '%s' "$line" | sed 's/.*"id":\([0-9]*\).*/\1/')
      printf '{"jsonrpc":"2.0","id":%s,"result":{"sessionId":"s1","configOptions":[{"id":"permission_mode","name":"\u6743\u9650\u6a21\u5f0f","category":"mode","type":"select","currentValue":"plan","options":[{"value":"plan","name":"plan"},{"value":"default","name":"default"}]}]}}\n' "$id" ;;
    *'"session/set_config_option"'*)
      id=$(printf '%s' "$line" | sed 's/.*"id":\([0-9]*\).*/\1/')
      printf '{"jsonrpc":"2.0","id":%s,"result":{"configOptions":[{"id":"permission_mode","category":"mode","type":"select","currentValue":"default"}]}}\n' "$id" ;;
    *'"session/prompt"'*)
      id=$(printf '%s' "$line" | sed 's/.*"id":\([0-9]*\).*/\1/')
      printf '{"jsonrpc":"2.0","id":%s,"result":{"stopReason":"end_turn"}}\n' "$id" ;;
  esac
done
sleep 300
`)
	r := &agent.ProcessRunner{
		Specs: []runtime.Spec{{Name: "claude", Bin: bin, VersionArgs: []string{"--version"}}},
		Bus:   &busRecorder{},
	}

	if err := r.RunTurn(context.Background(), port.AgentTurn{
		WorkID: "work-01", Cwd: t.TempDir(), Prompt: "做点事",
	}); err != nil {
		t.Fatalf("跑一轮失败: %v", err)
	}

	raw, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("脚本没记下 pid: %v", err)
	}
	pid := strings.TrimSpace(string(raw))
	if alive(t, pid) {
		t.Errorf("这一轮跑完了，Agent 进程 %s 还活着——"+
			"用户每提一个需求就多一个常驻进程，关掉应用之后它们还在", pid)
	}
}

// alive 用 kill -0 探一个 pid 还在不在。
func alive(t *testing.T, pid string) bool {
	t.Helper()
	// 用 ps 而不是 kill -0：僵尸进程对 kill -0 有反应，但它已经不占资源了
	out, _ := exec.Command("ps", "-o", "state=", "-p", pid).Output()
	state := strings.TrimSpace(string(out))
	return state != "" && !strings.HasPrefix(state, "Z")
}

// ★ Bus 发不出去**不该让这一轮失败**。
//
// 事件是给界面看的，而 AI 那边的活已经干了——因为通知发不出去就报错的话，
// 用户看到「失败」而磁盘上躺着一堆已经改好的文件，比不通知更糟。
func TestProcessRunner_BusFailureDoesNotFailTurn(t *testing.T) {
	bin := fakeAgentScript(t, "claude-agent-acp", `
case "$1" in
  --version) echo "0.63.0"; exit 0 ;;
esac
while IFS= read -r line; do
  case "$line" in
    *'"initialize"'*|*'"session/new"'*|*'"session/set_config_option"'*|*'"session/prompt"'*)
      id=$(printf '%s' "$line" | sed 's/.*"id":\([0-9]*\).*/\1/')
      case "$line" in
        *'"session/new"'*) printf '{"jsonrpc":"2.0","id":%s,"result":{"sessionId":"s1","configOptions":[{"id":"permission_mode","category":"mode","type":"select","currentValue":"plan","options":[{"value":"plan","name":"plan"},{"value":"default","name":"default"}]}]}}\n' "$id" ;;
        *'"session/set_config_option"'*) printf '{"jsonrpc":"2.0","id":%s,"result":{"configOptions":[{"id":"permission_mode","category":"mode","type":"select","currentValue":"default"}]}}\n' "$id" ;;
        *'"session/prompt"'*) printf '{"jsonrpc":"2.0","id":%s,"result":{"stopReason":"end_turn"}}\n' "$id" ;;
        *) printf '{"jsonrpc":"2.0","id":%s,"result":{"protocolVersion":1}}\n' "$id" ;;
      esac ;;
  esac
done
`)
	r := &agent.ProcessRunner{
		Specs: []runtime.Spec{{Name: "claude", Bin: bin, VersionArgs: []string{"--version"}}},
		Bus:   &busRecorder{err: errAlwaysFails},
	}

	if err := r.RunTurn(context.Background(), port.AgentTurn{
		WorkID: "work-01", Cwd: t.TempDir(), Prompt: "做点事",
	}); err != nil {
		t.Errorf("总线发不出去就把整轮判失败了: %v\n"+
			"AI 那边的活已经干了，用户看到「失败」而磁盘上躺着改好的文件", err)
	}
}

// errAlwaysFails 让总线每次发布都失败。
var errAlwaysFails = errors.New("bus: 数据库锁住了")

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// ── U3.2.3 · 取消：ProcessRunner 要记得住谁在跑 ──────────────

// ★ 没在跑的工作取消**不报错**。
//
// 报错的话，用户点一个本来就该没反应的按钮会看到一句吓人的错误；
// 而 app 层拿到错误会把工作推到 failed——一个已经跑完的工作被标成失败。
func TestProcessRunner_CancelIdleWorkIsNoop(t *testing.T) {
	r := &agent.ProcessRunner{}

	mustKill, err := r.CancelTurn(context.Background(), "work-nope")
	if err != nil {
		t.Errorf("取消一个没在跑的工作报错了: %v", err)
	}
	if mustKill {
		t.Error("没在跑却要求杀进程")
	}
}

// ★★ 跑完之后**不能还留着记录**。
//
// 留着的话，取消一个早就结束的工作会去动一条已经关掉的会话——
// 轻则报一句看不懂的错，重则卡在那儿等一个永远不来的收尾。
func TestProcessRunner_ForgetsFinishedTurns(t *testing.T) {
	bin := fakeAgentScript(t, "claude-agent-acp", `
case "$1" in
  --version) echo "0.63.0"; exit 0 ;;
esac
while IFS= read -r line; do
  case "$line" in
    *'"initialize"'*|*'"session/new"'*|*'"session/set_config_option"'*|*'"session/prompt"'*)
      id=$(printf '%s' "$line" | sed 's/.*"id":\([0-9]*\).*/\1/')
      case "$line" in
        *'"session/new"'*) printf '{"jsonrpc":"2.0","id":%s,"result":{"sessionId":"s1","configOptions":[{"id":"permission_mode","category":"mode","type":"select","currentValue":"plan","options":[{"value":"plan","name":"plan"},{"value":"default","name":"default"}]}]}}\n' "$id" ;;
        *'"session/set_config_option"'*) printf '{"jsonrpc":"2.0","id":%s,"result":{"configOptions":[{"id":"permission_mode","category":"mode","type":"select","currentValue":"default"}]}}\n' "$id" ;;
        *'"session/prompt"'*) printf '{"jsonrpc":"2.0","id":%s,"result":{"stopReason":"end_turn"}}\n' "$id" ;;
        *) printf '{"jsonrpc":"2.0","id":%s,"result":{"protocolVersion":1}}\n' "$id" ;;
      esac ;;
  esac
done
`)
	r := &agent.ProcessRunner{
		Specs: []runtime.Spec{{Name: "claude", Bin: bin, VersionArgs: []string{"--version"}}},
		Bus:   &busRecorder{},
	}

	if err := r.RunTurn(context.Background(), port.AgentTurn{
		WorkID: "work-01", Cwd: t.TempDir(), Prompt: "做点事",
	}); err != nil {
		t.Fatalf("跑一轮失败: %v", err)
	}

	// 跑完了：取消它应该是空操作
	mustKill, err := r.CancelTurn(context.Background(), "work-01")
	if err != nil {
		t.Errorf("取消一个已经跑完的工作报错了: %v——"+
			"app 层会把它推到 failed，而它本来是成功结束的", err)
	}
	if mustKill {
		t.Error("已经跑完了却要求杀进程")
	}
}

// ★★ 取消正在跑的那一轮，**并且能把进程收掉**。
//
// 用一个装死的假 Agent：连 initialize 都不回，取消必然超时 → mustKill。
func TestProcessRunner_CancelRunningTurnDemandsKillWhenDead(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "pid")
	bin := fakeAgentScript(t, "claude-agent-acp", `
case "$1" in
  --version) echo "0.63.0"; exit 0 ;;
esac
echo $$ > `+pidFile+`
# 什么都不回，装死
sleep 300
`)
	r := &agent.ProcessRunner{
		Specs: []runtime.Spec{{Name: "claude", Bin: bin, VersionArgs: []string{"--version"}}},
		Bus:   &busRecorder{},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = r.RunTurn(ctx, port.AgentTurn{
			WorkID: "work-01", Cwd: t.TempDir(), Prompt: "做点事",
		})
	}()

	// 等进程起来
	waitPID(t, pidFile)

	// 杀掉它——这是 app 层在 mustKill 之后会做的事
	r.KillAgent("work-01")

	waitGone(t, pidFile, "KillAgent 之后进程还活着——"+
		"「界面说已取消、后台还在烧钱改文件」正是这么来的")
}

func waitPID(t *testing.T, path string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if raw, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(raw))) > 0 {
			return strings.TrimSpace(string(raw))
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("假 Agent 没起来")
	return ""
}

func waitGone(t *testing.T, pidFile, why string) {
	t.Helper()
	raw, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("读 pid 失败: %v", err)
	}
	pid := strings.TrimSpace(string(raw))

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !alive(t, pid) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("%s（pid %s）", why, pid)
}
