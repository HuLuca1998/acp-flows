package fake

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/acp/protocol"
)

// Dur 是可读的时长，JSON 里写 "10ms" / "5s" 而不是纳秒数字。
//
// 脚本要能被人读懂、被 e2e 从磁盘加载，纳秒整数两条都做不到。
type Dur time.Duration

// MarshalJSON 写成 "10ms" 这样的字符串。
func (d Dur) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// UnmarshalJSON 接受 "10ms" 这样的字符串；空串按零处理。
func (d *Dur) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("fake: 时长必须是 \"10ms\" 这样的字符串: %w", err)
	}
	if s == "" {
		*d = 0
		return nil
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("fake: 解析时长 %q 失败: %w", s, err)
	}
	*d = Dur(parsed)
	return nil
}

// Script 是一份可回放的脚本。
//
// 进程内形态与子进程形态**共用同一份格式** —— e2e 里跑的和单测里跑的
// 必须是同一个 Fake，否则「单测绿 + e2e 红」时你不知道该信谁。
type Script struct {
	Name       string              `json:"name"`
	NewSession *NewSessionBehavior `json:"new_session,omitempty"`
	Turns      []Turn              `json:"turns"`
	// Default 在轮次用完后复用，用于「随便再问几轮」的场景。
	Default *Turn `json:"default,omitempty"`
}

// NewSessionBehavior 定制 session/new 的响应。
type NewSessionBehavior struct {
	// SessionID 为空时用 defaultSessionID。
	SessionID     string                 `json:"session_id"`
	Modes         []protocol.SessionMode `json:"modes,omitempty"`
	CurrentModeID string                 `json:"current_mode_id,omitempty"`
	Delay         Dur                    `json:"delay,omitempty"`
}

// Turn 是一轮 session/prompt 的行为。
type Turn struct {
	Steps []Step `json:"steps"`
	// StopReason 为空时 ★ **永不响应 session/prompt**，用于测超时。
	StopReason protocol.StopReason `json:"stop_reason,omitempty"`
	StopDelay  Dur                 `json:"stop_delay,omitempty"`
}

// Step 是一轮里的一步。
type Step struct {
	// Delay 是执行这一步**之前**等待的时长。
	Delay Dur `json:"delay,omitempty"`
	// Emit 是一条完整的 SessionUpdate 载荷，原样下发。
	Emit json.RawMessage `json:"emit,omitempty"`
	// Ask 是一条权限请求：发出去之后**阻塞等应答**，收到之前这一轮不往下走。
	//
	// ★ 阻塞是它的全部意义。不阻塞的话，上层的裁决逻辑就没有真实的对手方——
	// 测试会以为「问过了」，而实际上 Agent 根本没等答案就往下干了。
	Ask *PermissionAsk `json:"ask,omitempty"`
}

// PermissionAsk 是脚本里的一条权限请求。
//
// ★ Fake **不许对它做任何加工**：optionId 与 kind 语义对不上也原样发。
// 「顺手纠正」的话，「客户端按类别猜 id」这类 bug 就永远测不出来
// （U3.1.1 的 forbidden_changes 明写「Fake 自己去重或纠正收到的消息」是禁止的）。
type PermissionAsk struct {
	// ToolCallID 是这次请求针对的工具调用。界面靠它说清楚「要允许的是什么」。
	ToolCallID string            `json:"tool_call_id"`
	Title      string            `json:"title,omitempty"`
	Kind       protocol.ToolKind `json:"kind,omitempty"`
	// Options 原样发给客户端。留空时用 defaultAskOptions()。
	Options []protocol.PermissionOption `json:"options,omitempty"`
}

// defaultAskOptions 是脚本没写选项时的一组，覆盖 allow/reject 两类。
func defaultAskOptions() []protocol.PermissionOption {
	return []protocol.PermissionOption{
		{OptionID: "allow", Name: "允许一次", Kind: protocol.PermissionAllowOnce},
		{OptionID: "reject", Name: "拒绝", Kind: protocol.PermissionRejectOnce},
	}
}

