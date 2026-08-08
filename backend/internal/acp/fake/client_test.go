package fake_test

// 测试里扮演「客户端」的最小 JSON-RPC 实现。
//
// ★ 故意**不** import internal/acp/jsonrpc。
// Fake 是那个包的参照物：拿被测代码当客户端来验证 Fake，
// 分帧写错时两边会一起错、测试照样绿 —— 这正是「mock 喂 mock」的变体。
// 所以这里只用标准库直接读写 ndjson。

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"
)

// frame 是线上一帧的通用形态。
type frame struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *frameError     `json:"error,omitempty"`
}

type frameError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// client 按 ndjson 与 Fake 收发，把响应按 id 归位、通知投进队列。
type client struct {
	t  *testing.T
	rw io.ReadWriteCloser

	mu      sync.Mutex
	nextID  int64
	pending map[int64]chan frame

	notifications chan frame

	// ★ 连接结束用 close(done) 广播，不用「往 channel 里塞一个 error」。
	// 塞值的话，同时在等的 call() 与 nextNotification() 会竞争同一个值，
	// 谁先抢到谁醒 —— 另一个一直等到超时。这个坑真踩过（R4 断流测试）。
	done    chan struct{}
	doneMu  sync.Mutex
	doneErr error
}

func newClient(t *testing.T, rw io.ReadWriteCloser) *client {
	t.Helper()
	c := &client{
		t:             t,
		rw:            rw,
		pending:       make(map[int64]chan frame),
		notifications: make(chan frame, 64),
		done:          make(chan struct{}),
	}
	go c.readLoop()
	t.Cleanup(func() { _ = rw.Close() })
	return c
}

// finish 记下连接结束的原因并广播。只有第一个原因算数。
func (c *client) finish(err error) {
	c.doneMu.Lock()
	defer c.doneMu.Unlock()
	if c.doneErr != nil {
		return
	}
	c.doneErr = err
	close(c.done)
}

func (c *client) finishErr() error {
	c.doneMu.Lock()
	defer c.doneMu.Unlock()
	return c.doneErr
}

func (c *client) readLoop() {
	sc := bufio.NewScanner(c.rw)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var f frame
		if err := json.Unmarshal(line, &f); err != nil {
			c.finish(fmt.Errorf("客户端收到坏帧: %w (%s)", err, line))
			return
		}
		if f.ID != nil && f.Method == "" {
			c.mu.Lock()
			ch, ok := c.pending[*f.ID]
			delete(c.pending, *f.ID)
			c.mu.Unlock()
			if ok {
				ch <- f
			}
			continue
		}
		select {
		case c.notifications <- f:
		default:
			c.finish(fmt.Errorf("通知队列满了，测试没及时消费"))
			return
		}
	}
	// Scanner 正常结束 = 对端关流。广播 EOF，避免等待方永久阻塞。
	if err := sc.Err(); err != nil {
		c.finish(fmt.Errorf("客户端读流失败: %w", err))
		return
	}
	c.finish(io.EOF)
}

// call 发一个请求并等响应。
func (c *client) call(method string, params any, timeout time.Duration) (frame, error) {
	c.t.Helper()

	c.mu.Lock()
	id := c.nextID
	c.nextID++
	ch := make(chan frame, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	c.write(frame{JSONRPC: "2.0", ID: &id, Method: method, Params: mustJSON(c.t, params)})

	select {
	case f := <-ch:
		return f, nil
	case <-c.done:
		return frame{}, fmt.Errorf("等 %s 响应时连接结束: %w", method, c.finishErr())
	case <-time.After(timeout):
		return frame{}, fmt.Errorf("等 %s 响应超时（%s）", method, timeout)
	}
}

// notify 发一个通知，不等响应。
func (c *client) notify(method string, params any) {
	c.t.Helper()
	c.write(frame{JSONRPC: "2.0", Method: method, Params: mustJSON(c.t, params)})
}

func (c *client) write(f frame) {
	c.t.Helper()
	b, err := json.Marshal(f)
	if err != nil {
		c.t.Fatalf("序列化请求失败: %v", err)
	}
	if _, err := c.rw.Write(append(b, '\n')); err != nil {
		c.t.Fatalf("写请求失败: %v", err)
	}
}

// nextNotification 取下一条通知；超时或连接结束时返回错误。
func (c *client) nextNotification(timeout time.Duration) (frame, error) {
	c.t.Helper()
	select {
	case f := <-c.notifications:
		return f, nil
	case <-c.done:
		// 队列里可能还压着断流前发出的事件，先把它们放完再报结束。
		select {
		case f := <-c.notifications:
			return f, nil
		default:
		}
		return frame{}, c.finishErr()
	case <-time.After(timeout):
		return frame{}, fmt.Errorf("等通知超时（%s）", timeout)
	}
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("序列化参数失败: %v", err)
	}
	return b
}

// ── U3.1.1 用到的三项：异步发请求、收反向请求、回响应 ──────────

// callAsync 发一个请求但**不等响应**，返回等待用的通道。
//
// 权限请求那组测试必须这样：发完 session/prompt 要先去收 Fake 主动发来的
// 反向请求，同步等的话两边互相等，直接死锁。
func (c *client) callAsync(method string, params any) chan frame {
	c.t.Helper()

	c.mu.Lock()
	id := c.nextID
	c.nextID++
	ch := make(chan frame, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	c.write(frame{JSONRPC: "2.0", ID: &id, Method: method, Params: mustJSON(c.t, params)})
	return ch
}

// await 等一个 callAsync 的响应。
func (c *client) await(ch chan frame, timeout time.Duration) (frame, error) {
	c.t.Helper()
	select {
	case f := <-ch:
		return f, nil
	case <-c.done:
		return frame{}, fmt.Errorf("等响应时连接结束: %w", c.finishErr())
	case <-time.After(timeout):
		return frame{}, fmt.Errorf("等响应超时（%s）", timeout)
	}
}

// tryAwait 等一小会儿；没等到返回 ok=false，**这不算错误**。
//
// 用来断言「这一轮到现在还没结束」——那正是权限请求阻塞语义的核心。
func (c *client) tryAwait(ch chan frame, window time.Duration) (frame, bool) {
	c.t.Helper()
	select {
	case f := <-ch:
		return f, true
	case <-time.After(window):
		return frame{}, false
	}
}

// nextRequest 取下一条**带 id 的入站帧**，也就是 Agent 发来的反向请求。
//
// 不带 id 的通知会被跳过：session/update 与 session/request_permission
// 走的是同一条队列，而这里只关心后者。
func (c *client) nextRequest(method string, timeout time.Duration) (frame, error) {
	c.t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		left := time.Until(deadline)
		if left <= 0 {
			return frame{}, fmt.Errorf("等反向请求 %s 超时（%s）", method, timeout)
		}
		f, err := c.nextNotification(left)
		if err != nil {
			return frame{}, err
		}
		if f.ID != nil && f.Method == method {
			return f, nil
		}
	}
}

// respond 回一条响应给 Agent 的反向请求。
func (c *client) respond(id *int64, result any) {
	c.t.Helper()
	c.write(frame{JSONRPC: "2.0", ID: id, Result: mustJSON(c.t, result)})
}
