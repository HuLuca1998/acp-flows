package work_test

// AI 的回复样本。
//
// ★ 与 `reply` 包里那两份**各自独立**：那边测的是解析器本身，这边测的是
// 「这条链路通了」。共用一份的话，改解析器的测试会连带打断这边——
// 而两者本来就该各自演化。

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
