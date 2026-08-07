// Package fake 是一个可编排的假 ACP Agent —— 所有上层测试的地基。
//
// 没有它，上层任何测试都得依赖真实 claude-agent-acp / codex-acp：
// 慢、不确定、要账号、要网络，AI 自测通道直接废掉。
//
// **依赖约束：本包只 import protocol。**
// 若复用了 session 或 jsonrpc 的实现，测试就变成「用被测代码验证被测代码」——
// 是 mock 喂 mock 的变体，只是伪装得更好。Fake 必须**独立地**按规范说话，
// 才有资格当参照物（docs/spec/acp-integration.md §3.3 硬规则 2）。
// 所以这里自己实现 ndjson 分帧与 JSON-RPC 收发，明知 jsonrpc 包里有。
//
// 两种运行形态共用同一份脚本：Transport() 走内存管道给单测，
// Serve() 走 stdio 给 e2e 的真子进程。
package fake

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/acp/protocol"
)

// maxFrameBytes 是单帧上限。真实 runtime 会发很大的 diff，留足余量。
const maxFrameBytes = 8 << 20

// Clock 是注入的时间源。
//
// 本包自己定义它而不是复用 app/port：`fake` 只允许 import `protocol`。
// 一个只有 Now() 的接口重复定义的代价，远小于打破依赖约束的代价。
type Clock interface {
	Now() time.Time
}

// Options 是构造 Fake 的参数。
type Options struct {
	// Script 必填。
	Script *Script
	// Clock 必填。禁止 time.Now()（testing-strategy.md §5）。
	Clock Clock
	// Latency 零值 = 无额外延迟、不乱序。
	Latency Latency
	// Stderr 是 Fake 自己的诊断输出，默认丢弃。
	Stderr io.Writer
}

// Runtime 是一个假 ACP Agent。
type Runtime struct {
	script  *Script
	clock   Clock
	stderr  io.Writer
	rec     *recorder
	latency Latency

	// neverStops 让所有轮次都不响应 session/prompt（预设 NeverStops）。
	neverStops bool
	// silentAfter > 0 时，首个 prompt 之后 d 彻底断流（预设 SilentAfter）。
	silentAfter time.Duration

	transportOnce sync.Once
	transport     io.ReadWriteCloser
	cancelServe   context.CancelFunc

	mu       sync.Mutex
	turnSeq  int
	closed   bool
	closeOut func()
}

// New 构造一个 Fake ACP Runtime。
//
// opts.Script 或 opts.Clock 为 nil 时 panic ——
// 测试夹具构造失败必须立刻暴露，不该返回 error 让调用方顺手忽略。
func New(opts Options) *Runtime {
	if opts.Script == nil {
		panic("fake: Options.Script 必填")
	}
	if opts.Clock == nil {
		panic("fake: Options.Clock 必填 —— 禁止 time.Now()（testing-strategy.md §5）")
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = io.Discard
	}
	return &Runtime{
		script:  opts.Script,
		clock:   opts.Clock,
		stderr:  stderr,
		rec:     newRecorder(),
		latency: opts.Latency,
	}
}

// Transport 返回进程内双工管道，直接喂给客户端。多次调用返回同一个实例。
func (r *Runtime) Transport() io.ReadWriteCloser {
	r.transportOnce.Do(func() {
		// clientW → agentR：客户端写的，我们读
		agentR, clientW := io.Pipe()
		// agentW → clientR：我们写的，客户端读
		clientR, agentW := io.Pipe()

		ctx, cancel := context.WithCancel(context.Background())
		r.cancelServe = cancel

		r.transport = &duplex{Reader: clientR, Writer: clientW, closers: []io.Closer{clientW, clientR}}
		go func() {
			if err := r.Serve(ctx, agentR, agentW); err != nil {
				_, _ = fmt.Fprintf(r.stderr, "fake: Serve 结束: %v\n", err)
			}
			_ = agentW.Close()
		}()
	})
	return r.transport
}

