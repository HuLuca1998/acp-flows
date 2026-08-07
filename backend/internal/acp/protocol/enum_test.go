package protocol_test

// protocol 线格式包 —— 枚举穷举（原 U0.2.3，编号已废弃）
//
// 取值全集来自 @agentclientprotocol/sdk@1.3.0 的 dist/schema/types.gen.d.ts。
// 原单元编号 U0.2.3（已废弃）。能力归属见 docs/plan/roadmap.md 的「已经就绪的地基」。

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/acp/protocol"
)

// R3：四个封闭枚举各自穷举，且拒绝表外取值。
//
// 这四个是同一条契约（全集正确 + IsValid 拒绝表外）在四个类型上的实例，
// 所以用一张表 —— 加第五个枚举时只加一行，不用复制一遍测试函数。
//
// ★ 为什么必须断言「字面量」而不只是「数量」：常量值写错时数量照样对，
// 但线上发出去的 JSON 是错的，对方只会回一个语焉不详的 -32602。
func TestProtocolEnums_R3_ExhaustiveAndRejectUnknown(t *testing.T) {
	tests := []struct {
		name string
		// all 是该枚举的取值全集，按官方 schema 的声明顺序。
		all []string
		// unknown 是一个表外取值，必须被 IsValid() 拒绝。
		unknown string
		// isValid 把字符串套进具名类型再问它合不合法。
		isValid func(string) bool
	}{
		{
			name:    "StopReason",
			all:     stopReasonLiterals(),
			unknown: "timeout", // 看起来很合理，但协议里没有——这正是容易被编出来的取值
			isValid: func(s string) bool { return protocol.StopReason(s).IsValid() },
		},
		{
			name:    "ToolKind",
			all:     toolKindLiterals(),
			unknown: "write", // 协议里是 "edit"，不是 "write"
			isValid: func(s string) bool { return protocol.ToolKind(s).IsValid() },
		},
		{
			name:    "ToolCallStatus",
			all:     toolCallStatusLiterals(),
			unknown: "cancelled", // stopReason 才有 cancelled，工具状态没有
			isValid: func(s string) bool { return protocol.ToolCallStatus(s).IsValid() },
		},
		{
			name:    "PermissionOptionKind",
			all:     permissionOptionKindLiterals(),
			unknown: "allow", // 真实取值是 allow_once / allow_always，没有裸 allow
			isValid: func(s string) bool { return protocol.PermissionOptionKind(s).IsValid() },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, v := range tt.all {
				if !tt.isValid(v) {
					t.Errorf("%q 属于 %s 全集，IsValid() 必须为 true", v, tt.name)
				}
			}
			if tt.isValid(tt.unknown) {
				t.Errorf("%q 不在 %s 全集里，IsValid() 必须为 false", tt.unknown, tt.name)
			}
			// 空值一律非法：漏填字段与合法取值不能混为一谈。
			if tt.isValid("") {
				t.Errorf("%s 的空值必须非法（漏填 ≠ 合法取值）", tt.name)
			}
		})
	}
}

// 下面四个函数把「全集」写成字面量再与包里的 All* 对照。
// 两边都从包里取的话，常量写错时测试跟着一起错，等于没测。

func stopReasonLiterals() []string {
	want := []string{"end_turn", "max_tokens", "max_turn_requests", "refusal", "cancelled"}
	return want
}

func toolKindLiterals() []string {
	return []string{"read", "edit", "delete", "move", "search", "execute",
		"think", "fetch", "switch_mode", "other"}
}

func toolCallStatusLiterals() []string {
	return []string{"pending", "in_progress", "completed", "failed"}
}

func permissionOptionKindLiterals() []string {
	return []string{"allow_once", "allow_always", "reject_once", "reject_always"}
}

// All* 返回的全集必须与官方 schema 逐字一致 —— 上面那张表只验证了
// 「字面量是合法的」，这里补上「包里声明的全集不多不少」。
func TestProtocolEnums_R3_AllSlicesMatchSchema(t *testing.T) {
	t.Run("StopReason", func(t *testing.T) {
		got := make([]string, 0, len(protocol.AllStopReasons()))
		for _, v := range protocol.AllStopReasons() {
			got = append(got, string(v))
		}
		if !slices.Equal(stopReasonLiterals(), got) {
			t.Errorf("want %v, got %v", stopReasonLiterals(), got)
		}
	})
	t.Run("ToolKind", func(t *testing.T) {
		got := make([]string, 0, len(protocol.AllToolKinds()))
		for _, v := range protocol.AllToolKinds() {
			got = append(got, string(v))
		}
		if !slices.Equal(toolKindLiterals(), got) {
			t.Errorf("want %v, got %v", toolKindLiterals(), got)
		}
	})
	t.Run("ToolCallStatus", func(t *testing.T) {
		got := make([]string, 0, len(protocol.AllToolCallStatuses()))
		for _, v := range protocol.AllToolCallStatuses() {
			got = append(got, string(v))
		}
		if !slices.Equal(toolCallStatusLiterals(), got) {
			t.Errorf("want %v, got %v", toolCallStatusLiterals(), got)
		}
	})
	t.Run("PermissionOptionKind", func(t *testing.T) {
		got := make([]string, 0, len(protocol.AllPermissionOptionKinds()))
		for _, v := range protocol.AllPermissionOptionKinds() {
			got = append(got, string(v))
		}
		if !slices.Equal(permissionOptionKindLiterals(), got) {
			t.Errorf("want %v, got %v", permissionOptionKindLiterals(), got)
		}
	})
}

