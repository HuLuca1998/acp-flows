package eventbus_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/eventbus"
)

// U2.3.1 · 事件总线（验收点 V6）
//
// ★ 这一层最要紧的是 R5「先落库再扇出」。反过来的话会出现
// 「前端收到了，重启后库里没有」——用户看着时间线上的一条记录，
// 重启后它消失了，而没有任何提示。这种不一致比丢事件更糟：
// 丢了至少还能说「没收到」，不一致会让人怀疑自己记错了。

// recordingStore 记下落库顺序，并能按需失败。
type recordingStore struct {
	mu     sync.Mutex
	saved  []eventbus.Event
	failOn string // 这个 type 的事件落库失败
	nextID int64
}

func (s *recordingStore) AppendEvent(_ context.Context, e *eventbus.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failOn != "" && e.Type == s.failOn {
		return errors.New("disk full")
	}
	s.nextID++
	e.Seq = s.nextID // 序号由存储层发，保证跨重启连续
	s.saved = append(s.saved, *e)
	return nil
}

func (s *recordingStore) MaxSeq(context.Context) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.nextID, nil
}

func (s *recordingStore) snapshot() []eventbus.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]eventbus.Event(nil), s.saved...)
}

func newEvent(typ string) eventbus.Event {
	return eventbus.Event{WorkID: "work-01", Source: "acp", Type: typ}
}

// ★★ R5：**落库失败就不扇出**。
func TestPublish_DoesNotFanOutWhenStoreFails(t *testing.T) {
	store := &recordingStore{failOn: "tool_call"}
	bus := eventbus.New(store)

	sub := bus.Subscribe(context.Background(), 0)
	defer sub.Close()

	err := bus.Publish(context.Background(), newEvent("tool_call"))
	if err == nil {
		t.Fatal("落库失败却报成功了")
	}

	select {
	case e := <-sub.Events():
		t.Errorf("落库失败却扇出了 %+v——前端会显示一条重启后就消失的记录", e)
	case <-time.After(100 * time.Millisecond):
	}
}

// R1：序号单调递增、无洞。
func TestPublish_SeqIsMonotonic(t *testing.T) {
	store := &recordingStore{}
	bus := eventbus.New(store)

	for range 20 {
		if err := bus.Publish(context.Background(), newEvent("message_chunk")); err != nil {
			t.Fatal(err)
		}
	}

	saved := store.snapshot()
	if len(saved) != 20 {
		t.Fatalf("落了 %d 条，想要 20 条", len(saved))
	}
	for i, e := range saved {
		if e.Seq != int64(i+1) {
			t.Fatalf("第 %d 条的 seq 是 %d，序号有洞或回退了", i, e.Seq)
		}
	}
}

// R1 的另一半：**跨重启连续**。
//
// 新 bus 接上同一个存储，序号要接着往下发而不是从 1 重来。
// 从 1 重来的话，前端按 seq 去重会把新事件当成旧的丢掉。
func TestPublish_SeqContinuesAcrossRestart(t *testing.T) {
	store := &recordingStore{}

	first := eventbus.New(store)
	for range 5 {
		if err := first.Publish(context.Background(), newEvent("message_chunk")); err != nil {
			t.Fatal(err)
		}
	}

	// 换一个 bus，模拟 duetd 重启
	second := eventbus.New(store)
	if err := second.Publish(context.Background(), newEvent("message_chunk")); err != nil {
		t.Fatal(err)
	}

	saved := store.snapshot()
	last := saved[len(saved)-1]
	if last.Seq != 6 {
		t.Errorf("重启后的第一条 seq = %d，想要 6——从 1 重来的话前端会把新事件当旧的丢掉", last.Seq)
	}
}

