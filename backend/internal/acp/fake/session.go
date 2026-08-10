package fake

import (
	"encoding/json"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/acp/protocol"
)

// session/new 与 session/load 的应答。
//
// ★ 从 runtime.go 拆出来（2026-08-08，加 configOptions 之后那个文件到了 409 行）。
// 这两个方法是一类：都在建/接一条会话，都要交代模式与配置项的初始状态。

func (r *Runtime) respondNewSession(w *frameWriter, id json.RawMessage) error {
	behavior := r.script.NewSession
	if behavior != nil && behavior.Delay > 0 {
		time.Sleep(time.Duration(behavior.Delay))
	}

	resp := protocol.NewSessionResponse{SessionID: r.script.sessionID()}

	// 声明了配置项就带上，并把它们记成会话的当前状态——
	// 后面 set_config_option 改的就是这份。
	if behavior != nil && len(behavior.ConfigOptions) > 0 {
		r.mu.Lock()
		r.configOptions = append([]protocol.ConfigOption(nil), behavior.ConfigOptions...)
		resp.ConfigOptions = r.configSnapshotLocked()
		r.mu.Unlock()
	}
	// 不声明 modes 时响应里就**真的没有** modes 字段 ——
	// 假的能力声明必须表现为真的协议行为，否则测的是我们自己的探针代码。
	if behavior != nil && len(behavior.Modes) > 0 {
		resp.Modes = &protocol.SessionModeState{
			CurrentModeID:  behavior.CurrentModeID,
			AvailableModes: behavior.Modes,
		}
		r.mu.Lock()
		r.currentModeID = behavior.CurrentModeID
		r.mu.Unlock()
	}
	return w.respond(id, resp)
}

// respondLoadSession 处理 session/load。
//
// ★ 不支持时回 **-32601**，那是真 Agent 的做法。回一个空的成功响应
// 会让上层以为恢复成功了——而 Agent 那边根本没有这条会话，
// 下一轮打过去就是「会话不存在」，而用户看到的只是「AI 忽然变笨了」。
func (r *Runtime) respondLoadSession(w *frameWriter, f wireFrame) error {
	behavior := r.script.Load
	if behavior == nil || !behavior.Supported {
		return w.respondError(f.ID, -32601, "fake: 本 Agent 不支持 session/load")
	}
	if behavior.Delay > 0 {
		time.Sleep(time.Duration(behavior.Delay))
	}

	// ★ 记下被恢复的是哪一条：上层「恢复了却打在别的会话上」这类 bug
	// 只有对着它才测得出来。
	var req protocol.LoadSessionRequest
	if err := json.Unmarshal(f.Params, &req); err == nil && req.SessionID != "" {
		r.mu.Lock()
		r.loadedSessionID = req.SessionID
		r.mu.Unlock()
	}
	return w.respond(f.ID, protocol.LoadSessionResponse{})
}

// LoadedSessionID 返回最后一次 session/load 恢复的会话标识。
func (r *Runtime) LoadedSessionID() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.loadedSessionID
}
