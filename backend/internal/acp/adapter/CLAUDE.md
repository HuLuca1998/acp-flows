# CLAUDE.md · backend/internal/acp/adapter

本目录的规则见同目录 [`AGENTS.md`](AGENTS.md)，仓库总纲见根 [`AGENTS.md`](../../../../AGENTS.md)。

**不要在本文件里复制 `AGENTS.md` 的内容**——重复的内容必然漂移。

## 本目录必用的 skill

- 写任何 `*_test.go` 前：先调全局 `go-unit-testing`，再看项目级 `go-unit-test`
- 开始一个开发单元：`tdd-unit`

## 改之前先查实测记录

两端的真实字段形态在 `docs/notes/acp-field-notes.md` §7.1。
**那是实测出来的，不是文档抄的**——照文档写的代码在真机上会撞坑。