// 只有 end_turn 算正常收尾 —— 其余四种都是「没干完」。
//
// 前一个项目的 H-5 就是把所有 stopReason 都当成功处理，
// 结果 max_tokens 截断的半成品被当作已完成验收（acp-field-notes.md §1）。
// 这条契约放在 protocol 层，是为了让上层不可能"忘了判断"。
func TestStopReason_OnlyEndTurnIsSuccess(t *testing.T) {
	if !protocol.StopReasonEndTurn.IsSuccess() {
		t.Error("end_turn 必须算正常收尾")
	}
	for _, r := range protocol.AllStopReasons() {
		if r == protocol.StopReasonEndTurn {
			continue
		}
		if r.IsSuccess() {
			t.Errorf("%q 不是正常收尾，却被 IsSuccess() 判为成功", r)
		}
	}
}

// 取消的应答必须能被判别出来：outcome 是 "cancelled" 还是 "selected"。
//
// 两段式取消要求「取消时用 cancelled 应答所有 pending 的权限请求」
// （acp-integration.md §6.3 义务 2）。这一层分不出来的话，
// 上层每次取消都会超时，M1 的 prepare 永远返回 blocked。
func TestRequestPermissionOutcome_DistinguishesCancelledFromSelected(t *testing.T) {
	t.Run("cancelled", func(t *testing.T) {
		var resp protocol.RequestPermissionResponse
		raw := []byte(`{"outcome":{"outcome":"cancelled"}}`)
		if err := json.Unmarshal(raw, &resp); err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		if !resp.Outcome.IsCancelled() {
			t.Error("outcome=cancelled 必须被判为已取消")
		}
		if id := resp.Outcome.SelectedOptionID(); id != "" {
			t.Errorf("取消时不该有选中项，got %q", id)
		}
	})

	t.Run("selected", func(t *testing.T) {
		var resp protocol.RequestPermissionResponse
		raw := []byte(`{"outcome":{"outcome":"selected","optionId":"allow-once"}}`)
		if err := json.Unmarshal(raw, &resp); err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		if resp.Outcome.IsCancelled() {
			t.Error("outcome=selected 不该被判为已取消")
		}
		if id := resp.Outcome.SelectedOptionID(); id != "allow-once" {
			t.Errorf("选中项: want %q, got %q", "allow-once", id)
		}
	})

	// 应答要能发得出去，不只是读得进来 —— Fake Runtime 收到的就是这个。
	// ★ selected 带 optionId、cancelled 不带：多写一个 optionId:"" 会让
	// Agent 以为客户端选了一个 id 为空串的选项。
	t.Run("发出 cancelled", func(t *testing.T) {
		resp := protocol.RequestPermissionResponse{
			Outcome: protocol.CancelledOutcome(),
		}
		out, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("序列化失败: %v", err)
		}
		const want = `{"outcome":{"outcome":"cancelled"}}`
		if string(out) != want {
			t.Errorf("want %s, got %s", want, out)
		}
	})

	t.Run("发出 selected", func(t *testing.T) {
		resp := protocol.RequestPermissionResponse{
			Outcome: protocol.SelectedOutcome("allow-once"),
		}
		out, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("序列化失败: %v", err)
		}
		const want = `{"outcome":{"outcome":"selected","optionId":"allow-once"}}`
		if string(out) != want {
			t.Errorf("want %s, got %s", want, out)
		}
	})
}

// 按 category 取配置项时，「字段缺失」与「空字符串」都不能让调用方崩。
//
// ★ 实测：claude 的 agent 配置项 **category 是空字符串**（acp-field-notes.md §7.1 N2）。
// 差异内化的整个方案建立在「按 category 取」之上（推理强度两端 id 不同、category 相同），
// 所以这条路径上的 panic 会直接打掉 adapter 层的核心机制。
func TestConfigOption_CategoryOrEmpty_HandlesMissingAndBlank(t *testing.T) {
	raw := []byte(`[
	  {"id":"reasoning_effort","name":"Reasoning effort","category":"thought_level","type":"select","currentValue":"medium"},
	  {"id":"agent","name":"Agent","category":"","type":"boolean","currentValue":true},
	  {"id":"legacy","name":"Legacy","type":"boolean","currentValue":false}
	]`)

	var opts []protocol.ConfigOption
	if err := json.Unmarshal(raw, &opts); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(opts) != 3 {
		t.Fatalf("want 3 个配置项, got %d", len(opts))
	}

	if got := opts[0].CategoryOrEmpty(); got != "thought_level" {
		t.Errorf("正常 category: want %q, got %q", "thought_level", got)
	}
	if got := opts[1].CategoryOrEmpty(); got != "" {
		t.Errorf("空字符串 category: want %q, got %q", "", got)
	}
	if got := opts[2].CategoryOrEmpty(); got != "" {
		t.Errorf("缺失 category: want %q, got %q", "", got)
	}

	// 两者在线上要能区分开：空字符串原样写回，缺失的仍然缺失。
	// 混同的话，回写配置时会给本来没有 category 的选项凭空加一个。
	out, err := json.Marshal(opts)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var back []map[string]any
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatal(err)
	}
	if _, ok := back[1]["category"]; !ok {
		t.Error("空字符串 category 被丢掉了 —— 它是实测存在的真实取值")
	}
	if _, ok := back[2]["category"]; ok {
		t.Error("原本没有 category 的选项被凭空加上了")
	}
}
