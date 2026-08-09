package work

import (
	"github.com/HuLuca1998/acp-flows/backend/internal/app/work/reply"
	"github.com/HuLuca1998/acp-flows/backend/internal/constant"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// roleForState 说这个状态下该由**哪个角色**跟用户说话。
//
// ★★ 这条映射是「常驻会话只读」的**唯一落点**（M5 完成标志第 6 条）。
//
// 不传的话 acp 层会退到 `DefaultRoleID`（实现工程师，受控写）——
// 那意味着澄清需求阶段的会话**能改用户的文件**，而用户以为自己只是在聊天。
// 「常驻会话是只读的」那一整套收权代码全在，只是从来没被这条路径用上。
//
// ★ 认不出的状态返回空串，交给 acp 层的默认值。**不猜**：
// 猜错的方向如果是「更松」，那就是把写权限发给了不该有的角色。
func roleForState(state constant.WorkState) string {
	switch state {
	case constant.WorkStateInitializing, constant.WorkStateClarifying:
		// 澄清需求：只读。它读得到代码与记忆，但一个字节都写不了。
		return "requirement_analyst"
	case constant.WorkStatePlanning:
		return "plan_architect"
	case constant.WorkStateExecuting:
		return "implementer"
	case constant.WorkStateReviewingUnit:
		return "unit_reviewer"
	default:
		return ""
	}
}

// systemPromptFor 取一个角色要拼在会话最前面的那段话。
//
// ★ 角色认不出来时返回空串，**不编一段**：编出来的提示词与角色页上
// 那张卡片对不上，用户会以为 AI 收到的是他看到的那四行。
func systemPromptFor(roleID string) string {
	role, err := model.RoleByID(roleID)
	if err != nil {
		return ""
	}
	// ★★ 每个角色的提示词末尾都带上「怎么提记忆」。
	//
	// 不带的话 AI 永远不会输出 `duet-memory` 围栏，而解析那一整套代码
	// 就是死的——单测全绿，真实路径上一条候选都不会出现。
	// 提取器与「让它提」必须成对上线。
	return role.SystemPrompt() + reply.MemoryInstructions()
}
