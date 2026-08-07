# CLAUDE.md · backend/internal/store/entity

本目录的规则见同目录 [`AGENTS.md`](AGENTS.md)，仓库总纲见根 [`AGENTS.md`](../../../../AGENTS.md)。

**不要在本文件里复制 `AGENTS.md` 的内容**——重复的内容必然漂移。

## 本目录必用的 skill

- **要连数据库看数据 / 排查「数据对不上」**：`db-operate`
- 写任何 `*_test.go` 前：先调全局 `go-unit-testing`，再看项目级 `go-unit-test`
