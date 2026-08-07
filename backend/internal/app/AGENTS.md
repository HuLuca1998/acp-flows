# AGENTS.md · backend/internal/app

> **就近优先**。分层规则见 [`../../../docs/architecture.md`](../../../docs/spec/architecture.md) §3。

## 负责什么

**用例编排与事务边界。** 把领域规则、持久化、外部系统拼成一个可被 API 调用的操作。

```
app/
├── app.go        包门面：Application 结构体 + New()
├── port/       ★ 全部对外依赖的抽象，只有 interface，零实现
└── usecase/      按用例族分子包：work/ plan/ unit/ memory/ system/
```

## 不负责什么

- **不写业务规则。** 「状态能不能从 A 到 B」是 `domain` 的事
- **不碰具体实现。** 不 import `store` / `acp` / `gitx` / `ghx` 的具体类型
- **不拼 SQL。** 想拼的时候说明 port 接口设计错了，回去改接口
- **不做协议翻译。** HTTP 状态码、JSON 序列化是 `api` 层的事

## 依赖方向

| | |
|---|---|
| 允许 import | `domain/**` · `constant` · `util` · 自己的 `port` |
| 禁止 import | `store` `acp` `gitx` `ghx` `fsstore` `api` 的**具体类型**；任何第三方 ORM/HTTP 库 |

由 depguard 强制。**基础设施包反过来实现 `port` 的接口**（Go 是结构化类型，
它们甚至不需要 import `port`）。

## `port/` 的硬规则

- **只有 `interface` 和它们用到的领域类型，零 struct 实现**
- 接口要**小**。一个用例只依赖它真正需要的两三个方法，不要上帝接口
- 命名：持久化 `<聚合>Repo` · 外部系统 `<系统>Gateway` · 系统能力 `Clock` `IDGen` `Paths`
- **禁止** `IWorkRepo` / `WorkRepoInterface` / `WorkService`

> 小接口的直接收益：测试里的 fake 只要实现两三个方法。
> 巨型 fake 是假测试的温床——太难写，于是就去用 mock 框架，然后就 mock 喂 mock 了。

## 事务

- **事务边界在这里**，不在 `store`
- 通过 `context` 传递，repo 方法签名上看不到 `*gorm.DB`
- **事务里禁止做 IO**：调 ACP、跑 git、读写文件、发 HTTP。
  SQLite 只允许一个写者，在事务里做 IO 会卡死所有并发的 Work

## 测试

用例测试，port 用**手写 fake**（不是 mock 框架）。覆盖率门槛 **≥ 75%**。

手写 fake 的好处：它必须真的实现行为，写不出「返回我刚设的值」这种假测试。

## 检查命令

```bash
cd backend && go test ./internal/app/... -count=1 -race
```

## 本域特有的坑

- **`port` 包里出现 struct 实现** —— 立刻挪走，它会成为分层塌陷的起点
- **用例函数超过 60 行** —— 大概率把领域规则写进来了，挪回 `domain`
- **在 `app` 里判断 runtime 是 claude 还是 codex** —— 差异必须在 `acp/adapter` 内填平，
  这里只能查能力。`grep -rn 'codex\|claude'` 在本包必须为空
