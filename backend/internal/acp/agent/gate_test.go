package agent

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// M5 U5.1.5 · 同一条会话上的轮次排队
//
// ★★ ACP 的一条会话一次只跑一轮。两个 prompt 同时打进去的后果是真机验过的：
// 库里连着两条 `turn_end`，而**第二句的回答一个字都没有**——
// 用户补充一句之后没有任何回应，而界面看起来一切正常。
//
// ★ 这一族测的是包内的 turnGate（小写），所以在 package agent 里。
// 它没有对外的接口，而它守的那件事又太容易被「顺手改成 mutex」破坏。

var sessKey = sessionKey{workID: "work-08", roleID: "requirement_analyst"}

// ★★ R1 · 同一条会话上的两轮**串行**，不重叠。
func TestTurnGate_R1_SerializesTurnsOnOneSession(t *testing.T) {
	g := newTurnGate()
	ctx := context.Background()

	var inFlight, maxSeen atomic.Int32
	var wg sync.WaitGroup

	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release, err := g.enter(ctx, sessKey)
			if err != nil {
				t.Errorf("排队失败: %v", err)
				return
			}
			defer release()

			n := inFlight.Add(1)
			for {
				old := maxSeen.Load()
				if n <= old || maxSeen.CompareAndSwap(old, n) {
					break
				}
			}
			time.Sleep(5 * time.Millisecond) // 假装在跑一轮
			inFlight.Add(-1)
		}()
	}
	wg.Wait()

	if got := maxSeen.Load(); got != 1 {
		t.Fatalf("同时有 %d 轮在跑——ACP 的一条会话一次只跑一轮，"+
			"两个 prompt 打进去的话第二句的回答会整个丢掉", got)
	}
}

// ★ 不同会话之间**互不阻塞**：需求分析师在跑，实现工程师不用等。
func TestTurnGate_DifferentSessionsDoNotBlock(t *testing.T) {
	g := newTurnGate()
	ctx := context.Background()

	release, err := g.enter(ctx, sessKey)
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	other := sessionKey{workID: "work-08", roleID: "implementer"}
	done := make(chan struct{})
	go func() {
		r2, err2 := g.enter(ctx, other)
		if err2 == nil {
			r2()
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("另一条会话被挡住了——两个角色各有自己的会话，本不该互相等")
	}
}

// ★★ R2 · 排着的那一轮**不丢**：前一轮结束后它拿得到。
func TestTurnGate_R2_QueuedTurnGetsItsChance(t *testing.T) {
	g := newTurnGate()
	ctx := context.Background()

	first, err := g.enter(ctx, sessKey)
	if err != nil {
		t.Fatal(err)
	}

	got := make(chan struct{})
	go func() {
		release, err2 := g.enter(ctx, sessKey)
		if err2 != nil {
			t.Errorf("排队的那一轮报错: %v", err2)
			return
		}
		defer release()
		close(got)
	}()

	// 前一轮还占着，它就该在等。
	//
	// ★ 窗口给到 100ms 而不是 30ms：`-race` 下调度慢好几倍，
	// 而一个偶尔红的测试比没有测试更糟——人会开始忽略它。
	select {
	case <-got:
		t.Fatal("前一轮还没结束，第二轮就进去了")
	case <-time.After(100 * time.Millisecond):
	}

	first()
	select {
	case <-got:
	case <-time.After(time.Second):
		t.Fatal("前一轮结束了，排着的那一句却永远没轮到——用户补充的话石沉大海")
	}
}

// ★★ R4 · 取消时**等着的那一轮放弃**，不会在取消之后照样跑。
//
// 用 mutex 的话这条做不到：mutex 等不了 ctx，用户明明点了停，
// 排在后面的那几轮还是会一句句跑完。
func TestTurnGate_R4_CancelledWhileWaitingGivesUp(t *testing.T) {
	g := newTurnGate()

	first, err := g.enter(context.Background(), sessKey)
	if err != nil {
		t.Fatal(err)
	}
	defer first()

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		release, err2 := g.enter(ctx, sessKey)
		if release != nil {
			release()
		}
		errCh <- err2
	}()

	waitUntil(t, "它还没排上队", func() bool { return g.queuedOn(sessKey) == 1 })
	cancel()

	select {
	case got := <-errCh:
		if !errors.Is(got, context.Canceled) {
			t.Errorf("err = %v，想要能判定成 context.Canceled", got)
		}
	case <-time.After(time.Second):
		t.Fatal("取消之后还在等——用户明明点了停")
	}

	// ★ 判据：名额还回去了，否则这条会话的位置会被一个走掉的人永久占着
	if n := g.queuedOn(sessKey); n != 0 {
		t.Errorf("排队数 = %d，取消之后名额没还回去", n)
	}
}

// ★★ R5 · 排队深度有上限，超了**明确拒绝**。
//
// 无限排队的话，排到第五十句时前面四十九句的回答用户早就不想看了，
// 而 Agent 还在一句句地跑——账单继续涨。
func TestTurnGate_R5_RefusesBeyondTheQueueLimit(t *testing.T) {
	g := newTurnGate()
	ctx := context.Background()

	first, err := g.enter(ctx, sessKey)
	if err != nil {
		t.Fatal(err)
	}

	// 把队排满
	var wg sync.WaitGroup
	for range maxQueuedTurns - 1 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if release, err2 := g.enter(ctx, sessKey); err2 == nil {
				release()
			}
		}()
	}
	waitUntil(t, "队没排满", func() bool { return g.queuedOn(sessKey) == maxQueuedTurns-1 })

	// 第 maxQueuedTurns 个能排上（正在跑的那个已经出队了）
	extra := make(chan error, 2)
	go func() {
		release, err2 := g.enter(ctx, sessKey)
		if release != nil {
			release()
		}
		extra <- err2
	}()
	waitUntil(t, "队没满", func() bool { return g.queuedOn(sessKey) == maxQueuedTurns })

	// 再来一个：拒绝
	if _, err := g.enter(ctx, sessKey); !errors.Is(err, ErrTooManyQueued) {
		t.Fatalf("排到第 %d 句还收下了：%v——用户在反复戳一个没反应的界面，"+
			"而每一句都会真的跑一轮", maxQueuedTurns+1, err)
	}

	first()
	wg.Wait()
	<-extra
}

// ★ release 调两次也只放开一次。
//
// 多调一次会取走下一轮刚放进去的令牌——那一轮永远等不到自己结束，
// 整条会话从此卡死，而表现是「界面一直转圈」。
func TestTurnGate_ReleaseIsIdempotent(t *testing.T) {
	g := newTurnGate()
	ctx := context.Background()

	release, err := g.enter(ctx, sessKey)
	if err != nil {
		t.Fatal(err)
	}
	release()
	release() // 多调一次

	done := make(chan struct{})
	go func() {
		r2, err2 := g.enter(ctx, sessKey)
		if err2 == nil {
			r2()
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("下一轮进不去了——多调的那次 release 取走了它的令牌")
	}
}

// ★ 没人排队时把 lane 收掉——跑了几百个工作的进程不该留着几百个空 lane。
func TestTurnGate_ReleasesEmptyLanes(t *testing.T) {
	g := newTurnGate()
	release, err := g.enter(context.Background(), sessKey)
	if err != nil {
		t.Fatal(err)
	}
	release()

	g.mu.Lock()
	n := len(g.lanes)
	g.mu.Unlock()
	if n != 0 {
		t.Errorf("留下了 %d 条空 lane", n)
	}
}

func waitUntil(t *testing.T, why string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("等超时了：%s", why)
}
