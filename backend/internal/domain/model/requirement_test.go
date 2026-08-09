package model_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// M5 U5.2.1 · 需求的版本与冻结
//
// 契约来源：docs/spec/domain-model.md §5（INV-REQ-1..4）
//
// ★★ 这一族守的是**「当时到底要做什么」永远有答案**。
// 冻结后可改的话，计划、契约、单元全是照着某一版做的，
// 而那一版已经不存在了。

func newReq(t *testing.T) *model.RequirementSnapshot {
	t.Helper()
	r, err := model.NewRequirement("work-08",
		[]string{"R1 取消必须幂等", "R2 现场要保留"},
		[]string{"取消时是否需要回滚已写入的文件"})
	if err != nil {
		t.Fatalf("造需求: %v", err)
	}
	return r
}

// ★★ R1 · 冻结后**不可变**（INV-REQ-2）。
//
// 用反射守：加一个 setter 毫不费力，而加完之后所有测试还是绿的——
// 直到有人想查「上周那版需求说的是什么」。
func TestRequirement_R1_FrozenIsImmutable(t *testing.T) {
	// ★ 对**指针类型**取方法集：值类型的方法集不含指针接收者的方法，
	// 加一个 setter 上去照样绿（PlanVersion 那条负例的教训）。
	rt := reflect.TypeOf(&model.RequirementSnapshot{})
	allowed := map[string]bool{
		"WorkID": true, "Version": true, "Frozen": true,
		"Items": true, "OpenFacts": true,
		// 这几个是受控的状态迁移，不是任意写入：
		// 前三个都**只在未冻结时**生效，Revise 造的是新对象
		"Freeze": true, "ResolveFact": true, "ReviseDraft": true,
		"Revise": true, "IsNextOf": true, "CanStartPlanning": true,
	}
	for i := range rt.NumMethod() {
		name := rt.Method(i).Name
		if !allowed[name] {
			t.Errorf("RequirementSnapshot 多了一个方法 %q——"+
				"冻结后的需求只能读，要改就出新版本（INV-REQ-2）", name)
		}
	}

	// 冻结之后连待确认清单都动不了
	r := newReq(t)
	if err := r.ResolveFact("取消时是否需要回滚已写入的文件"); err != nil {
		t.Fatal(err)
	}
	if err := r.Freeze(); err != nil {
		t.Fatal(err)
	}
	if err := r.ResolveFact("随便什么"); !errors.Is(err, model.ErrRequirementFrozen) {
		t.Errorf("冻结后还能改清单：%v", err)
	}
	if err := r.ReviseDraft([]string{"改过的"}, nil); !errors.Is(err, model.ErrRequirementFrozen) {
		t.Errorf("冻结后还能原地改内容：%v", err)
	}
	if len(r.Items()) != 2 || r.Items()[0] != "R1 取消必须幂等" {
		t.Errorf("被拒之后内容却变了：%v", r.Items())
	}
}

// ★★ 未冻结的版本能**原地改**——追问一轮就改一次，不该升版本号。
//
// 没有它的话，app 层唯一的出路是 RestoreRequirement，
// 而那个方法绕过所有校验：「条目不能全空」在追问路径上会彻底失效。
func TestRequirement_ReviseDraft_UpdatesInPlaceWhileUnfrozen(t *testing.T) {
	r := newReq(t)

	err := r.ReviseDraft(
		[]string{"R1 取消必须幂等", "R2 现场要保留", "R3 取消后不回滚已写入的文件"}, nil)
	if err != nil {
		t.Fatalf("改草稿: %v", err)
	}
	if len(r.Items()) != 3 {
		t.Errorf("条目 = %v", r.Items())
	}
	if r.Version() != 1 {
		t.Errorf("版本 = v%d，改草稿不该升版本号", r.Version())
	}
	// 待确认清单跟着一起被替换掉了，于是能冻结
	if err := r.Freeze(); err != nil {
		t.Errorf("清单空了却冻不上：%v", err)
	}

	// ★ 校验照样生效：全空的条目要被拒
	r2 := newReq(t)
	if err := r2.ReviseDraft([]string{"", "  "}, nil); !errors.Is(err, model.ErrNoRequirementItems) {
		t.Errorf("空条目却改成功了：%v", err)
	}
	if len(r2.Items()) != 2 {
		t.Errorf("被拒之后内容却变了：%v", r2.Items())
	}
}

