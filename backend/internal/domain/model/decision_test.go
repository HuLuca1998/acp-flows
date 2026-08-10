package model_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// M9 U9.1.1 · 决策
//
// ★★ 这一族守的是 Duet 的底线：**决定权在用户手里**。
// AI 自己选一个往下走的话，用户是在几十个文件之后才发现「它怎么这么做了」。

func twoOptions() []model.DecisionOption {
	return []model.DecisionOption{
		{ID: "a", Text: "不回滚，仅停止", Impact: "已写入的文件留着，下次从这里接着干"},
		{ID: "b", Text: "回退到检查点", Impact: "回到 ck-07，这一轮的三个文件改动会没有"},
	}
}

func mustDecision(t *testing.T, opts []model.DecisionOption, recommended string) *model.Decision {
	t.Helper()
	d, err := model.NewDecision("dec-01", "work-08", "unit-013", model.DecisionD2,
		"取消之后要不要回滚已写入的文件？", opts, recommended)
	if err != nil {
		t.Fatalf("造决策: %v", err)
	}
	return d
}

// ★★ R1 · 等级封闭 D0–D3，且 **D2/D3 必须问用户**。
func TestDecision_R1_LevelIsClosed(t *testing.T) {
	for _, l := range []model.DecisionLevel{
		model.DecisionD0, model.DecisionD1, model.DecisionD2, model.DecisionD3,
	} {
		if !l.IsValid() {
			t.Errorf("%q 被判成了非法等级", l)
		}
	}
	if _, err := model.NewDecision("dec-01", "work-08", "", "D4",
		"?", twoOptions(), ""); !errors.Is(err, model.ErrUnknownDecisionLevel) {
		t.Errorf("D4 却收下了：%v", err)
	}

	// ★★ D2/D3 必须问用户——AI 自己定的话，用户是在几十个文件之后才发现
	if model.DecisionD0.NeedsUser() || model.DecisionD1.NeedsUser() {
		t.Error("D0/D1 被判成了必须问用户——那样用户会被烦死")
	}
	if !model.DecisionD2.NeedsUser() || !model.DecisionD3.NeedsUser() {
		t.Error("D2/D3 没被判成必须问用户——那是「改变外部行为」与「回滚已验收的东西」")
	}
}

// ★★ R2 · **至少两个选项**才成其为决策。
//
// 一个选项的「决策」不是在问，是在通知——而通知不该占用用户
// 「停下来做个决定」的注意力。
func TestDecision_R2_NeedsAtLeastTwoOptions(t *testing.T) {
	one := []model.DecisionOption{{ID: "a", Text: "就这么办", Impact: "会怎样"}}

	_, err := model.NewDecision("dec-01", "work-08", "", model.DecisionD2, "?", one, "")
	if !errors.Is(err, model.ErrNotEnoughOptions) {
		t.Fatalf("一个选项也成了决策：%v——那不是在问，是在通知", err)
	}
	if _, err := model.NewDecision("dec-01", "work-08", "", model.DecisionD2,
		"?", nil, ""); !errors.Is(err, model.ErrNotEnoughOptions) {
		t.Errorf("零个选项也成了决策：%v", err)
	}
}

// ★★ R4 · 每个选项都要写明**影响**。
//
// 没有影响说明的选项，用户是在盲选：他看到三个名字，
// 而不知道选哪个会发生什么。
func TestDecision_R4_EveryOptionNeedsImpact(t *testing.T) {
	opts := twoOptions()
	opts[1].Impact = "  "

	_, err := model.NewDecision("dec-01", "work-08", "", model.DecisionD2, "?", opts, "")
	if !errors.Is(err, model.ErrOptionNeedsImpact) {
		t.Fatalf("没有影响说明的选项却收下了：%v——用户会在盲选", err)
	}
	if !strings.Contains(err.Error(), "b") {
		t.Errorf("错误里没说是哪个选项：%v", err)
	}
}

// ★★ R3 · 「推荐」必须指向**存在的**选项。
func TestDecision_R3_RecommendationMustExist(t *testing.T) {
	_, err := model.NewDecision("dec-01", "work-08", "", model.DecisionD2,
		"?", twoOptions(), "不存在的")
	if !errors.Is(err, model.ErrUnknownRecommendation) {
		t.Fatalf("推荐指向了不存在的选项：%v——界面上那个标记会落在谁身上说不清", err)
	}

	// ★ 空推荐是允许的：AI 也可以拿不准
	if _, err := model.NewDecision("dec-01", "work-08", "", model.DecisionD2,
		"?", twoOptions(), ""); err != nil {
		t.Errorf("AI 拿不准时不给推荐却被拒了：%v", err)
	}
}

// ★★ R5 · **只能答一次**。
//
// 答过还能改的话，「他当时选了什么」就没有答案——
// 而后面几十个文件的改动都是照着那个选择做的。
func TestDecision_R5_AnswersOnlyOnce(t *testing.T) {
	d := mustDecision(t, twoOptions(), "a")

	if d.Answered() {
		t.Error("刚造出来就是答过的")
	}
	if err := d.Answer("b"); err != nil {
		t.Fatal(err)
	}
	if d.AnsweredWith() != "b" {
		t.Errorf("答案 = %q", d.AnsweredWith())
	}

	err := d.Answer("a")
	if !errors.Is(err, model.ErrDecisionAnswered) {
		t.Fatalf("答了第二次：%v", err)
	}
	// ★ 判据：**答案没被改掉**
	if d.AnsweredWith() != "b" {
		t.Errorf("第二次作答改掉了答案：%q", d.AnsweredWith())
	}
}

// ★ 选一个不存在的选项要报错，**不静默收下**。
//
// 收下的话，「他选了什么」会变成一个谁都不认识的字符串。
func TestDecision_RejectsUnknownOption(t *testing.T) {
	d := mustDecision(t, twoOptions(), "")

	if err := d.Answer("根本没有这个"); err == nil {
		t.Fatal("选了不存在的选项却成功了")
	}
	if d.Answered() {
		t.Error("被拒之后却成了已答")
	}
}

// ★★ R5 · 决策**不可变**（除了作答那一次）。
func TestDecision_R5_IsImmutable(t *testing.T) {
	rt := reflect.TypeOf(&model.Decision{})
	allowed := map[string]bool{
		"ID": true, "WorkID": true, "UnitID": true, "Level": true,
		"Question": true, "Options": true, "Recommended": true,
		"AnsweredWith": true, "Answered": true,
		// 唯一的受控迁移
		"Answer": true,
	}
	for i := range rt.NumMethod() {
		if name := rt.Method(i).Name; !allowed[name] {
			t.Errorf("Decision 多了一个方法 %q——"+
				"答过还能改的话，「他当时选了什么」就没有答案", name)
		}
	}

	d := mustDecision(t, twoOptions(), "a")
	opts := d.Options()
	opts[0].Text = "篡改"
	if d.Options()[0].Text == "篡改" {
		t.Error("Options() 返回了内部切片")
	}
}

// ★ R6 · 「稍后决定」不改变现场：没答的决策照样查得到。
func TestDecision_R6_UnansweredStaysQueryable(t *testing.T) {
	d := mustDecision(t, twoOptions(), "a")

	if d.Answered() {
		t.Error("没答却算答过了")
	}
	if len(d.Options()) != 2 || d.Recommended() != "a" {
		t.Error("没答的决策丢了内容——用户回头点开时看到的会是半张卡片")
	}
}
