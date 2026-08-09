package model_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// M8 U8.1.1 · 证据
//
// ★★ 这一族守的是「证据由应用采集」。让 AI 报告自己干了什么，等于让被
// 考核的人填自己的考勤表——它不需要撒谎，只需要「记错了」一次，
// 用户就再也不知道该信哪一条。

func mustEvidence(t *testing.T, id string, src model.EvidenceSource, criteria ...string) model.Evidence {
	t.Helper()
	e, err := model.NewEvidence(id, "unit-013", model.EvidenceDiff, src,
		"3 个文件 +64 −12", "diff --git a/x b/x\n…", criteria)
	if err != nil {
		t.Fatalf("造证据 %s: %v", id, err)
	}
	return e
}

// ★★ R1 · 四类封闭，第五类被拒。
func TestEvidence_R1_KindIsClosed(t *testing.T) {
	for _, k := range []model.EvidenceKind{
		model.EvidenceDiff, model.EvidenceTest, model.EvidenceCommand, model.EvidenceReview,
	} {
		if !k.IsValid() {
			t.Errorf("%q 被判成了非法类别", k)
		}
	}

	_, err := model.NewEvidence("ev-1", "unit-013", "screenshot", model.EvidenceFromApp, "", "", nil)
	if !errors.Is(err, model.ErrUnknownEvidenceKind) {
		t.Errorf("第五类却收下了：%v", err)
	}
}

// ★★ R3 · 来源**必填**。
//
// 留空的话，一条 AI 转述会和一份应用采集的 diff 长得一样——
// 而用户判断「该不该信」全靠这一个字段。
func TestEvidence_R3_SourceIsRequired(t *testing.T) {
	_, err := model.NewEvidence("ev-1", "unit-013", model.EvidenceDiff, "", "", "", nil)
	if !errors.Is(err, model.ErrEvidenceSourceRequired) {
		t.Fatalf("没写来源却造出来了：%v——一条 AI 转述会和真 diff 长得一样", err)
	}

	_, err = model.NewEvidence("ev-1", "unit-013", model.EvidenceDiff, "someone_else", "", "", nil)
	if !errors.Is(err, model.ErrEvidenceSourceRequired) {
		t.Errorf("认不出的来源却收下了：%v", err)
	}
}

// ★★ R4 · AI 转述的与应用采集的**分得开**。
func TestEvidence_R4_AgentReportsAreMarked(t *testing.T) {
	fromApp := mustEvidence(t, "ev-441", model.EvidenceFromApp)
	fromAgent := mustEvidence(t, "ev-442", model.EvidenceFromAgent)

	if !fromApp.Trustworthy() {
		t.Error("应用采集的被判成了不可信")
	}
	if fromAgent.Trustworthy() {
		t.Error("AI 转述的被判成了可信——它会和真 diff 混在一起，" +
			"而用户判断「该不该信」全靠这一点")
	}
}

// ★★ R2 · 证据**不可变**。
//
// 被改写过的话它就不再是证据了——「当时到底跑出了什么」没有第二个地方可查。
func TestEvidence_R2_IsImmutable(t *testing.T) {
	rt := reflect.TypeOf(&model.Evidence{})
	allowed := map[string]bool{
		"ID": true, "UnitID": true, "Kind": true, "Source": true,
		"Summary": true, "Body": true, "Criteria": true, "Trustworthy": true,
	}
	for i := range rt.NumMethod() {
		if name := rt.Method(i).Name; !allowed[name] {
			t.Errorf("Evidence 多了一个方法 %q——证据被改写过就不再是证据了", name)
		}
	}

	e := mustEvidence(t, "ev-441", model.EvidenceFromApp, "ac-1")
	cs := e.Criteria()
	cs[0] = "篡改"
	if e.Criteria()[0] == "篡改" {
		t.Error("Criteria() 返回了内部切片")
	}
}

// ★ 原始输出**原样存**，不截断不美化——截断过的输出在排查时等于没有。
func TestEvidence_KeepsTheRawBody(t *testing.T) {
	const raw = "=== RUN   TestFoo\n--- FAIL: TestFoo (0.00s)\n    foo_test.go:12: 想要 3，拿到 2\n"
	e, err := model.NewEvidence("ev-441", "unit-013", model.EvidenceTest,
		model.EvidenceFromApp, "1 个测试挂了", raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	if e.Body() != raw {
		t.Errorf("原始输出被动过了：\n%s", e.Body())
	}
}

// ★★ R5 · 一条证据可以支持多条标准，一条标准也可以有多条证据。
func TestEvidence_R5_ManyToMany(t *testing.T) {
	criteria := []model.Criterion{
		{ID: "ac-1", Text: "取消必须幂等"},
		{ID: "ac-2", Text: "现场证据可读"},
		{ID: "ac-3", Text: "停之后能接着干"},
	}
	evidence := []model.Evidence{
		// 一条证据支持两条标准
		mustEvidence(t, "ev-441", model.EvidenceFromApp, "ac-1", "ac-2"),
		// 一条标准有两条证据
		mustEvidence(t, "ev-442", model.EvidenceFromApp, "ac-2"),
	}

	cov := model.CriteriaCoverage(criteria, evidence)
	if len(cov["ac-1"]) != 1 || cov["ac-1"][0] != "ev-441" {
		t.Errorf("ac-1 的证据 = %v", cov["ac-1"])
	}
	if len(cov["ac-2"]) != 2 {
		t.Errorf("ac-2 的证据 = %v，想要两条——"+
			"只标第一条的话用户以为另一条没派上用场", cov["ac-2"])
	}
	// ★★ 没有证据的那条**留在结果里**，值是空切片。
	// 去掉的话调用方会以为「所有标准都有证据」，
	// 而实际上它们根本没出现在这张表里。
	got, ok := cov["ac-3"]
	if !ok {
		t.Fatal("没有证据的标准被从结果里去掉了——调用方会以为所有标准都有证据")
	}
	if len(got) != 0 {
		t.Errorf("ac-3 凭空有了证据：%v", got)
	}
}

// ★★ 证据指向一条**契约里没有的**标准时不计入。
//
// 静静收下的话，「已覆盖 5 条」里会有一条根本不在契约里。
func TestCriteriaCoverage_IgnoresUnknownCriteria(t *testing.T) {
	criteria := []model.Criterion{{ID: "ac-1", Text: "取消必须幂等"}}
	evidence := []model.Evidence{
		mustEvidence(t, "ev-441", model.EvidenceFromApp, "ac-1", "ac-999"),
	}

	cov := model.CriteriaCoverage(criteria, evidence)
	if len(cov) != 1 {
		t.Errorf("覆盖表里有 %d 条，想要 1 条——"+
			"契约里没有的标准混进来的话，「已覆盖」这个数就是假的", len(cov))
	}
}
