package store_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
	"github.com/HuLuca1998/acp-flows/backend/internal/store"
)

// M7 U7.1.2 · 契约落库
//
// ★★ 契约冻结后不能改（INV-UC-2）：冻结的那一版被证据、验收、决策
// 引用着——改它就是改历史。

func contractV1(t *testing.T) *model.UnitContract {
	t.Helper()
	c := model.NewUnitContract("unit-012", 1)
	for _, crit := range []model.Criterion{
		{ID: "ac-1", Text: "取消必须幂等，连点两次只发一次协议取消"},
		{ID: "ac-2", Text: "取消后 diff 与最后事件游标可读"},
	} {
		if err := c.AddCriterion(crit.ID, crit.Text); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.SetBoundary(model.WriteBoundary{
		Allowed:   []string{"internal/acp/", "internal/app/work/"},
		Forbidden: []string{"internal/api/gen/"},
	}); err != nil {
		t.Fatal(err)
	}
	return c
}

// ★★ R1 · 存进去再取出来，**边界与标准逐字相同**，顺序也在。
func TestContractRepo_R1_RoundTrip(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()

	if err := db.Contracts().SaveContract(ctx, contractV1(t)); err != nil {
		t.Fatalf("存: %v", err)
	}

	got, err := db.Contracts().LatestContract(ctx, "unit-012")
	if err != nil {
		t.Fatalf("取: %v", err)
	}
	crits := got.Criteria()
	if len(crits) != 2 || crits[0].ID != "ac-1" {
		t.Fatalf("验收标准 = %v，顺序或内容变了", crits)
	}
	// ★ 标准里有逗号——用逗号当分隔符的话这一条会被拆成两条
	if !strings.Contains(crits[0].Text, "连点两次") {
		t.Errorf("标准的正文被拆坏了：%q", crits[0].Text)
	}

	// ★★ 边界要**原样**回来：判定全靠它，少一条就等于放宽了一次
	b := got.Boundary()
	if len(b.Allowed) != 2 || len(b.Forbidden) != 1 {
		t.Fatalf("边界 = %+v，条数不对", b)
	}
	if got.Judge("internal/acp/session.go") != model.BoundaryInside {
		t.Error("允许项没过库")
	}
	if got.Judge("internal/api/gen/api.gen.go") != model.BoundaryOutside {
		t.Error("禁止项没过库——被单独拎出来禁止的文件会被放行")
	}
}

// ★★ R2 · 改写**已冻结**的版本被拒，且库里那条一个字没变。
func TestContractRepo_R2_RefusesToRewriteFrozen(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()
	repo := db.Contracts()

	c := contractV1(t)
	if err := c.Freeze(); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveContract(ctx, c); err != nil {
		t.Fatal(err)
	}

	// 同一版，边界被放宽到全放行
	tampered := model.RestoreUnitContract("unit-012", 1, c.Criteria(),
		model.WriteBoundary{Allowed: []string{"/"}}, true)
	if err := repo.SaveContract(ctx, tampered); err == nil {
		t.Fatal("改写已冻结的契约却成功了——边界随时可以被放宽到全放行")
	}

	got, err := repo.LatestContract(ctx, "unit-012")
	if err != nil {
		t.Fatal(err)
	}
	if got.Judge("README.md") != model.BoundaryOutside {
		t.Error("边界被改了")
	}
}

// ★ 未冻结的是草稿，原地覆盖——单元设计师还在往里加标准。
func TestContractRepo_DraftIsUpdatedInPlace(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()
	repo := db.Contracts()

	c := contractV1(t)
	if err := repo.SaveContract(ctx, c); err != nil {
		t.Fatal(err)
	}
	if err := c.AddCriterion("ac-3", "停之后能接着干"); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveContract(ctx, c); err != nil {
		t.Fatalf("改草稿: %v", err)
	}

	got, err := repo.LatestContract(ctx, "unit-012")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Criteria()) != 3 {
		t.Errorf("标准 = %d 条，草稿的改动没落盘", len(got.Criteria()))
	}
	all, err := repo.ContractVersions(ctx, "unit-012")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Errorf("库里 %d 版，改草稿不该升版本号", len(all))
	}
}

// ★★ 整版重写时**旧条目要清干净**：留着的话删掉的那条标准会复活，
// 而用户以为自己去掉了它。
func TestContractRepo_DraftRewriteClearsRemovedItems(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()
	repo := db.Contracts()

	if err := repo.SaveContract(ctx, contractV1(t)); err != nil {
		t.Fatal(err)
	}

	// 同一版，只剩一条标准、边界也窄了
	slim := model.RestoreUnitContract("unit-012", 1,
		[]model.Criterion{{ID: "ac-1", Text: "取消必须幂等"}},
		model.WriteBoundary{Allowed: []string{"internal/acp/"}}, false)
	if err := repo.SaveContract(ctx, slim); err != nil {
		t.Fatal(err)
	}

	got, err := repo.LatestContract(ctx, "unit-012")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Criteria()) != 1 {
		t.Errorf("标准 = %d 条，删掉的那条复活了——用户以为自己去掉了它",
			len(got.Criteria()))
	}
	if len(got.Boundary().Allowed) != 1 || len(got.Boundary().Forbidden) != 0 {
		t.Errorf("边界 = %+v，旧的没清干净——那等于边界比用户以为的更宽",
			got.Boundary())
	}
}

// ★★ R3 · 版本链从新到旧，旧版原样留着。
func TestContractRepo_R3_KeepsEveryVersion(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()
	repo := db.Contracts()

	v1 := contractV1(t)
	if err := v1.Freeze(); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveContract(ctx, v1); err != nil {
		t.Fatal(err)
	}

	v2, err := v1.Revise(2)
	if err != nil {
		t.Fatal(err)
	}
	if err := v2.AddCriterion("ac-3", "停之后能接着干"); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveContract(ctx, v2); err != nil {
		t.Fatal(err)
	}

	all, err := repo.ContractVersions(ctx, "unit-012")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("版本 = %d 条，想要 2 条", len(all))
	}
	if all[0].Version() != 2 || all[1].Version() != 1 {
		t.Errorf("顺序 = v%d v%d，想要 v2 v1", all[0].Version(), all[1].Version())
	}
	if len(all[1].Criteria()) != 2 {
		t.Errorf("v1 的标准变了：%d 条", len(all[1].Criteria()))
	}
	if !all[1].IsFrozen() {
		t.Error("v1 的冻结状态被改了")
	}
}

// ★★ R4 · 仓储**没有改写类方法**（INV-UC-2）。
func TestContractRepo_R4_HasNoRewriteMethods(t *testing.T) {
	rt := reflect.TypeOf(&store.ContractRepo{})
	forbidden := []string{"update", "delete", "remove", "overwrite", "replace", "purge"}
	for i := range rt.NumMethod() {
		name := strings.ToLower(rt.Method(i).Name)
		for _, bad := range forbidden {
			if strings.Contains(name, bad) {
				t.Errorf("ContractRepo 有方法 %q——契约冻结后改不动（INV-UC-2）",
					rt.Method(i).Name)
			}
		}
	}
}

// 还没有契约时是空不是错——新单元的常态。
func TestContractRepo_NoContractYet(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()

	all, err := db.Contracts().ContractVersions(ctx, "unit-nope")
	if err != nil {
		t.Errorf("新单元报了错：%v", err)
	}
	if all == nil {
		t.Error("返回了 nil 而不是空切片")
	}
	if _, err := db.Contracts().LatestContract(ctx, "unit-nope"); err == nil {
		t.Error("没有契约却取到了一版")
	}
}
