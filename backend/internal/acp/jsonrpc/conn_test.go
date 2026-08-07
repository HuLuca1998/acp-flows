package jsonrpc_test

// M0 U0.2.1 · ndjson 编解码与双向路由
//
// 全部对着 io.Pipe 做，**不起子进程** —— 传输层不该知道进程的存在。
// 验收标准见 docs/plan/milestones/M0-acp-foundation.md § S0.2 U0.2.1。

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
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
