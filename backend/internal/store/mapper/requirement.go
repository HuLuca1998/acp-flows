package mapper

import (
	"strings"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
	"github.com/HuLuca1998/acp-flows/backend/internal/store/entity"
)

// lineSep 是 items / open_facts 在一列里的连接符。
//
// ★ 用换行而不是逗号：需求条目里出现逗号是很正常的事
// （「取消必须幂等，且现场要保留」），用逗号会把一条拆成两条。
const lineSep = "\n"

// RequirementToEntity 把领域模型转成行结构。
func RequirementToEntity(r *model.RequirementSnapshot) *entity.Requirement {
	return &entity.Requirement{
		WorkID:    r.WorkID(),
		Version:   r.Version(),
		Items:     strings.Join(r.Items(), lineSep),
		OpenFacts: strings.Join(r.OpenFacts(), lineSep),
		Frozen:    r.Frozen(),
	}
}

// RequirementToModel 把行结构转回领域模型。
//
// ★ 走 model.RestoreRequirement 而不是 NewRequirement：
// 后者会校验「至少一条条目」，而这里读的是已经存过的东西——
// 校验规则变严时不该让老数据读不出来。
func RequirementToModel(e *entity.Requirement) *model.RequirementSnapshot {
	return model.RestoreRequirement(
		e.WorkID, e.Version, splitLines(e.Items), splitLines(e.OpenFacts), e.Frozen,
	)
}

// splitLines 把存的字符串拆回切片，丢掉空行。
//
// ★ 空串拆出来是**空切片**而不是 `[""]`：后者会让
// 「需求 6 · 已映射 6」这种统计变成假的——那个 6 里有一条没内容。
func splitLines(s string) []string {
	parts := strings.Split(s, lineSep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
