package work_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/tests/testutil"
)

// M7 U7.4.1 · 本轮结束时的四行小结
//
// ★★ 四行**全部来自应用记的事实**，不解析 AI 的回复。它说
// 「我改了计划」时可能什么都没改——而用户看小结正是为了
// 不用往回滚就知道这轮发生了什么。

// summaryOf 取时间线上最后一条小结的载荷。
func summaryOf(bus *recordingBus) map[string]any {
	var last map[string]any
	for _, e := range bus.snapshot() {
		if e.Type == "turn_summary" {
			last = e.Payload
		}
	}
	return last
}

// R1 ★★ 小结的内容来自应用记的事实，**不跟着 AI 的说法走**。
func TestTurnSummary_IgnoresWhatTheAgentClaims(t *testing.T) {
	// AI 在回复里胡说自己改了计划
	runner := &fakeRunner{reply: "我已经把计划改成了 v9，并冻结了契约。"}
	project := testutil.NewGitRepo(t)
	bus := &recordingBus{}
	svc := newServiceWithRunner(t, &memWorks{}, bus, runner)
	svc.SetRequirements(newMemRequirements())
	svc.SetPlans(newMemPlans())

	_, err := svc.Start(context.Background(), project, "让取消真的停下来", "")
	require.NoError(t, err)
	waitFor(t, "小结没发出来", func() bool { return summaryOf(bus) != nil })

	s := summaryOf(bus)
	assert.NotContains(t, s, "plan_version",
		"AI 说改了计划就真记一笔——那这四行是它自己写的，不是我们观察到的")
}

// R2 ★★ 没发生的那一行**不显示**。
//
// 塞一个「计划：无变更」进去的话，四行里有三行是废话，
// 用户会开始整块跳过——那正好淹掉真正变了的那一行。
func TestTurnSummary_OmitsWhatDidNotHappen(t *testing.T) {
	runner := &fakeRunner{}
	project := testutil.NewGitRepo(t)
	bus := &recordingBus{}
	svc := newServiceWithRunner(t, &memWorks{}, bus, runner)
	svc.SetRequirements(newMemRequirements())

	_, err := svc.Start(context.Background(), project, "让取消真的停下来", "")
	require.NoError(t, err)
	waitFor(t, "小结没发出来", func() bool { return summaryOf(bus) != nil })

	s := summaryOf(bus)
	for _, key := range []string{"plan_version", "contract_version", "memory_ids", "skill_refs"} {
		assert.NotContains(t, s, key, "这一轮什么都没发生，却给 %s 留了一行", key)
	}
	// ★ 收场方式总是有：这一行是用户最先看的
	assert.Equal(t, "done", s["outcome"])
}

// R3 ★★ 注入清单那一行与 `injection` 事件**同源**。
//
// 两处各数一次的话它们迟早对不上，而用户没有第三个地方去核对。
func TestTurnSummary_InjectionMatchesTheEvent(t *testing.T) {
	repo, bodies := &memMemories{}, newBodies()
	runner := &fakeRunner{}
	project := testutil.NewGitRepo(t)
	bus := &recordingBus{}
	svc := newServiceWithRunner(t, &memWorks{}, bus, runner)
	svc.SetRequirements(newMemRequirements())
	svc.SetMemories(repo, bodies)
	require.NoError(t, repo.SaveMemory(context.Background(),
		activeMemory(t, "mem-01", project, bodies)))

	_, err := svc.Start(context.Background(), project, "让取消真的停下来", "")
	require.NoError(t, err)
	waitFor(t, "小结没发出来", func() bool { return summaryOf(bus) != nil })

	var injected any
	for _, e := range bus.snapshot() {
		if e.Type == "injection" {
			injected = e.Payload["memory_ids"]
		}
	}
	require.NotNil(t, injected, "注入事件都没发")
	assert.Equal(t, injected, summaryOf(bus)["memory_ids"],
		"小结与注入事件对不上——用户没有第三个地方去核对")
}

// R4 ★★ 一轮被取消时**也出小结**。
//
// 他点停正是因为想看看现在到哪了——那时最需要这四行。
func TestTurnSummary_AlsoOnCancel(t *testing.T) {
	runner := &fakeRunner{err: context.Canceled}
	project := testutil.NewGitRepo(t)
	bus := &recordingBus{}
	svc := newServiceWithRunner(t, &memWorks{}, bus, runner)
	svc.SetRequirements(newMemRequirements())

	_, err := svc.Start(context.Background(), project, "让取消真的停下来", "")
	require.NoError(t, err)
	waitFor(t, "跑挂了也该出小结", func() bool { return summaryOf(bus) != nil })

	assert.Equal(t, "failed", summaryOf(bus)["outcome"],
		"跑挂了的收场方式该是 failed")
}

// ★ 排队没排上**不算失败**：用户只是手快点了几下，
// 那几句话一句都没丢，只是没排上。
func TestTurnSummary_QueueFullIsNotAFailure(t *testing.T) {
	runner := &fakeRunner{err: port.ErrTurnQueueFull}
	project := testutil.NewGitRepo(t)
	bus := &recordingBus{}
	svc := newServiceWithRunner(t, &memWorks{}, bus, runner)
	svc.SetRequirements(newMemRequirements())

	_, err := svc.Start(context.Background(), project, "让取消真的停下来", "")
	require.NoError(t, err)
	waitFor(t, "小结没发出来", func() bool { return summaryOf(bus) != nil })

	assert.Equal(t, "queue_full", summaryOf(bus)["outcome"],
		"排队没排上被说成 failed——用户会以为自己那几句话丢了")
}
