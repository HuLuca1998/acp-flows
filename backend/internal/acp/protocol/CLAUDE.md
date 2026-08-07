# CLAUDE.md · backend/internal/acp/protocol

本目录的规则见同目录 [`AGENTS.md`](AGENTS.md)，仓库总纲见根 [`AGENTS.md`](/AGENTS.md)。

**不要在本文件里复制 `AGENTS.md` 的内容**——重复的内容必然漂移。
只有 Claude Code 独有的东西（skill、subagent、slash command）才写在这里。

## 本目录必用的 skill

- 写任何 `*_test.go` 前：先调全局 `go-unit-testing`，再看项目级 `go-unit-test`
- 开始一个开发单元：`tdd-unit`

## 改协议类型前先复核事实

本包的每个字段名都能在官方 schema 里找到出处。**不要凭记忆改**：

```bash
SDK=$(npm root -g)/@agentclientprotocol/codex-acp/node_modules/@agentclientprotocol/sdk
grep -o 'sessionUpdate: "[a-z_]*"' $SDK/dist/schema/types.gen.d.ts | sort -u
```

拿不到 SDK（换机器、包被删）时**停下来问人**，不要按 Duet 文档里的描述写——
那些数字已经漂移过一次了（见 `AGENTS.md` 的坑）。
