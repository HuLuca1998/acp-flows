// Package permission 是「Agent 在等」与「用户还没点」之间的中转站。
//
// 会话线程调 Ask 挂住，HTTP 线程调 Answer 把它叫醒。
//
// ★ 这个包里几乎每一行都在守同一件事：**没人替用户做决定**。
// 没有超时、没有默认选项、没有「猜一个」——用户可能去泡了杯咖啡，
// 替他超时等于替他做决定。要中止只能取消整个工作。
package permission

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
)

// 应答相关的错误。
var (
	// ErrNotPending 表示这条请求不存在、已经应答过、或者不属于这个工作。
	//
	// ★ 三种情况合成一个错误码是刻意的：对界面来说处理方式一样
	// （提示「这条已经处理过了」并刷新），而分得太细会诱使前端去猜内部状态。
	ErrNotPending = errors.New("permission: 这条请求不在等待中")
	// ErrCancelled 表示等待被取消（工作被取消，或 ctx 结束）。
	ErrCancelled = errors.New("permission: 等待被取消")
)

// Option 是一个可选应答。
//
// ★ OptionID 是 Agent 定义的**不透明字符串**，全程原样传递。
type Option struct {
	OptionID string `json:"option_id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
}

// Ask 是一次待裁决的权限请求。
type Ask struct {
	WorkID     string
	ToolCallID string
	Runtime    string
	Kind       string
	Path       string
	// OutOfBounds 只在**真的越界**时填 true。
	// 乱填的话用户会对所有提示脱敏，真正越界那次他也不会看。
	OutOfBounds bool
	Options     []Option
}

// Pending 是一条正在等的请求，供界面刷新后重新画出来。
type Pending struct {
	AskID string
	Ask   Ask
}

// Result 是 Ask 的结果，测试与调用方都用得上。
type Result struct {
	OptionID string
	Err      error
}

// waiter 是一次等待。
type waiter struct {
	workID string
	ask    Ask
	// done 在被应答或取消时关闭。
	done chan struct{}
	// optionID 在 done 关闭后才可读。
	optionID string
	err      error
}

// Broker 管住所有待裁决的权限请求。
type Broker struct {
	bus port.WorkEventBus
	ids port.IDGen

	mu      sync.Mutex
	waiters map[string]*waiter
}

// New 组装。
func New(bus port.WorkEventBus, ids port.IDGen) *Broker {
	return &Broker{bus: bus, ids: ids, waiters: make(map[string]*waiter)}
}

// Ask 把请求发到界面，**阻塞到用户做出选择**。
//
// ★ 没有超时。用户可能去泡了杯咖啡；替他超时等于替他做决定。
// 解除途径只有三条：用户应答、工作被取消、ctx 结束。
//
// ★ 事件发不出去时**立刻失败，不挂起**。挂起的话界面根本不知道有这条请求
// （事件没发出去），而 AI 一直等——用户对着一个不动的界面，没有任何提示。
func (b *Broker) Ask(ctx context.Context, ask Ask) (string, error) {
	askID := b.ids.NextID("ask")
	w := &waiter{workID: ask.WorkID, ask: ask, done: make(chan struct{})}

	b.mu.Lock()
	b.waiters[askID] = w
	b.mu.Unlock()

	if err := b.publish(ctx, askID, ask); err != nil {
		b.take(askID)
		return "", fmt.Errorf("permission: 发权限请求事件: %w", err)
	}

	select {
	case <-w.done:
		return w.optionID, w.err
	case <-ctx.Done():
		b.take(askID)
		return "", fmt.Errorf("%w: %w", ErrCancelled, ctx.Err())
	}
}

// Answer 送回用户的选择。
//
// ★ 同一条请求**只能应答一次**：第二次返回 ErrNotPending，
// 而且不会把新值交给等着的那一方——Agent 那边只在等一个响应，
// 多发的会被当成不认识的请求而静静丢弃，界面上却什么提示都没有。
//
// ★ 校验 workID：不校验的话，两个工作同时开着时，
// 一次误点会应答到另一个工作头上。
func (b *Broker) Answer(workID, askID, optionID string) error {
	b.mu.Lock()
	w, ok := b.waiters[askID]
	if ok && w.workID != workID {
		ok = false
	}
	if ok {
		delete(b.waiters, askID)
	}
	b.mu.Unlock()

	if !ok {
		return fmt.Errorf("%w: %s", ErrNotPending, askID)
	}
	// ★ 原样交回，不做任何加工
	w.optionID = optionID
	close(w.done)
	return nil
}

// CancelWork 解除某个工作**全部**待裁决的请求。
//
// ★ 漏一条的话，那条会话永远挂着——进程退不出去，而用户什么都看不到。
// 只动这个工作的：用户取消一个，另一个不该跟着废。
func (b *Broker) CancelWork(workID string) {
	b.mu.Lock()
	var hit []*waiter
	for id, w := range b.waiters {
		if w.workID == workID {
			hit = append(hit, w)
			delete(b.waiters, id)
		}
	}
	b.mu.Unlock()

	for _, w := range hit {
		w.err = ErrCancelled
		close(w.done)
	}
}

// Pending 列出某个工作还在等的请求。界面刷新后照它重新画卡片。
func (b *Broker) Pending(workID string) []Pending {
	b.mu.Lock()
	defer b.mu.Unlock()

	out := make([]Pending, 0)
	for id, w := range b.waiters {
		if w.workID == workID {
			out = append(out, Pending{AskID: id, Ask: w.ask})
		}
	}
	return out
}

// take 摘掉一个等待项，不叫醒它（调用方自己处理返回值）。
func (b *Broker) take(askID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.waiters, askID)
}

// publish 把请求发成一条事件。
//
// 载荷字段名与 api/openapi.yaml 的 PermissionAskPayload 一一对应——
// 界面照着它渲染按钮。
func (b *Broker) publish(ctx context.Context, askID string, ask Ask) error {
	options := make([]any, 0, len(ask.Options))
	for _, o := range ask.Options {
		options = append(options, map[string]any{
			"option_id": o.OptionID,
			"name":      o.Name,
			"kind":      o.Kind,
		})
	}

	payload := map[string]any{
		"ask_id":       askID,
		"tool_call_id": ask.ToolCallID,
		"runtime":      ask.Runtime,
		"kind":         ask.Kind,
		"options":      options,
	}
	// 没有的字段就不放——放一个空串会让界面画出一个空的等宽块
	if ask.Path != "" {
		payload["path"] = ask.Path
	}
	if ask.OutOfBounds {
		payload["out_of_bounds"] = true
	}

	return b.bus.PublishWorkEvent(ctx, port.WorkEvent{
		WorkID:  ask.WorkID,
		Source:  "acp",
		Type:    "request_permission",
		Payload: payload,
	})
}
