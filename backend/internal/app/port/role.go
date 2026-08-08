package port

import "github.com/HuLuca1998/acp-flows/backend/internal/domain/model"

// RoleBindings 回答「这个角色现在绑哪个 Runtime、在那一端该设什么档位」。
//
// ★★ **为什么是接口而不是直接调用**：档名（`plan` / `read-only` /
// `default`）是**某一端的品牌知识**，只有 adapter 该认识它。
// app 与 api 一旦认识了，加第三个 Runtime 就要在几十个地方改 if，
// 而每漏一处都是一个只在那个 Runtime 下出现的 bug。
//
// 实现在 `acp/runtime`，装配在 `cmd/duetd`。
type RoleBindings interface {
	// RuntimeFor 返回角色当前绑定的 Runtime 名。
	//
	// ★ 没登记的角色**报错**，不给「反正 claude 能干」的默认值——
	// 悄悄派给某一端的话，用户不会知道。
	RuntimeFor(roleID string) (string, error)

	// ModeNameOn 把语义档翻译成那一端的实际档名。
	ModeNameOn(runtimeName string, mode model.SessionMode) (string, error)
}
