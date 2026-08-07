package jsonrpc_test

// ndjson 编解码与双向路由（原 U0.2.1，编号已废弃）
//
// 全部对着 io.Pipe 做，**不起子进程** —— 传输层不该知道进程的存在。
// 原单元编号 U0.2.1（已废弃）。传输层服务于 M2/M3，见 docs/plan/roadmap.md。

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/acp/jsonrpc"
)

// peer 把一个 Conn 接到内存管道上，另一端由测试扮演「对方」。
//
// ★ 输出必须由后台 goroutine 持续抽干。io.Pipe 是同步的——
// 没人读时 Conn 的写会一直阻塞，测试就会挂死而不是失败。
// 这个坑踩过一次：R5（超时）与 R3（通知不阻塞）两条同时挂住。
type peer struct {
	conn   *jsonrpc.Conn
	toUs   *io.PipeWriter      // 测试往这里写 → Conn 读到
	frames chan map[string]any // Conn 写出的帧，已解析
}

func newPeer(t *testing.T, handler jsonrpc.Handler) *peer {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()

	p := &peer{
		conn:   jsonrpc.New(inR, outW, handler),
		toUs:   inW,
		frames: make(chan map[string]any, 32),
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = p.conn.Serve(ctx) }()

	// 后台抽干输出，解析成帧丢进 channel
	go func() {
		sc := bufio.NewScanner(outR)
		sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for sc.Scan() {
			var m map[string]any
			if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
				continue
			}
			select {
			case p.frames <- m:
			case <-ctx.Done():
				return
			}
		}
	}()

	t.Cleanup(func() {
		cancel()
		_ = inW.Close()
		_ = outW.Close()
	})
	return p
}

// readFrame 取一帧。超时即失败——挂死的测试比失败的测试难排查得多。
func (p *peer) readFrame(t *testing.T) map[string]any {
	t.Helper()
	select {
	case m := <-p.frames:
		return m
	case <-time.After(3 * time.Second):
		t.Fatal("等待帧超时")
		return nil
	}
}

func (p *peer) write(t *testing.T, raw string) {
	t.Helper()
	if _, err := io.WriteString(p.toUs, raw+"\n"); err != nil {
		t.Fatalf("写入失败: %v", err)
	}
}

// R1 ★ 换行分帧：单行完整 JSON，内容里的换行必须被转义。
//
// ACP 用的是 ndjson，**不是** LSP 的 Content-Length 分帧。
// 一条消息跨了两行，对方就再也解析不出来了。
func TestConn_R1_NewlineDelimited(t *testing.T) {
	p := newPeer(t, nil)

	go func() {
		_, _ = p.conn.Call(context.Background(), "test/echo",
			map[string]string{"text": "第一行\n第二行\n第三行"})
	}()

	m := p.readFrame(t)

	// 帧能被逐行解析出来，本身就证明分帧没被内容里的换行破坏
	// （后台 scanner 是按 \n 切的）。再确认内容确实被转义保留了。
	params, _ := m["params"].(map[string]any)
	if text, _ := params["text"].(string); !strings.Contains(text, "\n第二行") {
		t.Errorf("内容里的换行没被正确转义保留: %q", text)
	}
	if m["jsonrpc"] != "2.0" {
		t.Errorf("jsonrpc = %v, want 2.0", m["jsonrpc"])
	}
	if m["method"] != "test/echo" {
		t.Errorf("method = %v, want test/echo", m["method"])
	}
	if m["id"] == nil {
		t.Error("请求缺少 id")
	}
}

