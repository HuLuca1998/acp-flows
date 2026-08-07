package adapter_test

import (
	"encoding/json"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/acp/adapter"
	"github.com/HuLuca1998/acp-flows/backend/internal/acp/protocol"
)

// U2.2.3 · 两个 Agent 的差异内化（验收点 V5）
//
// ★ **这一批断言跑遍两端，测试代码里没有一处 `if impl ==`。**
// 有的话就说明差异没被内化——只是从生产代码搬进了测试代码。

// 两端 session/new 响应里配置项的**真实形态**（acp-field-notes.md §7.1 实测）。
//
// 推理强度的 id 两端不同，但 category 都是 thought_level ——
// 这正是「按 category 取」的由来。
var realWorldConfigs = map[string][]protocol.ConfigOption{
	"claude": {
		{ID: "effort", Name: "Effort", Category: strptr("thought_level"), Type: "select",
			CurrentValue: json.RawMessage(`"medium"`)},
		// ★ claude 的 agent 选项 category 是**空字符串**，不是缺失
		{ID: "agent", Name: "Agent", Category: strptr(""), Type: "select",
			CurrentValue: json.RawMessage(`"default"`)},
	},
	"codex": {
		{ID: "reasoning_effort", Name: "Reasoning effort", Category: strptr("thought_level"),
			Type: "select", CurrentValue: json.RawMessage(`"high"`)},
		// codex 的某些选项**根本没有 category 字段**
		{ID: "sandbox", Name: "Sandbox", Type: "boolean",
			CurrentValue: json.RawMessage(`true`)},
	},
}

func strptr(s string) *string { return &s }

// ★★ R1：配置项**按 category 取，不按 id 取**。
//
// 按 id 取的话就要维护一张 claude→effort / codex→reasoning_effort 的映射表，
// 而每加一个 Runtime 就要加一行——那正是「差异没内化」的样子。
// 协议本身给了语义层的稳定键，用它就好。
func TestThoughtLevel_FoundByCategoryOnBothEnds(t *testing.T) {
	for name, configs := range realWorldConfigs {
		t.Run(name, func(t *testing.T) {
			opt, ok := adapter.ConfigByCategory(configs, adapter.CategoryThoughtLevel)
			if !ok {
				t.Fatalf("按 category 取不到推理强度——"+
					"退化成按 id 取的话，每加一个 Runtime 就要加一行映射（配置项：%+v）", configs)
			}
			if opt.CurrentValue == nil {
				t.Error("取到了选项但没有当前值")
			}
		})
	}
}

// category 缺失与空串是两回事，两种都不能 panic。
//
// claude 的 agent 选项 category 是空字符串，codex 的 sandbox 根本没这个字段——
// 直接解引用会在其中一端崩掉，而崩的是用户打开设置页的时候。
func TestConfigByCategory_ToleratesMissingAndEmpty(t *testing.T) {
	for name, configs := range realWorldConfigs {
		t.Run(name, func(t *testing.T) {
			// 查一个不存在的 category：不能 panic，返回 false 就好
			if _, ok := adapter.ConfigByCategory(configs, "no_such_category"); ok {
				t.Error("查不存在的 category 却返回了 true")
			}
		})
	}
}

// 空列表不是错误——有的 Runtime 可能一个配置项都不给。
func TestConfigByCategory_EmptyList(t *testing.T) {
	if _, ok := adapter.ConfigByCategory(nil, adapter.CategoryThoughtLevel); ok {
		t.Error("空列表却返回了 true")
	}
}

// ★ R5：能力矩阵**由探测结果算出来**，不是写死的。
//
// 写死的话，一个 Runtime 悄悄不支持某能力时矩阵还显示通过——
// 上层据此做降级判断，于是走进一条它其实走不通的路。
func TestCapabilities_DerivedFromProbes(t *testing.T) {
	tests := []struct {
		name       string
		probes     []adapter.Probe
		wantPassed int
		wantTotal  int
	}{
		{
			name: "全部通过",
			probes: []adapter.Probe{
				{ID: "streaming_thoughts", Passed: true},
				{ID: "session_modes", Passed: true},
			},
			wantPassed: 2, wantTotal: 2,
		},
		{
			// Fake 声明不支持会话模式时，矩阵对应项必须是不通过
			name: "有一项不支持",
			probes: []adapter.Probe{
				{ID: "streaming_thoughts", Passed: true},
				{ID: "session_modes", Passed: false, Detail: "not observed"},
			},
			wantPassed: 1, wantTotal: 2,
		},
		{
			name:       "一个探针都没跑",
			probes:     nil,
			wantPassed: 0, wantTotal: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := adapter.NewCapabilityMatrix(tt.probes)

			if m.Passed != tt.wantPassed {
				t.Errorf("Passed = %d, 想要 %d", m.Passed, tt.wantPassed)
			}
			if m.Total != tt.wantTotal {
				t.Errorf("Total = %d, 想要 %d", m.Total, tt.wantTotal)
			}
			if len(m.Probes) != len(tt.probes) {
				t.Errorf("探针明细少了：%d 条，想要 %d 条", len(m.Probes), len(tt.probes))
			}
		})
	}
}

// 上层靠 Supports 做降级判断，**不靠 Runtime 名字**。
func TestCapabilityMatrix_Supports(t *testing.T) {
	m := adapter.NewCapabilityMatrix([]adapter.Probe{
		{ID: "streaming_thoughts", Passed: true},
		{ID: "session_modes", Passed: false},
	})

	if !m.Supports("streaming_thoughts") {
		t.Error("通过的能力却说不支持")
	}
	if m.Supports("session_modes") {
		t.Error("没通过的能力却说支持——上层会走进一条它走不通的路")
	}
	// 没探过的能力：保守当成不支持，而不是乐观假设
	if m.Supports("never_probed") {
		t.Error("没探过的能力却说支持——保守判断才安全")
	}
}
