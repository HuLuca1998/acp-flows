package agent

import (
	"context"
	"fmt"
	"sync"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
)

// ErrTooManyQueued 表示这条会话上排队的轮次已经到上限。
//
// ★ 明确报错而不是无限排队：排到第五十句时，前面四十九句的回答用户
// 早就不想看了，而 Agent 还在一句句地跑——账单继续涨。
//
// ★★ 它**包着 port.ErrTurnQueueFull**：app 层不许 import acp，
// 而它必须分得出「排队满」与「AI 跑挂了」——后者会把工作推到 failed，
// 而用户只是手快点了几下。
var ErrTooManyQueued = fmt.Errorf("agent: 这条会话上排队的消息太多了: %w", port.ErrTurnQueueFull)

// maxQueuedTurns 是一条会话上最多能排几轮（不含正在跑的那一轮）。
//
// ★ 4 不是拍脑袋：用户连着补充四五句是正常的（「还有」「另外」「对了」），
// 而排到第十句通常说明他在反复戳一个没反应的界面。
const maxQueuedTurns = 4

// turnGate 让**同一条会话上的轮次串行**。
//
// ★★ ACP 的一条会话一次只跑一轮。两个 prompt 同时打进去的后果是
// 真机验过的：库里连着两条 `turn_end`，而**第二句的回答一个字都没有**——
// 用户补充一句之后没有任何回应，而界面看起来一切正常。
//
// ★ 用带 ctx 的 channel 而不是 sync.Mutex：mutex 等不了 ctx，
// 用户取消整个工作时，排在后面的那几轮会**照样跑完**——他明明点了停。
type turnGate struct {
	mu    sync.Mutex
	lanes map[sessionKey]*lane
}

type lane struct {
	// token 容量 1：拿到的那一个才有资格下发 prompt。
	token chan struct{}
	// queued 是**正在等**的轮数，不含跑着的那一轮。
	queued int
}

func newTurnGate() *turnGate {
	return &turnGate{lanes: map[sessionKey]*lane{}}
}

// enter 排队等这条会话空出来。
//
// 返回的 release 必须调用（用 defer），否则这条会话会永远卡住。
// 返回错误时 release 为 nil —— **没进去就不用出来**。
func (g *turnGate) enter(ctx context.Context, key sessionKey) (release func(), err error) {
	ln, err := g.reserve(key)
	if err != nil {
		return nil, err
	}

	select {
	case ln.token <- struct{}{}:
		g.leaveQueue(key)
		// ★ release **调两次也只放开一次**。多调一次的后果是取走了
		// 下一轮刚放进去的令牌——那一轮于是永远等不到自己结束，
		// 整条会话从此卡死。用 Once 把这个使用错误挡在这里，
		// 而不是让它在几层调用之外表现成「界面一直转圈」。
		var once sync.Once
		return func() { once.Do(func() { g.exit(key, ln) }) }, nil
	case <-ctx.Done():
		// ★ 等的过程中被取消：把排队计数还回去，否则这条会话的名额
		// 会被一个已经走掉的人永久占着。
		g.leaveQueue(key)
		// ★ 同时包上 port.ErrTurnAbandoned：用户点了停，排在后面的那几句
		// 放弃是**预期行为**，不该被 app 层当成失败。
		return nil, fmt.Errorf("agent: 等这条会话空出来时被放弃: %w（%w）",
			port.ErrTurnAbandoned, ctx.Err())
	}
}

// reserve 占一个排队名额；超过上限就拒绝。
func (g *turnGate) reserve(key sessionKey) (*lane, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	ln := g.lanes[key]
	if ln == nil {
		ln = &lane{token: make(chan struct{}, 1)}
		g.lanes[key] = ln
	}
	if ln.queued >= maxQueuedTurns {
		return nil, fmt.Errorf("%w：已经排了 %d 句", ErrTooManyQueued, ln.queued)
	}
	ln.queued++
	return ln, nil
}

func (g *turnGate) leaveQueue(key sessionKey) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if ln := g.lanes[key]; ln != nil && ln.queued > 0 {
		ln.queued--
	}
}

// exit 放开这条会话，让排在后面的那一轮进来。
func (g *turnGate) exit(key sessionKey, ln *lane) {
	<-ln.token

	// ★ 没人在排队时把这条 lane 收掉——一个跑了几百个工作的进程
	// 不该留着几百个空 lane。
	g.mu.Lock()
	defer g.mu.Unlock()
	if cur := g.lanes[key]; cur == ln && ln.queued == 0 && len(ln.token) == 0 {
		delete(g.lanes, key)
	}
}

// queuedOn 返回这条会话上正在等的轮数，供测试与诊断用。
func (g *turnGate) queuedOn(key sessionKey) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	if ln := g.lanes[key]; ln != nil {
		return ln.queued
	}
	return 0
}
