// Package eventbus 是事件的发布与扇出。
//
// ★ **先落库，再扇出。** 反过来会出现「前端收到了，重启后库里没有」——
// 用户看着时间线上的一条记录，重启后它消失了，而没有任何提示。
// 这种不一致比丢事件更糟：丢了至少还能说「没收到」，
// 不一致会让人怀疑自己记错了。
//
// 所以本包**不做**「为性能异步落库」（M2 的 U2.3.1 forbidden_changes）。
package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// defaultBuffer 是订阅者的默认缓冲。
//
// 给得太小，正常速度的消费者也会丢事件；给得太大，一个卡住的页面会占住
// 一大块内存。64 条约等于「界面卡了两三秒」的量。
const defaultBuffer = 64

// Event 是一条事件。字段与 api/openapi.yaml 的 Event 对应。
type Event struct {
	ID string `json:"id"`
	// Seq 由存储层在落库时发放，**不由内存计数器发**——
	// 内存计数器一重启就归零，前端按 seq 去重会把新事件当旧的丢掉。
	Seq     int64           `json:"seq"`
	WorkID  string          `json:"work_id,omitempty"`
	Source  string          `json:"source"`
	Type    string          `json:"type"`
	TS      time.Time       `json:"ts"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// Store 是事件的持久化抽象。接口定义在使用方（Go 的惯例）。
type Store interface {
	// AppendEvent 落一条事件，**并把发放的序号写回 e.Seq**。
	AppendEvent(ctx context.Context, e *Event) error
	// MaxSeq 返回已落库的最大序号，供启动时接续。
	MaxSeq(ctx context.Context) (int64, error)
}

// Subscription 是一个订阅者。
type Subscription struct {
	ch        chan Event
	bus       *Bus
	closeOnce sync.Once
}

// Events 是事件流。订阅者从这里读。
func (s *Subscription) Events() <-chan Event { return s.ch }

// Close 取消订阅。可重复调用——SSE 的清理路径可能走两遍（ctx 取消 + defer）。
func (s *Subscription) Close() {
	s.closeOnce.Do(func() {
		s.bus.remove(s)
		close(s.ch)
	})
}

// Bus 是事件总线。
type Bus struct {
	store Store

	mu   sync.RWMutex
	subs map[*Subscription]struct{}
}

// New 构造总线。
func New(store Store) *Bus {
	return &Bus{store: store, subs: make(map[*Subscription]struct{})}
}

// Publish 落库并扇出。
//
// ★ **落库失败就不扇出，并如实返回错误。** 顺序不可颠倒。
func (b *Bus) Publish(ctx context.Context, e Event) error {
	if e.TS.IsZero() {
		e.TS = time.Now()
	}

	if err := b.store.AppendEvent(ctx, &e); err != nil {
		return fmt.Errorf("落事件 %s: %w", e.Type, err)
	}

	b.mu.RLock()
	targets := make([]*Subscription, 0, len(b.subs))
	for s := range b.subs {
		targets = append(targets, s)
	}
	b.mu.RUnlock()

	for _, s := range targets {
		select {
		case s.ch <- e:
		default:
			// ★ 缓冲满了就跳过这一个，**绝不阻塞**。
			// 阻塞的话，一个卡住的页面（切到后台、断网）会让整条发布链路停住，
			// 表现成「AI 的进度整个不动了」。
			//
			// 这一条对那个订阅者是丢了，但它可以按 Last-Event-ID 重连补齐——
			// 事件已经落库了，补得回来。
		}
	}
	return nil
}

// Subscribe 订阅事件流。buffer 为 0 时用默认值。
//
// ctx 取消时自动回收——SSE 的客户端断开表现为 ctx 取消，不会有人来调 Close。
// 不回收的话，用户每开一次页面就多一个订阅者，开一天之后应用会肉眼可见地变慢。
func (b *Bus) Subscribe(ctx context.Context, buffer int) *Subscription {
	if buffer <= 0 {
		buffer = defaultBuffer
	}
	s := &Subscription{ch: make(chan Event, buffer), bus: b}

	b.mu.Lock()
	b.subs[s] = struct{}{}
	b.mu.Unlock()

	if ctx.Done() != nil {
		go func() {
			<-ctx.Done()
			s.Close()
		}()
	}
	return s
}

// SubscriberCount 是当前订阅者数量，用来在测试里证明没有泄漏。
func (b *Bus) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subs)
}

func (b *Bus) remove(s *Subscription) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.subs, s)
}
