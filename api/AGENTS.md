# AGENTS.md · api

> **就近优先**：与根 [`AGENTS.md`](../AGENTS.md) 冲突时以本文件为准。

## 负责什么

`openapi.yaml` 是**前后端接口的唯一真源**。它定义 `duetd` 暴露的全部 HTTP 端点、
请求/响应 schema、错误格式，以及 SSE 事件信封。

后端的 handler 接口和前端的调用客户端**都由它生成**，不手写。

## 不负责什么

- **不定义 ACP 协议**。ACP 是 duetd 与 Runtime 子进程之间的 JSON-RPC，与本文件无关，
  见 [`../docs/spec/acp-integration.md`](../docs/spec/acp-integration.md)。
- **不定义数据库 schema**。DB 结构是 `backend/internal/store` 的私事，
  不得直接把表结构映射成 API 响应。
- **不放业务规则**。schema 里的 `enum` 只描述"合法取值"，不描述"什么时候能转成这个值"——
  那是状态机的事，见 [`../docs/spec/domain-model.md`](../docs/spec/domain-model.md)。

## 铁律：改接口的顺序

```
① 改 openapi.yaml   →   ② make gen   →   ③ 改 Go 实现 / TS 调用
```

**反过来做即视为不合规。** CI 上 `make check-gen` 会重新生成并比对 diff，
生成物与 spec 不一致就红。

前后端靠 spec 并行开发——spec 落后于代码，并行开发当场失效。

### 生成器与生成物

| | 工具 | 配置 | 产出（**不许手改**） |
|---|---|---|---|
| Go | `oapi-codegen` v2.4.1 | [`oapi-codegen.yaml`](oapi-codegen.yaml) | `backend/internal/api/gen/api.gen.go` |
| TS | `openapi-typescript` 7 | 无 | `frontend/src/api/gen/schema.d.ts` |

**版本写死在 `scripts/gen/gen-api.sh` 里，不用 `@latest`** ——
生成器版本一变生成物就变，`check-gen` 会在一个与本次改动完全无关的 PR 上炸。

生成物**进 git**：前端与 CI 直接读它，不能要求每个人先跑一遍生成器。

> ⚠️ `oapi-codegen` 会对 3.1 的 spec 打一条 WARNING。
> **不要因为这条警告把 spec 降级到 3.0** —— 我们用到的构造
> （object / string / enum / `$ref`）在两个版本里语义相同，实测生成正确。
> 脚本里已把这条警告过滤掉。真出问题的信号是「生成的 Go 编译不过」，脚本会单独检查。

### 生成物有没有用，靠一个编译期测试守着

`frontend/src/api/schema.contract.test.ts` 断言：枚举没退化成 `string`、
`required` 没被丢、`Runtime.name` 没被 examples 误判成封闭枚举。

**它靠 `@ts-expect-error`**：类型一放宽，那行就不再报错，
tsc 转而报「未使用的 directive」——**编译不过**。
所以 `vitest` 跑绿不代表通过，真正的门是 `make lint-frontend`。

## 约定

| 项 | 约定 |
|---|---|
| 版本前缀 | 全部端点在 `/v1` 下 |
| 命名 | 路径用复数名词 kebab-case（`/v1/works/{workId}/units`）；字段用 `snake_case` |
| ID | 字符串，带类型前缀（`work-08` `unit-012` `mem-203` `ck-07`），**不用裸整数** |
| 时间 | RFC3339 带毫秒与时区（`2026-08-07T03:12:44.512Z`） |
| 分页 | cursor 分页：`?cursor=&limit=`，响应带 `next_cursor` |
| 错误 | 统一 `Problem` 对象：`{type, title, status, detail, params, instance}`（RFC 9457） |
| **错误文案** | ★ `type` 是机器可读错误码（`snake_case`），前端据此查词条；`detail` 只给开发者看，界面不展示；`params` 供词条插值。**后端绝不返回用户可见的中文文案**，见 [`../docs/rules/i18n.md`](../docs/rules/i18n.md) §3 |
| 长任务 | 一律 `202 Accepted` + 事件流推进，**不做长轮询** |
| 鉴权 | 全部端点要求 `Authorization: Bearer <token>`，无 token 一律 401 |

## SSE 单独约定

SSE 无法用 OpenAPI 完整表达。做法是：

- 端点本身在 spec 里声明（`text/event-stream`）
- 事件 payload 的 schema 定义在 `components/schemas/Event*` 下，
  在端点描述里用 `x-sse-events` 扩展字段列出该端点会推哪些 `type`
- 前端从这些 schema 生成判别联合（discriminated union），`type` 是判别字段

13 类事件是**封闭枚举**，新增一类需要同时改：本文件 + `docs/spec/architecture.md` §4 +
`design/Duet Spec.dc.html` 第 07 节 + 前端事件渲染器注册表。**四处缺一不可。**

## 检查命令

```bash
make gen                              # 重新生成前后端代码
make check-gen                        # 校验生成物与 spec 一致（CI 用）
npx @redocly/cli lint api/openapi.yaml   # spec 自身的规范性
```

## 改这里之前必读

- [`../docs/spec/architecture.md`](../docs/spec/architecture.md) §4 事件流 —— 事件信封与 13 类枚举
- [`../docs/spec/domain-model.md`](../docs/spec/domain-model.md) —— 别把领域概念翻译错
- [`../docs/rules/testing-strategy.md`](../docs/rules/testing-strategy.md) §契约测试 —— spec 即测试基准

## 本域特有的坑

- **别把 SQLite 表结构直接抄成 API schema。** API 是给前端用的视图，
  DB 是持久化细节，两者会各自演化。
- **别用裸整数 ID。** 设计规范要求界面上大量展示等宽 ID（`unit-012`），
  用整数会导致前端到处拼字符串。
- **`enum` 一旦发布就很难删。** 加值容易删值难，新增前想清楚。
- **状态词不要翻译，也不要改写。** `waiting_user` 就是 `waiting_user`，
  前端会原样等宽显示。
