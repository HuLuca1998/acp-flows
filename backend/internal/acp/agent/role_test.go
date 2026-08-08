package agent_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/acp/agent"
	"github.com/HuLuca1998/acp-flows/backend/internal/acp/runtime"
	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
)

// U2.1.2 的**最后一公里**：收权代码接进执行链路了吗。
//
// ★★ 为什么单独写这一族：`session.applyMode` 有六条测试守着，全绿——
// 而在接上之前，**没有任何调用方传 RequiredModeID**，那段代码一次都没跑过。
//
// 这和实测里那个失败模式一模一样：客户端写了一份「一律拒绝」的权限代码，
// 看起来很严，而 codex 默认档下它一次都不会被调用到
// （acp-field-notes.md §3：权限请求 0 次、文件照建）。
//
// **一段没人调用的安全代码，和没写是一样的。**

// recordingAgent 造一个把收到的每一帧原样写进文件的假 Agent。
//
// ★ 判据是**线上真的发出了什么**，不是「我们调用了那个函数」。
func recordingAgent(t *testing.T, name, logPath string, supportsMode bool) string {
	t.Helper()

	newSessionResult := `{"sessionId":"s1"}`
	if supportsMode {
		newSessionResult = `{"sessionId":"s1","configOptions":[{"id":"permission_mode","category":"mode","type":"select","currentValue":"plan","options":[{"value":"plan","name":"plan"},{"value":"default","name":"default"},{"value":"bypassPermissions","name":"bypassPermissions"}]}]}`
	}

	body := `
case "$1" in
  --version) echo "0.63.0"; exit 0 ;;
esac
while IFS= read -r line; do
  printf '%s\n' "$line" >> "` + logPath + `"
  id=$(printf '%s' "$line" | sed 's/.*"id":\([0-9]*\).*/\1/')
  case "$line" in
    *'"initialize"'*)
      printf '{"jsonrpc":"2.0","id":%s,"result":{"protocolVersion":1}}\n' "$id" ;;
    *'"session/new"'*)
      printf '{"jsonrpc":"2.0","id":%s,"result":` + newSessionResult + `}\n' "$id" ;;
    *'"session/set_config_option"'*)
      value=$(printf '%s' "$line" | sed 's/.*"value":"\([^"]*\)".*/\1/')
      printf '{"jsonrpc":"2.0","id":%s,"result":{"configOptions":[{"id":"permission_mode","category":"mode","type":"select","currentValue":"'"$value"'"}]}}\n' "$id" ;;
    *'"session/prompt"'*)
      printf '{"jsonrpc":"2.0","id":%s,"result":{"stopReason":"end_turn"}}\n' "$id" ;;
  esac
done
`
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func runOneTurn(t *testing.T, bin, runtimeName, roleID string) error {
	t.Helper()
	r := &agent.ProcessRunner{
		Specs: []runtime.Spec{{Name: runtimeName, Bin: bin, VersionArgs: []string{"--version"}}},
		Bus:   &busRecorder{},
	}
	return r.RunTurn(context.Background(), port.AgentTurn{
		WorkID: "work-01", Cwd: t.TempDir(), Prompt: "做点事", RoleID: roleID,
	})
}

// ★★ 收权请求**真的发到线上了**。
//
// 判据是 Agent 那侧收到的帧，不是我们这侧调了什么。
func TestRunTurn_SendsSetConfigOptionOnTheWire(t *testing.T) {
	log := filepath.Join(t.TempDir(), "frames.ndjson")
	bin := recordingAgent(t, "claude-agent-acp", log, true)

	if err := runOneTurn(t, bin, "claude", "implementer"); err != nil {
		t.Fatalf("跑一轮: %v", err)
	}

	frames := readLog(t, log)
	if !strings.Contains(frames, "session/set_config_option") {
		t.Fatalf("线上没有 set_config_option——收权代码没被调用到。\n收到的帧：\n%s", frames)
	}
	// 实现工程师是「受控写」，在 claude 上是 default
	if !strings.Contains(frames, `"value":"default"`) {
		t.Errorf("发出去的档位不对。\n收到的帧：\n%s", frames)
	}
	if !strings.Contains(frames, `"configId"`) {
		t.Error("参数名不是 configId")
	}
}

// ★★ 收权在**任何 prompt 之前**。
//
// 顺序反了的话，中间那个窗口里 codex 跑在 workspace-write 沙箱，
// 而沙箱内的写操作连审批都不触发——AI 在那一瞬间可以改任何文件。
func TestRunTurn_RestrictsBeforeAnyPrompt(t *testing.T) {
	log := filepath.Join(t.TempDir(), "frames.ndjson")
	bin := recordingAgent(t, "claude-agent-acp", log, true)

	if err := runOneTurn(t, bin, "claude", "implementer"); err != nil {
		t.Fatalf("跑一轮: %v", err)
	}

	frames := readLog(t, log)
	setAt := strings.Index(frames, "session/set_config_option")
	promptAt := strings.Index(frames, "session/prompt")
	if setAt < 0 || promptAt < 0 {
		t.Fatalf("两条帧没都收到：\n%s", frames)
	}
	if setAt > promptAt {
		t.Errorf("先发了 prompt 才收权——中间那个窗口里 AI 可以改任何文件。\n%s", frames)
	}
}

// ★★ 角色不同，发出去的档位就不同。
//
// 这条把「角色 → 语义档 → 那一端的档名」整条链路验穿。
// 少了它的话，所有角色都发同一个档位也测不出来。
func TestRunTurn_ModeFollowsTheRole(t *testing.T) {
	cases := []struct {
		roleID, runtimeName, wantMode string
	}{
		{"implementer", "claude", "default"},      // 受控写
		{"unit_reviewer", "claude", "plan"},       // 只读
		{"requirement_analyst", "claude", "plan"}, // 只读
		{"", "claude", "default"},                 // 留空 = 实现工程师
	}

	for _, c := range cases {
		t.Run(c.roleID+"/"+c.runtimeName, func(t *testing.T) {
			log := filepath.Join(t.TempDir(), "frames.ndjson")
			bin := recordingAgent(t, "claude-agent-acp", log, true)

			if err := runOneTurn(t, bin, c.runtimeName, c.roleID); err != nil {
				t.Fatalf("跑一轮: %v", err)
			}
			frames := readLog(t, log)
			want := `"value":"` + c.wantMode + `"`
			if !strings.Contains(frames, want) {
				t.Errorf("角色 %q 发出的档位不是 %s。\n%s", c.roleID, c.wantMode, frames)
			}
		})
	}
}

// ★★ Agent 收不了权 → **拒绝开工，一句 prompt 都不发**。
func TestRunTurn_RefusesWhenAgentCannotRestrict(t *testing.T) {
	log := filepath.Join(t.TempDir(), "frames.ndjson")
	bin := recordingAgent(t, "claude-agent-acp", log, false) // 不声明任何 mode 能力

	err := runOneTurn(t, bin, "claude", "implementer")
	if err == nil {
		t.Fatal("Agent 收不了权却照跑——用户以为它受限，实际它什么都能改")
	}

	frames := readLog(t, log)
	if strings.Contains(frames, "session/prompt") {
		t.Errorf("收权失败之后还发了 prompt：\n%s", frames)
	}
}

// ★ 认不出的角色报错，**不回落到默认角色**。
//
// 回落的后果是「本该由审查员做的事被实现工程师做了」，
// 而实现方审查自己的产出正是 INV-ATT-8 禁止的——且这种错没有任何症状：
// 审查照常「通过」。
func TestRunTurn_UnknownRoleIsRefusedNotDefaulted(t *testing.T) {
	log := filepath.Join(t.TempDir(), "frames.ndjson")
	bin := recordingAgent(t, "claude-agent-acp", log, true)

	err := runOneTurn(t, bin, "claude", "architect") // 差一点点的名字最危险
	if err == nil {
		t.Fatal("认不出的角色却照跑了")
	}
	if !strings.Contains(err.Error(), "architect") {
		t.Errorf("错误信息里没提是哪个角色：%v", err)
	}

	frames := readLog(t, log)
	if strings.Contains(frames, "session/prompt") {
		t.Errorf("角色认不出还发了 prompt：\n%s", frames)
	}
}

func readLog(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		return "" // 一帧都没收到
	}
	return string(b)
}
