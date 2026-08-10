package work_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/internal/app/work"
	"github.com/HuLuca1998/acp-flows/backend/internal/constant"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
	workspacestore "github.com/HuLuca1998/acp-flows/backend/internal/fsstore/workspace"
	"github.com/HuLuca1998/acp-flows/backend/tests/testutil"
)

// M5 U5.1.3 · 在同一个工作里接着说
//
// ★★ 这一族守的是完成标志第 5 条：**连着说三句，AI 记得前两句**。
//
// 之前后端的会话池已经能复用会话了，但界面上没有多轮的入口——
// 用户说第二句时开的是一个新工作：新 worktree、新会话、新时间线，
// 前一句彻底不在上下文里，而他以为自己只是补充了一句。

// ★★ R1 · 三句话进的是**同一个工作**，不新建。
func TestSay_R1_StaysInTheSameWork(t *testing.T) {
	project := testutil.NewGitRepo(t)
	repo := &memWorks{}
	runner := &fakeRunner{}
	svc := newServiceWithRunner(t, repo, &recordingBus{}, runner)
	ctx := context.Background()

	view, err := svc.Start(ctx, project, "用户能取消正在运行的 turn", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, more := range []string{"取消后现场证据要保留", "先别写代码，先把范围说清楚"} {
		if err := svc.Say(ctx, view.ID, more); err != nil {
			t.Fatalf("接着说 %q: %v", more, err)
		}
	}

	works, err := repo.ListWorks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(works) != 1 {
		t.Fatalf("说了三句却有 %d 个工作——第二句开了个新工作，"+
			"新 worktree 新会话新时间线，前一句彻底不在上下文里", len(works))
	}

	// ★★ 三轮都送到**同一个 workID + 同一个 worktree**——
	// 会话池正是按这两样分键的，对不上就复用不到那条会话。
	waitFor(t, "三轮没都跑起来", func() bool { return len(runner.snapshot()) == 3 })
	for i, turn := range runner.snapshot() {
		if turn.WorkID != view.ID {
			t.Errorf("第 %d 轮的 workID = %q，想要 %q", i+1, turn.WorkID, view.ID)
		}
		if turn.Cwd != view.Worktree {
			t.Errorf("第 %d 轮的 cwd = %q，想要 %q", i+1, turn.Cwd, view.Worktree)
		}
	}
}

// ★★ R2 · 每一句都进时间线，文本各不相同。
//
// 漏掉一句的话，用户回头看「我当时到底说了什么」会少一段，
// 而 AI 的回复还在——他会以为 AI 答非所问。
func TestSay_R2_EverySentenceLandsOnTheTimeline(t *testing.T) {
	project := testutil.NewGitRepo(t)
	bus := &recordingBus{}
	svc := newServiceWithRunner(t, &memWorks{}, bus, &fakeRunner{})
	ctx := context.Background()

	said := []string{
		"用户能取消正在运行的 turn",
		"取消后现场证据要保留",
		"先别写代码，先把范围说清楚",
	}
	view, err := svc.Start(ctx, project, said[0], "")
	if err != nil {
		t.Fatal(err)
	}
	for _, more := range said[1:] {
		if err := svc.Say(ctx, view.ID, more); err != nil {
			t.Fatal(err)
		}
	}

	var got []string
	for _, e := range bus.snapshot() {
		if e.Type == "user_message" {
			text, _ := e.Payload["text"].(string)
			got = append(got, text)
		}
	}
	if len(got) != len(said) {
		t.Fatalf("时间线上有 %d 句用户消息，想要 %d 句：%v", len(got), len(said), got)
	}
	for i := range said {
		if got[i] != said[i] {
			t.Errorf("第 %d 句 = %q，想要 %q", i+1, got[i], said[i])
		}
	}
}

// ★★ R4 · 终态的工作**拒绝**，并说清原因。
//
// 静默收下的话，用户对着一个永远不动的时间线干等，
// 以为 AI 在想事情——而那句话根本没人接。
func TestSay_R4_TerminalWorkRefusesAndSaysWhy(t *testing.T) {
	repo := &memWorks{}
	svc := newServiceWithRunner(t, repo, &recordingBus{}, &fakeRunner{})
	ctx := context.Background()

	// ★ 用 NewWorkAt 直接造在终态上——那正是「从库里读回一个已失败的工作」，
	// 也是这个场景在生产里真实的样子（用户重开应用，看到一条失败的工作）。
	w := model.NewWorkAt("work-done", constant.WorkStateFailed)
	w.SetWorktree(t.TempDir(), "duet/work-done", "abc1234")
	if err := repo.SaveWork(ctx, w); err != nil {
		t.Fatal(err)
	}

	err := svc.Say(ctx, "work-done", "再试一次好吗")
	if !errors.Is(err, work.ErrNotAcceptingMessages) {
		t.Fatalf("终态工作收下了消息：%v", err)
	}
	// ★ 前端要按机器可读的码查词条，不能把 error 的文字摊给用户
	if code := work.ErrorCode(err); code != "work_not_accepting_messages" {
		t.Errorf("错误码 = %q，想要 work_not_accepting_messages", code)
	}
}

// ★ 被拒之后**一句 prompt 都没发**，时间线上也不留那句话。
//
// 留下的话，时间线上会有一句「用户说了什么」而永远没有下文。
func TestSay_R4_RefusedMeansNothingHappened(t *testing.T) {
	repo := &memWorks{}
	bus := &recordingBus{}
	runner := &fakeRunner{}
	svc := newServiceWithRunner(t, repo, bus, runner)
	ctx := context.Background()

	w := model.NewWorkAt("work-done", constant.WorkStateCompleted)
	w.SetWorktree(t.TempDir(), "duet/work-done", "abc1234")
	if err := repo.SaveWork(ctx, w); err != nil {
		t.Fatal(err)
	}
	_ = svc.Say(ctx, "work-done", "再试一次好吗")

	if n := len(runner.snapshot()); n != 0 {
		t.Errorf("被拒之后还是跑了 %d 轮", n)
	}
	for _, e := range bus.snapshot() {
		if e.Type == "user_message" {
			t.Error("被拒之后那句话却留在了时间线上——用户会等一个永远不来的回复")
		}
	}
}

// 工作不存在时报错，且能判定成 ErrNotFound（上层要据此回 404）。
func TestSay_UnknownWorkIsNotFound(t *testing.T) {
	svc := newServiceWithRunner(t, &memWorks{}, &recordingBus{}, &fakeRunner{})

	err := svc.Say(context.Background(), "work-nope", "在吗")
	if !errors.Is(err, model.ErrNotFound) {
		t.Errorf("err = %v，想要能判定成 ErrNotFound", err)
	}
}

// 空话不发：发出去的话 Agent 会为一句空白跑一整轮。
func TestSay_RejectsBlank(t *testing.T) {
	project := testutil.NewGitRepo(t)
	runner := &fakeRunner{}
	svc := newServiceWithRunner(t, &memWorks{}, &recordingBus{}, runner)
	ctx := context.Background()

	view, err := svc.Start(ctx, project, "第一句", "")
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, "第一轮没跑起来", func() bool { return len(runner.snapshot()) == 1 })

	for _, blank := range []string{"", "   ", "\n\t"} {
		if err := svc.Say(ctx, view.ID, blank); err == nil {
			t.Errorf("空话 %q 却发出去了", blank)
		}
	}
	if n := len(runner.snapshot()); n != 1 {
		t.Errorf("空话触发了 %d 轮", n-1)
	}
}