// R2 ★ 乱序响应能按 id 正确归位。
//
// ACP 的 agent 不保证按请求顺序回复，按顺序配对必错。
func TestConn_R2_OutOfOrderResponses(t *testing.T) {
	p := newPeer(t, nil)

	type result struct {
		n   int
		err error
	}
	results := make(chan result, 3)
	var wg sync.WaitGroup

	for i := 1; i <= 3; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			var out struct {
				N int `json:"n"`
			}
			err := p.conn.CallInto(context.Background(), "test/n", map[string]int{"n": n}, &out)
			results <- result{n: out.N, err: err}
		}(i)
	}

	// 收集三个请求的 id，然后**逆序**回复
	ids := make([]any, 0, 3)
	nToID := map[float64]any{}
	for range 3 {
		f := p.readFrame(t)
		ids = append(ids, f["id"])
		params, _ := f["params"].(map[string]any)
		n, _ := params["n"].(float64)
		nToID[n] = f["id"]
	}
	for _, id := range slices.Backward(ids) {
		var n float64
		for k, v := range nToID {
			if v == id {
				n = k
			}
		}
		body, _ := json.Marshal(map[string]any{
			"jsonrpc": "2.0", "id": id, "result": map[string]any{"n": n},
		})
		p.write(t, string(body))
	}

	wg.Wait()
	close(results)

	seen := map[int]bool{}
	for r := range results {
		if r.err != nil {
			t.Fatalf("调用失败: %v", r.err)
		}
		seen[r.n] = true
	}
	for i := 1; i <= 3; i++ {
		if !seen[i] {
			t.Errorf("第 %d 个请求没拿到自己的响应——响应按顺序配对了", i)
		}
	}
}

// R3 · 通知不带 id、不等响应。
func TestConn_R3_NotifyHasNoID(t *testing.T) {
	p := newPeer(t, nil)

	done := make(chan struct{})
	go func() {
		_ = p.conn.Notify(context.Background(), "session/cancel", map[string]string{"sessionId": "s1"})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Notify 阻塞了——它不该等响应")
	}

	f := p.readFrame(t)
	if _, has := f["id"]; has {
		t.Error("通知带了 id")
	}
	if f["method"] != "session/cancel" {
		t.Errorf("method = %v", f["method"])
	}
}

// R4 ★ 反向请求能路由到注册的 handler。
//
// ACP 是双向的：agent 会反过来调我们（request_permission / fs 读写）。
// 不实现这条，agent 会吃到 -32601，整轮可能失败。
func TestConn_R4_HandlesIncomingRequest(t *testing.T) {
	var gotMethod string
	handler := jsonrpc.HandlerFunc(func(_ context.Context, method string, params json.RawMessage) (any, error) {
		gotMethod = method
		var p struct {
			SessionID string `json:"sessionId"`
		}
		_ = json.Unmarshal(params, &p)
		return map[string]any{"outcome": map[string]string{"outcome": "cancelled"}}, nil
	})
	p := newPeer(t, handler)

	p.write(t, `{"jsonrpc":"2.0","id":0,"method":"session/request_permission","params":{"sessionId":"s1"}}`)

	f := p.readFrame(t)
	if gotMethod != "session/request_permission" {
		t.Errorf("handler 收到的 method = %q", gotMethod)
	}
	if f["id"] != float64(0) {
		t.Errorf("响应 id = %v, want 0（注意 ACP 的 id 从 0 开始）", f["id"])
	}
	res, _ := f["result"].(map[string]any)
	outcome, _ := res["outcome"].(map[string]any)
	if outcome["outcome"] != "cancelled" {
		t.Errorf("result 不对: %v", f["result"])
	}
}

// 没有注册 handler 时，反向请求要回 -32601 而不是静默丢弃。
func TestConn_UnhandledIncomingReturnsMethodNotFound(t *testing.T) {
	p := newPeer(t, nil)

	p.write(t, `{"jsonrpc":"2.0","id":7,"method":"fs/read_text_file","params":{}}`)

	f := p.readFrame(t)
	e, ok := f["error"].(map[string]any)
	if !ok {
		t.Fatalf("期望 error 响应，得到: %v", f)
	}
	if e["code"] != float64(-32601) {
		t.Errorf("code = %v, want -32601 (method not found)", e["code"])
	}
}

