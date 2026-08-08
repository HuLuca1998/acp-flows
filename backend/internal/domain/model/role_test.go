package model_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// 契约来源：docs/spec/domain-model.md §16（INV-ROLE-1..6）
//           design/INVENTORY.md §八（八行角色表，2026-08-08 更正过一次）
//
// ★ 这一族测试守的是「谁能干什么、有多大权限」。它错了不会崩，
// 只会让某个角色悄悄拿到它不该有的权限——**没有任何症状的那种错**。

// R1 · 八个预置角色，名字与操作逐条对上设计稿。
//
// ★ 数量单独断言：第一版 INVENTORY 只抽到 5 个（漏了测试执行者、决策顾问、
// 记忆管理员），因为我是从渲染出的可见区域抽的，滚动区外那三行没进视野。
// 这条断言让「又漏了一个」当场红。
func TestPresetRoles_R1_EightRolesMatchDesign(t *testing.T) {
	want := []struct {
		id   string
		name string
		ops  []model.Operation
	}{
		{"requirement_analyst", "需求分析师", []model.Operation{model.OpClarify, model.OpSnapshot}},
		{"plan_architect", "计划架构师", []model.Operation{model.OpPlan, model.OpSubplanDAG}},
		{"unit_designer", "单元设计师", []model.Operation{model.OpUnitContract}},
		{"implementer", "实现工程师", []model.Operation{model.OpImplement}},
		{"test_runner", "测试执行者", []model.Operation{model.OpTest, model.OpReport}},
		{"unit_reviewer", "实现审查员", []model.Operation{model.OpReviewUnit}},
		{"decision_advisor", "决策顾问", []model.Operation{model.OpAdviseDecision}},
		{"memory_curator", "记忆管理员", []model.Operation{model.OpCurateMemory}},
	}

	got := model.PresetRoles()
	if len(got) != len(want) {
		t.Fatalf("预置角色 %d 个，设计稿是 %d 个——对照 design/INVENTORY.md §八", len(got), len(want))
	}

	for i, w := range want {
		g := got[i]
		if g.ID() != w.id {
			t.Errorf("第 %d 个角色 id = %q，想要 %q（顺序就是设计稿的行序）", i, g.ID(), w.id)
		}
		if g.DisplayName() != w.name {
			t.Errorf("%s 的显示名 = %q，想要 %q", w.id, g.DisplayName(), w.name)
		}
		if !reflect.DeepEqual(g.Operations(), w.ops) {
			t.Errorf("%s 承担的操作 = %v，想要 %v", w.id, g.Operations(), w.ops)
		}
		if !g.IsPreset() {
			t.Errorf("%s 应该标成预置角色", w.id)
		}
	}
}

// R1b · INV-ROLE-6：11 个 AI 操作每个都有人干。
//
// ★★ 这条最值钱。漏派一个操作的后果是**跑到那一步才发现没人认领**——
// 而那时用户已经等了半天，计划也已经排好了。
func TestPresetRoles_R1b_EveryOperationHasAnOwner(t *testing.T) {
	covered := map[model.Operation]string{}
	for _, r := range model.PresetRoles() {
		for _, op := range r.Operations() {
			if prev, dup := covered[op]; dup {
				t.Errorf("操作 %q 被 %s 和 %s 同时认领——职责重叠说明角色划分有问题", op, prev, r.ID())
			}
			covered[op] = r.ID()
		}
	}

	all := model.AllOperations()
	if len(all) != 11 {
		t.Fatalf("AI 操作全集 %d 个，规格 §16.3 是 11 个", len(all))
	}
	for _, op := range all {
		if _, ok := covered[op]; !ok {
			t.Errorf("操作 %q 没有任何角色认领（INV-ROLE-6）——加操作时要同时派人", op)
		}
	}
}

