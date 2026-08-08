package model_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// 契约来源：docs/spec/domain-model.md §14（INV-MEM-1..12）
//           design/INVENTORY.md §九（记忆页：类型 · 三档筛选）
//
// ★★ 这一族里最值钱的是 INV-MEM-2「绝不自动写入」。它错了的后果不是
// 「多了一条记忆」，而是 **AI 把自己的一次臆断变成了以后每一轮的前提**——
// 而用户从没看过那句话。

func candidate(t *testing.T) *model.Memory {
	t.Helper()
	m, err := model.ProposeCandidate(
		"mem-203", model.MemoryConstraint, "acp-engine",
		[]string{"ev-412", "unit-009"}, "memory_curator",
	)
	if err != nil {
		t.Fatalf("提候选: %v", err)
	}
	return m
}

// R1 · 类型是封闭枚举。
func TestMemoryKind_R1_ClosedEnum(t *testing.T) {
	all := model.AllMemoryKinds()
	if len(all) != 3 {
		t.Fatalf("类型 %d 个，§14.3 是 constraint / experience / fact 三个", len(all))
	}
	for _, k := range all {
		if !k.IsValid() {
			t.Errorf("%q 在全集里却判定为非法", k)
		}
	}
	if model.MemoryKind("note").IsValid() {
		t.Error("「note」被判成合法")
	}
	if model.MemoryKind("").IsValid() {
		t.Error("空串被判成合法")
	}
}

// R2 · 状态五态，且迁移只有 INV-MEM-4 那四条。
//
// ★ 设计稿的筛选器只有三档（active / 候选 / 已失效），那是**界面的分组**：
// 「已失效」同时装着 invalid 与 obsolete。两者对用户长得一样，
// 对系统不一样——废弃要带理由、可指向 supersedes。
func TestMemoryStatus_R2_FiveStatesFourTransitions(t *testing.T) {
	all := model.AllMemoryStatuses()
	if len(all) != 5 {
		t.Fatalf("状态 %d 个，§14.4 是 candidate/active/discarded/invalid/obsolete 五个", len(all))
	}

	want := map[model.MemoryStatus][]model.MemoryStatus{
		model.MemoryCandidate: {model.MemoryActive, model.MemoryDiscarded},
		model.MemoryActive:    {model.MemoryInvalid, model.MemoryObsolete},
		model.MemoryDiscarded: {},
		model.MemoryInvalid:   {},
		model.MemoryObsolete:  {},
	}
	for _, s := range all {
		got := model.AllowedMemoryTransitionsFrom(s)
		if !reflect.DeepEqual(got, want[s]) {
			t.Errorf("%s 能去 %v，想要 %v", s, got, want[s])
		}
	}

	// ★ 没有任何一条边指回 candidate——它只能是**创建时**的状态。
	// 有回边的话，一条被人确认过、已经注入了几十轮的记忆能重新变成待审。
	for from, tos := range want {
		for _, to := range tos {
			if to == model.MemoryCandidate {
				t.Errorf("%s → candidate 是一条回边，不该存在", from)
			}
		}
	}
}

// ★★ INV-MEM-2 · 绝不自动写入：创建出来的只能是 candidate。
//
// 没有第二个构造函数能直接造出 active 的——有的话，那就是「自动写入」
// 的后门，而这条规矩在三份文档里被写死过。
func TestProposeCandidate_INVMEM2_OnlyCreatesCandidates(t *testing.T) {
	m := candidate(t)
	if m.Status() != model.MemoryCandidate {
		t.Errorf("新建的记忆状态 = %q，只能是 candidate（INV-MEM-2）", m.Status())
	}
	if m.Injectable() {
		t.Error("刚提出来就可注入——那正是「自动写入」的样子")
	}
	if m.ConfirmedBy() != "" {
		t.Errorf("没人确认过却记着 confirmedBy = %q", m.ConfirmedBy())
	}
}

