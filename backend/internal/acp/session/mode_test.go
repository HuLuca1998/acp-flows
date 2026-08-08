package session_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/acp/fake"
	"github.com/HuLuca1998/acp-flows/backend/internal/acp/protocol"
	"github.com/HuLuca1998/acp-flows/backend/internal/acp/session"
)

// U2.1.2 · 用 set_config_option 收权
//
// ★★ 这一族测试守的是**收权真的收住了**。
//
// 它错了不会崩、不会报错、界面上风平浪静——实测数据摆着
// （acp-field-notes.md §3）：codex 默认档 `agent` 是 workspace-write 沙箱，
// **沙箱内的写操作根本不触发审批**。客户端那份「一律拒绝」的代码
// 权限请求收到 0 次，而文件建出来了。
//
// 换句话说：这里每一条失败的表现都是「用户的文件被静默改掉」。

// modeOption 造一个「会话模式」类的配置项。
//
// ★ 两端的 id 故意取不同的名字（真实情况：claude 的 effort /
// codex 的 reasoning_effort），而 category 相同——协议给的稳定语义键。
func modeOption(id string, values ...string) protocol.ConfigOption {
	category := protocol.ConfigCategoryMode
	opts := make([]protocol.ConfigSelectOption, len(values))
	for i, v := range values {
		opts[i] = protocol.ConfigSelectOption{Value: v, Name: v}
	}
	return protocol.ConfigOption{
		ID:           id,
		Name:         "会话模式",
		Category:     &category,
		Type:         "select",
		CurrentValue: json.RawMessage(`"` + values[0] + `"`),
		Options:      opts,
	}
}

func openWithMode(t *testing.T, rt *fake.Runtime, modeID string) (*session.Session, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	s, err := session.Open(ctx, session.Options{
		Transport:      rt.Transport(),
		Cwd:            t.TempDir(),
		RequiredModeID: modeID,
	})
	if s != nil {
		t.Cleanup(func() { _ = s.Close() })
	}
	return s, err
}

// R1 · 首选 set_config_option，不是已废弃的 set_mode。
//
// ★ `set_mode` 官方已挂废弃告示。先试旧的会让代码一直走在废弃路径上，
// 直到某天它被移除——而那时收权会**整个失效**。
func TestApplyMode_R1_PrefersSetConfigOption(t *testing.T) {
	rt := newFake(t, fake.NewScript("mode").
		Session("s-mode-1").
		NewSessionConfigOptions(modeOption("permission_mode", "agent", "read-only")).
		Modes("agent", protocol.SessionMode{ID: "agent"}, protocol.SessionMode{ID: "read-only"}).
		Build())

	if _, err := openWithMode(t, rt, "read-only"); err != nil {
		t.Fatalf("开会话: %v", err)
	}

	if n := rt.CountMethod(protocol.MethodSessionSetConfig); n != 1 {
		t.Errorf("set_config_option 调了 %d 次，想要 1 次", n)
	}
	// ★ 两个都能用时，**绝不能**走废弃的那个
	if n := rt.CountMethod(protocol.MethodSessionSetMode); n != 0 {
		t.Errorf("set_mode 调了 %d 次——它已废弃，两个都支持时该走新的", n)
	}
}

