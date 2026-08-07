// Package adapter 把不同 ACP Runtime 的差异内化掉。
//
// ★ **上层只表达意图，不认识「这是 Claude 还是 Codex」。**
// 上层一旦写 `if name == "codex"`，加第三个 Runtime 就要在几十个地方改 if，
// 而每漏一处都是一个只在那个 Runtime 下出现的 bug。
// `scripts/check/lib/no_brand_in_upper.py` 把这条接进了 CI。
//
// 差异只有两条出路：
//   - **在这里填平**（比如按 category 取配置项，而不是按 id）
//   - **升级成能力查询**（探针跑一遍，上层问「支不支持 X」）
//
// 两条路都不需要上层知道品牌。
package adapter

import "github.com/HuLuca1998/acp-flows/backend/internal/acp/protocol"

// 语义层的配置项分类。
//
// ★ **按 category 取，不按 id 取。** 推理强度在 claude 是 `effort`、
// 在 codex 是 `reasoning_effort`，但两端 category 都是 `thought_level`。
//
// 按 id 取就要维护一张 claude→effort / codex→reasoning_effort 的映射表，
// 每加一个 Runtime 加一行——那正是「差异没内化」的样子。
// 协议本身给了语义层的稳定键（acp-field-notes.md §7.1），用它就好。
const (
	// CategoryThoughtLevel 是推理强度。
	CategoryThoughtLevel = "thought_level"
	// CategoryModel 是模型选择。
	CategoryModel = "model"
)

// ConfigByCategory 按语义分类取配置项。
//
// ★ 用 `CategoryOrEmpty` 而不是直接解引用：**claude 的 agent 选项 category
// 是空字符串，codex 的某些选项根本没有这个字段**——直接解引用会在其中一端
// panic，而崩的是用户打开设置页的时候。
func ConfigByCategory(options []protocol.ConfigOption, category string) (protocol.ConfigOption, bool) {
	for _, opt := range options {
		if opt.CategoryOrEmpty() == category {
			return opt, true
		}
	}
	return protocol.ConfigOption{}, false
}

// Probe 是一条能力探测的结果。
type Probe struct {
	ID     string `json:"id"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail,omitempty"`
}

// CapabilityMatrix 是一个 Runtime 的能力全貌。
//
// ★ **上层靠它做降级判断，绝不靠 Runtime 名字。**
// 写死矩阵的话，一个 Runtime 悄悄不支持某能力时它还显示通过——
// 上层据此走进一条它其实走不通的路，而错误会在很远的地方才浮现。
type CapabilityMatrix struct {
	Passed int     `json:"passed"`
	Total  int     `json:"total"`
	Probes []Probe `json:"probes"`
}

// NewCapabilityMatrix 从探测结果算出矩阵。
func NewCapabilityMatrix(probes []Probe) CapabilityMatrix {
	m := CapabilityMatrix{Total: len(probes), Probes: probes}
	for _, p := range probes {
		if p.Passed {
			m.Passed++
		}
	}
	return m
}

// Supports 报告某项能力可不可用。
//
// ★ **没探过的一律当成不支持。** 乐观假设的代价是上层走进一条走不通的路；
// 保守判断的代价只是少用一个可能可用的特性——两者不对等。
func (m CapabilityMatrix) Supports(probeID string) bool {
	for _, p := range m.Probes {
		if p.ID == probeID {
			return p.Passed
		}
	}
	return false
}
