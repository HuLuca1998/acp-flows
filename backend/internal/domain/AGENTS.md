# AGENTS.md · backend/internal/domain

> **就近优先**。这一层的规矩最严，因为它是整套测试策略的地基。

## 负责什么

产品的**业务规则本身**。零 IO、零框架、纯 Go。

```
domain/
├── model/      ★ 聚合、实体、值对象 —— 一个聚合一个文件，充血
└── policy/       跨聚合的业务策略：决策等级判定、DAG 无环校验、注入选择
```

规格在 [`../../../docs/spec/domain-model.md`](../../../docs/spec/domain-model.md)：
115 条不变量，**每一条都要能变成一句可执行断言**。改这里之前必须读完对应章节。

## 不负责什么

**这一层不认识外面的世界。**

| ✗ 不该出现在这里 | 该去哪 |
|---|---|
| `context.Context` | 出现在这里通常意味着分层错了——领域逻辑是纯计算 |
| `time.Now()` / `rand` | 注入 `Clock` / `IDGen`，否则测试变成薛定谔的 |
| SQL、HTTP、JSON-RPC、文件系统 | `store` / `api` / `acp` / `fsstore` |
| API 的请求响应 DTO | `api/gen`，那是生成物不是领域模型 |
| 数据库行结构体 | `store` 的私有类型 |
| 事务、并发编排 | `app` |

## 依赖方向

| | |
|---|---|
| 允许 import | **仅标准库**，且不做 IO |
| 禁止 import | **本仓库任何其他包**、任何第三方库 |

由 `golangci-lint` 的 `depguard` 强制。这条没有例外。

## 充血模型，不是贫血

状态流转、不变量校验、自身派生属性**都写在模型上**：

```go
func (w *Work) Transition(to WorkState) error   // ✓ 属于模型
func (c *UnitContract) Freeze() error           // ✓
func (p *PlanVersion) Supersede(next *PlanVersion) error  // ✓
```

> **贫血模型是失败信号。** 如果 `model/` 里全是纯字段的 struct、
> 逻辑都跑到 service 里去了，说明分层做反了。**停下来重新设计，不要继续加。**

## 测试要求

| | |
|---|---|
| 覆盖率门槛 | **≥ 90%**（`model` 与 `policy` 各自） |
| 形态 | 表驱动，**真实实例真实数据，零 mock** |
| 必覆盖 | 每个状态迁移的合法与非法路径；每条不变量的正反例 |

写测试前必须调用 `go-unit-test` skill。

**最好用的自查：把实现里的关键一行删掉，测试会红吗？**

## 检查命令

```bash
cd backend && go test ./internal/domain/... -count=1 -v
make -C ../../.. cover
```

## 改这里之前必读

- [`../../../docs/spec/domain-model.md`](../../../docs/spec/domain-model.md) —— 规格，含 115 条不变量与 §18 的开放项
- [`../../../docs/plan/open-questions.md`](../../../docs/plan/open-questions.md) —— **撞上未决问题就停，不要猜**
- [`../../../docs/rules/design-principles.md`](../../../docs/rules/design-principles.md)
- [`../../../docs/rules/coding-standards.md`](../../../docs/rules/coding-standards.md) §1.1

## 本域特有的坑

- **状态枚举要有穷举测试。** 加一个新状态却忘了处理时，测试必须红。
  只写 `switch` 的 `default` 分支挡不住这个。
- **`PlanVersion` 是 append-only。** 任何"修改计划"的方法都是设计错误——
  只能产生新版本，且必须声明已验收工作的处置（仍有效/需补充/需回滚/已废弃）。
- **`UnitContract` 冻结后不可变。** 需要改 → 出新版本号，不是改字段。
- **`Evidence` 只能由应用直接采集。** 不许存在任何以 Agent 文本创建 Evidence 的构造路径——
  这条要有测试守住。
- **记忆「失效」不等于删除。** 历史运行仍要能追溯当时用过它。
- **别把 ACP 的 `plan` 更新映射成 `PlanVersion`。** 前者是 Agent 的 TODO 清单，
  误映射会污染只增不改的版本链。`acp` 层有测试防这个，领域层也别接受这种输入。
