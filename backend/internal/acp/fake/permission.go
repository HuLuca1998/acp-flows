package fake

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/HuLuca1998/acp-flows/backend/internal/acp/protocol"
)

// askState 记录一次权限请求的等待。
type askState struct {
	// done 在收到应答时关闭。
	done chan struct{}
	// outcome 是客户端回的结果，done 关闭后才可读。
	outcome askOutcome
}

// askOutcome 是客户端应答里我们关心的两个字段。
//
// ★ 故意不复用 protocol.RequestPermissionOutcome：那个类型的字段是私有的，
// 只能通过构造函数产生。Fake 要的是**线上真实回来了什么**，
// 包括「客户端回了一个我们没发过的 optionId」这种情况——
// 用被测代码的类型来解，等于让它替我们判断对错。
type askOutcome struct {
	Outcome  string `json:"outcome"`
	OptionID string `json:"optionId"`
}

// askResponse 是客户端对 session/request_permission 的响应体。
type askResponse struct {
	Outcome askOutcome `json:"outcome"`
}

// permissionAsks 管住所有 pending 的权限请求。
type permissionAsks struct {
	mu sync.Mutex
	// nextID 是反向请求的 id 序列。
	//
	// ★ 从 0 开始：ACP 里 Agent 侧的请求 id 就是从 0 起的
	// （acp-field-notes 记过一次，我们的 jsonrpc 从 1 起，两边不能混）。
	nextID int64
	byID   map[int64]*askState
	// lastOptionID 是最后一次应答里客户端回的 optionId，供测试断言。
	lastOptionID string
	// lastOutcome 同上，记 outcome 本身（selected / cancelled）。
	lastOutcome string
}

func newPermissionAsks() *permissionAsks {
	return &permissionAsks{byID: make(map[int64]*askState)}
}

// begin 登记一次新的权限请求，返回它的 id 与等待句柄。
func (p *permissionAsks) begin() (int64, *askState) {
	p.mu.Lock()
	defer p.mu.Unlock()
	id := p.nextID
	p.nextID++
	st := &askState{done: make(chan struct{})}
	p.byID[id] = st
	return id, st
}

// settle 收下一条应答并解除对应的等待。认不出 id 时返回 false。
func (p *permissionAsks) settle(id int64, outcome askOutcome) bool {
	p.mu.Lock()
	st, ok := p.byID[id]
	if ok {
		delete(p.byID, id)
		p.lastOptionID = outcome.OptionID
		p.lastOutcome = outcome.Outcome
	}
	p.mu.Unlock()

	if !ok {
		return false
	}
	st.outcome = outcome
	close(st.done)
	return true
}

// abortAll 在断流时解除所有等待，避免回放 goroutine 永久挂住。
func (p *permissionAsks) abortAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for id, st := range p.byID {
		delete(p.byID, id)
		close(st.done)
	}
}

func (p *permissionAsks) snapshot() (optionID, outcome string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lastOptionID, p.lastOutcome
}

// LastPermissionOptionID 返回客户端最后一次应答里回的 optionId。
//
// 断言「原样回传」用：Agent 给的是不透明字符串，客户端按类别去猜的话，
// 这里拿到的就不是我们发出去的那个。
func (r *Runtime) LastPermissionOptionID() string {
	id, _ := r.asks.snapshot()
	return id
}

// LastPermissionOutcome 返回客户端最后一次应答的 outcome（selected / cancelled）。
func (r *Runtime) LastPermissionOutcome() string {
	_, outcome := r.asks.snapshot()
	return outcome
}

// ask 发一条权限请求并**一直等**到客户端应答。
//
// ★ 没有超时，这是刻意的（U3.1.1 R5）。真 Agent 会一直等用户——
// 用户可能去泡了杯咖啡。Fake 自作主张地超时，上层「等用户裁决」的逻辑
// 就测不出来：测试里那一轮会自己结束。
//
// 唯一的解除途径是 ctx 结束或断流（abortAll），那时返回 ok=false，
// 调用方据此安静收场，而不是把这一轮当成「用户同意了」。
func (r *Runtime) ask(ctx context.Context, w *frameWriter, spec *PermissionAsk) (askOutcome, bool) {
	options := spec.Options
	if len(options) == 0 {
		options = defaultAskOptions()
	}

	id, st := r.asks.begin()
	// ★ 原样发：optionId 与 kind 语义对不上也不纠正。纠正的话，
	// 「客户端按类别猜 id」这类 bug 就永远测不出来。
	err := w.request(id, protocol.MethodRequestPermission, protocol.RequestPermissionRequest{
		SessionID: r.script.sessionID(),
		ToolCall: protocol.ToolCallUpdate{
			ToolCallID: spec.ToolCallID,
			Title:      spec.Title,
			Kind:       spec.Kind,
		},
		Options: options,
	})
	if err != nil {
		r.asks.settle(id, askOutcome{})
		return askOutcome{}, false
	}

	select {
	case <-st.done:
		return st.outcome, true
	case <-ctx.Done():
		return askOutcome{}, false
	}
}

// handleAskResponse 处理客户端对反向请求的响应。
//
// ★ 认不出的 id 只记一行日志，不报错、不断流：客户端重发一条陈旧的响应
// 不该让整条连接死掉——那会把一个小问题放大成「Agent 挂了」。
func (r *Runtime) handleAskResponse(id int64, result json.RawMessage) {
	var resp askResponse
	if len(result) > 0 {
		if err := json.Unmarshal(result, &resp); err != nil {
			_, _ = fmt.Fprintf(r.stderr, "fake: 权限应答载荷非法: %v (%s)\n", err, result)
			return
		}
	}
	if !r.asks.settle(id, resp.Outcome) {
		_, _ = fmt.Fprintf(r.stderr, "fake: 收到不认识的权限应答 id=%d\n", id)
	}
}
