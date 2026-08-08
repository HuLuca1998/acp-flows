package permission_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/permission"
	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
)

// U3.1.4 · 权限请求接线（验收点 V8）
//
// ★ Broker 是「Agent 在等」与「用户还没点」之间那个中转站：
// 会话线程调 Ask 挂住，HTTP 线程调 Answer 把它叫醒。
//
// 这里几乎每条断言都在守同一件事：**没人替用户做决定**。

type recordingBus struct {
	mu     sync.Mutex
	events []port.WorkEvent
	err    error
}

func (b *recordingBus) PublishWorkEvent(_ context.Context, e port.WorkEvent) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, e)
	return b.err
}

func (b *recordingBus) snapshot() []port.WorkEvent {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]port.WorkEvent(nil), b.events...)
}

type seqIDs struct {
	mu sync.Mutex
	n  int
}

func (g *seqIDs) NextID(prefix string) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.n++
	return prefix + "-0" + string(rune('0'+g.n))
}
func (g *seqIDs) NextULID() string { return g.NextID("ulid") }

func newBroker(bus port.WorkEventBus) *permission.Broker {
	return permission.New(bus, &seqIDs{})
}

func sampleAsk() permission.Ask {
	return permission.Ask{
		WorkID:     "work-01",
		ToolCallID: "tool-1",
		Runtime:    "codex",
		Kind:       "edit",
		Path:       "src/main.go",
		Options: []permission.Option{
			{OptionID: "opt-allow", Name: "允许一次", Kind: "allow_once"},
			{OptionID: "opt-deny", Name: "拒绝", Kind: "reject_once"},
		},
	}
}

// 起一个后台 Ask，返回它的结果通道。
func startAsk(ctx context.Context, b *permission.Broker, ask permission.Ask) chan permission.Result {
	out := make(chan permission.Result, 1)
	go func() {
		id, err := b.Ask(ctx, ask)
		out <- permission.Result{OptionID: id, Err: err}
	}()
	return out
}

// ★★ R1：权限请求要**作为事件发出去**，载荷里带齐界面要用的东西。
//
// 只放在内存里的话，界面永远不知道有人在等它——用户对着一个不动的
// 时间线，而 AI 挂在那儿。
func TestAsk_R1_PublishesEventWithOptions(t *testing.T) {
	bus := &recordingBus{}
	b := newBroker(bus)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startAsk(ctx, b, sampleAsk())

	waitFor(t, "权限请求没发成事件——界面永远不知道有人在等它", func() bool {
		return len(bus.snapshot()) > 0
	})

	e := bus.snapshot()[0]
	if e.Type != "request_permission" {
		t.Fatalf("事件类型 = %q, 想要 request_permission", e.Type)
	}
	if e.WorkID != "work-01" {
		t.Errorf("work_id = %q——前端靠它过滤，不带的话会串台", e.WorkID)
	}
	for _, k := range []string{"ask_id", "tool_call_id", "runtime", "options"} {
		if _, ok := e.Payload[k]; !ok {
			t.Errorf("载荷缺 %q——界面照着它渲染按钮", k)
		}
	}
	opts, _ := e.Payload["options"].([]any)
	if len(opts) != 2 {
		t.Errorf("载荷里有 %d 个选项, 想要 2——少一个用户就少一个选择", len(opts))
	}
}