// R4 · 认不出的角色名一律拒绝，不静默回落。
//
// ★ 回落的后果是「本该由审查员做的事被实现工程师做了」，
// 而实现方审查自己的产出正是 INV-ATT-8 明令禁止的。
func TestRoleByID_R4_UnknownRoleErrsNoFallback(t *testing.T) {
	got, err := model.RoleByID("architect") // 差一点点的名字最危险
	if !errors.Is(err, model.ErrUnknownRole) {
		t.Errorf("未知角色的错误 = %v，想要 ErrUnknownRole", err)
	}
	if got.ID() != "" {
		t.Errorf("认不出却返回了角色 %q——静默回落会让审查形同虚设", got.ID())
	}
}

func TestRoleByID_R4b_EveryPresetIsRetrievable(t *testing.T) {
	for _, want := range model.PresetRoles() {
		got, err := model.RoleByID(want.ID())
		if err != nil {
			t.Errorf("按 id 取 %q: %v", want.ID(), err)
			continue
		}
		if got.DisplayName() != want.DisplayName() {
			t.Errorf("%q 取出来是 %q", want.ID(), got.DisplayName())
		}
	}
}

// INV-ROLE-2 · Role 不含 model / reasoning_effort。
//
// ★ 用反射守，因为这条错误加起来毫不费力——
// 有人觉得「加个 model 字段方便」，加完测试还是绿的，直到发现
// 设置里改了模型却没有任何效果：ACP 根本不提供这个设置项，
// 模型由各 Runtime 自身的配置决定（domain-model.md §16.5）。
func TestRole_INVROLE2_HasNoModelOrEffortField(t *testing.T) {
	forbidden := []string{"model", "reasoningeffort", "effort", "thoughtlevel"}

	rt := reflect.TypeOf(model.Role{})
	for i := range rt.NumField() {
		name := strings.ToLower(rt.Field(i).Name)
		for _, bad := range forbidden {
			if name == bad {
				t.Errorf("Role 有字段 %q——ACP 不提供这个设置，它是观测结果不是配置项"+
					"（INV-ROLE-2，见 domain-model.md §16.5）", rt.Field(i).Name)
			}
		}
	}
}

// R5 · 与用户对话的三个角色必须是只读（Q42）。
//
// ★★ 判据在这里而不是在权限裁决上：实测证明**客户端全拒是拦不住的**——
// codex 默认档下 request_permission 一次都不会来，文件照建
// （acp-field-notes.md §3）。只读必须落在会话档位上。
func TestPresetRoles_R5_UserFacingRolesAreReadOnly(t *testing.T) {
	// 设计稿角色表 + Q42：这五个都不该有写的能力
	readOnly := []string{
		"requirement_analyst", "plan_architect", "unit_designer",
		"unit_reviewer", "memory_curator",
	}

	for _, id := range readOnly {
		r, err := model.RoleByID(id)
		if err != nil {
			t.Fatalf("取 %q: %v", id, err)
		}
		if r.SessionMode() != model.SessionModeReadOnly {
			t.Errorf("%s（%s）的档位是 %q，应该是只读——它的边界写着「%s」",
				id, r.DisplayName(), r.SessionMode(), r.Boundary())
		}
	}
}

// R5b · 只有实现工程师能写。
//
// ★ 反过来查一遍：漏掉某个角色的收权时，上一条测不出来
// （它只查名单里的五个），这条会红。
func TestPresetRoles_R5b_OnlyImplementerMayWrite(t *testing.T) {
	for _, r := range model.PresetRoles() {
		if r.SessionMode() == model.SessionModeReadOnly {
			continue
		}
		if r.ID() != "implementer" {
			t.Errorf("%s（%s）的档位是 %q——除了实现工程师，谁都不该能写",
				r.ID(), r.DisplayName(), r.SessionMode())
		}
	}
	// 放开档一个都不该用上
	for _, r := range model.PresetRoles() {
		if r.SessionMode() == model.SessionModeUnrestricted {
			t.Errorf("%s 用了「放开」档——预置角色一个都不该用它", r.ID())
		}
	}
}

