package work_test

import (
	"context"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/work"
	"github.com/HuLuca1998/acp-flows/backend/tests/testutil"
)

// M5 完成标志第 1、6 条 · 跟用户说话的是**需求分析师**
//
// ★★ 这条测试守的是「常驻会话只读」的唯一落点。
//
// 不传角色的话 acp 层会退到 `DefaultRoleID`（实现工程师，受控写）——
// 那意味着澄清需求阶段的会话**能改用户的文件**，而用户以为自己只是在聊天。
// 「常驻会话是只读的」那一整套收权代码全在，只是从来没被这条路径用上。

func TestStart_TalksToTheUserAsTheRequirementAnalyst(t *testing.T) {
	project := testutil.NewGitRepo(t)
	runner := &fakeRunner{}
	svc := newServiceWithRunner(t, &memWorks{}, &recordingBus{}, runner)

	if _, err := svc.Start(context.Background(), project, "用户能取消正在运行的 turn", ""); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "那一轮没跑起来", func() bool { return len(runner.snapshot()) == 1 })

	if got := runner.snapshot()[0].RoleID; got != "requirement_analyst" {
		t.Fatalf("角色 = %q，想要 requirement_analyst——"+
			"留空的话 acp 层退到实现工程师（受控写），"+
			"那意味着用户以为自己只是在聊天，而对面能改他的文件", got)
	}
}

// 接着说的那几轮也是同一个角色——否则会话池会按角色开出第二条会话。
func TestSay_KeepsTheSameRole(t *testing.T) {
	project := testutil.NewGitRepo(t)
	runner := &fakeRunner{}
	svc := newServiceWithRunner(t, &memWorks{}, &recordingBus{}, runner)
	ctx := context.Background()

	view, err := svc.Start(ctx, project, "第一句", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Say(ctx, view.ID, "第二句"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "两轮没都跑起来", func() bool { return len(runner.snapshot()) == 2 })

	turns := runner.snapshot()
	if turns[0].RoleID != turns[1].RoleID {
		t.Errorf("两轮角色不同（%q / %q）——会话池按「工作 + 角色」分键，"+
			"角色变了就会另开一条会话，前一句彻底不在上下文里",
			turns[0].RoleID, turns[1].RoleID)
	}
}

var _ = work.ErrNotAcceptingMessages