// ★★ R2：用户选的 optionId **原样交回给等着的那一方**。
func TestAsk_R2_AnswerReturnsTheChosenOptionVerbatim(t *testing.T) {
	bus := &recordingBus{}
	b := newBroker(bus)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := startAsk(ctx, b, sampleAsk())

	askID := waitAskID(t, bus)
	if err := b.Answer("work-01", askID, "opt-deny"); err != nil {
		t.Fatalf("应答失败: %v", err)
	}

	select {
	case got := <-out:
		if got.Err != nil {
			t.Fatalf("Ask 返回了错误: %v", got.Err)
		}
		if got.OptionID != "opt-deny" {
			t.Errorf("交回的 optionId = %q, 想要 opt-deny——"+
				"中间有人按类别重新匹配了", got.OptionID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("应答之后 Ask 还挂着")
	}
}

// ★★ R4：同一条请求**只能应答一次**。
//
// 重复应答要报错，而且**不能把第二个值交给等着的那一方**——
// Agent 那边只在等一个响应，多发的会被当成不认识的请求而静静丢弃。
func TestAnswer_R4_SecondAnswerIsRejected(t *testing.T) {
	bus := &recordingBus{}
	b := newBroker(bus)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := startAsk(ctx, b, sampleAsk())

	askID := waitAskID(t, bus)
	if err := b.Answer("work-01", askID, "opt-deny"); err != nil {
		t.Fatalf("第一次应答失败: %v", err)
	}

	err := b.Answer("work-01", askID, "opt-allow")
	if err == nil {
		t.Fatal("同一条请求被应答了两次却没报错")
	}
	if !errors.Is(err, permission.ErrNotPending) {
		t.Errorf("错误 = %v, 想要 ErrNotPending", err)
	}

	got := <-out
	if got.OptionID != "opt-deny" {
		t.Errorf("交回的是 %q——第二次应答覆盖了第一次，"+
			"用户点「拒绝」的结果被后来的「允许」顶掉了", got.OptionID)
	}
}

// 应答一条不存在的请求要报错，而不是静静成功。
//
// 静静成功的话，界面以为处理完了，而 AI 那边还在等。
func TestAnswer_UnknownAskIsRejected(t *testing.T) {
	b := newBroker(&recordingBus{})

	if err := b.Answer("work-01", "ask-nope", "opt-allow"); !errors.Is(err, permission.ErrNotPending) {
		t.Errorf("应答不存在的请求得到 %v, 想要 ErrNotPending——"+
			"静静成功的话界面以为处理完了，而 AI 还在等", err)
	}
}

// ★ 别的工作的 ask_id 不能拿来应答这个工作的请求。
//
// 不校验的话，两个工作同时开着时，一次误点会应答到另一个工作头上。
func TestAnswer_WrongWorkIsRejected(t *testing.T) {
	bus := &recordingBus{}
	b := newBroker(bus)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startAsk(ctx, b, sampleAsk())
	askID := waitAskID(t, bus)

	if err := b.Answer("work-99", askID, "opt-allow"); !errors.Is(err, permission.ErrNotPending) {
		t.Errorf("用别的工作的 id 应答成功了（%v）——"+
			"两个工作同时开着时，一次误点会应答到另一个头上", err)
	}
}

// ★★ R5：取消一个工作时，它 pending 的请求**全部**被叫醒。
//
// 漏一条的话，那条会话永远挂着——进程退不出去，而用户什么都看不到。
func TestCancelWork_R5_ReleasesEveryPendingAsk(t *testing.T) {
	bus := &recordingBus{}
	b := newBroker(bus)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	outs := []chan permission.Result{}
	for i := 0; i < 3; i++ {
		ask := sampleAsk()
		ask.ToolCallID = "tool-" + string(rune('a'+i))
		outs = append(outs, startAsk(ctx, b, ask))
	}
	waitFor(t, "三条请求没都发出来", func() bool { return len(bus.snapshot()) >= 3 })

	b.CancelWork("work-01")

	for i, out := range outs {
		select {
		case got := <-out:
			if got.Err == nil {
				t.Errorf("第 %d 条被取消了却返回成功（%q）", i, got.OptionID)
			}
			if !errors.Is(got.Err, permission.ErrCancelled) {
				t.Errorf("第 %d 条的错误 = %v, 想要 ErrCancelled", i, got.Err)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("第 %d 条没被叫醒——那条会话永远挂着，进程退不出去", i)
		}
	}
}

// 取消一个工作**不影响别的工作**——它们各等各的。
func TestCancelWork_DoesNotTouchOtherWorks(t *testing.T) {
	bus := &recordingBus{}
	b := newBroker(bus)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mine := sampleAsk()
	other := sampleAsk()
	other.WorkID = "work-02"
	startAsk(ctx, b, mine)
	otherOut := startAsk(ctx, b, other)

	waitFor(t, "两条请求没都发出来", func() bool { return len(bus.snapshot()) >= 2 })
	b.CancelWork("work-01")

	select {
	case got := <-otherOut:
		t.Fatalf("取消 work-01 把 work-02 的请求也解除了（%+v）——"+
			"用户取消一个工作，另一个跟着废了", got)
	case <-time.After(300 * time.Millisecond):
	}
}

// ★ ctx 结束时也要解除等待，不能永久挂着。
func TestAsk_ContextCancelReleases(t *testing.T) {
	bus := &recordingBus{}
	b := newBroker(bus)

	ctx, cancel := context.WithCancel(context.Background())
	out := startAsk(ctx, b, sampleAsk())
	waitFor(t, "请求没发出来", func() bool { return len(bus.snapshot()) > 0 })

	cancel()

	select {
	case got := <-out:
		if got.Err == nil {
			t.Errorf("ctx 取消了却返回成功（%q）", got.OptionID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ctx 取消之后 Ask 还挂着")
	}
}

// ★★ 事件发不出去时**不许挂起等待**。
//
// 挂起的话，界面根本不知道有这条请求（事件没发出去），而 AI 一直等——
// 用户对着一个不动的界面，没有任何提示。这时候直接失败更诚实。
func TestAsk_BusFailureDoesNotHang(t *testing.T) {
	bus := &recordingBus{err: errors.New("数据库锁住了")}
	b := newBroker(bus)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := startAsk(ctx, b, sampleAsk())

	select {
	case got := <-out:
		if got.Err == nil {
			t.Error("事件没发出去却当成功处理了")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("事件发不出去，Ask 却挂在那儿——" +
			"界面不知道有这条请求，而 AI 一直等，用户什么提示都没有")
	}
}

// Pending 能查——界面刷新后要重新画出待裁决的卡片。
func TestPending_ListsOpenAsksForWork(t *testing.T) {
	bus := &recordingBus{}
	b := newBroker(bus)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startAsk(ctx, b, sampleAsk())
	askID := waitAskID(t, bus)

	if got := b.Pending("work-01"); len(got) != 1 || got[0].AskID != askID {
		t.Fatalf("Pending = %+v, 想要一条 %s", got, askID)
	}
	if got := b.Pending("work-02"); len(got) != 0 {
		t.Errorf("别的工作的 Pending 里混进了 %d 条", len(got))
	}

	_ = b.Answer("work-01", askID, "opt-allow")
	if got := b.Pending("work-01"); len(got) != 0 {
		t.Errorf("应答之后还留在 Pending 里（%d 条）——界面会重复画出卡片", len(got))
	}
}

// ── 小工具 ───────────────────────────────────────────────

func waitFor(t *testing.T, why string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("等超时了：%s", why)
}

func waitAskID(t *testing.T, bus *recordingBus) string {
	t.Helper()
	var id string
	waitFor(t, "权限请求事件没发出来", func() bool {
		for _, e := range bus.snapshot() {
			if s, ok := e.Payload["ask_id"].(string); ok && s != "" {
				id = s
				return true
			}
		}
		return false
	})
	return id
}
