package runtime

import (
	"errors"
	"fmt"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// ErrUnknownRuntime 表示这个 Runtime 上没有登记档位映射。
//
// ★ **绝不静默回落。** 查不到映射却继续开工，等于在不知道自己有多大权限的
// 情况下让 AI 动用户的代码——而实测告诉我们这不是理论风险：
// codex 的默认档是 workspace-write 沙箱，**沙箱内的写操作根本不触发审批**，
// 客户端那份「全拒」的代码一次都不会被调用到
// （docs/notes/acp-field-notes.md §3）。
var ErrUnknownRuntime = errors.New("runtime: 没有登记档位映射的 Runtime")

// modeNames 把语义档翻译成各 Runtime 的实际档名。
//
// ★★ **这张表是适配器的全部意义所在。** 同一个「只读」，
// claude 叫 `plan`、codex 叫 `read-only`——一个字都不一样。
// 上层只说「我要只读」，翻译在这里做完，于是加第三个 Runtime 时
// 只有这张表要改。
//
// 取值来自真机实测（`acp-field-notes.md` §3），**不是设计稿**：
// 设计稿给 codex 写的 `auto` 是 0.16.0 的旧档名，1.1.7 已经改成
// `read-only` / `agent` / `agent-full-access`。照设计稿抄的话，
// 发过去 codex 不认，收权失败——而失败的表现正是「沙箱照旧放行」。
//
// ★ 这里给的是**期望档名**。真值以 `session/new` 返回的 availableModes
// 为准（INV-ROLE-4），拿到响应后要当场校验。
var modeNames = map[string]map[model.SessionMode]string{
	"claude": {
		model.SessionModeReadOnly:     "plan",
		model.SessionModeGuardedWrite: "default",
		model.SessionModeUnrestricted: "bypassPermissions",
	},
	"codex": {
		model.SessionModeReadOnly:     "read-only",
		model.SessionModeGuardedWrite: "agent",
		model.SessionModeUnrestricted: "agent-full-access",
	},
}

// ModeNameOn 返回语义档在指定 Runtime 上的档名。
func ModeNameOn(runtimeName string, mode model.SessionMode) (string, error) {
	modes, ok := modeNames[runtimeName]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnknownRuntime, runtimeName)
	}
	name, ok := modes[mode]
	if !ok {
		return "", fmt.Errorf("%w: %q 上没有 %q 这一档", ErrUnknownRuntime, runtimeName, mode)
	}
	return name, nil
}

// recommendedRuntime 是每个预置角色的**推荐**绑定。
//
// ★ 「推荐」两个字是认真的：设计稿的角色表页脚写着「恢复推荐绑定」，
// 说明用户改得动。这张表只提供默认值，不是约束。
//
// ★ 为什么在 adapter 而不在 domain：设计稿的原则是
// 「角色先定义、再绑定 Runtime——同一个角色换成另一端不影响状态机」。
// 角色是业务概念，绑定到哪个牌子的 Runtime 不是。
//
// 取值来自设计稿角色表（`design/INVENTORY.md` §八）。
var recommendedRuntime = map[string]string{
	"requirement_analyst": "claude",
	"plan_architect":      "claude",
	"unit_designer":       "claude",
	"implementer":         "codex",
	"test_runner":         "codex",
	"unit_reviewer":       "claude",
	"decision_advisor":    "claude",
	"memory_curator":      "claude",
}

// ErrNoRecommendation 表示这个角色没有推荐绑定。
var ErrNoRecommendation = errors.New("runtime: 角色没有推荐的 Runtime 绑定")

// RecommendedRuntimeFor 返回角色的推荐 Runtime。
//
// ★ 查不到就报错，不给一个「反正 claude 能干」的默认值——
// 自定义角色没登记推荐绑定是正常的，让调用方明确处理，
// 而不是悄悄派给某一端。
func RecommendedRuntimeFor(roleID string) (string, error) {
	name, ok := recommendedRuntime[roleID]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrNoRecommendation, roleID)
	}
	return name, nil
}

// Bindings 实现 port.RoleBindings。
//
// ★ 这个类型存在的意义就是**把品牌知识关在这一层**：
// 上面两张表（档名映射 + 推荐绑定）都只有这个包认识。
type Bindings struct{}

// RuntimeFor 返回角色的推荐 Runtime。
func (Bindings) RuntimeFor(roleID string) (string, error) {
	return RecommendedRuntimeFor(roleID)
}

// ModeNameOn 把语义档翻译成那一端的档名。
func (Bindings) ModeNameOn(runtimeName string, mode model.SessionMode) (string, error) {
	return ModeNameOn(runtimeName, mode)
}