// R3：客户端断开时订阅者被回收。**防泄漏。**
//
// 不回收的话，用户每开一次页面就多一个订阅者，而每条事件都要往所有订阅者
// 投递一遍——开一天之后应用会肉眼可见地变慢。
func TestSubscribe_ClosingReleasesSubscriber(t *testing.T) {
	bus := eventbus.New(&recordingStore{})

	a := bus.Subscribe(context.Background(), 0)
	b := bus.Subscribe(context.Background(), 0)
	if n := bus.SubscriberCount(); n != 2 {
		t.Fatalf("订阅者 = %d，想要 2", n)
	}

	a.Close()
	if n := bus.SubscriberCount(); n != 1 {
		t.Errorf("关掉一个之后订阅者 = %d，想要 1", n)
	}

	b.Close()
	if n := bus.SubscriberCount(); n != 0 {
		t.Errorf("全部关掉后订阅者 = %d，想要 0——泄漏了", n)
	}
}

// ctx 取消也要回收——SSE 的客户端断开表现为 ctx 取消，不会有人来调 Close。
func TestSubscribe_ContextCancelReleasesSubscriber(t *testing.T) {
	bus := eventbus.New(&recordingStore{})

	ctx, cancel := context.WithCancel(context.Background())
	bus.Subscribe(ctx, 0)
	if n := bus.SubscriberCount(); n != 1 {
		t.Fatalf("订阅者 = %d，想要 1", n)
	}

	cancel()

	deadline := time.After(2 * time.Second)
	for bus.SubscriberCount() != 0 {
		select {
		case <-deadline:
			t.Fatal("ctx 取消后订阅者没被回收——SSE 客户端断开就是这个路径，会一直泄漏")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// ★ R4：**慢消费者不阻塞其他人**。
//
// 一个前端页面卡住（切到后台、断网），不该让其他页面也收不到事件，
// 更不该卡住整个发布链路——那会让 AI 的进度整个停下来。
func TestPublish_SlowSubscriberDoesNotBlockOthers(t *testing.T) {
	bus := eventbus.New(&recordingStore{})

	slow := bus.Subscribe(context.Background(), 1) // 缓冲 1，故意不读
	defer slow.Close()
	fast := bus.Subscribe(context.Background(), 64)
	defer fast.Close()

	const n = 20
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range n {
			// 发布不能被慢消费者卡住
			_ = bus.Publish(context.Background(), newEvent("message_chunk"))
		}
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("发布被慢消费者卡住了——一个卡住的页面会让 AI 的进度整个停下来")
	}

	// 快的那个要收到全部
	got := 0
	for range n {
		select {
		case <-fast.Events():
			got++
		case <-time.After(500 * time.Millisecond):
		}
	}
	if got != n {
		t.Errorf("正常订阅者只收到 %d/%d 条", got, n)
	}
}

// 扇出给所有订阅者，不是只给第一个。
func TestPublish_FansOutToAllSubscribers(t *testing.T) {
	bus := eventbus.New(&recordingStore{})

	subs := make([]*eventbus.Subscription, 3)
	for i := range subs {
		subs[i] = bus.Subscribe(context.Background(), 8)
		defer subs[i].Close()
	}

	if err := bus.Publish(context.Background(), newEvent("state_change")); err != nil {
		t.Fatal(err)
	}

	for i, s := range subs {
		select {
		case e := <-s.Events():
			if e.Type != "state_change" {
				t.Errorf("订阅者 %d 收到 %q", i, e.Type)
			}
		case <-time.After(500 * time.Millisecond):
			t.Errorf("订阅者 %d 什么也没收到", i)
		}
	}
}

// 关掉之后再关一次不 panic——SSE 的清理路径可能走两遍（ctx 取消 + defer）。
func TestSubscription_CloseIsIdempotent(t *testing.T) {
	bus := eventbus.New(&recordingStore{})
	s := bus.Subscribe(context.Background(), 0)

	s.Close()
	s.Close() // 不能 panic

	// 顺带断言它真的被摘掉了——只验「没 panic」的话，
	// 一个什么都不做的 Close 也能让这条测试绿。
	if n := bus.SubscriberCount(); n != 0 {
		t.Errorf("重复 Close 之后订阅者 = %d，想要 0", n)
	}
}