const defaultSessionID = "sess_fake_0001"

// sessionID 返回脚本指定的会话 id。
func (s *Script) sessionID() string {
	if s.NewSession == nil || s.NewSession.SessionID == "" {
		return defaultSessionID
	}
	return s.NewSession.SessionID
}

// turnAt 返回第 n 轮的行为；超出 Turns 长度时用 Default，没有 Default 就返回 nil。
func (s *Script) turnAt(n int) *Turn {
	if n < len(s.Turns) {
		return &s.Turns[n]
	}
	return s.Default
}

// LoadScript 从磁盘读一份 JSON 脚本。给 e2e 用。
func LoadScript(path string) (*Script, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("fake: 读脚本 %s 失败: %w", path, err)
	}
	var s Script
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("fake: 解析脚本 %s 失败: %w", path, err)
	}
	return &s, nil
}

// MustLoadScript 同 LoadScript，失败即 panic。
//
// 测试夹具构造失败必须立刻暴露，不该返回 error 让调用方顺手忽略。
func MustLoadScript(path string) *Script {
	s, err := LoadScript(path)
	if err != nil {
		panic(err)
	}
	return s
}

// ── builder DSL ───────────────────────────────────────────────────
//
// 单测里手写 JSON 太噪，builder 产出同一个 *Script。

// ScriptBuilder 链式构造脚本。全部方法返回自身。
type ScriptBuilder struct {
	script Script
	// pendingDelay 是下一个 Step 的延迟，由 After 设置、被 Step 消费。
	pendingDelay Dur
	// err 记住第一个构造错误，在 Build 时 panic。
	err error
}

// NewScript 开一份新脚本。
func NewScript(name string) *ScriptBuilder {
	return &ScriptBuilder{script: Script{Name: name}}
}

// Session 指定 session/new 返回的会话 id。
func (b *ScriptBuilder) Session(id string) *ScriptBuilder {
	if b.script.NewSession == nil {
		b.script.NewSession = &NewSessionBehavior{}
	}
	b.script.NewSession.SessionID = id
	return b
}

// Modes 声明会话模式。不调用时 session/new 的响应里**没有** modes 字段。
func (b *ScriptBuilder) Modes(current string, modes ...protocol.SessionMode) *ScriptBuilder {
	if b.script.NewSession == nil {
		b.script.NewSession = &NewSessionBehavior{}
	}
	b.script.NewSession.CurrentModeID = current
	b.script.NewSession.Modes = modes
	return b
}

// Turn 开一个新轮次。
func (b *ScriptBuilder) Turn() *ScriptBuilder {
	b.script.Turns = append(b.script.Turns, Turn{})
	b.pendingDelay = 0
	return b
}

// After 设置**下一步**执行前的等待时长。
func (b *ScriptBuilder) After(d time.Duration) *ScriptBuilder {
	b.pendingDelay = Dur(d)
	return b
}

// Emit 追加一步：下发一条 SessionUpdate 载荷。
func (b *ScriptBuilder) Emit(payload any) *ScriptBuilder {
	raw, err := json.Marshal(payload)
	if err != nil {
		b.recordErr(fmt.Errorf("序列化 emit 载荷失败: %w", err))
		return b
	}
	// 提前校验判别值，否则错误会推迟到回放时才炸，且现场已经没了。
	var u protocol.SessionUpdate
	if err := json.Unmarshal(raw, &u); err != nil {
		b.recordErr(fmt.Errorf("emit 载荷不是合法的 SessionUpdate: %w", err))
		return b
	}
	return b.appendStep(Step{Delay: b.takeDelay(), Emit: raw})
}

