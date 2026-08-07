# CLAUDE.md · backend/internal/acp/fake

本目录的规则见同目录 [`AGENTS.md`](AGENTS.md)，仓库总纲见根 [`AGENTS.md`](/AGENTS.md)。

**不要在本文件里复制 `AGENTS.md` 的内容**——重复的内容必然漂移。
只有 Claude Code 独有的东西（skill、subagent、slash command）才写在这里。

## 本目录必用的 skill

- 写任何 `*_test.go` 前：先调全局 `go-unit-testing`，再看项目级 `go-unit-test`
- 开始一个开发单元：`tdd-unit`

## 改 Fake 之前先想清楚一件事

**你要加的这个行为，是「Agent 真会这么做」，还是「这样上层测试更好写」？**

只有前者该进来。后者的典型症状是给 Fake 加纠正、去重、补默认值——
每加一个，就有一条上层断言从此永远绿。

拿不准就去 `AGENTS.md` 看「Fake 如实记录它收到的一切」那一节。
