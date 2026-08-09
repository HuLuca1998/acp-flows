package reply

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// ErrNoContractInReply 表示 AI 的回复里没有可解析的契约。
var ErrNoContractInReply = errors.New("work: AI 的回复里没有可解析的契约")

// contractFence 是我们要求单元设计师用的围栏标记。
//
// ★ 与 `duet-plan` 分开：一轮回复里可能同时讲到计划与契约，
// 共用一个标记的话，抠出来的那段可能是另一样东西。
const contractFence = "duet-contract"

// contractPayload 是单元设计师要输出的形状。
type contractPayload struct {
	Criteria []struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	} `json:"criteria"`
	Boundary struct {
		Allowed   []string `json:"allowed"`
		Forbidden []string `json:"forbidden"`
	} `json:"boundary"`
}

// ContractInstructions 告诉单元设计师怎么输出。
//
// ★★ 例子比规则管用：只写「输出 JSON」的话它会给一段字段名自创的 JSON，
// 而那解析不出来，用户得到的是一次白等。
func ContractInstructions() string {
	return "\n\n想清楚之后，**在回复的最后**输出一段用 ```" + contractFence +
		" 围起来的 JSON，形如：\n\n```" + contractFence + `
{
  "criteria": [
    {"id": "ac-1", "text": "连点两次取消只发一次协议取消请求"},
    {"id": "ac-2", "text": "取消后 diff 与最后事件游标可读"}
  ],
  "boundary": {
    "allowed": ["internal/acp/", "internal/app/work/"],
    "forbidden": ["internal/api/gen/"]
  }
}
` + "```\n\n" +
		"`boundary.allowed` 是**相对仓库根的路径前缀**，以 `/` 结尾表示目录。" +
		"★ 它是用户唯一的防线，所以**只列这个单元真的要改的地方**——" +
		"写一个 `/` 等于没有边界。`forbidden` 压过 `allowed`。"
}

// ParseContractReply 从单元设计师的回复里提取契约。
//
// ★★ 解析不出来时把原话带上：静默失败的话，用户看到「正在设计契约」
// 然后永远没有下文。
func ParseContractReply(reply, unitID string, version int) (*model.UnitContract, error) {
	raw, ok := extractFenced(reply, contractFence)
	if !ok {
		return nil, fmt.Errorf("%w（它说：%s）", ErrNoContractInReply, firstChars(reply, 200))
	}

	var payload contractPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, fmt.Errorf("%w: JSON 解析失败: %v（它给的是：%s）",
			ErrNoContractInReply, err, firstChars(raw, 200))
	}
	if len(payload.Criteria) == 0 {
		// ★★ 空契约冻结之后，「做完了」这件事没有任何判据——
		// AI 说做完了就是做完了。
		return nil, fmt.Errorf("%w: 一条验收标准都没有", ErrNoContractInReply)
	}

	c := model.NewUnitContract(unitID, version)
	for _, crit := range payload.Criteria {
		if err := c.AddCriterion(crit.ID, crit.Text); err != nil {
			return nil, fmt.Errorf("加验收标准 %s: %w", crit.ID, err)
		}
	}
	if err := c.SetBoundary(model.WriteBoundary{
		Allowed:   payload.Boundary.Allowed,
		Forbidden: payload.Boundary.Forbidden,
	}); err != nil {
		return nil, fmt.Errorf("设置边界: %w", err)
	}
	return c, nil
}
