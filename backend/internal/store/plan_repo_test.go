package store_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
	"github.com/HuLuca1998/acp-flows/backend/internal/store"
)

// M6 U6.1.2 · 计划落库
//
// ★★ 计划版本链**只增不改**（INV-PLAN-4）：用户打开计划面板要能回答
// 「上周那版拆成了什么、为什么改」——覆盖掉的话那个问题永远没有答案。

func planV1(t *testing.T) model.PlanVersion {
	t.Helper()

	u12, err := model.NewUnit("unit-012", "取消协议", "implementer", nil)
	if err != nil {
		t.Fatal(err)
	}
	u13, err := model.NewUnit("unit-013", "取消后证据读取", "unit_reviewer", []string{"unit-012"})
	if err != nil {
		t.Fatal(err)
	}
	sp, err := model.NewSubplan("subplan-01", "ACP Runtime 抽象层", []model.Unit{u12, u13})
	if err != nil {
		t.Fatal(err)
	}

	v, err := model.NewPlanVersion(1, "取消运行中的 Agent turn", nil).
		WithSubplans([]model.Subplan{sp})
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// ★★ R1 · 存进去再取出来，**DAG 结构完整**：角色、依赖、顺序都在。
func TestPlanRepo_R1_RoundTripKeepsTheGraph(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()

	if err := db.Plans().SavePlan(ctx, "work-08", planV1(t)); err != nil {
		t.Fatalf("存: %v", err)
	}

	got, err := db.Plans().LatestPlan(ctx, "work-08")
	if err != nil {
		t.Fatalf("取: %v", err)
	}
	if n, m := got.Counts(); n != 1 || m != 2 {
		t.Fatalf("计数 = %d 子计划 · %d 单元，想要 1 · 2", n, m)
	}

	units := got.Subplans()[0].Units()
	// ★ 顺序要保住：按 id 排的话 unit-10 会排在 unit-02 前面
	if units[0].ID() != "unit-012" || units[1].ID() != "unit-013" {
		t.Errorf("单元顺序变了：%s %s", units[0].ID(), units[1].ID())
	}
	// ★★ 角色**必须**过库（裁定三）：丢了的话执行时没人认领
	if units[1].RoleID() != "unit_reviewer" {
		t.Errorf("角色 = %q，想要 unit_reviewer——丢了的话执行时没人认领这个单元",
			units[1].RoleID())
	}
	if deps := units[1].DependsOn(); len(deps) != 1 || deps[0] != "unit-012" {
		t.Errorf("依赖 = %v——丢了的话一个本该等着的单元会提前开工", deps)
	}
}

// ★★ R2 · 改写已有版本**被拒**，且库里那条一个字没变。
func TestPlanRepo_R2_RefusesToRewriteAVersion(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()
	repo := db.Plans()

	if err := repo.SavePlan(ctx, "work-08", planV1(t)); err != nil {
		t.Fatal(err)
	}

	tampered := model.NewPlanVersion(1, "完全不同的计划", nil)
	if err := repo.SavePlan(ctx, "work-08", tampered); err == nil {
		t.Fatal("改写 v1 却成功了——「上周那版拆成了什么」会随时间变化")
	}

	got, err := repo.LatestPlan(ctx, "work-08")
	if err != nil {
		t.Fatal(err)
	}
	if got.Title() != "取消运行中的 Agent turn" {
		t.Errorf("标题被改了：%q", got.Title())
	}
	if _, m := got.Counts(); m != 2 {
		t.Errorf("单元数 = %d，内容被改了", m)
	}
}

// ★★ R3 · 版本链**从新到旧**，旧版原样留着。
func TestPlanRepo_R3_KeepsEveryVersion(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()
	repo := db.Plans()

	if err := repo.SavePlan(ctx, "work-08", planV1(t)); err != nil {
		t.Fatal(err)
	}

	// v2：重规划，声明已验收工作的处置
	v2, err := planV1(t).Next(2, "取消运行中的 Agent turn（重规划）",
		[]string{"unit-012"}, map[string]model.Disposition{"unit-012": model.DispositionStillValid})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SavePlan(ctx, "work-08", v2); err != nil {
		t.Fatal(err)
	}

	all, err := repo.PlanVersions(ctx, "work-08")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("版本 = %d 条，想要 2 条", len(all))
	}
	if all[0].Version() != 2 || all[1].Version() != 1 {
		t.Errorf("顺序 = v%d v%d，想要 v2 v1", all[0].Version(), all[1].Version())
	}
	// ★ 处置要过库：不然「那次重规划怎么处理已验收的东西」没有答案
	if d := all[0].Dispositions(); d["unit-012"] != model.DispositionStillValid {
		t.Errorf("处置 = %v，没过库", d)
	}
	// v1 的内容原样
	if _, m := all[1].Counts(); m != 2 {
		t.Errorf("v1 的单元数变了：%d", m)
	}
}

// ★★ R4 · 仓储**没有改写类方法**（INV-PLAN-4）。
func TestPlanRepo_R4_HasNoRewriteMethods(t *testing.T) {
	rt := reflect.TypeOf(&store.PlanRepo{})
	forbidden := []string{"update", "delete", "remove", "overwrite", "replace", "purge"}
	for i := range rt.NumMethod() {
		name := strings.ToLower(rt.Method(i).Name)
		for _, bad := range forbidden {
			if strings.Contains(name, bad) {
				t.Errorf("PlanRepo 有方法 %q——计划改了就存新版本（INV-PLAN-4）",
					rt.Method(i).Name)
			}
		}
	}
}

// 还没规划时是**空**不是错——新工作的常态。
func TestPlanRepo_NoPlanYet(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()

	all, err := db.Plans().PlanVersions(ctx, "work-new")
	if err != nil {
		t.Errorf("新工作报了错：%v", err)
	}
	if all == nil {
		t.Error("返回了 nil 而不是空切片")
	}
	if _, err := db.Plans().LatestPlan(ctx, "work-new"); err == nil {
		t.Error("没有计划却取到了一版")
	} else if !strings.Contains(err.Error(), model.ErrNotFound.Error()) {
		t.Errorf("错误 = %v，想要 model.ErrNotFound", err)
	}
}

// 两个工作的计划互不干扰。
func TestPlanRepo_ScopedByWork(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()
	repo := db.Plans()

	if err := repo.SavePlan(ctx, "work-a", planV1(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.LatestPlan(ctx, "work-b"); err == nil {
		t.Error("拿到了别的工作的计划")
	}
}
