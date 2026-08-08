package model

import (
	"errors"
	"fmt"
)

var (
	// ErrUnknownRole 表示角色标识不在角色库里。
	//
	// ★ **绝不静默回落到某个默认角色**：回落的后果是「本该由审查员做的事
	// 被实现工程师做了」——而实现方审查自己的产出，正是 INV-ATT-8 明令禁止的。
	// 这种错误不会有任何症状，只会让审查形同虚设。
	ErrUnknownRole = errors.New("model: 认不出的角色")

	// ErrUnknownRuntime 表示这个 Runtime 上没有登记档位映射。
	//
	// ★ 同样不静默回落。查不到映射却继续开工，等于**在不知道自己有多大权限
	// 的情况下让 AI 动用户的代码**。实测数据摆着：codex 默认档是
	// workspace-write 沙箱，沙箱内的写操作根本不触发审批
	// （docs/notes/acp-field-notes.md §3）。
	ErrUnknownRuntime = errors.New("model: 没有登记档位映射的 Runtime")

	// ErrInvalidPermissionPolicy 表示权限裁决取值不在全集内。
	ErrInvalidPermissionPolicy = errors.New("model: 非法的权限裁决")
)

// Operation 是一个 AI 操作。
//
// ★ 取值会原样出现在事件载荷与界面词条 key 里，改名等于破坏已有数据。
type Operation string

// 11 个 AI 操作，见 docs/spec/domain-model.md §16.3。
const (
	OpClarify        Operation = "clarify"         // 追问需求细节
	OpSnapshot       Operation = "snapshot"        // 冻结需求快照
	OpPlan           Operation = "plan"            // 产出计划
	OpSubplanDAG     Operation = "subplan_dag"     // 拆子计划与依赖
	OpUnitContract   Operation = "unit_contract"   // 写单元契约
	OpImplement      Operation = "implement"       // 实现
	OpTest           Operation = "test"            // 跑测试
	OpReport         Operation = "report"          // 出执行报告
	OpReviewUnit     Operation = "review_unit"     // 审查单元
	OpAdviseDecision Operation = "advise_decision" // 给决策选项
	OpCurateMemory   Operation = "curate_memory"   // 提记忆候选
)

var allOperations = [...]Operation{
	OpClarify, OpSnapshot, OpPlan, OpSubplanDAG, OpUnitContract,
	OpImplement, OpTest, OpReport, OpReviewUnit, OpAdviseDecision, OpCurateMemory,
}

// AllOperations 返回 AI 操作全集。
//
// INV-ROLE-6 要求每个操作至少被一个角色覆盖，穷举测试照这个全集查——
// 加了新操作却忘了派人，测试会红。
func AllOperations() []Operation {
	out := make([]Operation, len(allOperations))
	copy(out, allOperations[:])
	return out
}

// SessionMode 是**语义**上的会话档位，不是某一端的档名。
//
// ★★ 为什么要有这一层，而不直接存 `plan` / `read-only` 这样的档名：
//
// 设计稿的角色表存的是具体档名（需求分析师 `plan`、测试执行者 `read-only`），
// 而这些档名**两端根本对不上**——claude 有 6 档、codex 有 3 档，一个名字都不重合。
// 直接存档名的话，用户把「需求分析师」从 claude 换成 codex 时，`plan` 在 codex
// 上不存在，INV-ROLE-4 会拒绝保存，于是**他换不了 Runtime**——
// 而 INV-ROLE-1 明说改绑 Runtime 不该改变任何行为。
//
// 存语义、落地时翻译，两条不变量才能同时成立。
type SessionMode string

const (
	// SessionModeReadOnly 只读：读得到代码与库，写不了任何文件。
	SessionModeReadOnly SessionMode = "read_only"
	// SessionModeGuardedWrite 受控写：能写，但动文件要走权限请求。
	SessionModeGuardedWrite SessionMode = "guarded_write"
	// SessionModeUnrestricted 放开：不问。**预置角色一个都不用它**，
	// 留着是因为 Runtime 那侧确实有这一档，不登记的话映射表是残缺的。
	SessionModeUnrestricted SessionMode = "unrestricted"
)

// ★★ **档名的映射表不在这一层。**
//
// 「只读」在两端叫什么（claude 的 `plan` / codex 的 `read-only`）是
// **适配器的知识**，domain 认识的只有语义。上层一旦写 `if name == "codex"`，
// 加第三个 Runtime 就要在几十个地方改 if，每漏一处都是只在那一端出现的 bug。
//
// 翻译在 `backend/internal/acp/runtime/mode.go`。

