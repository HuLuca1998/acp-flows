package reply_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/work/reply"
)

// M7 完成标志 1 · 单元设计师产出契约
//
// ★★ 契约由**单元设计师**产出，不是实现工程师自己写：自己给自己定验收
// 标准与边界等于没有边界——他会写一个刚好装得下自己想改的东西的范围。

const goodContractReply = "这个单元要动取消链路，边界收在 acp 与 work 两处。\n\n" +
	"```duet-contract\n" +
	`{
  "criteria": [
    {"id": "ac-1", "text": "连点两次取消只发一次协议取消请求"},
    {"id": "ac-2", "text": "取消后 diff 与最后事件游标可读"}
  ],
  "boundary": {
    "allowed": ["internal/acp/", "internal/app/work/"],
    "forbidden": ["internal/api/gen/"]
  }
}` + "\n```\n"

// ★★ 带围栏的回复解析出验收标准与边界，且**边界真的判得出来**。
func TestParseContractReply_ExtractsCriteriaAndBoundary(t *testing.T) {
	c, err := reply.ParseContractReply(goodContractReply, "unit-013", 1)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	if len(c.Criteria()) != 2 || c.Criteria()[0].ID != "ac-1" {
		t.Fatalf("验收标准 = %v", c.Criteria())
	}
	// ★ 判据落在**判定行为**上，不是「切片长度是 2」——
	// 后者只证明字段被填了，前者证明边界真的能用
	if c.Judge("internal/acp/session.go") != "in_boundary" {
		t.Error("允许项没生效")
	}
	if c.Judge("internal/api/gen/x.go") != "out_of_boundary" {
		t.Error("禁止项没生效")
	}
	// ★★ 产出的是**草稿不是冻结的**：冻结是用户的动作。
	// 自动冻结的话，用户还没看过这份契约，AI 就已经照着它改文件了
	if c.IsFrozen() {
		t.Error("契约一出来就冻结了——用户还没看过它")
	}
}

// ★★ 一条验收标准都没有 → 报错。
//
// 空契约冻结之后，「做完了」这件事没有任何判据——AI 说做完了就是做完了。
func TestParseContractReply_EmptyCriteriaIsRejected(t *testing.T) {
	agentSay := "```duet-contract\n" + `{"criteria":[],"boundary":{"allowed":["/"]}}` + "\n```"

	if _, err := reply.ParseContractReply(agentSay, "unit-013", 1); !errors.Is(err, reply.ErrNoContractInReply) {
		t.Errorf("空契约却过了：%v——「做完了」会变成 AI 说了算", err)
	}
}

// ★ 一段散文 → 报错并带上原话。
func TestParseContractReply_ProseSaysWhatItSaid(t *testing.T) {
	_, err := reply.ParseContractReply("我觉得这个单元主要是改取消链路……", "unit-013", 1)
	if !errors.Is(err, reply.ErrNoContractInReply) {
		t.Fatalf("散文却解析成功了：%v", err)
	}
	if !strings.Contains(err.Error(), "取消链路") {
		t.Errorf("错误里没有原话：%v", err)
	}
}

// ★★ 契约的围栏与计划的**分开**：一轮回复里可能同时讲到两者。
//
// 共用一个标记的话，抠出来的那段可能是另一样东西——
// 而那会变成一份验收标准是子计划标题的契约。
func TestParseContractReply_DoesNotPickUpAPlanFence(t *testing.T) {
	agentSay := "先看计划：\n```duet-plan\n" +
		`{"title":"t","subplans":[{"id":"s","title":"s","units":[]}]}` + "\n```"

	if _, err := reply.ParseContractReply(agentSay, "unit-013", 1); !errors.Is(err, reply.ErrNoContractInReply) {
		t.Errorf("把计划当成契约解析了：%v", err)
	}
}