// Say 追加一条 agent_message_chunk。
func (b *ScriptBuilder) Say(messageID, text string) *ScriptBuilder {
	return b.Emit(protocol.ContentChunkUpdate{
		SessionUpdate: protocol.UpdateAgentMessageChunk,
		MessageID:     messageID,
		Content:       protocol.TextBlock(text),
	})
}

// Think 追加一条 agent_thought_chunk。
func (b *ScriptBuilder) Think(messageID, text string) *ScriptBuilder {
	return b.Emit(protocol.ContentChunkUpdate{
		SessionUpdate: protocol.UpdateAgentThoughtChunk,
		MessageID:     messageID,
		Content:       protocol.TextBlock(text),
	})
}

// Tool 追加一条 tool_call（首次声明，状态 pending）。
func (b *ScriptBuilder) Tool(toolCallID string, kind protocol.ToolKind, title string) *ScriptBuilder {
	return b.Emit(protocol.ToolCallStartUpdate{
		SessionUpdate: protocol.UpdateToolCall,
		ToolCall: protocol.ToolCall{
			ToolCallID: toolCallID,
			Title:      title,
			Kind:       kind,
			Status:     protocol.ToolCallStatusPending,
		},
	})
}

// ToolStatus 追加一条 tool_call_update，只改状态。
func (b *ScriptBuilder) ToolStatus(toolCallID string, status protocol.ToolCallStatus) *ScriptBuilder {
	return b.Emit(protocol.ToolCallDeltaUpdate{
		SessionUpdate:  protocol.UpdateToolCallUpdate,
		ToolCallUpdate: protocol.ToolCallUpdate{ToolCallID: toolCallID, Status: status},
	})
}

// ToolDone 追加一条 tool_call_update：完成，并附上产出。
func (b *ScriptBuilder) ToolDone(toolCallID string, content ...protocol.ToolCallContent) *ScriptBuilder {
	return b.Emit(protocol.ToolCallDeltaUpdate{
		SessionUpdate: protocol.UpdateToolCallUpdate,
		ToolCallUpdate: protocol.ToolCallUpdate{
			ToolCallID: toolCallID,
			Status:     protocol.ToolCallStatusCompleted,
			Content:    content,
		},
	})
}

// Stop 让当前轮以 reason 收尾。
func (b *ScriptBuilder) Stop(reason protocol.StopReason) *ScriptBuilder {
	return b.StopAfter(0, reason)
}

// StopAfter 让当前轮在 d 之后以 reason 收尾。
//
// **不调用 Stop / StopAfter 的轮次永不响应 session/prompt** —— 这是测超时的开关。
func (b *ScriptBuilder) StopAfter(d time.Duration, reason protocol.StopReason) *ScriptBuilder {
	turn := b.currentTurn()
	if turn == nil {
		b.recordErr(fmt.Errorf("StopAfter 之前要先 Turn()"))
		return b
	}
	turn.StopReason = reason
	turn.StopDelay = Dur(d)
	return b
}

// Build 产出脚本。构造过程中出过错就 panic —— 夹具坏了必须立刻暴露。
func (b *ScriptBuilder) Build() *Script {
	if b.err != nil {
		panic(fmt.Sprintf("fake: 构造脚本 %q 失败: %v", b.script.Name, b.err))
	}
	s := b.script
	return &s
}

func (b *ScriptBuilder) currentTurn() *Turn {
	if len(b.script.Turns) == 0 {
		return nil
	}
	return &b.script.Turns[len(b.script.Turns)-1]
}

func (b *ScriptBuilder) appendStep(step Step) *ScriptBuilder {
	turn := b.currentTurn()
	if turn == nil {
		b.recordErr(fmt.Errorf("追加事件之前要先 Turn()"))
		return b
	}
	turn.Steps = append(turn.Steps, step)
	return b
}

func (b *ScriptBuilder) takeDelay() Dur {
	d := b.pendingDelay
	b.pendingDelay = 0
	return d
}

func (b *ScriptBuilder) recordErr(err error) {
	if b.err == nil {
		b.err = err
	}
}
