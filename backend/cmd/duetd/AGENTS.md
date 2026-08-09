# AGENTS.md · backend/cmd/duetd

> 本目录的规则。**就近优先**：与根 [`AGENTS.md`](../../../AGENTS.md) 冲突时以本文件为准。

## 负责什么

**唯一做装配的地方。** 把 store、eventbus、acp、app、api 这几层接起来，
起 HTTP 服务。

★ 这里也是**类型翻译层**的家：`store.Event` 与 `eventbus.Event` 字段一致
但**具名结构体之间不能互相赋值**（Go 的结构化类型只对 interface 生效），
而 depguard 不许基础设施互相 import——所以接缝只能落在这里。

## 不负责什么

| 不在这里 | 去哪 | 为什么 |
|---|---|---|
| 任何业务判断 | `internal/app/**` | 装配层做判断的话，它永远没有测试 |
| HTTP handler | `internal/api/**` | 这里只 `NewRouter(cfg)` |
| 领域规则 | `internal/domain/model` | 同第一条 |

## 依赖方向

| | |
|---|---|
| 允许 import | 所有 `internal/**`（它是唯一能横跨全部层的地方） |
| 禁止 import | 无 —— 但**别把逻辑搬进来**：这里的代码最难测 |

## 检查命令

```bash
go test ./cmd/...      # 装配层的测试
go build ./...         # 编译
```

## 改这里之前必读

- [`docs/rules/coding-standards.md`](../../../docs/rules/coding-standards.md) 的依赖方向一节

## 本域特有的坑

- ★★ **逐字段手抄的翻译层一定会漏。** 2026-08-09 真机撞到过：给事件加了
  `role` / `requirement_version` 五个字段，domain、store、eventbus 三处都
  改了，**唯独 `eventStore` 没抄**——事件照样落库、照样读得回来，只是角色
  标签没了，而所有单测都绿（它们不过这一层）。
  **加字段时这里是第四处**，`TestEventStore_FieldCountsMatch` 守着它。
- ★ 这一层的代码**最难写测试**（它要真起服务），所以任何能往下推的判断
  都往下推。推不动的，至少用反射守住结构性的东西（字段数、字段名）。
