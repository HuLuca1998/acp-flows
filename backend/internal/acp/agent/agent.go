// Package agent 把一条 ACP 会话跑完，并把 Agent 说的话翻成契约里的工作事件。
//
// ★ 这一层只做**翻译与转发**，不碰持久化、不认识 Work 的状态机。
// 谁在什么时候拉起它，是 app 层的事（见 internal/app/work）。
//
// ★ 翻译是显式表驱动的：ACP 的 13 类 sessionUpdate 与契约的 13 类 Event
// **不是**一一对应（`agent_message_chunk` → `message_chunk`，
// `tool_call` 与 `tool_call_update` 都归到 `tool_call`）。
// 靠字符串相等碰运气的话，前端注册表会收到一堆认不出的类型，
// 全部落到「暂时看不懂的记录」上。
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/HuLuca1998/acp-flows/backend/internal/acp/protocol"
	"github.com/HuLuca1998/acp-flows/backend/internal/acp/session"
	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
)

// WorkEvent 就是契约里的那条事件。
//
// ★ 用**类型别名**而不是重新定义一个同形结构体：两份定义必然漂移，
// 而漂移的表现是「后端发了事件、前端收不到某个字段」，没有任何检查会红。
type WorkEvent = port.WorkEvent

// Sink 收下翻译好的事件。
//
// ★ 实现**必须立刻返回**：它跑在 jsonrpc 的读 goroutine 上，
// 在里面做慢操作等于卡住整条会话（见 session.Session.onEvent 的说明）。
type Sink interface {
	Emit(WorkEvent)
}

// SinkFunc 让普通函数当 Sink 用。
type SinkFunc func(WorkEvent)

// Emit 实现 Sink。
func (f SinkFunc) Emit(e WorkEvent) { f(e) }

// Spec 是跑一轮需要的东西。
type Spec struct {
	// Transport 是与 Agent 进程的双工通道（runtime.Process 的 stdio，
	// 或测试里 Fake Runtime 的管道）。
	Transport io.ReadWriteCloser
	// Cwd 是 Agent 的工作目录，必须是已存在的绝对路径。
	//
	// ★ 这里应当是工作**自己的 worktree**，不是用户的仓库目录——
	// 用户把代码交给 Duet 时并没有同意我们直接在他的分支上改。
	Cwd string
	// WorkID 会打在每条事件上，前端靠它过滤。
	WorkID string
	// Prompt 是要发给 Agent 的需求。
	Prompt string
	// SystemPrompt 只在**这条会话的第一轮**拼到需求前面。
	//
	// ★ 每轮都发的话，几十轮之后上下文里全是重复的同一段话，
	// 既烧 token 又稀释用户真正说的内容（U2.2.2 的 R3）。
	SystemPrompt string
	// Sink 收事件；为 nil 时事件被丢弃（只跑不看）。
	Sink Sink
	// Log 可选；为 nil 时用 slog.Default()。
	Log *slog.Logger
}

// timelineType 是 ACP 判别值 → 契约事件类型的**唯一真源**。
//
// ★ 值为空串表示「认识它，但它不上时间线」——会话元数据（可用命令、用量、
// 配置项）对用户没有可读的形态，塞进时间线只会淹掉真正的进展。
// 空串是**显式决定**，不是遗漏：新增一类 ACP 事件时，
// TestTimelineType_CoversEveryKind 会因为表里缺一行而红。
var timelineType = map[protocol.SessionUpdateKind]string{
	protocol.UpdateUserMessageChunk:  "message_chunk",
	protocol.UpdateAgentMessageChunk: "message_chunk",
	protocol.UpdateAgentThoughtChunk: "thought_chunk",

	// tool_call 是「开始调工具」，tool_call_update 是同一次调用的状态变化。
	// 前端把它们渲染成同一张卡片（靠 payload 里的 toolCallId 归并），
	// 所以这里归到同一类。
	protocol.UpdateToolCall:       "tool_call",
	protocol.UpdateToolCallUpdate: "tool_call",

	// 计划的三种变化都是「计划又变了一版」。
	protocol.UpdatePlan:        "plan_version",
	protocol.UpdatePlanUpdate:  "plan_version",
	protocol.UpdatePlanRemoved: "plan_version",

	// 模式切换是用户看得见的状态变化（比如从 plan 切到 execute）。
	protocol.UpdateCurrentModeUpdate: "state_change",

	// ↓ 认识但不上时间线，理由见上。
	protocol.UpdateAvailableCommandsUpdate: "",
	protocol.UpdateConfigOptionUpdate:      "",
	protocol.UpdateSessionInfoUpdate:       "",
	protocol.UpdateUsageUpdate:             "",
}