// R2 · 参数名是 `configId`，不是 `optionId`。
//
// ★★ 这条最凶险：写错字段名时 Agent 收到一个它不认识的键，
// **什么都不设，而响应仍然是成功的**。我们这边一路绿灯，
// 而会话实际跑在不受限的默认档上。
//
// 判据是**档位真的变了**，不是「请求发出去了」。
func TestApplyMode_R2_ParamIsConfigIdNotOptionId(t *testing.T) {
	rt := newFake(t, fake.NewScript("mode").
		Session("s-mode-2").
		NewSessionConfigOptions(modeOption("permission_mode", "agent", "read-only")).
		Build())

	if _, err := openWithMode(t, rt, "read-only"); err != nil {
		t.Fatalf("开会话: %v", err)
	}

	if got := rt.CurrentConfigValue("permission_mode"); got != "read-only" {
		t.Errorf("Agent 那侧的档位是 %q，想要 read-only——"+
			"字段名写成 optionId 的话就是这个症状：请求成功、档位没变", got)
	}

	// 线上帧里的键名也直接查一遍：Fake 认得 configId 是因为我们写对了，
	// 但换一个更宽容的 Agent 就查不出来了。
	var sawConfigID bool
	for _, rec := range rt.Requests() {
		if rec.Method != protocol.MethodSessionSetConfig {
			continue
		}
		var params map[string]json.RawMessage
		if err := json.Unmarshal(rec.Params, &params); err != nil {
			t.Fatalf("解 params: %v", err)
		}
		if _, ok := params["optionId"]; ok {
			t.Error("params 里出现了 optionId——协议里没有这个字段")
		}
		if _, ok := params["configId"]; ok {
			sawConfigID = true
		}
	}
	if !sawConfigID {
		t.Error("params 里没有 configId")
	}
}

// R3 · 按 category 取配置项，不按 id 取。
//
// ★ 两端 id 完全不同（这里模拟成 `claude_mode` / `codex_mode`），
// 而 category 都是 `mode`。按 id 取的话，每接一端就要维护一张映射表，
// 而漏一条的表现是「那一端根本没收权」。
func TestApplyMode_R3_LooksUpByCategoryNotID(t *testing.T) {
	for _, id := range []string{"claude_mode", "codex_mode", "某个没见过的id"} {
		rt := newFake(t, fake.NewScript("mode").
			Session("s-mode-3").
			NewSessionConfigOptions(
				// 故意在前面塞一个别的 category，确保不是「取第一项」蒙对的
				protocol.ConfigOption{ID: "effort", Type: "select"},
				modeOption(id, "agent", "read-only"),
			).
			Build())

		if _, err := openWithMode(t, rt, "read-only"); err != nil {
			t.Fatalf("id=%q 开会话: %v", id, err)
		}
		if got := rt.CurrentConfigValue(id); got != "read-only" {
			t.Errorf("id=%q 时档位是 %q，想要 read-only——按 id 取就会漏掉它", id, got)
		}
	}
}

// R4 · 没有 mode 类配置项时，降级到 set_mode。
//
// ★ 「不支持」的判据是 Agent **自己声明的能力**，不是我们猜的。
func TestApplyMode_R4_FallsBackToSetMode(t *testing.T) {
	rt := newFake(t, fake.NewScript("mode").
		Session("s-mode-4").
		// 只有 modes，没有 configOptions
		Modes("default", protocol.SessionMode{ID: "default"}, protocol.SessionMode{ID: "plan"}).
		Build())

	if _, err := openWithMode(t, rt, "plan"); err != nil {
		t.Fatalf("开会话: %v", err)
	}

	if n := rt.CountMethod(protocol.MethodSessionSetMode); n != 1 {
		t.Errorf("set_mode 调了 %d 次，想要 1 次", n)
	}
	if got := rt.CurrentModeID(); got != "plan" {
		t.Errorf("Agent 那侧的档位是 %q，想要 plan", got)
	}
}

// R5 · 两个都不支持 → 报错拒绝开工，**且一句 prompt 都不发**。
//
// ★★ 这是这个单元的底线。收不了权还继续跑，等于让 AI 在
// 不受限的档位上动用户的代码——而用户以为它是只读的。
func TestApplyMode_R5_RefusesWhenNeitherSupported(t *testing.T) {
	rt := newFake(t, fake.NewScript("mode").
		Session("s-mode-5").
		// 既没有 configOptions 也没有 modes
		Build())

	s, err := openWithMode(t, rt, "read-only")
	if !errors.Is(err, session.ErrCannotRestrictMode) {
		t.Errorf("错误 = %v，想要 ErrCannotRestrictMode", err)
	}
	if s != nil {
		t.Error("收不了权却返回了一条可用的会话")
	}
	if n := rt.CountMethod(protocol.MethodSessionPrompt); n != 0 {
		t.Errorf("发了 %d 次 prompt——收权失败后一句话都不该说", n)
	}
	if n := rt.CountMethod(protocol.MethodSessionSetConfig); n != 0 {
		t.Errorf("Agent 没声明这个能力，却发了 %d 次 set_config_option", n)
	}
}

