package fake

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Kind 区分收到的是请求还是通知。
//
// 分不清的话没法断言「session/cancel 是通知不是请求」这类协议契约。
type Kind int

// 消息种类。
const (
	KindRequest Kind = iota
	KindNotification
)

// String 让断言失败时打印得出人话。
func (k Kind) String() string {
	switch k {
	case KindRequest:
		return "request"
	case KindNotification:
		return "notification"
	default:
		return fmt.Sprintf("Kind(%d)", int(k))
	}
}

// Recorded 是 Fake 收到的一条消息的完整记录。
type Recorded struct {
	// N 是接收序号，从 1 开始。
	N int
	// At 来自注入的 Clock，不是 time.Now()。
	At time.Time
	// Kind 是请求还是通知。
	Kind Kind
	// ID 在 Kind == KindRequest 时非空。
	ID json.RawMessage
	// Method 是方法名。
	Method string
	// Params 是原始参数字节，测试自己解 —— 解成什么类型是测试的事，
	// Fake 不该替它决定。
	Params json.RawMessage
}

// recorder 如实记录收到的一切。
//
// ★ **绝不去重、绝不过滤。** 去重是被测代码的职责：
// Fake 若自己去重，「连续取消两次只发一次协议请求」就永远绿，
// 这正是「测试制造虚假安全感」的典型（acp-integration.md §12.7）。
type recorder struct {
	mu       sync.Mutex
	messages []Recorded
	// waiters 是等待特定消息出现的订阅者，每次新消息到来时逐个求值。
	waiters map[chan Recorded]func(Recorded) bool
}

func newRecorder() *recorder {
	return &recorder{waiters: make(map[chan Recorded]func(Recorded) bool)}
}

func (r *recorder) record(at time.Time, kind Kind, id json.RawMessage, method string, params json.RawMessage) {
	r.mu.Lock()
	defer r.mu.Unlock()

	rec := Recorded{
		N:      len(r.messages) + 1,
		At:     at,
		Kind:   kind,
		ID:     id,
		Method: method,
		Params: params,
	}
	r.messages = append(r.messages, rec)

	for ch, pred := range r.waiters {
		if pred(rec) {
			// 缓冲为 1 且只投一次，投完立刻摘掉，不阻塞记录路径。
			ch <- rec
			delete(r.waiters, ch)
		}
	}
}

func (r *recorder) snapshot() []Recorded {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Recorded, len(r.messages))
	copy(out, r.messages)
	return out
}

func (r *recorder) countMethod(method string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, m := range r.messages {
		if m.Method == method {
			n++
		}
	}
	return n
}

func (r *recorder) waitFor(ctx context.Context, pred func(Recorded) bool) (Recorded, error) {
	r.mu.Lock()
	// 先扫已经收到的 —— 否则「等的消息在注册订阅之前就到了」会永久阻塞。
	for _, m := range r.messages {
		if pred(m) {
			r.mu.Unlock()
			return m, nil
		}
	}
	ch := make(chan Recorded, 1)
	r.waiters[ch] = pred
	r.mu.Unlock()

	select {
	case m := <-ch:
		return m, nil
	case <-ctx.Done():
		r.mu.Lock()
		delete(r.waiters, ch)
		r.mu.Unlock()
		return Recorded{}, fmt.Errorf("fake: 等待消息超时: %w", ctx.Err())
	}
}