// ★★ R2 · 改需求产生 v2，**v1 仍可读**。
//
// 改写旧版的话，「上周那版说的是什么」永远没有答案。
func TestRequirement_R2_ReviseKeepsTheOldVersion(t *testing.T) {
	v1 := newReq(t)
	if err := v1.ResolveFact("取消时是否需要回滚已写入的文件"); err != nil {
		t.Fatal(err)
	}
	if err := v1.Freeze(); err != nil {
		t.Fatal(err)
	}
	before := v1.Items()

	v2, err := v1.Revise([]string{"R1 取消必须幂等", "R2 现场要保留", "R3 停之后能接着干"}, nil)
	if err != nil {
		t.Fatalf("修订: %v", err)
	}

	if v2.Version() != 2 {
		t.Errorf("新版本 = v%d，想要 v2", v2.Version())
	}
	if len(v2.Items()) != 3 {
		t.Errorf("新版条目 = %v", v2.Items())
	}
	// ★★ 判据：v1 一个字都没变
	if !reflect.DeepEqual(v1.Items(), before) {
		t.Errorf("修订改动了旧版本：%v → %v", before, v1.Items())
	}
	if !v1.Frozen() {
		t.Error("旧版本的冻结状态被改了")
	}
	// ★ 新版本**不是**冻结的：它还要再被确认一次
	if v2.Frozen() {
		t.Error("新版本一出来就是冻结的——那用户就没机会再看一眼了")
	}
}