// Serve 以子进程形态运行：从 in 读、往 out 写，直到 ctx 结束或 in EOF。
//
// cmd/fakeacp/main.go 只做参数解析 + 调这个函数 ——
// e2e 里跑的和单测里跑的必须是同一份实现。
func (r *Runtime) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	w := &frameWriter{w: out}

	r.mu.Lock()
	r.closeOut = w.close
	r.mu.Unlock()

	var turns sync.WaitGroup
	defer turns.Wait()

	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 64*1024), maxFrameBytes)

	for sc.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		// 必须拷贝：Scanner 会复用底层缓冲，直接留着会在下一帧被覆写。
		frameBytes := append([]byte(nil), line...)

		var f wireFrame
		if err := json.Unmarshal(frameBytes, &f); err != nil {
			// 坏帧不致命 —— 真实 runtime 也不会因为一行垃圾就断开。
			_, _ = fmt.Fprintf(r.stderr, "fake: 跳过坏帧: %v\n", err)
			continue
		}

		kind := KindNotification
		if len(f.ID) > 0 {
			kind = KindRequest
		}
		r.rec.record(r.clock.Now(), kind, f.ID, f.Method, f.Params)

		if err := r.dispatch(ctx, &turns, w, f); err != nil {
			return err
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("fake: 读入站帧失败: %w", err)
	}
	return nil
}

// dispatch 把一条入站消息交给对应的处理。
func (r *Runtime) dispatch(ctx context.Context, turns *sync.WaitGroup, w *frameWriter, f wireFrame) error {
	switch f.Method {
	case protocol.MethodInitialize:
		return w.respond(f.ID, protocol.InitializeResponse{
			ProtocolVersion: protocol.ProtocolVersionV1,
			AgentInfo: &protocol.Implementation{
				Name: "fake-acp", Title: "Fake ACP Runtime", Version: "0.0.0",
			},
		})

	case protocol.MethodSessionNew:
		return r.respondNewSession(w, f.ID)

	case protocol.MethodSessionPrompt:
		// 回放放进独立 goroutine：主循环要继续收 session/cancel，
		// 否则「取消一个正在跑的轮次」根本无从测起。
		turn, promptID := r.nextTurn(), f.ID
		turns.Add(1)
		go func() {
			defer turns.Done()
			r.replay(ctx, w, promptID, turn)
		}()
		return nil

	case protocol.MethodSessionCancel:
		// 只记录，不做任何事 —— 取消语义是 U0.4.2 与 S0.6 的题目。
		// ★ 这里绝不能顺手去重：去重是被测代码的职责。
		return nil

	default:
		if len(f.ID) == 0 {
			return nil // 不认识的通知，安静丢弃
		}
		return w.respondError(f.ID, -32601, fmt.Sprintf("fake: 未实现的方法 %s", f.Method))
	}
}

func (r *Runtime) respondNewSession(w *frameWriter, id json.RawMessage) error {
	behavior := r.script.NewSession
	if behavior != nil && behavior.Delay > 0 {
		time.Sleep(time.Duration(behavior.Delay))
	}

	resp := protocol.NewSessionResponse{SessionID: r.script.sessionID()}
	// 不声明 modes 时响应里就**真的没有** modes 字段 ——
	// 假的能力声明必须表现为真的协议行为，否则测的是我们自己的探针代码。
	if behavior != nil && len(behavior.Modes) > 0 {
		resp.Modes = &protocol.SessionModeState{
			CurrentModeID:  behavior.CurrentModeID,
			AvailableModes: behavior.Modes,
		}
	}
	return w.respond(id, resp)
}

// nextTurn 取下一轮的脚本，并把轮次计数往前推。
func (r *Runtime) nextTurn() *Turn {
	r.mu.Lock()
	defer r.mu.Unlock()
	turn := r.script.turnAt(r.turnSeq)
	r.turnSeq++
	return turn
}