// R6 · 设完当场回读校验。
//
// ★ 「发出去成功了」不等于「设进去了」。这里让 Agent 收下请求但不改值
// （真 Agent 在档位名不认识时就是这个行为），被测代码必须发现。
func TestApplyMode_R6_VerifiesByReadingBack(t *testing.T) {
	rt := newFake(t, fake.NewScript("mode").
		Session("s-mode-6").
		// 可选值里**没有** read-only：Fake 会照样回成功，但不改值
		NewSessionConfigOptions(modeOption("permission_mode", "agent", "agent-full-access")).
		Build())

	s, err := openWithMode(t, rt, "read-only")
	if err == nil {
		t.Fatal("档位设不进去却没报错——这正是「收权静默失败」的样子")
	}
	if !errors.Is(err, session.ErrModeNotAvailable) {
		t.Errorf("错误 = %v，想要 ErrModeNotAvailable", err)
	}
	if s != nil {
		t.Error("收权失败却返回了一条可用的会话")
	}
}

// R6b · 档位在可选值里、Agent 也收了，但回读发现值没变 → 报错。
//
// ★ 与 R6 的区别：那条是**发之前**就查出来不可用，这条是
// **发之后**才发现没生效。两条都要有——只有前者的话，
// Agent 悄悄忽略一个合法请求时我们照样一路绿灯。
func TestApplyMode_R6b_ErrsWhenReadBackDiffers(t *testing.T) {
	rt := newFake(t, fake.NewScript("mode").
		Session("s-mode-6b").
		NewSessionConfigOptions(modeOption("permission_mode", "agent", "read-only")).
		IgnoreConfigWrites().
		Build())

	s, err := openWithMode(t, rt, "read-only")
	if !errors.Is(err, session.ErrModeNotApplied) {
		t.Errorf("错误 = %v，想要 ErrModeNotApplied", err)
	}
	if s != nil {
		t.Error("回读对不上却返回了一条可用的会话")
	}
}

// 留空 RequiredModeID 时不发任何收权请求——不是所有会话都需要限制。
func TestApplyMode_EmptyModeSkipsRestriction(t *testing.T) {
	rt := newFake(t, fake.NewScript("mode").
		Session("s-mode-7").
		NewSessionConfigOptions(modeOption("permission_mode", "agent", "read-only")).
		Build())

	if _, err := openWithMode(t, rt, ""); err != nil {
		t.Fatalf("开会话: %v", err)
	}
	if n := rt.CountMethod(protocol.MethodSessionSetConfig); n != 0 {
		t.Errorf("没要求档位却发了 %d 次 set_config_option", n)
	}
}

// 降级路径上，档位不在 availableModes 里也要拒绝。
func TestApplyMode_LegacyRejectsUnavailableMode(t *testing.T) {
	rt := newFake(t, fake.NewScript("mode").
		Session("s-mode-8").
		Modes("default", protocol.SessionMode{ID: "default"}).
		Build())

	if _, err := openWithMode(t, rt, "plan"); !errors.Is(err, session.ErrModeNotAvailable) {
		t.Errorf("错误 = %v，想要 ErrModeNotAvailable", err)
	}
	if n := rt.CountMethod(protocol.MethodSessionSetMode); n != 0 {
		t.Errorf("明知不可用还发了 %d 次 set_mode", n)
	}
}
