package api_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/api"
	"github.com/HuLuca1998/acp-flows/backend/internal/eventbus"
)

// U2.3.1 · SSE 事件流（验收点 V6）
//
// ★ 这里用**真的 HTTP 服务器**（httptest.NewServer）而不是 ResponseRecorder：
// SSE 是长连接 + 流式写，Recorder 拿到的是「全部写完之后」的结果，
// 那正好把「边发边到」这件事测没了。

// stubEvents 是补发用的历史事件源。
type stubEvents struct {
	all []eventbus.Event
	// asked 记下被问过的游标，用来断言「只补没收到的」
	asked []int64
}

func (s *stubEvents) EventsAfter(_ context.Context, after int64, limit int) ([]eventbus.Event, error) {
	s.asked = append(s.asked, after)
	out := make([]eventbus.Event, 0, limit)
	for _, e := range s.all {
		if e.Seq > after && len(out) < limit {
			out = append(out, e)
		}
	}
	return out, nil
}

type nopStore struct{ seq int64 }

func (s *nopStore) AppendEvent(_ context.Context, e *eventbus.Event) error {
	s.seq++
	e.Seq = s.seq
	return nil
}
func (s *nopStore) MaxSeq(context.Context) (int64, error) { return s.seq, nil }

func newEventServer(t *testing.T, bus *eventbus.Bus, hist *stubEvents) *httptest.Server {
	t.Helper()
	h, err := api.NewRouter(api.Config{
		Token: testToken, Version: "1.4.2",
		Bus: bus, EventHistory: hist,
	})
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

// sseFrame 是解析出来的一条 SSE 消息。
type sseFrame struct {
	id    string
	event string
	data  string
}

// readFrames 从流里读 n 条 SSE 消息。
func readFrames(t *testing.T, r *bufio.Reader, n int) []sseFrame {
	t.Helper()
	var out []sseFrame
	var cur sseFrame

	for len(out) < n {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatalf("只读到 %d/%d 条就断了: %v", len(out), n, err)
		}
		line = strings.TrimRight(line, "\r\n")

		switch {
		case line == "":
			if cur.data != "" {
				out = append(out, cur)
				cur = sseFrame{}
			}
		case strings.HasPrefix(line, "id: "):
			cur.id = strings.TrimPrefix(line, "id: ")
		case strings.HasPrefix(line, "event: "):
			cur.event = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			cur.data = strings.TrimPrefix(line, "data: ")
		}
	}
	return out
}

func openStream(t *testing.T, srv *httptest.Server, lastEventID string) (*http.Response, *bufio.Reader) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, srv.URL+"/v1/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+testToken)
	if lastEventID != "" {
		req.Header.Set("Last-Event-ID", lastEventID)
	}

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("连接失败: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp, bufio.NewReader(resp.Body)
}

// SSE 的响应头必须对，否则浏览器的 EventSource 根本不会当成事件流。
func TestStreamEvents_SetsSSEHeaders(t *testing.T) {
	srv := newEventServer(t, eventbus.New(&nopStore{}), &stubEvents{})
	resp, _ := openStream(t, srv, "")

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Errorf("Content-Type = %q，浏览器不会把它当事件流", ct)
	}
	// 没有 no-cache 的话，中间层可能缓存整条流——用户看到的是几分钟前的进度
	if cc := resp.Header.Get("Cache-Control"); !strings.Contains(cc, "no-cache") {
		t.Errorf("Cache-Control = %q，缺 no-cache", cc)
	}
}

