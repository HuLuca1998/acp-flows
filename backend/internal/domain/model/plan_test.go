package model_test

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// U4.2.1 · 计划只增不改（验收点 V11）
//
// ★ 一句话规矩（`docs/spec/domain-model.md` INV-PLAN-4）：
// **计划只能新增版本，不能改写；任何重规划都要说明哪些已验收工作
// 「仍有效 / 需补充 / 需回滚 / 已废弃」。**
//
// 改写的后果是用户看不到「为什么改」——他打开计划面板只看到当前这版，
// 而 AI 上周为什么推翻了原方案，答案不在任何地方。

// ★★ R1：**不存在任何能改已有版本的导出方法。**
//
// 靠人自觉「别写 setter」是守不住的：下一个人加一个
// `func (v *PlanVersion) SetTitle(...)` 完全合情合理，而计划从此可以被改写。
// 用反射把这条变成机器检查。
//
// ★ `PlanVersion` 是**值类型**，所以「有没有指针接收者的方法」在这里
// 是个有效判据——值接收者根本改不到原对象。契约那边不一样（见下面那条）。
func TestPlanVersion_R1_HasNoMutators(t *testing.T) {
	v := model.NewPlanVersion(1, "第一版", nil)
	typ := reflect.TypeOf(v)

	for i := 0; i < typ.NumMethod(); i++ {
		m := typ.Method(i)
		// ★ 判据是「接收者是不是指针」：值接收者改不到原对象，
		// 而指针接收者的导出方法是唯一能改写它的途径。
		if m.Type.In(0).Kind() == reflect.Ptr {
			t.Errorf("PlanVersion 有一个指针接收者的导出方法 %q——\n"+
				"计划只能新增版本不能改写（INV-PLAN-4）。"+
				"要新增内容就造一个新版本，别在旧版本上动手。", m.Name)
		}
	}
	if typ.NumMethod() == 0 {
		t.Fatal("一个导出方法都没有，这条检查什么也没证明")
	}
}

// ★★ 契约**冻结之后**没有任何导出方法能改到它。
//
// ★ 判据不能是「有没有指针接收者的方法」——`UnitID()` 这类读方法用指针
// 接收者完全正常（第一版这么写，把六个读方法全报成了违规）。
// 真正的判据是**调用之后字段有没有变**：对着一份冻结的契约把每个导出方法
// 都调一遍，然后核对内容。
func TestUnitContract_R1_FrozenIsImmutable(t *testing.T) {
	c := model.NewUnitContract("unit-01", 1)
	if err := c.AddCriterion("R1", "断言 X"); err != nil {
		t.Fatal(err)
	}
	if err := c.Freeze(); err != nil {
		t.Fatal(err)
	}

	before := snapshotContract(c)

	// 把每个导出方法都试着调一遍——能不带参数调的就调
	typ := reflect.TypeOf(c)
	called := 0
	for i := 0; i < typ.NumMethod(); i++ {
		m := typ.Method(i)
		args := []reflect.Value{reflect.ValueOf(c)}
		ok := true
		for j := 1; j < m.Type.NumIn(); j++ {
			// 只造得出零值参数的方法；造不出的跳过（下面单独有测试）
			args = append(args, reflect.Zero(m.Type.In(j)))
		}
		if !ok {
			continue
		}
		func() {
			defer func() { _ = recover() }() // 参数不合适时的 panic 不算数
			m.Func.Call(args)
			called++
		}()
	}
	if called == 0 {
		t.Fatal("一个方法都没调到，这条检查什么也没证明")
	}

	if after := snapshotContract(c); after != before {
		t.Errorf("冻结的契约被改了：\n  之前 %s\n  之后 %s\n"+
			"冻结的意思是「这就是这次要做的事，说定了」——"+
			"之后还能改的话，AI 可以一边做一边把标准改成自己刚好达到的样子",
			before, after)
	}
}

// snapshotContract 把契约的全部可见内容压成一个字符串，用来比对「变没变」。
func snapshotContract(c *model.UnitContract) string {
	parts := []string{c.UnitID(), fmt.Sprint(c.Version()), fmt.Sprint(c.IsFrozen())}
	for _, cr := range c.Criteria() {
		parts = append(parts, cr.ID+"="+cr.Text)
	}
	return strings.Join(parts, "|")
}

