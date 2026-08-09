package store_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
	"github.com/HuLuca1998/acp-flows/backend/internal/store"
)

// M8 U8.1.2 · 证据落库
//
// ★★ 证据被改写过就不再是证据了——「当时到底跑出了什么」没有第二个
// 地方可查。所以这个仓储没有 Update / Delete。

func diffEvidence(t *testing.T, id string, criteria ...string) model.Evidence {
	t.Helper()
	e, err := model.NewEvidence(id, "unit-013", model.EvidenceDiff, model.EvidenceFromApp,
		"3 个文件 +64 −12",
		"internal/acp/session.go\t+40\t−8\ninternal/app/work/cancel.go\t+24\t−4\n",
		criteria)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// ★★ 存进去再取出来：**原始输出一个字节都不少**，来源与关系也在。
func TestEvidenceRepo_RoundTrip(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()

	in := diffEvidence(t, "ev-441", "ac-1", "ac-2")
	if err := db.Evidence().SaveEvidence(ctx, "work-08", in); err != nil {
		t.Fatalf("存: %v", err)
	}

	got, err := db.Evidence().EvidenceOf(ctx, "work-08", "unit-013")
	if err != nil {
		t.Fatalf("取: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("取回 %d 条，想要 1 条", len(got))
	}
	// ★ 原始输出**原样**：截断过的输出在排查时等于没有
	if got[0].Body() != in.Body() {
		t.Errorf("原始输出被动过了：\n%q", got[0].Body())
	}
	// ★★ 来源必须过库：丢了的话一条 AI 转述会和真 diff 长得一样
	if !got[0].Trustworthy() {
		t.Error("来源没过库——用户判断「该不该信」全靠这一点")
	}
	if len(got[0].Criteria()) != 2 {
		t.Errorf("支持的标准 = %v——丢了的话它在「标准 ✓ ev-441」里不会出现，"+
			"用户以为它没派上用场", got[0].Criteria())
	}
}

// ★★ AI 转述的证据**过库之后还是 agent**。
func TestEvidenceRepo_KeepsAgentSource(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()

	e, err := model.NewEvidence("ev-442", "unit-013", model.EvidenceReview,
		model.EvidenceFromAgent, "审查意见", "看起来没问题", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Evidence().SaveEvidence(ctx, "work-08", e); err != nil {
		t.Fatal(err)
	}

	got, err := db.Evidence().EvidenceOf(ctx, "work-08", "unit-013")
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Trustworthy() {
		t.Error("AI 转述的证据过库之后变成了「应用采集」——" +
			"那正好是这一层最不该弄错的一件事")
	}
}

// ★★ 仓储**没有改写类方法**。
func TestEvidenceRepo_HasNoRewriteMethods(t *testing.T) {
	rt := reflect.TypeOf(&store.EvidenceRepo{})
	forbidden := []string{"update", "delete", "remove", "overwrite", "replace", "purge"}
	for i := range rt.NumMethod() {
		name := strings.ToLower(rt.Method(i).Name)
		for _, bad := range forbidden {
			if strings.Contains(name, bad) {
				t.Errorf("EvidenceRepo 有方法 %q——证据改写过就不是证据了",
					rt.Method(i).Name)
			}
		}
	}
}

// 还没有证据时是空不是错。
func TestEvidenceRepo_NoEvidenceYet(t *testing.T) {
	db := openTestStore(t)

	got, err := db.Evidence().EvidenceOf(context.Background(), "work-08", "unit-nope")
	if err != nil {
		t.Errorf("新单元报了错：%v", err)
	}
	if got == nil {
		t.Error("返回了 nil 而不是空切片")
	}
}

// 两个单元的证据互不干扰。
func TestEvidenceRepo_ScopedByUnit(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()

	if err := db.Evidence().SaveEvidence(ctx, "work-08", diffEvidence(t, "ev-441")); err != nil {
		t.Fatal(err)
	}
	other, err := model.NewEvidence("ev-442", "unit-999", model.EvidenceDiff,
		model.EvidenceFromApp, "别的单元", "x", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Evidence().SaveEvidence(ctx, "work-08", other); err != nil {
		t.Fatal(err)
	}

	got, err := db.Evidence().EvidenceOf(ctx, "work-08", "unit-013")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID() != "ev-441" {
		t.Errorf("拿到了别的单元的证据：%v", got)
	}
}