// ★★ INV-MEM-2 · 晋升必须带一个**人**的动作。
//
// 允许空 actor 的话，调用方一句 Confirm("") 就绕过了整条规矩，
// 而代码读起来完全正常。
func TestMemory_INVMEM2_ConfirmNeedsAnActor(t *testing.T) {
	m := candidate(t)
	if err := m.Confirm(""); !errors.Is(err, model.ErrMemoryNeedsUserAction) {
		t.Errorf("空 actor 的错误 = %v，想要 ErrMemoryNeedsUserAction", err)
	}
	if m.Status() != model.MemoryCandidate {
		t.Errorf("确认被拒后状态变成了 %q", m.Status())
	}
	if err := m.Confirm("   "); !errors.Is(err, model.ErrMemoryNeedsUserAction) {
		t.Errorf("全空白的 actor 也该拒：%v", err)
	}

	if err := m.Confirm("luca"); err != nil {
		t.Fatalf("正常确认: %v", err)
	}
	if m.Status() != model.MemoryActive {
		t.Errorf("确认后状态 = %q", m.Status())
	}
	if m.ConfirmedBy() != "luca" {
		t.Errorf("没记下是谁确认的：%q——半年后要能查「这条谁放行的」", m.ConfirmedBy())
	}
}

// ★★ INV-MEM-3 · 没有证据就不能成为记忆。
//
// 空着的话，AI 的一句臆断就能变成以后每一轮的前提。
func TestProposeCandidate_INVMEM3_RequiresSourceRefs(t *testing.T) {
	for _, refs := range [][]string{nil, {}} {
		_, err := model.ProposeCandidate("mem-1", model.MemoryFact, "p", refs, "memory_curator")
		if !errors.Is(err, model.ErrMemoryNoSourceRefs) {
			t.Errorf("refs=%v 的错误 = %v，想要 ErrMemoryNoSourceRefs", refs, err)
		}
	}
}

func TestProposeCandidate_RejectsBadInput(t *testing.T) {
	refs := []string{"ev-1"}
	cases := []struct {
		name             string
		id               string
		kind             model.MemoryKind
		scope            model.MemoryScope
		wantErrSubstring string
	}{
		{"空 id", "", model.MemoryFact, "p", "id"},
		{"类型不认识", "mem-1", "note", "p", "记忆类型"},
		{"空 scope", "mem-1", model.MemoryFact, "", "scope"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := model.ProposeCandidate(c.id, c.kind, c.scope, refs, "x")
			if err == nil {
				t.Fatal("非法输入却构造成功了")
			}
			if !strings.Contains(err.Error(), c.wantErrSubstring) {
				t.Errorf("错误 = %v，应含 %q", err, c.wantErrSubstring)
			}
		})
	}
}

// R4 · 候选态**不参与注入**（INV-MEM-2 / INV-MEM-5）。
func TestMemory_R4_OnlyActiveIsInjectable(t *testing.T) {
	m := candidate(t)
	if m.Injectable() {
		t.Error("候选态可注入——那就等于自动写入了：AI 提的话还没人看过就成了前提")
	}

	if err := m.Confirm("luca"); err != nil {
		t.Fatal(err)
	}
	if !m.Injectable() {
		t.Error("active 却不可注入")
	}

	if err := m.MarkInvalid(); err != nil {
		t.Fatal(err)
	}
	if m.Injectable() {
		t.Error("失效的还在注入清单里")
	}
}

func TestMemory_R4b_DiscardedAndObsoleteNotInjectable(t *testing.T) {
	rejected := candidate(t)
	if err := rejected.Reject(); err != nil {
		t.Fatal(err)
	}
	if rejected.Injectable() {
		t.Error("被否决的还能注入")
	}

	dep := candidate(t)
	if err := dep.Confirm("luca"); err != nil {
		t.Fatal(err)
	}
	if err := dep.Deprecate("换成了 mem-500 的说法", "mem-500"); err != nil {
		t.Fatal(err)
	}
	if dep.Injectable() {
		t.Error("已废弃的还能注入")
	}
}