// ★★ R2：v ≥ 2 必须**对每一项已验收工作都给出处置**。
//
// 漏掉一项的后果是：那项工作既没说「仍有效」也没说「要回滚」，
// 用户不知道自己上周验收过的东西还算不算数。
func TestPlanVersion_R2_RequiresDispositionForEveryAccepted(t *testing.T) {
	accepted := []string{"subplan-01", "unit-07"}

	tests := []struct {
		name         string
		dispositions map[string]model.Disposition
		wantErr      error
	}{
		{
			"全都给了处置",
			map[string]model.Disposition{
				"subplan-01": model.DispositionStillValid,
				"unit-07":    model.DispositionObsolete,
			},
			nil,
		},
		{
			"漏了一项",
			map[string]model.Disposition{"subplan-01": model.DispositionStillValid},
			model.ErrDispositionMissing,
		},
		{
			"一项都没给",
			nil,
			model.ErrDispositionMissing,
		},
		{
			"给了一个没验收过的",
			map[string]model.Disposition{
				"subplan-01": model.DispositionStillValid,
				"unit-07":    model.DispositionStillValid,
				"unit-99":    model.DispositionObsolete, // 这个根本没验收过
			},
			model.ErrDispositionUnknownTarget,
		},
		{
			"处置取值不认识",
			map[string]model.Disposition{
				"subplan-01": model.Disposition("大概还行吧"),
				"unit-07":    model.DispositionStillValid,
			},
			model.ErrDispositionInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := model.NewReplan(2, "第二版", accepted, tt.dispositions)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("错误 = %v, 想要 %v", err, tt.wantErr)
			}
		})
	}
}

// v1 不需要处置——那时还没有任何已验收的工作。
func TestPlanVersion_FirstVersionNeedsNoDisposition(t *testing.T) {
	v := model.NewPlanVersion(1, "第一版", nil)
	if v.Version() != 1 {
		t.Errorf("版本号 = %d, 想要 1", v.Version())
	}
	if len(v.Dispositions()) != 0 {
		t.Errorf("第一版却有 %d 条处置", len(v.Dispositions()))
	}
}

// ★ 处置的取值是**封闭枚举**，穷举有据。
//
// 新增一种处置时，这条测试会逼人把它登记进全集——漏登记的话，
// 界面上的选择器少一项，而用户永远选不到它。
func TestDisposition_AllValuesAreValid(t *testing.T) {
	all := model.AllDispositions()
	if len(all) != 4 {
		t.Fatalf("处置有 %d 种, 想要 4（仍有效 / 需补充 / 需回滚 / 已废弃）", len(all))
	}
	for _, d := range all {
		if !d.IsValid() {
			t.Errorf("%q 在全集里却不合法", d)
		}
		// 标识要是机器可读的：界面按它查词条
		for _, r := range d {
			if r > 127 {
				t.Errorf("处置标识 %q 里有非 ASCII 字符——界面按它查 i18n 词条", d)
				break
			}
		}
	}
	if model.Disposition("随便").IsValid() {
		t.Error("认不出的取值被当成合法的了")
	}
}

// ★★ R4：版本号**严格递增，不跳号**。
//
// 跳号的话，用户看到 v3 之后是 v6，会以为自己漏看了两版——
// 而实际上那两版根本不存在。
func TestPlanVersion_R4_VersionIncrementsByOne(t *testing.T) {
	accepted := []string{"unit-01"}
	ok := map[string]model.Disposition{"unit-01": model.DispositionStillValid}

	tests := []struct {
		version int
		wantErr bool
	}{
		{2, false},
		{1, true}, // 倒退
		{3, true}, // 跳号（当前是 v1，下一版只能是 v2）
		{0, true},
		{-1, true},
	}

	for _, tt := range tests {
		v1 := model.NewPlanVersion(1, "第一版", nil)
		_, err := v1.Next(tt.version, "下一版", accepted, ok)
		if tt.wantErr && err == nil {
			t.Errorf("从 v1 造 v%d 却成功了——用户看到 v1 之后是 v%d，"+
				"会以为自己漏看了几版", tt.version, tt.version)
		}
		if !tt.wantErr && err != nil {
			t.Errorf("从 v1 造 v%d 失败了: %v", tt.version, err)
		}
	}
}

