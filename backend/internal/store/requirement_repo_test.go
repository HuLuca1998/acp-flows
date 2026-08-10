package store_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
	"github.com/HuLuca1998/acp-flows/backend/internal/store"
)

// M5 U5.2.1 · 需求快照落库
//
// ★★ 版本链**只增不改**（INV-REQ-2）。改写已有版本的话，
// 「v2 说的是什么」会随时间变化，而计划与契约都是照着某一刻的 v2 做的。

func reqV1(t *testing.T) *model.RequirementSnapshot {
	t.Helper()
	r, err := model.NewRequirement("work-08",
		[]string{"R1 取消必须幂等，且现场要保留", "R2 停之后能接着干"},
		[]string{"取消时是否需要回滚已写入的文件"})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// 存进去再取出来，条目与待确认清单都不丢。
func TestRequirementRepo_SaveAndLatestRoundTrip(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()

	v1 := reqV1(t)
	if err := db.Requirements().SaveRequirement(ctx, v1); err != nil {
		t.Fatalf("存: %v", err)
	}

	got, err := db.Requirements().LatestRequirement(ctx, "work-08")
	if err != nil {
		t.Fatalf("取: %v", err)
	}
	if !reflect.DeepEqual(got.Items(), v1.Items()) {
		t.Errorf("条目 = %v，想要 %v", got.Items(), v1.Items())
	}
	// ★ 条目里有逗号——用逗号当分隔符的话，这一条会被拆成两条
	if len(got.Items()) != 2 {
		t.Errorf("条目被拆坏了：%v", got.Items())
	}
	if !reflect.DeepEqual(got.OpenFacts(), v1.OpenFacts()) {
		t.Errorf("待确认 = %v", got.OpenFacts())
	}
	if got.Frozen() {
		t.Error("新存的版本不该是冻结的")
	}
}

// ★★ 改写**已冻结**的版本要报错，不是静默覆盖。
//
// 静默覆盖的话，一次重试就能把已经冻结的那一版换掉，而没有任何痕迹——
// 而计划、契约、单元全是照着那一版做的。
func TestRequirementRepo_RefusesToRewriteAFrozenVersion(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()
	repo := db.Requirements()

	v1 := reqV1(t)
	if err := v1.ResolveFact("取消时是否需要回滚已写入的文件"); err != nil {
		t.Fatal(err)
	}
	if err := v1.Freeze(); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveRequirement(ctx, v1); err != nil {
		t.Fatal(err)
	}

	// 同一个版本号，不同内容
	tampered := model.RestoreRequirement("work-08", 1, []string{"完全不同的需求"}, nil, true)
	if err := repo.SaveRequirement(ctx, tampered); err == nil {
		t.Fatal("改写已冻结的 v1 却成功了——「v1 说的是什么」会随时间变化，" +
			"而计划与契约都是照着某一刻的 v1 做的")
	}

	// ★ 判据：库里那条**一个字都没变**
	got, err := repo.LatestRequirement(ctx, "work-08")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Items(), v1.Items()) {
		t.Errorf("内容被改了：%v", got.Items())
	}
}

// 原样重存一份已冻结的版本是**幂等**的——重试与手快点两下都是常态。
func TestRequirementRepo_ResavingIdenticalFrozenIsIdempotent(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()
	repo := db.Requirements()

	frozen := model.RestoreRequirement("work-08", 1, []string{"R1", "R2"}, nil, true)
	for range 3 {
		if err := repo.SaveRequirement(ctx, frozen); err != nil {
			t.Fatalf("重存一模一样的一版报错：%v", err)
		}
	}
	all, err := repo.RequirementVersions(ctx, "work-08")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Errorf("存了 %d 条，重存不该造出新记录", len(all))
	}
}

// ★★ **未冻结的版本是草稿，原地覆盖**。
//
// 需求分析师追问一轮就划掉几条待确认事实、改几个字。每动一下就升一个
// 版本号的话，版本链记的就不再是「需求变过几次」而是「问过几个问题」。
func TestRequirementRepo_DraftIsUpdatedInPlace(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()
	repo := db.Requirements()

	v1 := reqV1(t)
	if err := repo.SaveRequirement(ctx, v1); err != nil {
		t.Fatal(err)
	}

	// 追问一轮：划掉待确认事实，顺手补一条需求
	if err := v1.ResolveFact("取消时是否需要回滚已写入的文件"); err != nil {
		t.Fatal(err)
	}
	revised := model.RestoreRequirement("work-08", 1,
		append(v1.Items(), "R3 取消后不回滚已写入的文件"), v1.OpenFacts(), false)
	if err := repo.SaveRequirement(ctx, revised); err != nil {
		t.Fatalf("更新草稿: %v", err)
	}

	got, err := repo.LatestRequirement(ctx, "work-08")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items()) != 3 {
		t.Errorf("条目 = %v，草稿的改动没落盘", got.Items())
	}
	// ★★ 判据：**最后一条待确认事实被划掉**要落盘。
	//
	// 空清单是零值——GORM 的 Updates 传 struct 会把它当「没设置」丢掉，
	// 于是用户看到「还剩 1 条待确认」而永远冻不上。
	if len(got.OpenFacts()) != 0 {
		t.Errorf("待确认 = %v，划掉最后一条没落盘——用户会永远冻不上", got.OpenFacts())
	}

	all, err := repo.RequirementVersions(ctx, "work-08")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Errorf("版本 = %d 条，改草稿不该升版本号", len(all))
	}
}