// ★★ INV-MEM-1 · 项目 P1 的记忆永不出现在 P2 的清单里。
//
// 串项目的后果是把 A 项目的约束当成 B 项目的前提——
// 而两个项目的约定常常正好相反。
func TestMemory_INVMEM1_ScopeIsolation(t *testing.T) {
	m := candidate(t) // scope = acp-engine

	if !m.VisibleIn("acp-engine") {
		t.Error("本项目的记忆在本项目不可见")
	}
	if m.VisibleIn("acp-sidecar") {
		t.Error("acp-engine 的记忆在 acp-sidecar 可见——两个项目的约定常常正好相反")
	}
	if m.VisibleIn(model.CrossProjectScope) {
		t.Error("项目级记忆出现在跨项目列表里（R3：两级要分开存）")
	}
}

// R3 · 跨项目记忆对所有项目可见。
func TestMemory_R3_CrossProjectVisibleEverywhere(t *testing.T) {
	m, err := model.ProposeCandidate(
		"mem-188", model.MemoryExperience, model.CrossProjectScope,
		[]string{"ev-900"}, "memory_curator",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Scope().IsCrossProject() {
		t.Error("跨项目 scope 没被认出来")
	}
	for _, p := range []model.MemoryScope{"acp-engine", "acp-sidecar", "duet-app"} {
		if !m.VisibleIn(p) {
			t.Errorf("跨项目记忆在 %s 不可见", p)
		}
	}
}

// INV-MEM-4 · 非法迁移一律拒绝，且状态不变。
func TestMemory_INVMEM4_RejectsIllegalTransitions(t *testing.T) {
	// 候选不能直接失效
	m := candidate(t)
	if err := m.MarkInvalid(); !errors.Is(err, model.ErrMemoryTransition) {
		t.Errorf("candidate → invalid 的错误 = %v，想要 ErrMemoryTransition", err)
	}
	if m.Status() != model.MemoryCandidate {
		t.Errorf("被拒后状态变成了 %q", m.Status())
	}

	// 终态没有出边
	done := candidate(t)
	if err := done.Reject(); err != nil {
		t.Fatal(err)
	}
	for name, fn := range map[string]func() error{
		"Confirm":     func() error { return done.Confirm("luca") },
		"MarkInvalid": done.MarkInvalid,
		"Deprecate":   func() error { return done.Deprecate("理由", "") },
	} {
		if err := fn(); err == nil {
			t.Errorf("discarded 之后还能 %s", name)
		}
	}
	if done.Status() != model.MemoryDiscarded {
		t.Errorf("终态被改成了 %q", done.Status())
	}
}

// INV-MEM-7 · 废弃必须给理由，supersedes 不能指向自身。
func TestMemory_INVMEM7_DeprecateNeedsReason(t *testing.T) {
	m := candidate(t)
	if err := m.Confirm("luca"); err != nil {
		t.Fatal(err)
	}

	if err := m.Deprecate("", "mem-500"); !errors.Is(err, model.ErrMemoryNeedsReason) {
		t.Errorf("没理由的错误 = %v，想要 ErrMemoryNeedsReason", err)
	}
	if m.Status() != model.MemoryActive {
		t.Errorf("被拒后状态变成了 %q", m.Status())
	}

	if err := m.Deprecate("理由", m.ID()); !errors.Is(err, model.ErrMemorySelfSupersede) {
		t.Errorf("自指的错误 = %v，想要 ErrMemorySelfSupersede", err)
	}
	if m.Status() != model.MemoryActive {
		t.Errorf("自指被拒后状态变成了 %q", m.Status())
	}

	if err := m.Deprecate("已被 mem-500 取代", "mem-500"); err != nil {
		t.Fatalf("正常废弃: %v", err)
	}
	if m.Reason() == "" {
		t.Error("废弃了却没留下理由")
	}
	if m.Supersedes() != "mem-500" {
		t.Errorf("supersedes = %q", m.Supersedes())
	}
}

// ★★ INV-MEM-6 · 没有 Delete。失效 ≠ 删除。
//
// 用反射守：加一个 `Delete()` 毫不费力，而加完所有测试还是绿的——
// 直到有人想查「半年前那次运行用的是哪条记忆」，答案已经不在任何地方了。
func TestMemory_INVMEM6_HasNoDeleteMethod(t *testing.T) {
	// ★ 对**指针类型**取方法集：值类型的方法集不含指针接收者的方法，
	// 加一个 `func (m *Memory) Delete()` 上去照样绿（PlanVersion 那条负例的教训）。
	rt := reflect.TypeOf(&model.Memory{})
	forbidden := []string{"delete", "remove", "purge", "drop", "erase"}
	for i := range rt.NumMethod() {
		name := strings.ToLower(rt.Method(i).Name)
		for _, bad := range forbidden {
			if strings.Contains(name, bad) {
				t.Errorf("Memory 有方法 %q——失效不等于删除（INV-MEM-6）。"+
					"删掉的话，半年前那次运行「当时用的是哪条记忆」就永远查不到了",
					rt.Method(i).Name)
			}
		}
	}
}

// ★★ INV-MEM-8 · 正文只在 md 文件里，模型不存正文。
//
// 存两份的话它们迟早对不上，而到时候「哪一份是真的」没有答案。
func TestMemory_INVMEM8_DoesNotHoldContent(t *testing.T) {
	rt := reflect.TypeOf(model.Memory{})
	forbidden := []string{"content", "body", "text", "markdown", "md"}
	for i := range rt.NumField() {
		name := strings.ToLower(rt.Field(i).Name)
		for _, bad := range forbidden {
			if name == bad {
				t.Errorf("Memory 有字段 %q——正文只存在于 md 文件里（INV-MEM-8）。"+
					"DB 也存一份的话，两边迟早对不上", rt.Field(i).Name)
			}
		}
	}
}

// INV-MEM-10 · 每次状态变更追加一条变更历史。
func TestMemory_INVMEM10_HistoryGrowsOnEveryChange(t *testing.T) {
	m := candidate(t)
	created := m.HistoryLen()
	if created < 1 {
		t.Fatalf("创建时的历史条数 = %d，创建本身就该是一条", created)
	}

	if err := m.Confirm("luca"); err != nil {
		t.Fatal(err)
	}
	if m.HistoryLen() != created+1 {
		t.Errorf("确认后历史 = %d，想要 %d", m.HistoryLen(), created+1)
	}

	if err := m.MarkInvalid(); err != nil {
		t.Fatal(err)
	}
	if m.HistoryLen() != created+2 {
		t.Errorf("失效后历史 = %d，想要 %d", m.HistoryLen(), created+2)
	}

	// ★ 被拒的迁移**不该**留下历史条目——它什么都没发生
	before := m.HistoryLen()
	_ = m.Confirm("luca")
	if m.HistoryLen() != before {
		t.Errorf("一次被拒的迁移也记了历史：%d → %d", before, m.HistoryLen())
	}
}

func TestMemory_SourceRefsReturnsCopy(t *testing.T) {
	m := candidate(t)
	refs := m.SourceRefs()
	refs[0] = "篡改"
	if again := m.SourceRefs(); again[0] != "ev-412" {
		t.Errorf("改了返回值之后再取 = %q", again[0])
	}
}

func TestAllMemoryStatuses_ReturnsCopy(t *testing.T) {
	first := model.AllMemoryStatuses()
	first[0] = "篡改"
	if second := model.AllMemoryStatuses(); second[0] != model.MemoryCandidate {
		t.Errorf("改了返回值之后再取 = %q", second[0])
	}
}

func TestAllMemoryKinds_ReturnsCopy(t *testing.T) {
	first := model.AllMemoryKinds()
	first[0] = "篡改"
	if second := model.AllMemoryKinds(); second[0] != model.MemoryConstraint {
		t.Errorf("改了返回值之后再取 = %q", second[0])
	}
}