// ★ 真流式：事件发出来就该到，不是等连接关闭才一起吐。
func TestStreamEvents_DeliversLive(t *testing.T) {
	bus := eventbus.New(&nopStore{})
	srv := newEventServer(t, bus, &stubEvents{})
	_, r := openStream(t, srv, "")

	// 等订阅真的建立起来——立刻 Publish 的话事件会发在订阅之前，
	// 那时测的是「有没有补发」而不是「实时投递」
	deadline := time.After(2 * time.Second)
	for bus.SubscriberCount() == 0 {
		select {
		case <-deadline:
			t.Fatal("SSE 连上了但没订阅")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	if err := bus.Publish(context.Background(), eventbus.Event{
		Source: "acp", Type: "message_chunk",
		Payload: json.RawMessage(`{"text":"你好"}`),
	}); err != nil {
		t.Fatal(err)
	}

	frames := readFrames(t, r, 1)
	if frames[0].event != "message_chunk" {
		t.Errorf("event = %q", frames[0].event)
	}
	if !strings.Contains(frames[0].data, "你好") {
		t.Errorf("data = %q", frames[0].data)
	}
	// id 必须是 seq：浏览器靠它维护 Last-Event-ID
	if frames[0].id != "1" {
		t.Errorf("id = %q，想要 seq(1)——浏览器靠这个字段维护 Last-Event-ID", frames[0].id)
	}
}

// ★★ R2：带 Last-Event-ID 重连时**只补它之后的**。
func TestStreamEvents_ResumesFromLastEventID(t *testing.T) {
	hist := &stubEvents{all: []eventbus.Event{
		{Seq: 1, Source: "acp", Type: "message_chunk"},
		{Seq: 2, Source: "acp", Type: "message_chunk"},
		{Seq: 3, Source: "acp", Type: "tool_call"},
		{Seq: 4, Source: "app", Type: "state_change"},
	}}
	srv := newEventServer(t, eventbus.New(&nopStore{}), hist)

	_, r := openStream(t, srv, "2")

	frames := readFrames(t, r, 2)
	if frames[0].id != "3" {
		t.Errorf("首条补发的 id = %q，想要 3——从 2 恢复时不该把第 2 条再发一遍", frames[0].id)
	}
	if frames[1].id != "4" {
		t.Errorf("第二条 id = %q，想要 4", frames[1].id)
	}

	if len(hist.asked) == 0 || hist.asked[0] != 2 {
		t.Errorf("问历史时用的游标是 %v，想要 2", hist.asked)
	}
}

// 全新连接不补历史——它要的是「从现在开始」，不是把过去重放一遍。
func TestStreamEvents_FreshConnectionDoesNotReplayHistory(t *testing.T) {
	hist := &stubEvents{all: []eventbus.Event{
		{Seq: 1, Source: "acp", Type: "message_chunk"},
		{Seq: 2, Source: "acp", Type: "message_chunk"},
	}}
	bus := eventbus.New(&nopStore{})
	srv := newEventServer(t, bus, hist)

	_, r := openStream(t, srv, "") // 不带 Last-Event-ID

	deadline := time.After(2 * time.Second)
	for bus.SubscriberCount() == 0 {
		select {
		case <-deadline:
			t.Fatal("没订阅")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	if err := bus.Publish(context.Background(), eventbus.Event{
		Source: "app", Type: "state_change",
	}); err != nil {
		t.Fatal(err)
	}

	// 第一条应当是刚发的那条（seq 由 nopStore 从 1 开始发），
	// 而不是历史里的
	frames := readFrames(t, r, 1)
	if frames[0].event != "state_change" {
		t.Errorf("全新连接却先收到 %q——历史被重放了", frames[0].event)
	}
}

// ★ R3：客户端断开时订阅者被回收。**防泄漏。**
func TestStreamEvents_DisconnectReleasesSubscriber(t *testing.T) {
	bus := eventbus.New(&nopStore{})
	srv := newEventServer(t, bus, &stubEvents{})

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/v1/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+testToken)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.After(2 * time.Second)
	for bus.SubscriberCount() == 0 {
		select {
		case <-deadline:
			t.Fatal("没订阅")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	_ = resp.Body.Close() // 客户端断开

	deadline = time.After(3 * time.Second)
	for bus.SubscriberCount() != 0 {
		select {
		case <-deadline:
			t.Fatal("客户端断开后订阅者没被回收——用户每开一次页面就多一个，开一天应用会明显变慢")
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}
}

// 无 token 一律 401，事件流也不例外——它能看到用户项目里发生的一切。
func TestStreamEvents_RequiresToken(t *testing.T) {
	srv := newEventServer(t, eventbus.New(&nopStore{}), &stubEvents{})

	resp, err := srv.Client().Get(srv.URL + "/v1/events")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("无 token 却回了 %d", resp.StatusCode)
	}
}

// Last-Event-ID 不是数字时按全新连接处理，不能 500。
// 中间层或用户手工重放请求时很容易带上乱七八糟的值。
func TestStreamEvents_TolerantToBadLastEventID(t *testing.T) {
	bus := eventbus.New(&nopStore{})
	srv := newEventServer(t, bus, &stubEvents{})

	resp, _ := openStream(t, srv, "not-a-number")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("非法 Last-Event-ID 回了 %d，应当按全新连接处理", resp.StatusCode)
	}
}
