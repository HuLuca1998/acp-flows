package fake

import (
	"encoding/json"
	"fmt"

	"github.com/HuLuca1998/acp-flows/backend/internal/acp/protocol"
)

// ★★ 这个文件让 Fake 学会说 `session/set_config_option` 与 `session/set_mode`。
//
// **它刻意模仿真 Agent 的坏脾气**：
//
//   - 字段名写错（`optionId` 而不是 `configId`）→ **响应仍然成功**，
//     只是什么都没设进去。真 Agent 就是这样，而这正是最难查的失败。
//   - 设一个不存在的档位 → 同样成功，同样什么都没发生。
//
// 如果 Fake 在这两种情况下报错，被测代码里那句「当场回读校验」
// 就永远测不出价值——而那句话是唯一能发现静默失败的东西。

// respondSetConfig 处理 session/set_config_option。
func (r *Runtime) respondSetConfig(w *frameWriter, id, params json.RawMessage) error {
	var req struct {
		SessionID string          `json:"sessionId"`
		ConfigID  string          `json:"configId"`
		Value     json.RawMessage `json:"value"`
	}
	// 解不开就当成空请求——真 Agent 也不会因为多了个不认识的字段就崩。
	_ = json.Unmarshal(params, &req)

	ignore := r.script.NewSession != nil && r.script.NewSession.IgnoreConfigWrites

	r.mu.Lock()
	applied := !ignore && r.applyConfigLocked(req.ConfigID, req.Value)
	out := r.configSnapshotLocked()
	r.mu.Unlock()

	// ★ **设没设进去，响应都是成功的。** 这一句是本文件存在的理由：
	// `configId` 拼错时 Agent 不会告诉你，它只是安静地什么都不做。
	// 被测代码只能靠回读发现——所以那句回读必须有测试守着。
	_ = applied
	return w.respond(id, protocol.SetConfigOptionResponse{ConfigOptions: out})
}

// applyConfigLocked 找到那一项并改掉它的当前值。调用方持锁。
func (r *Runtime) applyConfigLocked(configID string, value json.RawMessage) bool {
	if configID == "" || len(value) == 0 {
		return false
	}
	for i := range r.configOptions {
		if r.configOptions[i].ID != configID {
			continue
		}
		var want string
		if err := json.Unmarshal(value, &want); err != nil {
			return false
		}
		// 不在可选值里 → 不改，但也不报错（真 Agent 的行为）
		found := false
		for _, o := range r.configOptions[i].Options {
			if o.Value == want {
				found = true
				break
			}
		}
		if !found {
			return false
		}
		r.configOptions[i].CurrentValue = append(json.RawMessage(nil), value...)
		return true
	}
	return false
}

func (r *Runtime) configSnapshotLocked() []protocol.ConfigOption {
	out := make([]protocol.ConfigOption, len(r.configOptions))
	copy(out, r.configOptions)
	return out
}

// respondSetMode 处理已废弃的 session/set_mode。
//
// ★ 响应是**空对象**——没有回读的余地。这也是它被废弃的原因之一：
// 调用方无从知道设进去没有。
func (r *Runtime) respondSetMode(w *frameWriter, id, params json.RawMessage) error {
	var req struct {
		SessionID string `json:"sessionId"`
		ModeID    string `json:"modeId"`
	}
	_ = json.Unmarshal(params, &req)

	r.mu.Lock()
	for _, m := range r.script.modes() {
		if m.ID == req.ModeID {
			r.currentModeID = req.ModeID
			break
		}
	}
	r.mu.Unlock()

	return w.respond(id, protocol.SetModeResponse{})
}

// CurrentModeID 返回 Fake 当前处在哪个档位（走 set_mode 设的）。
//
// ★ 断言用：只发了请求不等于设进去了。
func (r *Runtime) CurrentModeID() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.currentModeID
}

// CurrentConfigValue 返回某个配置项的当前值（走 set_config_option 设的）。
//
// 找不到那一项时返回空串——测试里用它断言「拼错字段名时什么都没设进去」。
func (r *Runtime) CurrentConfigValue(configID string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, o := range r.configOptions {
		if o.ID != configID {
			continue
		}
		var v string
		if err := json.Unmarshal(o.CurrentValue, &v); err != nil {
			return fmt.Sprintf("<解不开: %s>", o.CurrentValue)
		}
		return v
	}
	return ""
}
