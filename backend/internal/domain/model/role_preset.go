package model

import "fmt"

// 八个预置角色，见 docs/spec/domain-model.md §16.1 与 design/INVENTORY.md §八。
//
// ★ 顺序就是设计稿表格里的行序，界面照这个顺序渲染。
//
// ★ id 的来源：设计稿只证实了两个——`implementer`（domain-model.md §11 的
// `role_id` 实例）与 `memory_curator`（记忆页的 `created_by`）。
// 其余六个按「操作名 → 角色」的对应关系定，规格 §16.3 的 OPEN-16 记着这件事。
var presetRoles = [...]Role{
	{
		id:          "requirement_analyst",
		displayName: "需求分析师",
		operations:  []Operation{OpClarify, OpSnapshot},
		duty:        "追问清楚需求，冻结成快照",
		personality: "追问式，逐条确认，不放过「大概/应该」",
		boundary:    "不写代码、不定技术方案；无法判定等级时上调 D2",
		output:      "requirement 快照",
		sessionMode: SessionModeReadOnly,
		permission:  PermissionAskEach,
		isPreset:    true,
	},
	{
		id:          "plan_architect",
		displayName: "计划架构师",
		operations:  []Operation{OpPlan, OpSubplanDAG},
		duty:        "产出计划与子计划 DAG，给每个单元派角色",
		personality: "结构优先、克制，不写含糊单元",
		boundary:    "不实现、不改验收标准；远期单元可留白但执行前必冻结",
		output:      "plan 版本与 subplan DAG",
		sessionMode: SessionModeReadOnly,
		permission:  PermissionAskEach,
		isPreset:    true,
	},
	{
		id:          "unit_designer",
		displayName: "单元设计师",
		operations:  []Operation{OpUnitContract},
		duty:        "把单元的交接写成契约",
		personality: "把交接写成契约而不是 Prompt，边界写死",
		boundary:    "不实现；契约冻结后不改，要改就出新版本",
		output:      "unit_contract 版本",
		sessionMode: SessionModeReadOnly,
		permission:  PermissionAskEach,
		isPreset:    true,
	},
	{
		id:          "implementer",
		displayName: "实现工程师",
		operations:  []Operation{OpImplement},
		duty:        "按契约实现，动文件走权限请求",
		personality: "沉默执行、报告详尽，触发停止条件立即停",
		boundary:    "不改目标、外部行为、测试标准与写入边界；D1 以上只报告",
		output:      "工作分支上的改动",
		sessionMode: SessionModeGuardedWrite,
		permission:  PermissionAskEach,
		isPreset:    true,
	},
	{
		id:          "test_runner",
		displayName: "测试执行者",
		operations:  []Operation{OpTest, OpReport},
		duty:        "跑测试并如实记录输出",
		personality: "只跑不改，失败原文照录，不修饰结论",
		boundary:    "不改代码、不改测试；失败就是失败",
		output:      "测试输出与执行报告",
		sessionMode: SessionModeReadOnly,
		permission:  PermissionAutoAllowRead,
		isPreset:    true,
	},
	{
		id:          "unit_reviewer",
		displayName: "实现审查员",
		operations:  []Operation{OpReviewUnit},
		duty:        "独立会话审查 diff 与证据，判定四类结果之一",
		personality: "怀疑式，「全绿」不等于验证过；无证据即未通过",
		boundary:    "不延续规划会话、不修改代码、不伪造通过",
		output:      "accepted / implementation_fix / contract_revision / global_replan",
		// ★ 设计稿这一格写的是 claude 的 `default`（会问的那一档），
		// 我收紧成只读。理由：它的边界明写「不修改代码」，而 Q42 也裁定
		// 审查是只读会话。两处都说不该写，那就别给它写的能力——
		// 松的那一侧的风险是「审查员改了它正在审的代码」，
		// 紧的那一侧的风险只是「它想写但写不了」，而它本来就不该写。
		sessionMode: SessionModeReadOnly,
		permission:  PermissionAutoAllowRead,
		isPreset:    true,
	},
	{
		id:          "decision_advisor",
		displayName: "决策顾问",
		operations:  []Operation{OpAdviseDecision},
		duty:        "给出选项与各自的代价，标出推荐项",
		personality: "给选项与代价，不替用户拍板，拿不准就上调 D2",
		boundary:    "不替用户做决定；影响分析必须写清代价",
		output:      "决策选项与影响分析",
		sessionMode: SessionModeReadOnly,
		permission:  PermissionAutoAllowRead,
		isPreset:    true,
	},
	{
		id:          "memory_curator",
		displayName: "记忆管理员",
		operations:  []Operation{OpCurateMemory},
		duty:        "从执行过程里提炼记忆候选，送人审核",
		personality: "保守，只提候选；跨项目一律去项目标识后送审",
		boundary:    "只提候选，不直接写进记忆库",
		output:      "记忆候选",
		// ★ 同实现审查员：设计稿写 `default`，收紧成只读。
		// 它的边界是「只提候选」，连记忆库都不直接写，更不该写用户的文件。
		sessionMode: SessionModeReadOnly,
		permission:  PermissionAutoAllowRead,
		isPreset:    true,
	},
}

// PresetRoles 返回八个预置角色。
//
// ★ 返回的是新 slice：调用方改 `out[0]` 不该动到预置表。
// Role 本身是值类型且字段私有，所以这一层就够了——
// 内部那个 operations slice 的边界在 `Operations()` 上，见那里的说明。
func PresetRoles() []Role {
	out := make([]Role, len(presetRoles))
	copy(out, presetRoles[:])
	return out
}

// RoleByID 按标识取预置角色。
//
// ★ 认不出就报错，**不回落到任何默认角色**——见 ErrUnknownRole 的说明。
func RoleByID(id string) (Role, error) {
	for _, r := range presetRoles {
		if r.id == id {
			return r, nil
		}
	}
	return Role{}, fmt.Errorf("%w: %q", ErrUnknownRole, id)
}