// PermissionPolicy 是角色的权限裁决策略：怎么应答 `session/request_permission`。
//
// ★ 只有两个取值。设计稿的下拉里没有「一律拒绝」——
// 别照着旧代码里 `session.Policy` 那三种策略来，那是放错层的东西。
type PermissionPolicy string

const (
	// PermissionAskEach 逐条询问：每个权限请求都交给用户。
	PermissionAskEach PermissionPolicy = "ask_each"
	// PermissionAutoAllowRead 自动允许读：读操作自动放行，写仍然问。
	PermissionAutoAllowRead PermissionPolicy = "auto_allow_read"
)

var allPermissionPolicies = [...]PermissionPolicy{
	PermissionAskEach,
	PermissionAutoAllowRead,
}

// AllPermissionPolicies 返回权限裁决全集。界面上的下拉照它渲染。
func AllPermissionPolicies() []PermissionPolicy {
	out := make([]PermissionPolicy, len(allPermissionPolicies))
	copy(out, allPermissionPolicies[:])
	return out
}

// IsValid 报告取值是否在全集内。
func (p PermissionPolicy) IsValid() bool {
	for _, v := range allPermissionPolicies {
		if v == p {
			return true
		}
	}
	return false
}

// Role 是一个「员工」：谁、干什么、用哪个 Runtime、有多大权限。
//
// ★ **不可变**：字段私有，只有值接收者的读方法。要改就用 `With*` 造一个新的，
// 别在原来那个上动手——预置角色是全局共享的，谁改一下所有工作都跟着变，
// 而这种串味没有任何症状。
//
// ★★ **不含 runtime 字段。** 设计稿的原则是「角色先定义、再绑定 Runtime——
// 同一个角色换成另一端不影响状态机」。绑定是**关系**，存在 adapter 那一侧
// （`acp/runtime` 的推荐绑定表 + 用户的覆盖），不是角色自身的属性。
// 放进来的话，domain 就得认识 claude / codex 这些品牌名了。
//
// ★ **不含 model / reasoning_effort**（INV-ROLE-2）：ACP 不提供这两个设置，
// 它们由各 Runtime 自身配置决定。对话角色卡上那句 `claude · sonnet · 高`
// 是**观测到的运行时事实**（记在 Attempt 上），不是可配置项。
// 有反射测试守着这条。
type Role struct {
	id          string
	displayName string
	operations  []Operation
	duty        string
	personality string
	boundary    string
	output      string
	prompt      string
	sessionMode SessionMode
	permission  PermissionPolicy
	isPreset    bool
}

// ID 返回角色标识。
func (r Role) ID() string { return r.id }

// DisplayName 返回界面上显示的角色名。
func (r Role) DisplayName() string { return r.displayName }

// Operations 返回这个角色承担的 AI 操作。
//
// ★★ **副本边界就在这一个方法上。** Role 的字段全私有，`operations` 这个
// slice 唯一的出口就是这里——所以副本只需要在这里做一次。
//
// 别在 `PresetRoles()` / `WithRuntime()` 里再拷一次「以防万一」：
// 那些拷贝**没有任何测试能证明它有用**（外部根本拿不到内部 slice），
// 而测不到的防御留着，只会让下一个人以为已经守住了。
//
// 不返回副本的后果：调用方一个 `sort.Slice` 就把预置表的顺序永久改了，
// 而界面是照预置表顺序渲染的——用户会看到角色表每次刷新顺序都不一样。
func (r Role) Operations() []Operation {
	out := make([]Operation, len(r.operations))
	copy(out, r.operations)
	return out
}

// Duty 返回职责。
func (r Role) Duty() string { return r.duty }

// Personality 返回性格与提示语气。
func (r Role) Personality() string { return r.personality }

// Boundary 返回边界：这个角色明令不做的事。
func (r Role) Boundary() string { return r.boundary }

// Output 返回产出物的描述。
func (r Role) Output() string { return r.output }

// Prompt 返回角色提示词。
func (r Role) Prompt() string { return r.prompt }

// SessionMode 返回语义档位。
func (r Role) SessionMode() SessionMode { return r.sessionMode }

// PermissionPolicy 返回权限裁决策略。
func (r Role) PermissionPolicy() PermissionPolicy { return r.permission }

// IsPreset 报告这是不是预置角色。
func (r Role) IsPreset() bool { return r.isPreset }

// WithPermissionPolicy 换权限裁决，返回新的 Role。
func (r Role) WithPermissionPolicy(p PermissionPolicy) (Role, error) {
	if !p.IsValid() {
		return Role{}, fmt.Errorf("%w: %q", ErrInvalidPermissionPolicy, p)
	}
	next := r
	next.permission = p
	return next, nil
}
