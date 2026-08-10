package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/HuLuca1998/acp-flows/backend/internal/acp/protocol"
)

var (
	// ErrModeNotAvailable 表示要设的档位不在 Agent 提供的可选值里。
	//
	// ★ 发之前先查，别指望 Agent 报错：实测里设一个不存在的档位
	// **响应仍然是成功的**，只是什么都没发生。
	ErrModeNotAvailable = errors.New("session: 这个档位不在 Agent 的可选值里")

	// ErrCannotRestrictMode 表示这个 Agent 既不支持 set_config_option
	// 也不支持 set_mode——**收不了权**。
	//
	// ★★ 遇到它必须**拒绝开工**，不能假装收了。
	//
	// 实测数据摆在那儿（acp-field-notes.md §3）：codex 默认档 `agent` 是
	// workspace-write 沙箱，**沙箱内的写操作根本不触发审批**——
	// 客户端那份「一律拒绝」的代码一次都不会被调用到，权限请求 0 次、
	// 文件照建。也就是说：收权失败时我们这侧**看不到任何异常**，
	// 用户的文件被改了而界面上风平浪静。
	ErrCannotRestrictMode = errors.New("session: 这个 Agent 收不了权（两个方法都不支持）")

	// ErrModeNotApplied 表示设完之后回读，值不是我们设的那个。
	ErrModeNotApplied = errors.New("session: 档位设了但没生效")
)

// applyMode 把会话档位设成 modeID。
//
// 顺序：**先试 `session/set_config_option`**，不支持再降级到 `session/set_mode`，
// 两个都不支持就报错。
//
// ★ 为什么新方法优先：`set_mode` 官方已挂废弃告示，迁移方向就是
// `set_config_option`（acp-field-notes.md §7 裁定 3）。先试旧的会让代码
// 一直走在废弃路径上，直到某天它被移除。
func (s *Session) applyMode(ctx context.Context, resp protocol.NewSessionResponse, modeID string) error {
	if opt, ok := modeConfigOption(resp.ConfigOptions); ok {
		return s.setConfigOption(ctx, opt, modeID)
	}
	if resp.Modes != nil && len(resp.Modes.AvailableModes) > 0 {
		return s.setModeLegacy(ctx, resp.Modes, modeID)
	}
	return fmt.Errorf("%w: 想设 %q", ErrCannotRestrictMode, modeID)
}

// modeConfigOption 从配置项里找出「会话模式」那一项。
//
// ★ **按 category 取，不按 id 取。** 两端的 id 不一样，category 才是协议给的
// 稳定语义键（acp-field-notes.md §7.1）——推理强度那一项就是最好的实证：
// claude 叫 `effort`、codex 叫 `reasoning_effort`，而 category 同为
// `thought_level`。按 id 取的话，每加一端就要维护一张映射表。
func modeConfigOption(opts []protocol.ConfigOption) (protocol.ConfigOption, bool) {
	for _, o := range opts {
		if o.CategoryOrEmpty() == protocol.ConfigCategoryMode {
			return o, true
		}
	}
	return protocol.ConfigOption{}, false
}

// setConfigOption 走新方法。
func (s *Session) setConfigOption(ctx context.Context, opt protocol.ConfigOption, modeID string) error {
	if !hasSelectValue(opt.Options, modeID) {
		return fmt.Errorf("%w: %q 不在 %v 里", ErrModeNotAvailable, modeID, selectValues(opt.Options))
	}

	value, err := json.Marshal(modeID)
	if err != nil {
		return fmt.Errorf("编码档位值 %q: %w", modeID, err)
	}

	var resp protocol.SetConfigOptionResponse
	if err := s.conn.CallInto(ctx, protocol.MethodSessionSetConfig, protocol.SetConfigOptionRequest{
		SessionID: s.id,
		// ★★ 字段名是 `configId`，不是 `optionId`。写错的话 Agent 收到一个
		// 不认识的字段，配置**静默不生效**而响应仍然成功。
		ConfigID: opt.ID,
		Value:    value,
	}, &resp); err != nil {
		return fmt.Errorf("%s: %w", protocol.MethodSessionSetConfig, err)
	}

	// ★ **当场回读校验。** 「发出去成功了」不等于「设进去了」——
	// 这条链路上的失败全都是静默的，不自己验就没人验。
	return verifyApplied(resp.ConfigOptions, opt.ID, modeID)
}

// setModeLegacy 走已废弃的 set_mode。
//
// ★ 只在 Agent 没有 mode 类配置项时走这里。
func (s *Session) setModeLegacy(ctx context.Context, modes *protocol.SessionModeState, modeID string) error {
	if !hasMode(modes.AvailableModes, modeID) {
		return fmt.Errorf("%w: %q 不在 %v 里", ErrModeNotAvailable, modeID, modeIDs(modes.AvailableModes))
	}

	var resp protocol.SetModeResponse
	if err := s.conn.CallInto(ctx, protocol.MethodSessionSetMode, protocol.SetModeRequest{
		SessionID: s.id,
		ModeID:    modeID,
	}, &resp); err != nil {
		return fmt.Errorf("%s: %w", protocol.MethodSessionSetMode, err)
	}
	// set_mode 的响应是空对象，回读不了——这也是它被废弃的原因之一。
	return nil
}

// verifyApplied 在响应带回的配置项里核对值真的变了。
func verifyApplied(opts []protocol.ConfigOption, configID, want string) error {
	for _, o := range opts {
		if o.ID != configID {
			continue
		}
		var got string
		if err := json.Unmarshal(o.CurrentValue, &got); err != nil {
			return fmt.Errorf("%w: 回读 %q 的值解不开: %w", ErrModeNotApplied, configID, err)
		}
		if got != want {
			return fmt.Errorf("%w: 设的是 %q，回读是 %q", ErrModeNotApplied, want, got)
		}
		return nil
	}
	// ★ 响应里没有这一项，就当没设成功。
	//
	// 宽容处理（「没回读到就算了」）的后果是：Agent 哪天改了响应格式，
	// 我们会一路绿灯地在不受限的档位上跑——而这正是最坏的失败。
	return fmt.Errorf("%w: 响应里没有 %q 这一项", ErrModeNotApplied, configID)
}

func hasSelectValue(opts []protocol.ConfigSelectOption, value string) bool {
	for _, o := range opts {
		if o.Value == value {
			return true
		}
	}
	return false
}

func selectValues(opts []protocol.ConfigSelectOption) []string {
	out := make([]string, len(opts))
	for i, o := range opts {
		out[i] = o.Value
	}
	return out
}

func hasMode(modes []protocol.SessionMode, id string) bool {
	for _, m := range modes {
		if m.ID == id {
			return true
		}
	}
	return false
}

func modeIDs(modes []protocol.SessionMode) []string {
	out := make([]string, len(modes))
	for i, m := range modes {
		out[i] = m.ID
	}
	return out
}