// ★ 冻结是唯一允许的原地变化：它只改状态不改内容。
func TestRequirementRepo_FreezingPersists(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()
	repo := db.Requirements()

	v1 := reqV1(t)
	if err := repo.SaveRequirement(ctx, v1); err != nil {
		t.Fatal(err)
	}

	if err := v1.ResolveFact("取消时是否需要回滚已写入的文件"); err != nil {
		t.Fatal(err)
	}
	if err := v1.Freeze(); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveRequirement(ctx, v1); err != nil {
		t.Fatalf("冻结后存: %v", err)
	}

	got, err := repo.LatestRequirement(ctx, "work-08")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Frozen() {
		t.Error("冻结没落盘")
	}
	if !got.CanStartPlanning() {
		t.Error("冻结了却不能开始规划")
	}
}

// ★★ **解冻是没有的事**。
func TestRequirementRepo_RefusesToUnfreeze(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()
	repo := db.Requirements()

	frozen := model.RestoreRequirement("work-08", 1, []string{"R1"}, nil, true)
	if err := repo.SaveRequirement(ctx, frozen); err != nil {
		t.Fatal(err)
	}

	thawed := model.RestoreRequirement("work-08", 1, []string{"R1"}, nil, false)
	if err := repo.SaveRequirement(ctx, thawed); err == nil {
		t.Fatal("解冻却成功了——冻结之后那一版就是历史的一部分")
	}

	got, err := repo.LatestRequirement(ctx, "work-08")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Frozen() {
		t.Error("被解冻了")
	}
}

// ★★ **旧版本全都留着**——用户要能回答「上周那版说的是什么」。
func TestRequirementRepo_KeepsEveryVersion(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()
	repo := db.Requirements()

	v1 := reqV1(t)
	if err := v1.ResolveFact("取消时是否需要回滚已写入的文件"); err != nil {
		t.Fatal(err)
	}
	if err := v1.Freeze(); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveRequirement(ctx, v1); err != nil {
		t.Fatal(err)
	}

	v2, err := v1.Revise([]string{"R1 改过的", "R2", "R3 新加的"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveRequirement(ctx, v2); err != nil {
		t.Fatal(err)
	}

	all, err := repo.RequirementVersions(ctx, "work-08")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("版本 = %d 条，想要 2 条——旧版本被覆盖的话，"+
			"「上周那版说的是什么」永远没有答案", len(all))
	}
	// ★ 从新到旧
	if all[0].Version() != 2 || all[1].Version() != 1 {
		t.Errorf("顺序 = v%d v%d，想要 v2 v1", all[0].Version(), all[1].Version())
	}
	// v1 的内容还在
	if len(all[1].Items()) != 2 {
		t.Errorf("v1 的条目变了：%v", all[1].Items())
	}
	// LatestRequirement 取到的是 v2
	latest, err := repo.LatestRequirement(ctx, "work-08")
	if err != nil {
		t.Fatal(err)
	}
	if latest.Version() != 2 {
		t.Errorf("最新版 = v%d", latest.Version())
	}
}

// ★★ 仓储**没有改写类方法**（INV-REQ-2）。
//
// 加一个 Update 毫不费力，而加完之后所有测试还是绿的——
// 直到有人想查「上周那版需求说的是什么」。
func TestRequirementRepo_HasNoRewriteMethods(t *testing.T) {
	rt := reflect.TypeOf(&store.RequirementRepo{})
	forbidden := []string{"update", "delete", "remove", "overwrite", "replace", "purge"}
	for i := range rt.NumMethod() {
		name := strings.ToLower(rt.Method(i).Name)
		for _, bad := range forbidden {
			if strings.Contains(name, bad) {
				t.Errorf("RequirementRepo 有方法 %q——版本链只增不改（INV-REQ-2）",
					rt.Method(i).Name)
			}
		}
	}
}

// 还没提需求时是**空**不是错——新工作的常态。
func TestRequirementRepo_NoRequirementYet(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()

	all, err := db.Requirements().RequirementVersions(ctx, "work-new")
	if err != nil {
		t.Errorf("新工作报了错：%v", err)
	}
	if all == nil {
		t.Error("返回了 nil 而不是空切片")
	}

	if _, err := db.Requirements().LatestRequirement(ctx, "work-new"); err == nil {
		t.Error("没有需求却取到了一版")
	} else if !strings.Contains(err.Error(), model.ErrNotFound.Error()) {
		t.Errorf("错误 = %v，想要 model.ErrNotFound", err)
	}
}

// 两个工作的需求互不干扰。
func TestRequirementRepo_ScopedByWork(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()
	repo := db.Requirements()

	for _, id := range []string{"work-a", "work-b"} {
		r, err := model.NewRequirement(id, []string{"R1 " + id}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.SaveRequirement(ctx, r); err != nil {
			t.Fatal(err)
		}
	}

	got, err := repo.LatestRequirement(ctx, "work-a")
	if err != nil {
		t.Fatal(err)
	}
	if got.Items()[0] != "R1 work-a" {
		t.Errorf("拿到了别的工作的需求：%v", got.Items())
	}
}
