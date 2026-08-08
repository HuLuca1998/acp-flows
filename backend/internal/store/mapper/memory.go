package mapper

import (
	"strings"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
	"github.com/HuLuca1998/acp-flows/backend/internal/store/entity"
)

// refSeparator 是 source_refs 在一列里的连接符。
//
// ★ 用逗号而不是空格：ref 本身（`ev-412` / `unit-009`）不含逗号，
// 而空格分隔在「某个 ref 将来带了空格」时会静默拆错。
const refSeparator = ","

// MemoryToEntity 把领域模型转成行结构。
//
// ★★ **不带正文**（INV-MEM-8）：正文只在 md 文件里。
// 时间戳由 repo 用注入的 Clock 填。
func MemoryToEntity(m *model.Memory) *entity.Memory {
	return &entity.Memory{
		ID:          m.ID(),
		Kind:        string(m.Kind()),
		Scope:       string(m.Scope()),
		Status:      string(m.Status()),
		SourceRefs:  strings.Join(m.SourceRefs(), refSeparator),
		CreatedBy:   m.CreatedBy(),
		ConfirmedBy: m.ConfirmedBy(),
		Reason:      m.Reason(),
		Supersedes:  m.Supersedes(),
		HistoryLen:  m.HistoryLen(),
	}
}

// MemoryToModel 把行结构转回领域模型。
//
// ★ 走 model.RestoreMemory 而不是 ProposeCandidate：后者会强制
// 「新建的一律是 candidate」，而这里读的是已经被人确认过的东西。
func MemoryToModel(e *entity.Memory) *model.Memory {
	return model.RestoreMemory(
		e.ID,
		model.MemoryKind(e.Kind),
		model.MemoryScope(e.Scope),
		model.MemoryStatus(e.Status),
		splitRefs(e.SourceRefs),
		e.CreatedBy, e.ConfirmedBy, e.Reason, e.Supersedes, e.HistoryLen,
	)
}

// splitRefs 把存的字符串拆回切片。
//
// ★ 空串要拆成**空切片而不是 [""]**：后者会让「有没有依据」这个判断
// 变成假的——一条没有依据的记忆看起来像是有一条空依据。
//
// 空串由下面那个 `p != ""` 的过滤兜住，**这里不再单独早返回**：
// 造负例时发现那个早返回拆掉后测试照样绿（`strings.Split("", ",")`
// 给出 `[""]`，正好被过滤掉）。测不到的分支留着只会让人以为它在防什么。
func splitRefs(s string) []string {
	parts := strings.Split(s, refSeparator)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