// R5 · 超时后请求被取消，且不泄漏 goroutine。
func TestConn_R5_ContextCancellation(t *testing.T) {
	p := newPeer(t, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// 对方永不回复
	_, err := p.conn.Call(ctx, "test/never", nil)
	if err == nil {
		t.Fatal("对方不回复时 Call 竟然返回了 nil error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("错误类型不符: %v", err)
	}
}

// R6 · 收到非法 JSON 不致命，记录后继续处理后续消息。
func TestConn_R6_MalformedLineDoesNotKillConnection(t *testing.T) {
	p := newPeer(t, jsonrpc.HandlerFunc(func(context.Context, string, json.RawMessage) (any, error) {
		return map[string]string{"ok": "yes"}, nil
	}))

	p.write(t, `{这不是合法 JSON`)
	p.write(t, `{"jsonrpc":"2.0","id":1,"method":"still/works","params":{}}`)

	f := p.readFrame(t)
	if f["id"] != float64(1) {
		t.Fatalf("非法 JSON 之后的消息没被处理，收到: %v", f)
	}
}

// 对方回 error 时，Call 要返回可辨识的 *jsonrpc.Error。
func TestConn_PropagatesRemoteError(t *testing.T) {
	p := newPeer(t, nil)

	go func() {
		f := p.readFrame(t)
		body, _ := json.Marshal(map[string]any{
			"jsonrpc": "2.0", "id": f["id"],
			"error": map[string]any{"code": -32000, "message": "authentication required"},
		})
		p.write(t, string(body))
	}()

	_, err := p.conn.Call(context.Background(), "session/new", nil)
	if err == nil {
		t.Fatal("远端返回 error，Call 却成功了")
	}
	var rpcErr *jsonrpc.Error
	if !errors.As(err, &rpcErr) {
		t.Fatalf("错误类型不是 *jsonrpc.Error: %T %v", err, err)
	}
	if rpcErr.Code != -32000 {
		t.Errorf("code = %d, want -32000（ACP 的认证错误码）", rpcErr.Code)
	}
	if !strings.Contains(err.Error(), "authentication required") {
		t.Errorf("错误信息丢了远端的 message: %v", err)
	}
}

// ── 反向通知（对方 → 我们）────────────────────────────────────
//
// ★ 这条链路承载 ACP 的 `session/update` —— **全部流式事件都是通知**。
// 它断了的症状不是报错，而是「界面什么都不显示」，
// 而 Call/Notify 的测试全绿——所以必须单独钉死。

// R7 ★ 收到通知时路由到 handler，且**不回响应**。
//
// 通知没有 id，按 JSON-RPC 规范不得回复。回了会让对方的
// 「无人认领的响应」告警一直响，严重时把它的 pending 表搞乱。
func TestConn_R7_IncomingNotificationRoutedAndNotAnswered(t *testing.T) {
	type call struct {
		method string
		params string
	}
	got := make(chan call, 4)

	p := newPeer(t, jsonrpc.HandlerFunc(
		func(_ context.Context, method string, params json.RawMessage) (any, error) {
			got <- call{method: method, params: string(params)}
			return "这个返回值必须被丢掉", nil
		}))

	p.write(t, `{"jsonrpc":"2.0","method":"session/update","params":{"sessionUpdate":"agent_message_chunk"}}`)

	select {
	case c := <-got:
		if c.method != "session/update" {
			t.Errorf("method = %q, want session/update", c.method)
		}
		if !strings.Contains(c.params, "agent_message_chunk") {
			t.Errorf("params 没传到 handler: %s", c.params)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("通知没有被路由到 handler —— 整个事件流会是死的")
	}

	// 不能有任何回帧。给足时间让错误的实现把响应写出来。
	select {
	case m := <-p.frames:
		t.Fatalf("通知被回了响应，违反 JSON-RPC 规范: %v", m)
	case <-time.After(300 * time.Millisecond):
	}
}

// R8 · handler 处理通知出错时不能打断连接。
//
// 一条通知处理失败就断连，等于让一个渲染不了的事件干掉整轮会话。
func TestConn_R8_NotificationHandlerErrorDoesNotKillConn(t *testing.T) {
	p := newPeer(t, jsonrpc.HandlerFunc(
		func(_ context.Context, method string, _ json.RawMessage) (any, error) {
			if method == "session/update" {
				return nil, errors.New("渲染不了这个事件")
			}
			return "ok", nil
		}))

	p.write(t, `{"jsonrpc":"2.0","method":"session/update","params":{}}`)

	// 连接仍然可用：紧接着发一个反向请求，必须能正常回
	p.write(t, `{"jsonrpc":"2.0","id":7,"method":"fs/read_text_file","params":{}}`)
	m := p.readFrame(t)
	if m["error"] != nil {
		t.Fatalf("出错通知之后的请求也失败了: %v", m["error"])
	}
	if m["result"] != "ok" {
		t.Errorf("result = %v, want ok —— 通知处理出错不该影响后续请求", m["result"])
	}
}

// R9 · 没注册 handler 时通知被静默丢弃，不崩、不回复。
func TestConn_R9_NotificationWithoutHandlerIsDropped(t *testing.T) {
	p := newPeer(t, nil)

	p.write(t, `{"jsonrpc":"2.0","method":"session/update","params":{}}`)

	select {
	case m := <-p.frames:
		t.Fatalf("没有 handler 时通知不该产生任何回帧: %v", m)
	case <-time.After(300 * time.Millisecond):
	}
}

// R10 ★ handler 返回 *jsonrpc.Error 时，code 与 message 原样传给对方。
//
// 权限裁决走的就是这条：拒绝时要回一个带确定 code 的错误，
// 被包装成 -32603 internal error 的话，对方无法区分「拒绝」与「我们崩了」。
func TestConn_R10_HandlerRPCErrorKeepsCode(t *testing.T) {
	p := newPeer(t, jsonrpc.HandlerFunc(
		func(context.Context, string, json.RawMessage) (any, error) {
			return nil, &jsonrpc.Error{Code: -32001, Message: "permission denied"}
		}))

	p.write(t, `{"jsonrpc":"2.0","id":3,"method":"fs/write_text_file","params":{}}`)
	m := p.readFrame(t)

	e, ok := m["error"].(map[string]any)
	if !ok {
		t.Fatalf("没有 error 字段: %v", m)
	}
	if code, _ := e["code"].(float64); int(code) != -32001 {
		t.Errorf("code = %v, want -32001 —— 被包装掉了，对方分不清拒绝与崩溃", e["code"])
	}
	if msg, _ := e["message"].(string); msg != "permission denied" {
		t.Errorf("message = %q, want permission denied", msg)
	}
}

// R11 · handler 返回普通 error 时包装成 -32603，不泄漏内部细节以外的东西。
func TestConn_R11_HandlerPlainErrorBecomesInternalError(t *testing.T) {
	p := newPeer(t, jsonrpc.HandlerFunc(
		func(context.Context, string, json.RawMessage) (any, error) {
			return nil, errors.New("boom")
		}))

	p.write(t, `{"jsonrpc":"2.0","id":4,"method":"fs/read_text_file","params":{}}`)
	m := p.readFrame(t)

	e, ok := m["error"].(map[string]any)
	if !ok {
		t.Fatalf("没有 error 字段: %v", m)
	}
	if code, _ := e["code"].(float64); int(code) != jsonrpc.CodeInternalError {
		t.Errorf("code = %v, want %d", e["code"], jsonrpc.CodeInternalError)
	}
}

// R12 · 收到没发过的 id 的响应时只告警，不崩、不影响后续。
//
// 真实 Runtime 出过这种事（重发、时序错乱）。崩掉的话整轮会话没了。
func TestConn_R12_UnknownResponseIDIsIgnored(t *testing.T) {
	p := newPeer(t, jsonrpc.HandlerFunc(
		func(context.Context, string, json.RawMessage) (any, error) { return "ok", nil }))

	p.write(t, `{"jsonrpc":"2.0","id":9999,"result":{"whatever":true}}`)

	// 连接仍可用
	p.write(t, `{"jsonrpc":"2.0","id":1,"method":"fs/read_text_file","params":{}}`)
	m := p.readFrame(t)
	if m["result"] != "ok" {
		t.Errorf("无人认领的响应之后连接不可用了: %v", m)
	}
}

// ★★ 通知必须**按到达顺序**处理。
//
// 真踩过（2026-08-08，做 U2.2.2 时发现）：原来是 `go c.serveNotification(...)`，
// 每条通知起一个 goroutine。对 ACP 来说这是**语义错误**——
// session/update 里的 agent_message_chunk 是流式文本，顺序就是用户看到的字序。
// 并发派发意味着「今天没复现」只是运气，用户迟早会看到乱掉的句子。
//
// 反向请求（session/request_permission）仍然要 goroutine：它等用户回答，
// 同步处理会卡死读循环。两者的区别是「会不会阻塞」，不是「重不重要」。
func TestConn_NotificationsAreProcessedInOrder(t *testing.T) {
	const n = 200

	var mu sync.Mutex
	var got []int

	in, w := io.Pipe()
	conn := jsonrpc.New(in, io.Discard, jsonrpc.HandlerFunc(func(_ context.Context, method string, params json.RawMessage) (any, error) {
		if method != "tick" {
			return nil, nil
		}
		var p struct {
			Seq int `json:"seq"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		mu.Lock()
		got = append(got, p.Seq)
		mu.Unlock()
		return nil, nil
	}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = conn.Serve(ctx) }()

	go func() {
		for i := range n {
			_, _ = fmt.Fprintf(w, `{"jsonrpc":"2.0","method":"tick","params":{"seq":%d}}`+"\n", i)
		}
		_ = w.Close()
	}()

	deadline := time.After(3 * time.Second)
	for {
		mu.Lock()
		done := len(got) == n
		mu.Unlock()
		if done {
			break
		}
		select {
		case <-deadline:
			mu.Lock()
			t.Fatalf("只收到 %d/%d 条通知", len(got), n)
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	for i, seq := range got {
		if seq != i {
			t.Fatalf("第 %d 条通知的 seq 是 %d——顺序乱了，"+
				"流式文本会以错误的字序显示给用户", i, seq)
		}
	}
}

// ★★ 读端关掉时，**所有等着的调用都要立刻失败**。
//
// 这是真机上撞出来的：Agent 进程启动失败直接退出，我们这边的 initialize
// 就永远等在那儿。用户看到的是界面停在「正在初始化」——没有转圈、没有报错、
// 没有超时，只能杀掉应用。
//
// ctx 上有超时的话最终能醒，但那是几分钟之后，而且错误说的是「超时」，
// 与真正的原因（Agent 起不来）差着十万八千里。
func TestCallInto_FailsWhenPeerDies(t *testing.T) {
	// 一个立刻 EOF 的读端 = 对方进程已经退出
	pr, pw := io.Pipe()
	conn := jsonrpc.New(pr, io.Discard, nil)

	go func() { _ = conn.Serve(context.Background()) }()
	// 对方退出
	_ = pw.Close()

	done := make(chan error, 1)
	go func() {
		done <- conn.CallInto(context.Background(), "initialize", map[string]any{}, nil)
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("对方已经退出了，这次调用却成功了")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("对方进程退出后调用还挂着——" +
			"用户看到的是界面停在「正在初始化」，没有转圈、没有报错，只能杀掉应用")
	}
}

// 连接断掉之后**新发起的**调用也要立刻失败，而不是挂到 ctx 超时。
func TestCallInto_FailsAfterServeStopped(t *testing.T) {
	pr, pw := io.Pipe()
	conn := jsonrpc.New(pr, io.Discard, nil)

	served := make(chan struct{})
	go func() { defer close(served); _ = conn.Serve(context.Background()) }()
	_ = pw.Close()
	<-served

	done := make(chan error, 1)
	go func() {
		done <- conn.CallInto(context.Background(), "session/new", map[string]any{}, nil)
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("连接已经断了，这次调用却成功了")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("连接断掉之后发起的调用还挂着")
	}
}