// ★★ R3：契约冻结之后改任何字段都要报错。
//
// 冻结的意思是「这就是这次要做的事，说定了」。之后还能改的话，
// AI 可以一边做一边把标准改成自己刚好达到的样子。
func TestUnitContract_R3_FrozenRejectsChanges(t *testing.T) {
	c := model.NewUnitContract("unit-01", 1)
	if err := c.AddCriterion("R1", "断言 X 成立"); err != nil {
		t.Fatalf("冻结前加验收标准应该成功: %v", err)
	}
	if err := c.Freeze(); err != nil {
		t.Fatalf("冻结失败: %v", err)
	}

	if err := c.AddCriterion("R2", "偷偷加一条"); !errors.Is(err, model.ErrContractFrozen) {
		t.Errorf("错误 = %v, 想要 ErrContractFrozen——\n"+
			"冻结之后还能改的话，AI 可以一边做一边把标准改成自己刚好达到的样子", err)
	}
	if len(c.Criteria()) != 1 {
		t.Errorf("冻结后验收标准变成了 %d 条", len(c.Criteria()))
	}
}

// 重复冻结不报错也不改变什么——用户点两下「冻结」是常态。
func TestUnitContract_FreezeIsIdempotent(t *testing.T) {
	c := model.NewUnitContract("unit-01", 1)
	_ = c.AddCriterion("R1", "断言 X")

	if err := c.Freeze(); err != nil {
		t.Fatal(err)
	}
	if err := c.Freeze(); err != nil {
		t.Errorf("第二次冻结报错了: %v", err)
	}
	if !c.IsFrozen() {
		t.Error("冻结之后 IsFrozen 却是 false")
	}
}

// ★ 一条验收标准都没有的契约**不许冻结**。
//
// 空契约冻结之后，「做完了」这件事没有任何判据——
// AI 说做完了就是做完了。
func TestUnitContract_EmptyCannotFreeze(t *testing.T) {
	c := model.NewUnitContract("unit-01", 1)

	if err := c.Freeze(); err == nil {
		t.Error("一条验收标准都没有却冻结成功了——\n" +
			"「做完了」这件事没有任何判据，AI 说做完了就是做完了")
	}
}

// ★★ 契约版本号严格递增（R4 的另一半）。
func TestUnitContract_R4_VersionIncrementsByOne(t *testing.T) {
	c := model.NewUnitContract("unit-01", 3)
	_ = c.AddCriterion("R1", "断言 X")
	_ = c.Freeze()

	if _, err := c.Revise(4); err != nil {
		t.Errorf("v3 之后造 v4 失败了: %v", err)
	}
	for _, bad := range []int{3, 5, 1, 0} {
		if _, err := c.Revise(bad); err == nil {
			t.Errorf("v3 之后造 v%d 却成功了", bad)
		}
	}
}

// 修订出来的新契约是**没冻结**的，可以继续加标准。
func TestUnitContract_RevisedIsUnfrozen(t *testing.T) {
	c := model.NewUnitContract("unit-01", 1)
	_ = c.AddCriterion("R1", "断言 X")
	_ = c.Freeze()

	next, err := c.Revise(2)
	if err != nil {
		t.Fatal(err)
	}
	if next.IsFrozen() {
		t.Error("修订出来的新契约是冻结的——那就没法改了，修订等于白做")
	}
	if err := next.AddCriterion("R2", "断言 Y"); err != nil {
		t.Errorf("修订版加标准失败: %v", err)
	}
	// ★ 旧的那份**一个字都不能变**
	if len(c.Criteria()) != 1 {
		t.Errorf("修订之后旧契约变成了 %d 条标准——那叫改写不叫修订", len(c.Criteria()))
	}
}

// domain 是纯计算：这个包不许 import 任何内部包，也不许出现时间与上下文。
//
// ★ 本单元的 forbidden_changes 明写这一条。它是「domain 100% 可单测」
// 这个前提的基础——一旦有了 IO，测试就要起服务。
func TestPlanModel_StaysPure(t *testing.T) {
	// 构造与校验都不需要 context，也不取当前时间
	v := model.NewPlanVersion(1, "第一版", nil)
	if v.Title() != "第一版" {
		t.Errorf("标题 = %q", v.Title())
	}
	// 这条断言的实际保障来自 .golangci.yml 的 depguard 与
	// check-naming 的第 8 节，这里只是把意图写下来
	if strings.TrimSpace(v.Title()) == "" {
		t.Error("标题不该是空的")
	}
}
