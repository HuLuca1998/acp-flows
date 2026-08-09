package work_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/work"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// M6 U6.2.1 R6 R7 · 把 AI 的回复变成结构化的计划
//
// ★★ 解析不出来时**把原话给用户看**。静默失败的话，用户看到「正在规划」
// 然后永远没有下文——而真正的原因（它输出了一段散文）躺在没人读的地方。

const goodReply = "我把它拆成一个子计划、两个单元。\n\n" +
	"```duet-plan\n" +
	`{
  "title": "取消运行中的 Agent turn",
  "subplans": [
    {
      "id": "subplan-01",
      "title": "ACP Runtime 抽象层",
      "units": [
        {"id": "unit-012", "title": "取消协议", "role_id": "implementer", "depends_on": []},
        {"id": "unit-013", "title": "取消后证据读取", "role_id": "unit_reviewer",
         "depends_on": ["unit-012"]}
      ]
    }
  ]
}` + "\n```\n"

// ★★ R6 · 带围栏的回复解析得出子计划与单元，**角色也在**。
func TestParsePlanReply_R6_ExtractsTheGraph(t *testing.T) {
	v, err := work.ParsePlanReply(goodReply, 1)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	if v.Version() != 1 {
		t.Errorf("版本 = v%d，想要 v1——版本号由我们给，不让 AI 猜", v.Version())
	}
	if n, m := v.Counts(); n != 1 || m != 2 {
		t.Fatalf("计数 = %d · %d，想要 1 · 2", n, m)
	}

	units := v.Subplans()[0].Units()
	if units[1].RoleID() != "unit_reviewer" {
		t.Errorf("角色 = %q，想要 unit_reviewer", units[1].RoleID())
	}
	if deps := units[1].DependsOn(); len(deps) != 1 || deps[0] != "unit-012" {
		t.Errorf("依赖 = %v", deps)
	}
}

// ★★ R7 · 一段散文（没有围栏）→ 报错，且**带上它的原话**。
func TestParsePlanReply_R7_PlainProseSaysWhatItSaid(t *testing.T) {
	const prose = "我觉得这个需求可以分成三部分来做，首先是协议层，然后是……"

	_, err := work.ParsePlanReply(prose, 1)
	if !errors.Is(err, work.ErrNoPlanInReply) {
		t.Fatalf("散文却解析成功了：%v", err)
	}
	// ★ 判据：错误里能看到**它到底说了什么**
	if !strings.Contains(err.Error(), "协议层") {
		t.Errorf("错误里没有原话，用户看不出它说了什么：%v", err)
	}
}

// 围栏里是坏 JSON → 报错并带上那段内容。
func TestParsePlanReply_BadJSONSaysSo(t *testing.T) {
	reply := "```duet-plan\n{这不是 JSON}\n```"

	_, err := work.ParsePlanReply(reply, 1)
	if !errors.Is(err, work.ErrNoPlanInReply) {
		t.Fatalf("坏 JSON 却过了：%v", err)
	}
	if !strings.Contains(err.Error(), "这不是 JSON") {
		t.Errorf("错误里没有那段内容：%v", err)
	}
}

// ★★ 派了个**不存在的角色** → 当场报错（裁定三），且说清是哪个单元。
//
// 放行的话，那个单元到执行时才发现没人认领，而用户已经等了几分钟。
func TestParsePlanReply_UnknownRoleIsRejected(t *testing.T) {
	reply := "```duet-plan\n" + `{
  "title": "t",
  "subplans": [{"id": "subplan-01", "title": "s", "units": [
    {"id": "unit-012", "title": "u", "role_id": "senior_vibe_coder", "depends_on": []}
  ]}]
}` + "\n```"

	_, err := work.ParsePlanReply(reply, 1)
	if !errors.Is(err, model.ErrUnknownRole) {
		t.Fatalf("不存在的角色却过了：%v", err)
	}
	for _, want := range []string{"unit-012", "senior_vibe_coder"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("错误里没有 %q：%v", want, err)
		}
	}
}

// ★★ 依赖成环 → 当场报错，且把环列出来。
func TestParsePlanReply_CycleIsRejected(t *testing.T) {
	reply := "```duet-plan\n" + `{
  "title": "t",
  "subplans": [{"id": "subplan-01", "title": "s", "units": [
    {"id": "unit-a", "title": "a", "role_id": "implementer", "depends_on": ["unit-b"]},
    {"id": "unit-b", "title": "b", "role_id": "implementer", "depends_on": ["unit-a"]}
  ]}]
}` + "\n```"

	_, err := work.ParsePlanReply(reply, 1)
	if !errors.Is(err, model.ErrDependencyCycle) {
		t.Fatalf("成环却过了：%v", err)
	}
}

// ★★ **取最后一段围栏**：AI 常常先贴一段示例再给真计划。
//
// 取第一段的话，示例会被当成真计划——而用户会得到一份写着
// 「取消运行中的 Agent turn」但内容是我们自己例子的计划。
func TestParsePlanReply_TakesTheLastFence(t *testing.T) {
	reply := "先给你看个格式：\n```duet-plan\n" + `{"title":"示例","subplans":[
{"id":"subplan-99","title":"示例子计划","units":[
{"id":"unit-999","title":"示例单元","role_id":"implementer","depends_on":[]}]}]}` +
		"\n```\n\n下面是真的：\n\n" + goodReply

	v, err := work.ParsePlanReply(reply, 1)
	if err != nil {
		t.Fatal(err)
	}
	if v.Title() != "取消运行中的 Agent turn" {
		t.Errorf("标题 = %q，取到了示例那一段——"+
			"用户会得到一份内容是我们自己例子的计划", v.Title())
	}
}

// 围栏没闭合（流式输出被截断）→ 当成没有，不去解析半截 JSON。
func TestParsePlanReply_UnclosedFence(t *testing.T) {
	reply := "```duet-plan\n{\"title\":\"半截"

	_, err := work.ParsePlanReply(reply, 1)
	if !errors.Is(err, work.ErrNoPlanInReply) {
		t.Errorf("半截围栏却过了：%v", err)
	}
}

// 一个子计划都没有 → 报错。空计划会让用户以为 AI 什么都没规划出来。
func TestParsePlanReply_EmptyPlanIsRejected(t *testing.T) {
	reply := "```duet-plan\n" + `{"title":"t","subplans":[]}` + "\n```"

	if _, err := work.ParsePlanReply(reply, 1); !errors.Is(err, work.ErrNoPlanInReply) {
		t.Errorf("空计划却过了：%v", err)
	}
}

// ★ 错误里的原话按**字符**截断，不按字节——按字节截会把中文切成乱码。
func TestParsePlanReply_TruncatesByRunes(t *testing.T) {
	_, err := work.ParsePlanReply(strings.Repeat("需求分析中", 200), 1)
	if err == nil {
		t.Fatal("该报错")
	}
	if strings.Contains(err.Error(), "�") {
		t.Errorf("原话被按字节切成了乱码：%v", err)
	}
}
