package session_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/acp/fake"
	"github.com/HuLuca1998/acp-flows/backend/internal/acp/protocol"
	"github.com/HuLuca1998/acp-flows/backend/internal/acp/session"
)

// U4.1.1 · 会话恢复（验收点 V10）
//
// ★ R2 是前一个项目最严重的一条错误：会话标识在某一层丢了，
// 表现为「第 2 轮不记得第 1 轮」，而**界面上看着完全正常**。
// 下面每条断言都在守它。

// ★★ R2：**同一条会话上的两轮落在同一个 sessionId 上。**
//
// 丢了的话，第 2 轮对 Agent 来说是全新的对话——它不记得第 1 轮说过什么，
// 而用户看到的只是「AI 忽然变笨了」。没有任何报错。
func TestPrompt_R2_SameSessionAcrossTurns(t *testing.T) {
	rt := newFakeRuntime(t, twoTurnScript())
	s := openWith(t, rt, session.Permission{})

	for i := 0; i < 2; i++ {
		if _, err := s.Prompt(context.Background(), "第几轮？", nil); err != nil {
			t.Fatalf("第 %d 轮失败: %v", i+1, err)
		}
	}

	ids := promptSessionIDs(t, rt)
	if len(ids) != 2 {
		t.Fatalf("发了 %d 次 session/prompt, 想要 2", len(ids))
	}
	if ids[0] != ids[1] {
		t.Errorf("两轮的 sessionId 不同：%q vs %q——\n"+
			"第 2 轮对 Agent 来说是全新的对话，它不记得第 1 轮说过什么，"+
			"而用户看到的只是「AI 忽然变笨了」，没有任何报错", ids[0], ids[1])
	}
	if ids[0] != s.ID() {
		t.Errorf("发出去的 sessionId (%q) 与会话自己的 ID (%q) 对不上", ids[0], s.ID())
	}
}