// ★★ R3 · 版本号只增不跳号。
func TestRequirement_R3_VersionChainIsStrict(t *testing.T) {
	v1 := newReq(t)
	if err := v1.IsNextOf(nil); err != nil {
		t.Errorf("v1 作为第一版被拒了：%v", err)
	}

	v2, err := v1.Revise([]string{"R1"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := v2.IsNextOf(v1); err != nil {
		t.Errorf("v2 接在 v1 后面被拒了：%v", err)
	}

	// 跳号：v3 直接接 v1
	v3, err := v2.Revise([]string{"R1"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := v3.IsNextOf(v1); !errors.Is(err, model.ErrRequirementVersionNotNext) {
		t.Errorf("跳号没被拒：%v——中间那一版去哪了没人说得清", err)
	}
	// 回退
	if err := v1.IsNextOf(v2); !errors.Is(err, model.ErrRequirementVersionNotNext) {
		t.Errorf("版本回退没被拒：%v", err)
	}
	// 第一版不是 v1
	if err := v2.IsNextOf(nil); !errors.Is(err, model.ErrRequirementVersionNotNext) {
		t.Errorf("v2 冒充第一版没被拒：%v", err)
	}
}

// ★★ R4 · 未冻结的需求**不能进计划**（INV-REQ-1）。
//
// 需求还在变的时候做出来的计划，做完也对不上。
func TestRequirement_R4_UnfrozenCannotStartPlanning(t *testing.T) {
	r := newReq(t)
	if r.CanStartPlanning() {
		t.Error("没冻结却能开始规划——需求还在变，做出来的计划做完也对不上")
	}

	if err := r.ResolveFact("取消时是否需要回滚已写入的文件"); err != nil {
		t.Fatal(err)
	}
	if err := r.Freeze(); err != nil {
		t.Fatal(err)
	}
	if !r.CanStartPlanning() {
		t.Error("冻结了却不能开始规划")
	}
}

// ★★ INV-REQ-1 · 还有待确认的事实时**拒绝冻结**。
//
// 带着没问清的问题往下走，AI 会自己替用户做决定——
// 而那些决定会一路固化进计划与契约，等他发现时已经改了几十个文件。
func TestRequirement_INVREQ1_OpenFactsBlockFreeze(t *testing.T) {
	r := newReq(t)

	err := r.Freeze()
	if !errors.Is(err, model.ErrOpenFactsRemain) {
		t.Fatalf("有待确认事实却冻上了：%v", err)
	}
	// ★ 错误里要**说清还剩什么**，不然用户不知道该去确认哪一条
	if !strings.Contains(err.Error(), "回滚") {
		t.Errorf("错误没说清还剩哪些：%v", err)
	}
	if r.Frozen() {
		t.Error("被拒之后状态却变了")
	}

	if err := r.ResolveFact("取消时是否需要回滚已写入的文件"); err != nil {
		t.Fatal(err)
	}
	if err := r.Freeze(); err != nil {
		t.Errorf("清单空了却还不给冻结：%v", err)
	}
}

// 重复冻结是幂等的——用户手快点两下是常态。
func TestRequirement_FreezeIsIdempotent(t *testing.T) {
	r := newReq(t)
	if err := r.ResolveFact("取消时是否需要回滚已写入的文件"); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if err := r.Freeze(); err != nil {
			t.Fatalf("重复冻结报错：%v", err)
		}
	}
	if !r.Frozen() {
		t.Error("冻了三次却没冻上")
	}
}

// ★ 一条条目都没有时拒绝创建。
//
// 空需求会一路走到「需求 0 · 已映射 0」，看起来一切正常而实际什么都没定。
func TestNewRequirement_RejectsEmpty(t *testing.T) {
	for _, items := range [][]string{nil, {}, {"", "   "}} {
		if _, err := model.NewRequirement("work-08", items, nil); !errors.Is(err, model.ErrNoRequirementItems) {
			t.Errorf("items=%v 却造出来了：%v", items, err)
		}
	}
	if _, err := model.NewRequirement("", []string{"R1"}, nil); err == nil {
		t.Error("没有 workID 却造出来了")
	}
}

// ★ 空白条目被丢掉。
//
// 留着的话「需求 6 · 已映射 6」这种统计会变成假的——那个 6 里有一条没内容。
func TestNewRequirement_DropsBlankItems(t *testing.T) {
	r, err := model.NewRequirement("work-08",
		[]string{"R1 真的条目", "   ", "", "R2 另一条"}, []string{"  ", "一条真的待确认"})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Items()) != 2 {
		t.Errorf("条目 = %v，空白的应该被丢掉", r.Items())
	}
	if len(r.OpenFacts()) != 1 {
		t.Errorf("待确认 = %v", r.OpenFacts())
	}
}

// 划掉一条不存在的待确认事实要报错，而不是静默成功。
func TestRequirement_ResolveUnknownFactErrs(t *testing.T) {
	r := newReq(t)
	if err := r.ResolveFact("根本没提过的问题"); err == nil {
		t.Error("划掉一条不存在的事实却成功了——用户会以为自己确认过了")
	}
	if len(r.OpenFacts()) != 1 {
		t.Errorf("清单被动了：%v", r.OpenFacts())
	}
}

// 返回的是副本，改它不影响快照。
func TestRequirement_ReturnsCopies(t *testing.T) {
	r := newReq(t)
	items := r.Items()
	items[0] = "篡改"
	if r.Items()[0] == "篡改" {
		t.Error("Items() 返回了内部切片")
	}
	facts := r.OpenFacts()
	if len(facts) > 0 {
		facts[0] = "篡改"
	}
	if len(r.OpenFacts()) > 0 && r.OpenFacts()[0] == "篡改" {
		t.Error("OpenFacts() 返回了内部切片")
	}
}
