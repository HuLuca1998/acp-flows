package store_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
	"github.com/HuLuca1998/acp-flows/backend/internal/store"
)

// M9 U9.1.2 · 决策落库
//
// ★★ 答过的决策**一个字都不能改**：改了的话「他当时选了什么」就没有答案，
// 而后面几十个文件的改动都是照着那个选择做的。

func newDecision(t *testing.T) *model.Decision {
	t.Helper()
	d, err := model.NewDecision("dec-01", "work-08", "unit-013", model.DecisionD2,
		"取消之后要不要回滚已写入的文件？",
		[]model.DecisionOption{
			{ID: "a", Text: "不回滚，仅停止", Impact: "已写入的文件留着，下次从这里接着干"},
			{ID: "b", Text: "回退到检查点", Impact: "回到 ck-07，这一轮的三个文件改动会没有"},
		}, "a")
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// ★★ R1 · 存进去再取出来，**选项与影响说明都在**。
func TestDecisionRepo_R1_RoundTrip(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()

	if err := db.Decisions().SaveDecision(ctx, newDecision(t)); err != nil {
		t.Fatalf("存: %v", err)
	}

	got, err := db.Decisions().FindDecision(ctx, "dec-01")
	if err != nil {
		t.Fatalf("取: %v", err)
	}
	opts := got.Options()
	if len(opts) != 2 || opts[0].ID != "a" {
		t.Fatalf("选项 = %v，顺序或内容变了", opts)
	}
	// ★★ 影响说明必须过库：丢了的话用户在盲选——
	// 他看到两个名字，而不知道选哪个会发生什么
	if !strings.Contains(opts[1].Impact, "ck-07") {
		t.Errorf("影响说明没过库：%q——用户会在盲选", opts[1].Impact)
	}
	if got.Recommended() != "a" {
		t.Errorf("推荐 = %q", got.Recommended())
	}
	if got.Answered() {
		t.Error("刚存的决策却是已答的")
	}
}

// ★★ R2 · 作答**只能一次**，第二次被拒且库里那条答案没变。
func TestDecisionRepo_R2_AnswersOnlyOnce(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()
	repo := db.Decisions()

	d := newDecision(t)
	if err := repo.SaveDecision(ctx, d); err != nil {
		t.Fatal(err)
	}
	if err := d.Answer("b"); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveDecision(ctx, d); err != nil {
		t.Fatalf("作答没存下：%v", err)
	}

	// 换一个答案再存
	tampered := model.RestoreDecision("dec-01", "work-08", "unit-013", model.DecisionD2,
		"?", d.Options(), "a", "a")
	if err := repo.SaveDecision(ctx, tampered); !errors.Is(err, model.ErrDecisionAnswered) {
		t.Fatalf("改答案却成功了：%v", err)
	}

	got, err := repo.FindDecision(ctx, "dec-01")
	if err != nil {
		t.Fatal(err)
	}
	if got.AnsweredWith() != "b" {
		t.Errorf("答案被改成了 %q——「他当时选了什么」就没有答案了", got.AnsweredWith())
	}
}

// ★★ R3 · 未作答的列得出来——**左栏那个亮蓝点靠它**。
//
// 不列的话，用户不知道有件事在等他，而工作停在 `waiting_user` 永远不动。
func TestDecisionRepo_R3_PendingOnly(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()
	repo := db.Decisions()

	if err := repo.SaveDecision(ctx, newDecision(t)); err != nil {
		t.Fatal(err)
	}
	answered := model.RestoreDecision("dec-02", "work-08", "", model.DecisionD2,
		"另一件事", []model.DecisionOption{{ID: "x", Text: "x", Impact: "x"}}, "", "x")
	if err := repo.SaveDecision(ctx, answered); err != nil {
		t.Fatal(err)
	}

	pending, err := repo.PendingDecisions(ctx, "work-08")
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].ID() != "dec-01" {
		t.Errorf("待决策 = %d 条——答过的混进来的话，"+
			"用户会一直看到一个点不掉的提醒", len(pending))
	}
}

// ★★ R4 · 仓储**没有 Delete**。
//
// 决策删得掉的话，「他当时被问了什么」也可以被抹掉。
func TestDecisionRepo_R4_HasNoDeleteMethod(t *testing.T) {
	rt := reflect.TypeOf(&store.DecisionRepo{})
	forbidden := []string{"delete", "remove", "purge", "overwrite"}
	for i := range rt.NumMethod() {
		name := strings.ToLower(rt.Method(i).Name)
		for _, bad := range forbidden {
			if strings.Contains(name, bad) {
				t.Errorf("DecisionRepo 有方法 %q——「他当时被问了什么」不该能被抹掉",
					rt.Method(i).Name)
			}
		}
	}
}

// 没有待决策时返回空切片不是错——那是常态。
func TestDecisionRepo_NoPending(t *testing.T) {
	db := openTestStore(t)

	got, err := db.Decisions().PendingDecisions(context.Background(), "work-none")
	if err != nil {
		t.Errorf("报了错：%v", err)
	}
	if got == nil {
		t.Error("返回了 nil 而不是空切片")
	}
}