// ★ worktree 还没切好时拒绝。
//
// 用空 cwd 去跑一轮的话，Agent 会在**进程自己的当前目录**里干活——
// 那是 duetd 的目录，不是用户的项目。
func TestSay_RefusesBeforeTheWorktreeIsReady(t *testing.T) {
	repo := &memWorks{}
	runner := &fakeRunner{}
	svc := newServiceWithRunner(t, repo, &recordingBus{}, runner)
	ctx := context.Background()

	// initializing 不是终态，但它还没有工作区
	w := model.NewWork("work-fresh")
	if err := repo.SaveWork(ctx, w); err != nil {
		t.Fatal(err)
	}

	if err := svc.Say(ctx, "work-fresh", "先说一句"); err == nil {
		t.Fatal("工作区还没准备好却收下了消息")
	}
	if n := len(runner.snapshot()); n != 0 {
		t.Errorf("跑了 %d 轮——Agent 会在 duetd 自己的目录里干活", n)
	}
}

// 装配里没有 runner 时不崩（只跑 API 冒烟的场景）。
func TestSay_WithoutRunnerDoesNotPanic(t *testing.T) {
	project := testutil.NewGitRepo(t)
	svc := newService(t, &memWorks{}, &recordingBus{})
	ctx := context.Background()

	view, err := svc.Start(ctx, project, "第一句", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Say(ctx, view.ID, "第二句"); err != nil {
		t.Errorf("没有 runner 时报错了：%v", err)
	}
}

var _ port.AgentRunner = (*fakeRunner)(nil)

// ── U10.7.3 · 引用文件 ──────────────────────────────────────
//
// ★★ 引用只显示不注入（界面说谎）是 forbidden_changes 的第一条——
// 这一族守的就是「chips 上那个文件真的进了 prompt」。

// R1 · 引用的文件内容真的进这一轮的 prompt（路径也在，AI 才知道它是哪个文件）。
func TestSay_R1Ref_InjectsReferencedFileIntoPrompt(t *testing.T) {
	project := testutil.NewGitRepo(t)
	runner := &fakeRunner{}
	svc := newServiceWithRunner(t, &memWorks{}, &recordingBus{}, runner)
	svc.SetWorkspaceFiles(workspacestore.Store{})
	ctx := context.Background()

	view, err := svc.Start(ctx, project, "先把取消的现状摸清楚", "")
	if err != nil {
		t.Fatal(err)
	}
	noteRel := filepath.Join("docs", "cancel-notes.md")
	noteAbs := filepath.Join(view.Worktree, noteRel)
	if err := os.MkdirAll(filepath.Dir(noteAbs), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "两段式取消：先协议 cancel，再等 stopReason 落盘。"
	if err := os.WriteFile(noteAbs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := svc.Say(ctx, view.ID, "照这份笔记核对实现", noteRel); err != nil {
		t.Fatalf("带引用说一句: %v", err)
	}

	waitFor(t, "第二轮没跑起来", func() bool { return len(runner.snapshot()) == 2 })
	prompt := runner.snapshot()[1].Prompt
	if !strings.Contains(prompt, content) {
		t.Fatalf("引用文件的内容没进 prompt——界面上的 chip 在说谎。prompt=%q", prompt)
	}
	if !strings.Contains(prompt, noteRel) {
		t.Errorf("prompt 里没带路径，AI 不知道这段内容来自哪个文件：%q", prompt)
	}
}

// R3 · 读不到 → 整句拒绝（错误带路径），user_message 都不发、也不跑轮。
//
// 静默丢弃的话，用户以为 AI 看过那个文件了，而它根本没看到。
func TestSay_R3Ref_UnreadableRejectsWholeMessage(t *testing.T) {
	project := testutil.NewGitRepo(t)
	runner := &fakeRunner{}
	bus := &recordingBus{}
	svc := newServiceWithRunner(t, &memWorks{}, bus, runner)
	svc.SetWorkspaceFiles(workspacestore.Store{})
	ctx := context.Background()

	view, err := svc.Start(ctx, project, "第一句", "")
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, "第一轮没跑起来", func() bool { return len(runner.snapshot()) == 1 })
	before := len(bus.snapshot())

	err = svc.Say(ctx, view.ID, "看看这个", filepath.Join("docs", "no-such.md"))
	if err == nil {
		t.Fatal("引用文件读不到却收下了消息")
	}
	if !strings.Contains(err.Error(), filepath.Join("docs", "no-such.md")) {
		t.Errorf("错误里没带路径，用户不知道哪个引用坏了：%v", err)
	}
	if n := len(runner.snapshot()); n != 1 {
		t.Errorf("跑了 %d 轮——引用坏了还是跑了", n)
	}
	if n := len(bus.snapshot()); n != before {
		t.Errorf("多了 %d 条事件——user_message 发出去了，时间线上会留一句永远没下文的话",
			n-before)
	}
}

// 引用路径不许逃出 worktree：绝对路径与 `..` 一律拒。
func TestSay_RefRejectsEscapingPaths(t *testing.T) {
	project := testutil.NewGitRepo(t)
	runner := &fakeRunner{}
	svc := newServiceWithRunner(t, &memWorks{}, &recordingBus{}, runner)
	svc.SetWorkspaceFiles(workspacestore.Store{})
	ctx := context.Background()

	view, err := svc.Start(ctx, project, "第一句", "")
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, "第一轮没跑起来", func() bool { return len(runner.snapshot()) == 1 })

	for _, ref := range []string{"../outside.md", "/etc/hosts", ""} {
		if err := svc.Say(ctx, view.ID, "看看这个", ref); err == nil {
			t.Errorf("引用 %q 应被拒绝", ref)
		}
	}
	if n := len(runner.snapshot()); n != 1 {
		t.Errorf("逃逸引用还是跑了 %d 轮", n)
	}
}