// R6 · 权限裁决是封闭枚举，只有设计稿那两个取值。
func TestPermissionPolicy_R6_ClosedEnumOfTwo(t *testing.T) {
	all := model.AllPermissionPolicies()
	if len(all) != 2 {
		t.Fatalf("权限裁决 %d 个取值，设计稿只有「逐条询问」「自动允许读」两个", len(all))
	}
	for _, p := range all {
		if !p.IsValid() {
			t.Errorf("%q 在全集里却判定为非法", p)
		}
	}
	// ★ 别把旧代码里 session.Policy 那三种策略搬过来
	if model.PermissionPolicy("reject_all").IsValid() {
		t.Error("「一律拒绝」被判成合法——设计稿的下拉里没有这一项")
	}
	if model.PermissionPolicy("").IsValid() {
		t.Error("空串被判成合法")
	}
}

func TestRole_R6b_WithPermissionPolicyRejectsInvalid(t *testing.T) {
	r, err := model.RoleByID("implementer")
	if err != nil {
		t.Fatalf("取实现工程师: %v", err)
	}

	if _, err := r.WithPermissionPolicy("reject_all"); !errors.Is(err, model.ErrInvalidPermissionPolicy) {
		t.Errorf("非法权限裁决的错误 = %v，想要 ErrInvalidPermissionPolicy", err)
	}

	next, err := r.WithPermissionPolicy(model.PermissionAutoAllowRead)
	if err != nil {
		t.Fatalf("换成自动允许读: %v", err)
	}
	if next.PermissionPolicy() != model.PermissionAutoAllowRead {
		t.Errorf("换完 = %q", next.PermissionPolicy())
	}
	if r.PermissionPolicy() != model.PermissionAskEach {
		t.Errorf("原角色被改成了 %q——必须返回新实例", r.PermissionPolicy())
	}
}

// R7 · 返回的是副本，改它不影响下一次调用。
//
// ★ 这条是造负例发现的：`Operations()` 直接返回内部 slice 时，
// 调用方一个 `sort.Slice` 就把预置表的顺序永久改了，
// 而界面是照预置表顺序渲染的——用户会看到角色表每次刷新顺序都不一样。
func TestPresetRoles_R7_ReturnsCopies(t *testing.T) {
	first := model.PresetRoles()
	first[0] = model.Role{}
	ops := first[1].Operations()
	if len(ops) > 0 {
		ops[0] = "篡改"
	}

	second := model.PresetRoles()
	if second[0].ID() != "requirement_analyst" {
		t.Errorf("改了返回值之后再取，第一个角色变成了 %q", second[0].ID())
	}
	if got := second[1].Operations(); len(got) > 0 && got[0] != model.OpPlan {
		t.Errorf("改了 Operations 的返回值之后再取，计划架构师的操作变成了 %v", got)
	}
}

func TestAllOperations_R7b_ReturnsCopy(t *testing.T) {
	first := model.AllOperations()
	if len(first) > 0 {
		first[0] = "篡改"
	}
	if second := model.AllOperations(); second[0] != model.OpClarify {
		t.Errorf("改了返回值之后再取 = %q，想要 %q", second[0], model.OpClarify)
	}
}

// 每个角色的四要素都填了——空着的话对话页的角色卡就是一片空白，
// 而用户根本不知道这个角色的边界在哪。
func TestPresetRoles_FourElementsAllFilled(t *testing.T) {
	for _, r := range model.PresetRoles() {
		if r.Duty() == "" {
			t.Errorf("%s 没写职责", r.ID())
		}
		if r.Personality() == "" {
			t.Errorf("%s 没写性格", r.ID())
		}
		if r.Boundary() == "" {
			t.Errorf("%s 没写边界——边界是可测的约束，不是文案", r.ID())
		}
		if r.Output() == "" {
			t.Errorf("%s 没写产出", r.ID())
		}
	}
}