// TimelineTypeOf 查翻译表：typ 为空串表示「认识它但不上时间线」，
// ok 为 false 表示表里压根没有这一类。
//
// 导出只为让测试能验「表盖住了全集」——那条测试是本包真正的守卫。
func TimelineTypeOf(kind protocol.SessionUpdateKind) (typ string, ok bool) {
	typ, ok = timelineType[kind]
	return typ, ok
}

// Run 开一条会话、发一次需求，把 Agent 说的话边说边翻成工作事件。
//
// ★ 返回 error 时事件**照样已经发出去了**：轮次被截断（max_tokens）或被拒绝
// （refusal）都算错误，但用户有权看到 AI 在被打断前说了什么。
// 唯一一个事件都不发的情况是这一轮压根没开始（比如 cwd 非法）。
func Run(ctx context.Context, spec Spec) error {
	log := spec.Log
	if log == nil {
		log = slog.Default()
	}

	s, err := session.Open(ctx, session.Options{
		Transport: spec.Transport,
		Cwd:       spec.Cwd,
	})
	if err != nil {
		return fmt.Errorf("agent: open session: %w", err)
	}
	defer func() { _ = s.Close() }()

	prompt := spec.Prompt
	if spec.SystemPrompt != "" {
		prompt = spec.SystemPrompt + "\n\n" + prompt
	}

	emit := func(e WorkEvent) {
		if spec.Sink != nil {
			spec.Sink.Emit(e)
		}
	}

	reason, promptErr := s.Prompt(ctx, prompt, func(ev session.Event) {
		e, ok := translate(spec.WorkID, ev)
		if !ok {
			// 认识但不上时间线的那几类，静静走过；真正没见过的记一条警告——
			// 静默吞掉的话，Agent 新增一类事件时界面上表现为
			// 「AI 好像少说了点什么」，没有任何报错。
			if ev.Unhandled {
				log.Warn("agent: 收到没见过的 sessionUpdate",
					"kind", string(ev.Kind), "work_id", spec.WorkID)
			}
			return
		}
		emit(e)
	})

	// ★ 轮次结束一定要发。不发的话，界面上 AI 说完最后一个字就停在那儿，
	// 用户不知道它是说完了还是卡住了。
	//
	// reason 即便在出错时也是真实值（session.Prompt 的契约），
	// 因为 max_tokens 与 end_turn 对用户是两件事——前者意味着产出是半截的。
	emit(WorkEvent{
		WorkID:  spec.WorkID,
		Source:  "acp",
		Type:    "turn_end",
		Payload: map[string]any{"reason": string(reason)},
	})

	if promptErr != nil && !errors.Is(promptErr, session.ErrTurnIncomplete) {
		return fmt.Errorf("agent: prompt: %w", promptErr)
	}
	return promptErr
}

// translate 把一条会话事件翻成工作事件；ok 为 false 表示这一条不上时间线。
func translate(workID string, ev session.Event) (WorkEvent, bool) {
	typ, known := timelineType[ev.Kind]
	if !known || typ == "" {
		return WorkEvent{}, false
	}

	payload := map[string]any{}

	// ★ 原始载荷**先铺进去**，我们的元信息后放且用带前缀的键。
	//
	// 反过来做会出事：ACP 的 tool_call 载荷自带一个 `kind`（工具种类
	// read/edit/execute），我们的判别值也叫 kind，于是同一个键有时是
	// "tool_call_update"、有时是 "read"，前端没法可靠使用——
	// 真机上的表现是四条工具调用卡片长得一模一样，看不出 AI 在读哪个文件。
	//
	// 原样带上而不是解成具体变体再拼回去：那样每加一个字段就要改这里。
	if len(ev.Raw) > 0 {
		var raw map[string]any
		if err := json.Unmarshal(ev.Raw, &raw); err == nil {
			for k, v := range raw {
				if k == "sessionUpdate" {
					continue
				}
				payload[k] = v
			}
		}
	}

	// ↓ 我们加的东西，一律 acp_ 前缀，永远不会和 Agent 的字段撞名
	payload["acp_kind"] = string(ev.Kind)
	if ev.Text != "" {
		payload["text"] = ev.Text
	}

	return WorkEvent{
		WorkID:  workID,
		Source:  "acp",
		Type:    typ,
		Payload: payload,
	}, true
}