// ★★ R1：恢复一条已有会话，**Agent 能接着上次继续**。
//
// 这是 V10 的全部意义：用户关掉再打开，AI 还记得之前聊了什么。
func TestResume_R1_ContinuesExistingSession(t *testing.T) {
	rt := newFakeRuntime(t, resumableScript())
	s, err := session.Resume(context.Background(), session.ResumeOptions{
		Options:   session.Options{Transport: rt.Transport(), Cwd: t.TempDir()},
		SessionID: "sess_before_restart",
	})
	if err != nil {
		t.Fatalf("恢复失败: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	// ★ 恢复的必须是**那一条**，不是新开一条
	if s.ID() != "sess_before_restart" {
		t.Errorf("恢复后的会话 ID = %q, 想要 sess_before_restart——"+
			"新开一条的话，AI 完全不记得之前聊了什么", s.ID())
	}
	if s.IsFresh() {
		t.Error("恢复成功却被标成了新会话")
	}

	// 走的是 session/load，不是 session/new
	if got := rt.CountMethod(protocol.MethodSessionLoad); got != 1 {
		t.Errorf("发了 %d 次 session/load, 想要 1", got)
	}
	if got := rt.CountMethod(protocol.MethodSessionNew); got != 0 {
		t.Errorf("恢复时还发了 %d 次 session/new——那会开出一条空白会话", got)
	}
}

// 恢复之后新一轮要**打在同一个会话上**。
func TestResume_NewTurnStaysOnResumedSession(t *testing.T) {
	rt := newFakeRuntime(t, resumableScript())
	s, err := session.Resume(context.Background(), session.ResumeOptions{
		Options:   session.Options{Transport: rt.Transport(), Cwd: t.TempDir()},
		SessionID: "sess_before_restart",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	if _, err := s.Prompt(context.Background(), "接着说", nil); err != nil {
		t.Fatal(err)
	}

	ids := promptSessionIDs(t, rt)
	if len(ids) != 1 || ids[0] != "sess_before_restart" {
		t.Errorf("恢复后那一轮打在 %v 上, 想要 sess_before_restart——"+
			"恢复了却打在别的会话上，等于白恢复", ids)
	}
}

// ★★ R3：恢复失败时**显式标记为新会话**，绝不假装成功。
//
// 假装的话，用户以为 AI 记得之前的事，实际它一无所知——
// 他会接着上文提问，而 AI 答非所问，双方都不知道发生了什么。
func TestResume_R3_FallsBackToFreshSessionExplicitly(t *testing.T) {
	// Agent 不支持 session/load：回 -32601
	rt := newFakeRuntime(t, noLoadScript())

	s, err := session.Resume(context.Background(), session.ResumeOptions{
		Options:   session.Options{Transport: rt.Transport(), Cwd: t.TempDir()},
		SessionID: "sess_before_restart",
	})
	if err != nil {
		t.Fatalf("降级路径也应该拿到一条可用的会话: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	// ★ 明确说「这是新的」——上层据此告诉用户「之前的上下文没了」
	if !s.IsFresh() {
		t.Fatal("恢复失败却没标成新会话——\n" +
			"用户以为 AI 记得之前的事，实际它一无所知；" +
			"他接着上文提问，而 AI 答非所问，双方都不知道发生了什么")
	}
	if s.ID() == "sess_before_restart" {
		t.Errorf("降级了却还用着旧 ID (%q)——那是一条 Agent 不认识的会话", s.ID())
	}
	// 降级原因要能看出来，否则排查时只知道「变成新会话了」
	if s.ResumeError() == nil {
		t.Error("降级了却没有留下原因")
	}

	// 真的开了一条新会话
	if got := rt.CountMethod(protocol.MethodSessionNew); got != 1 {
		t.Errorf("降级后发了 %d 次 session/new, 想要 1", got)
	}
}

// 恢复时**先校验 cwd**，和 Open 一样。
//
// 反过来的话，Agent 那边已经加载了一条会话而我们这边报了错——
// 它会挂在那儿占着资源，没人再去关它。
func TestResume_ValidatesCwdBeforeTalking(t *testing.T) {
	rt := newFakeRuntime(t, resumableScript())

	_, err := session.Resume(context.Background(), session.ResumeOptions{
		Options:   session.Options{Transport: rt.Transport(), Cwd: "relative/path"},
		SessionID: "sess_before_restart",
	})
	if err == nil {
		t.Fatal("相对路径却恢复成功了")
	}
	if got := rt.CountMethod(protocol.MethodSessionLoad); got != 0 {
		t.Errorf("cwd 非法却已经发了 %d 次 session/load——"+
			"Agent 那边加载了一条会话而我们报了错，它会挂在那儿占着资源", got)
	}
}

// 空的 sessionID 直接当新会话开，不发一个注定失败的 load。
func TestResume_EmptySessionIDOpensFresh(t *testing.T) {
	rt := newFakeRuntime(t, noLoadScript())

	s, err := session.Resume(context.Background(), session.ResumeOptions{
		Options: session.Options{Transport: rt.Transport(), Cwd: t.TempDir()},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	if !s.IsFresh() {
		t.Error("没有旧会话可恢复，却没标成新会话")
	}
	if got := rt.CountMethod(protocol.MethodSessionLoad); got != 0 {
		t.Errorf("没有旧 ID 却发了 %d 次 session/load", got)
	}
}

// ── 脚本与小工具 ─────────────────────────────────────────

func twoTurnScript() *fake.Script {
	turn := fake.Turn{StopReason: protocol.StopReasonEndTurn}
	return &fake.Script{Name: "two-turns", Turns: []fake.Turn{turn, turn}}
}

// resumableScript：支持 session/load。
func resumableScript() *fake.Script {
	return &fake.Script{
		Name:       "resumable",
		NewSession: &fake.NewSessionBehavior{SessionID: "sess_fresh"},
		Load:       &fake.LoadSessionBehavior{Supported: true},
		Turns:      []fake.Turn{{StopReason: protocol.StopReasonEndTurn}},
	}
}

// noLoadScript：**不支持** session/load，回 -32601。
func noLoadScript() *fake.Script {
	return &fake.Script{
		Name:       "no-load",
		NewSession: &fake.NewSessionBehavior{SessionID: "sess_fresh"},
		Load:       &fake.LoadSessionBehavior{Supported: false},
		Turns:      []fake.Turn{{StopReason: protocol.StopReasonEndTurn}},
	}
}

// promptSessionIDs 取每次 session/prompt 里带的 sessionId。
func promptSessionIDs(t *testing.T, rt *fake.Runtime) []string {
	t.Helper()
	var out []string
	for _, r := range rt.Requests() {
		if r.Method != protocol.MethodSessionPrompt {
			continue
		}
		var params struct {
			SessionID string `json:"sessionId"`
		}
		if err := json.Unmarshal(r.Params, &params); err != nil {
			t.Fatalf("解析 prompt 参数失败: %v", err)
		}
		out = append(out, params.SessionID)
	}
	return out
}

// 恢复失败的原因要能读，排查时得看得出「为什么降级了」。
func TestResume_ErrorMessageIsReadable(t *testing.T) {
	rt := newFakeRuntime(t, noLoadScript())
	s, err := session.Resume(context.Background(), session.ResumeOptions{
		Options:   session.Options{Transport: rt.Transport(), Cwd: t.TempDir()},
		SessionID: "sess_before_restart",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	msg := s.ResumeError().Error()
	if !strings.Contains(msg, "sess_before_restart") {
		t.Errorf("降级原因里没有那条会话的标识（%q）——排查时不知道是哪条没恢复上", msg)
	}
}