// replay 回放一轮：按脚本推事件，然后（或不）响应 session/prompt。
func (r *Runtime) replay(ctx context.Context, w *frameWriter, promptID json.RawMessage, turn *Turn) {
	if turn == nil {
		// 脚本没写这一轮 —— 不响应比编一个 end_turn 诚实：
		// 编出来的话，测试会以为脚本覆盖到了实际没覆盖的轮次。
		_, _ = fmt.Fprintf(r.stderr, "fake: 脚本没有第 %d 轮，session/prompt 不响应\n", r.turnSeq)
		return
	}

	// 断流计时从**首个 prompt** 开始 —— 语义是「跑着跑着断了」，
	// 从 Serve 起算的话握手耗时会算进去，测试时序变得不可控。
	r.armSilence()

	steps := turn.Steps
	if r.latency.Reorder {
		steps = reorderSteps(steps, r.latency.seed())
	}

	sessionID := r.script.sessionID()
	for i, step := range steps {
		if !sleepCtx(ctx, r.latency.delayFor(i, time.Duration(step.Delay))) {
			return
		}
		if len(step.Emit) == 0 {
			continue
		}
		var update protocol.SessionUpdate
		if err := json.Unmarshal(step.Emit, &update); err != nil {
			_, _ = fmt.Fprintf(r.stderr, "fake: 第 %d 步的 emit 载荷非法: %v\n", i, err)
			continue
		}
		if err := w.notify(protocol.MethodSessionUpdate, protocol.SessionNotification{
			SessionID: sessionID,
			Update:    update,
		}); err != nil {
			return // 流已断，安静退出
		}
	}

	// ★ 不回 stopReason：NeverStops 预设，或脚本里这一轮没写 stop_reason。
	// 这是 S0.6 测 ErrCancelTimeout 的开关（testing-strategy.md §3.5）。
	if r.neverStops || turn.StopReason == "" {
		return
	}
	if !sleepCtx(ctx, time.Duration(turn.StopDelay)) {
		return
	}
	_ = w.respond(promptID, protocol.PromptResponse{StopReason: turn.StopReason})
}

// armSilence 在首个 prompt 之后启动断流计时。
func (r *Runtime) armSilence() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.silentAfter <= 0 || r.closed {
		return
	}
	d := r.silentAfter
	r.silentAfter = 0 // 只装一次
	closeOut := r.closeOut
	time.AfterFunc(d, func() {
		if closeOut != nil {
			closeOut()
		}
	})
}

// Close 停止 Fake 并断开管道。可重复调用。
func (r *Runtime) Close() error {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil
	}
	r.closed = true
	closeOut := r.closeOut
	cancel := r.cancelServe
	r.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if closeOut != nil {
		closeOut()
	}
	if r.transport != nil {
		_ = r.transport.Close()
	}
	return nil
}

// Requests 返回至今收到的全部消息，按接收顺序。返回副本，调用方可随意持有。
func (r *Runtime) Requests() []Recorded { return r.rec.snapshot() }

// CountMethod 返回某个方法被调用的次数。
//
// ★ Fake **不去重**：连发两次 session/cancel 就是 2。
// 去重是被测代码的职责，Fake 替它去重的话那条断言永远绿。
func (r *Runtime) CountMethod(method string) int { return r.rec.countMethod(method) }

// WaitFor 阻塞直到出现满足 pred 的消息，或 ctx 结束。
//
// 用它替代 time.Sleep —— 测试里禁止睡眠等待。
func (r *Runtime) WaitFor(ctx context.Context, pred func(Recorded) bool) (Recorded, error) {
	return r.rec.waitFor(ctx, pred)
}

// duplex 把两个半双工端拼成一个 io.ReadWriteCloser。
type duplex struct {
	io.Reader
	io.Writer
	closers []io.Closer
}

func (d *duplex) Close() error {
	var first error
	for _, c := range d.closers {
		if err := c.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// sleepCtx 睡 d，ctx 结束时提前返回 false。d ≤ 0 时不睡。
func sleepCtx(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}
