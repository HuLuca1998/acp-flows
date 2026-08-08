package agent

import (
	"fmt"

	"github.com/HuLuca1998/acp-flows/backend/internal/acp/runtime"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// DefaultRoleID 是没指定角色时用的那个。
//
// ★★ 为什么是**实现工程师**而不是「不收权」：
//
// 「不收权」听起来像个中性的默认值，实际上是最松的那一档——
// codex 的默认档 `agent` 是 workspace-write 沙箱，**沙箱内的写操作
// 连审批都不触发**（acp-field-notes.md §3 实测：客户端全拒 +
// 默认档 → 权限请求 0 次、文件照建）。
//
// 装配漏了一根线的表现必须是「权限最小」，不能是「什么都放行」。
//
// ★ 为什么不是只读：Q42 裁定与用户对话的会话只读，但那要等 `M5`——
// 现在的链路是「用户提需求 → AI 直接改文件」，还没有需求/计划/契约的分层。
// 现在就改只读会让整条流程失效，那是把 `M2` 的活扩成 `M5` 的。
// 实现工程师是**现状的忠实描述**，而且比现状严。
const DefaultRoleID = "implementer"

// modeIDFor 算出某个角色在某个 Runtime 上该设的档名。
//
// ★ 两步：角色 → 语义档（domain 的知识）→ 那一端的档名（adapter 的知识）。
// 合成一步的话，上层就得认识 `plan` / `read-only` 这些品牌相关的取值了。
func modeIDFor(roleID, runtimeName string) (string, error) {
	if roleID == "" {
		roleID = DefaultRoleID
	}
	role, err := model.RoleByID(roleID)
	if err != nil {
		// ★ 认不出就报错，**不回落到默认角色**。
		//
		// 回落的后果是「本该由审查员做的事被实现工程师做了」——
		// 而实现方审查自己的产出正是 INV-ATT-8 明令禁止的。
		// 这种错误没有任何症状：审查照常「通过」。
		return "", fmt.Errorf("agent: 认不出角色 %q: %w", roleID, err)
	}
	modeID, err := runtime.ModeNameOn(runtimeName, role.SessionMode())
	if err != nil {
		return "", fmt.Errorf(
			"agent: 角色 %s（%s）要的 %q 档在 %s 上翻译不出来: %w",
			role.ID(), role.DisplayName(), role.SessionMode(), runtimeName, err)
	}
	return modeID, nil
}
