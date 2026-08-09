package work

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// ErrNoPlanInReply 表示 AI 的回复里没有可解析的计划。
//
// ★★ 这条错误**必须带上 AI 的原话**：静默失败的话，用户看到「正在规划」
// 然后永远没有下文，而真正的原因（它输出了一段散文）躺在没人读的地方。
var ErrNoPlanInReply = errors.New("work: AI 的回复里没有可解析的计划")

// planFence 是我们要求 AI 用的围栏标记。
//
// ★ 用一个**专属标记**而不是通用的 ```json：一轮回复里可能有好几段
// JSON（它在讲解自己读到的配置文件），挑错一段的后果是把配置当成了计划。
const planFence = "duet-plan"

// planPayload 是 AI 要输出的形状。
//
// ★ 字段名与 openapi 的 Plan 一致，但**这是给 AI 看的契约**，
// 不是给前端的——两者会各自演化，共用一个类型只会让某一天的改动
// 同时打断两边。
type planPayload struct {
	Title    string `json:"title"`
	Subplans []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Units []struct {
			ID        string   `json:"id"`
			Title     string   `json:"title"`
			RoleID    string   `json:"role_id"`
			DependsOn []string `json:"depends_on"`
		} `json:"units"`
	} `json:"subplans"`
}

// planInstructions 是拼在规划 prompt 末尾、告诉 AI 怎么输出的那段话。
//
// ★★ 例子比规则管用：只写「输出 JSON」的话，它会给一段带注释的、
// 字段名自创的 JSON——而那解析不出来，用户得到的是一次白等。
func planInstructions() string {
	return "\n\n拆完之后，**在回复的最后**输出一段用 ```" + planFence +
		" 围起来的 JSON，形如：\n\n```" + planFence + `
{
  "title": "取消运行中的 Agent turn",
  "subplans": [
    {
      "id": "subplan-01",
      "title": "ACP Runtime 抽象层",
      "units": [
        {"id": "unit-012", "title": "取消协议", "role_id": "implementer", "depends_on": []},
        {"id": "unit-013", "title": "取消后证据读取", "role_id": "unit_reviewer",
         "depends_on": ["unit-012"]}
      ]
    }
  ]
}
` + "```\n\n" +
		"`role_id` 只能取这八个之一：`requirement_analyst` `plan_architect` " +
		"`unit_designer` `implementer` `test_runner` `unit_reviewer` " +
		"`decision_advisor` `memory_curator`。`depends_on` 里只能写这份计划里" +
		"出现过的单元 id，且**不能成环**。"
}

// ParsePlanReply 从 AI 的一轮回复里提取计划。
//
// ★ version 由调用方给：AI 不知道现在是第几版，让它猜的话会给出一个
// 与库里对不上的版本号——而版本号是版本链的骨架。
//
// ★★ 解析失败时错误里**带着原话的开头**：用户要看得出「它到底说了什么」。
func ParsePlanReply(reply string, version int) (model.PlanVersion, error) {
	raw, ok := extractFenced(reply, planFence)
	if !ok {
		return model.PlanVersion{}, fmt.Errorf("%w（它说：%s）",
			ErrNoPlanInReply, firstChars(reply, 200))
	}

	var payload planPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return model.PlanVersion{}, fmt.Errorf("%w: JSON 解析失败: %v（它给的是：%s）",
			ErrNoPlanInReply, err, firstChars(raw, 200))
	}
	if len(payload.Subplans) == 0 {
		return model.PlanVersion{}, fmt.Errorf("%w: 一个子计划都没有", ErrNoPlanInReply)
	}

	subplans := make([]model.Subplan, 0, len(payload.Subplans))
	for _, sp := range payload.Subplans {
		units := make([]model.Unit, 0, len(sp.Units))
		for _, u := range sp.Units {
			// ★★ 角色**在这里就校验**（裁定三）：放行一个不存在的角色的话，
			// 那个单元到执行时才发现没人认领，而用户已经等了几分钟。
			built, err := model.NewUnit(u.ID, u.Title, u.RoleID, u.DependsOn)
			if err != nil {
				return model.PlanVersion{}, fmt.Errorf("计划里的单元有问题: %w", err)
			}
			units = append(units, built)
		}
		built, err := model.NewSubplan(sp.ID, sp.Title, units)
		if err != nil {
			return model.PlanVersion{}, fmt.Errorf("计划里的子计划有问题: %w", err)
		}
		subplans = append(subplans, built)
	}

	// WithSubplans 里做 DAG 校验（依赖存在、不成环、单元不重名）
	return model.NewPlanVersion(version, payload.Title, nil).WithSubplans(subplans)
}

// extractFenced 取出 ```<fence> ... ``` 里的内容。
//
// ★ 取**最后一段**：我们要求 AI 把计划放在回复末尾，而它前面可能
// 为了讲解先贴一段示例——取第一段的话，示例会被当成真计划。
func extractFenced(s, fence string) (string, bool) {
	open := "```" + fence
	start := strings.LastIndex(s, open)
	if start < 0 {
		return "", false
	}
	rest := s[start+len(open):]
	end := strings.Index(rest, "```")
	if end < 0 {
		// 围栏没闭合——多半是流式输出被截断了。当成没有，
		// 而不是把半截 JSON 送去解析（那会给出一个看不懂的语法错误）。
		return "", false
	}
	return strings.TrimSpace(rest[:end]), true
}

// firstChars 取前 n 个**字符**（不是字节）——按字节截会把中文切成乱码。
func firstChars(s string, n int) string {
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}
