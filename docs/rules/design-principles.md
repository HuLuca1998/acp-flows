# 设计原则与代码组织

> 命名与文件名规则在 [`coding-standards.md`](coding-standards.md)；**本文管的是更上一层：
> 抽象怎么切、接口放哪、包怎么分、文件层级怎么管。**
>
> 存在的理由很直白：**AI 写代码天然倾向于平铺和复制。**
> 它看不到全局，于是每次都在最近的文件里加一段；看到相似逻辑，复制比抽象更省事。
> 几十轮之后就是一个包里三十个文件、五份几乎一样的代码。本文是防这件事的。

---

## 1. AI 把后端写乱的四种典型模式

先认清敌人。**每次动手前对照一遍，中了哪条就停下来重新设计。**

| 症状 | 根因 | 对策 |
|---|---|---|
| **平铺**：一个包里几十个文件，全在同一层 | 没有按关注点分子包 | §5 文件层级规则 |
| **复制粘贴复用**：两个 adapter 有 80% 相同代码 | 不会用嵌入与模板方法 | §4 继承与复写 |
| **上帝文件**：`service.go` 两千行什么都干 | 没有用例边界 | §5.2 按用例族拆 |
| **接口只有一个实现，且和实现放一起** | 接口定义在了实现方 | §3 接口定义在使用方 |

> 一个可操作的信号：**当你想复制一段代码时，停下来。**
> 这是抽象缺失的信号，不是打字量的问题。

---

## 2. 五条设计原则（按重要性排序）

### 2.1 依赖倒转 —— 本仓库最重要的一条

高层不依赖低层，两者都依赖抽象。

```
app（高层，定义接口） ◀── store / acp / gitx（低层，实现接口）
```

`app` 定义 `WorkRepo`，`store` 去实现它。**`app` 永远不知道 SQLite 的存在。**

这条由 `depguard` 强制，不是靠自觉。见 [`architecture.md`](../spec/architecture.md) §3。

### 2.2 单一职责 —— 用「改动理由」判定

一个包/文件/类型**只应该因为一个理由而改变**。

判定方法：问「什么情况下我要改它？」如果答案有两个不相关的原因，拆。

```
✗ store/work.go     既做 SQL 映射，又做状态校验     （schema 变要改，业务规则变也要改）
✓ store/work_repo.go   只做持久化
✓ domain/model/work.go 只做状态规则
```

### 2.3 接口隔离 —— 小接口，多组合

**Go 的接口越小越好。** 一个方法的接口是好接口。

```go
// ✗ 上帝接口：实现方被迫实现一堆用不到的方法，测试要写巨型 fake
type Store interface {
    SaveWork(...) error
    FindWork(...) (*model.Work, error)
    SavePlan(...) error
    // …还有 30 个
}

// ✓ 按用例切分，每个用例只依赖它真正需要的
type WorkSaver interface  { SaveWork(ctx context.Context, w *model.Work) error }
type WorkFinder interface { FindWork(ctx context.Context, id model.WorkID) (*model.Work, error) }
```

**小接口的直接收益：测试里的 fake 只要实现两三个方法。**
巨型 fake 是 AI 写假测试的温床——它太难写，于是就去用 mock 框架，然后就 mock 喂 mock 了。

### 2.4 组合优于继承

Go 没有继承，只有嵌入。**这是好事**——但用户要的"继承与复写"的表达力在 Go 里完全有等价物，见 §4。

原则：**共享行为用嵌入，共享契约用接口，二者组合。**

### 2.5 开闭 —— 加新东西不改老代码

判定标准很具体：**加第 3 个 Runtime 适配器时，要改几个已有文件？**

- 改一堆 `switch runtime {}` → 设计失败
- 只加一个新包 + 在注册表登记一行 → 设计成功

本仓库有三处必须用注册表模式，不许用 switch：

| 场景 | 注册表 |
|---|---|
| ACP Runtime 适配器 | `acp/runtime/registry.go` |
| 13 类事件的前端渲染器 | `features/conversation/renderers/index.ts` |
| 8 个角色的提示词与绑定 | `domain/policy/role_registry.go` |

---

## 3. 接口：定义在哪、怎么分包

### 3.1 接口定义在**使用方**，不在实现方

这是 Go 与 Java 最大的差别，也是 AI 最容易搞反的一点。

```
✗ store/repo.go       定义 WorkRepo 接口 + SQLite 实现   ← Java 习惯，错
✓ app/port/repo.go    定义 WorkRepo 接口（使用方）
  store/work_repo.go  实现它（不需要 import port，Go 是结构化类型）
```

好处：`store` 包不需要 import `app`，依赖是单向的；换实现不需要动接口。

### 3.2 接口集中在 `port/` 子包

```
backend/internal/app/
├── AGENTS.md  CLAUDE.md
├── app.go                    包门面：Application 结构体 + New()
├── port/                   ★ 全部对外依赖的抽象，只有 interface，零实现
│   ├── repo.go               WorkRepo · PlanRepo · UnitRepo · ContractRepo …
│   ├── gateway.go            RuntimeGateway · GitGateway · GitHubGateway
│   ├── publisher.go          EventPublisher
│   └── system.go             Clock · IDGen · Paths
└── usecase/                  用例实现，按用例族分子包（§5.2）
```

**`port/` 里禁止出现任何 struct 实现、任何 import 基础设施包。**
它应该只有 `interface` 和它们用到的领域类型。

### 3.3 接口命名

| 形态 | 命名 | 例 |
|---|---|---|
| 单一能力 | `-er` | `Freezer` `Prober` `Publisher` |
| 持久化 port | `<聚合>Repo` | `WorkRepo` `ContractRepo` |
| 外部系统 port | `<系统>Gateway` | `RuntimeGateway` `GitGateway` |
| 系统能力 | 名词 | `Clock` `IDGen` `Paths` |

**禁止** `IWorkRepo`、`WorkRepoInterface`、`WorkService`。

---

## 4. Go 里的「继承与复写」

Go 没有 `extends`，但下面三种手段覆盖了继承想解决的全部问题。
**两个 ACP adapter（claude / codex）80% 逻辑相同，就是靠这个复用，不是靠复制。**

### 4.1 嵌入 + 复写 ≈ 继承 + override

```go
// ── 基类：放共同实现 ──────────────────────────────────────
type baseAdapter struct {
    conn *jsonrpc.Conn
    caps capability.Matrix
}

func (b *baseAdapter) Initialize(ctx context.Context) error {
    resp, err := b.conn.Call(ctx, constant.ACPInitialize, initParams{ProtocolVersion: 1})
    if err != nil {
        return fmt.Errorf("initialize: %w", err)
    }
    b.caps = capability.From(resp)
    return nil
}

// ── 子类：嵌入基类，只写差异 ──────────────────────────────
type CodexAdapter struct {
    baseAdapter          // ← 嵌入，等价于「继承」
}

// 复写 Initialize，并调用「super」
func (c *CodexAdapter) Initialize(ctx context.Context) error {
    if err := c.baseAdapter.Initialize(ctx); err != nil {   // ← 显式调父类实现
        return err
    }
    // codex 默认档是 agent（workspace-write + on-request），建会话后收权到只读。
    // 注意：设计稿的 mem-188 说 agent 档"不询问"是错的，已核实，见 open-questions.md Q4a。
    return c.restrictToReadOnly(ctx)
}

type ClaudeAdapter struct {
    baseAdapter          // 不复写 Initialize，直接用基类的
}
```

**比继承更清楚的地方：** 调父类实现是显式的 `c.baseAdapter.Initialize(ctx)`，
不存在"忘了调 super"这种隐蔽 bug。

### 4.2 模板方法 —— 固定流程，开放钩子

流程骨架相同、个别步骤不同时用这个。**比"复制整个流程再改两行"好得多。**

```go
// 骨架定义在一处
type sessionFlow struct {
    hooks SessionHooks
}

type SessionHooks interface {
    AfterNew(ctx context.Context, s *Session) error   // 差异点：codex 在这里收权
    MapStopReason(raw string) StopReason              // 差异点：两端取值不同
}

func (f *sessionFlow) Run(ctx context.Context, p Prompt) (*Result, error) {
    s, err := f.newSession(ctx)                       // 固定
    if err != nil { return nil, err }
    if err := f.hooks.AfterNew(ctx, s); err != nil {  // 可变
        return nil, err
    }
    return f.pump(ctx, s, p)                          // 固定
}
```

### 4.3 装饰器 —— 加横切能力不动原实现

日志、重试、指标、超时一律用装饰器，**不要塞进业务实现里**。

```go
type retryingRuntime struct {
    port.RuntimeGateway          // 嵌入接口 → 自动继承全部方法
    policy util.RetryPolicy
}

// 只复写需要重试的那个方法，其余原样透传
func (r *retryingRuntime) Prompt(ctx context.Context, p port.Prompt) (*port.Result, error) {
    return util.Retry(ctx, r.policy, func() (*port.Result, error) {
        return r.RuntimeGateway.Prompt(ctx, p)
    })
}

// 装配时一层层套上去，每层职责单一
rt = newRetrying(newLogging(newMetered(base)))
```

**嵌入接口是 Go 装饰器的关键招式**——只写你要改的方法，其余自动透传。

### 4.4 差异内化：外部一致，差距在内部填平 ★

**本项目最核心的设计问题。** claude 与 codex 的 ACP 实现有大量细微差异，
如果这些差异漏到上层，整个编排层会被 `if runtime == "codex"` 污染成筛子。

#### 红线：品牌判断不许离开 adapter

```go
// ✗ 一旦出现，就已经烂了 —— 每加一个 Runtime 都要全仓库搜一遍
if rt.Name() == "codex" {
    rt.SetMode(ctx, "read-only")
}

// ✓ 上层只表达意图，怎么做是 adapter 的事
rt.RestrictPermissions(ctx, port.PermissionReadOnly)
```

`grep -rn 'codex\|claude' backend/internal/{app,domain,api}` **必须是空的**。
出现即视为设计失败，见 §8 禁止清单。

#### 接口按「能力」定义，不按「协议方法」定义

上层要的是**做成一件事**，不是**调一个方法**。

| ✗ 照抄协议方法 | ✓ 表达意图 |
|---|---|
| `SetMode(mode string)` | `RestrictPermissions(p PermissionProfile)` |
| `SendPromptChunk(...)` | `Prompt(ctx, Prompt) (*Result, error)` |
| `NotifyCancel()` | `Cancel(ctx) error`（内含两段式 + 幂等） |

好处很实际：ACP 官方已给 `session/set_mode` 挂了废弃告示，将被 Session Config Options 取代。
接口叫 `RestrictPermissions` 时，这次协议演进**只改 adapter 内部**；
接口叫 `SetMode` 时，协议一变全仓库跟着改。

#### 三类差异，三种对策 —— 不要都用 `if`

| 差异类型 | 例子 | 对策 |
|---|---|---|
| **取值 / 命名不同** | codex 的 mode 是 `read-only`/`agent`/`agent-full-access`，claude 是另一套 id | **映射表**，adapter 私有 |
| **步骤多寡不同** | codex 建会话后要收权，claude 不需要 | **模板方法钩子**（§4.2） |
| **能力真的缺失** | 探针 12 项，某端只过 11 项 | **能力查询 + 显式降级**（见下） |

```go
// 取值差异 → 映射表，不是 switch 散落各处
var codexModes = map[port.PermissionProfile]string{
    port.PermissionReadOnly:  "read-only",
    port.PermissionWorkspace: "agent",
    port.PermissionFull:      "agent-full-access",
}

func (c *CodexAdapter) RestrictPermissions(ctx context.Context, p port.PermissionProfile) error {
    mode, ok := codexModes[p]
    if !ok {
        return fmt.Errorf("codex: 不支持权限档 %s: %w", p, port.ErrCapabilityMissing)
    }
    return c.setMode(ctx, mode)
}
```

映射表是**穷举**的，加一个 `PermissionProfile` 取值时，
配套的穷举测试会红——比 `switch` 的 `default` 分支可靠得多。

#### 「外部一致」不等于「假装没有差异」

这是最容易做错的地方。**能力真的缺失时，静默降级比暴露差异更危险**——
上层以为做了，其实没做。

正确做法：**用统一的方式暴露差异**，而不是让调用方去判断品牌。

```go
// ✓ 上层问的是「能不能」，不是「你是谁」
if !rt.Capabilities().Has(port.CapStreamingThoughts) {
    // 显式降级路径，且这个决定是可测的
    return f.runWithoutThoughts(ctx, p)
}
```

| | |
|---|---|
| **该填平**（内部消化） | 命名、取值、步骤顺序、协议版本演进、错误码翻译、重试策略 |
| **该暴露**（统一的能力查询） | 功能真的不存在、语义真的不同、性能特征差一个数量级 |

判断标准：**填平之后，上层的行为还正确吗？**
不正确就必须暴露——但要通过 `Capabilities()` 暴露，不是通过 `Name()`。

#### 一致性测试：同一套用例跑遍所有实现 ★

这是保证「外部一致」真的成立的唯一手段。

```go
// backend/tests/contract/runtime_contract_test.go
func TestRuntimeGatewayContract(t *testing.T) {
    for _, impl := range []struct {
        name string
        new  func(*testing.T) port.RuntimeGateway
    }{
        {"claude", newClaudeAgainstFake},
        {"codex",  newCodexAgainstFake},
        {"fake",   newFakeDirect},
    } {
        t.Run(impl.name, func(t *testing.T) {
            rt := impl.new(t)
            // 同一批断言，一个字都不改
            t.Run("取消是幂等的", func(t *testing.T) { ... })
            t.Run("收权后写操作被拒", func(t *testing.T) { ... })
            t.Run("取消会应答所有 pending 的权限请求", func(t *testing.T) { ... })
        })
    }
}
```

**加一个新 Runtime = 在这张表里加一行，不改任何断言。**
改不动断言就说明抽象漏了——那是设计问题，不是测试问题。

Fake Runtime 也要跑同一套契约测试：它如果和真实 adapter 行为不一致，
所有依赖它的上层测试都是假的。

#### 落到本项目的目录

```
internal/acp/adapter/
├── base.go              共同实现（初始化、prompt 泵、两段式取消骨架）
├── claude/
│   ├── adapter.go       嵌入 base，只写差异
│   └── modes.go         claude 的取值映射表
└── codex/
    ├── adapter.go       嵌入 base，复写 AfterNew 做收权
    └── modes.go         codex 的取值映射表
```

选哪个 adapter 由 `acp/runtime/registry.go` 决定——**注册表，不是 `switch`**。

---

### 4.5 什么时候不要抽

- 只有一个实现，且看不到第二个 → **不要抽接口**
- 两处相似但**改动理由不同** → 不要合并，它们只是碰巧长得像
- 为了"以后可能会用" → 不要。猜测中的复用是负债

**抽象的触发条件是第 2 个真实实现出现**，不是预感。

---

## 5. 文件层级管理

### 5.1 硬性上限

| 规则 | 上限 | 超了怎么办 |
|---|---|---|
| 单文件行数 | **400 行** | 按关注点拆文件 |
| 单函数行数 | **60 行** | 内部大概率藏着可抽的纯逻辑 |
| 一个包内直接的 `.go` 文件数 | **10 个** | 拆子包（§5.3） |
| 一个 struct 的方法数 | **15 个** | 职责太多，拆类型 |
| 函数参数个数 | **4 个** | 用 options struct |

前两条由 `scripts/check-naming.sh` 在 CI 强制。

### 5.2 包内文件按什么维度切

**按「领域概念」切，不按「技术种类」切。**

```
✗ 按技术种类切 —— 改一个功能要跳四个文件
store/
├── models.go        所有行结构体
├── queries.go       所有 SQL
├── mappers.go       所有映射
└── interfaces.go    所有接口

✓ 按领域概念切 —— 改 Work 相关只看 work_repo.go
store/
├── store.go         包门面：Store 结构体、New()、依赖装配
├── errors.go        包内错误
├── tx.go            事务封装（真正的横切关注点）
├── work_repo.go     Work 的行结构 + SQL + 映射，全在一起
├── plan_repo.go
├── unit_repo.go
└── contract_repo.go
```

例外：真正的横切关注点（事务、连接管理、错误定义）单独成文件。

### 5.3 什么时候拆子包

**任一条成立就拆：**

1. 包内直接文件数 **> 10**
2. 包内出现两组**互不引用**的类型 —— 它们本来就是两个包
3. 某个关注点已经占了 **≥ 3 个文件**
4. 需要对包内的一部分做独立的依赖限制

拆的维度和 5.2 一样：**按领域概念，不按技术层次**。

```
✗ acp/{types,handlers,utils,helpers}/          技术种类，毫无信息量
✓ acp/{protocol,jsonrpc,session,capability,runtime,adapter,fake}/   领域概念
```

### 5.4 目标层级：后端全貌

```
backend/
├── cmd/duetd/
│   └── main.go                    ← 唯一做依赖装配的地方，手工 new，不用 DI 框架
├── internal/
│   ├── api/                       HTTP 传输层
│   │   ├── router.go
│   │   ├── middleware/            鉴权、日志、恢复
│   │   ├── handler/               按资源分文件：work.go plan.go unit.go system.go
│   │   ├── sse/                   SSE 连接管理
│   │   └── gen/                   ★ openapi 生成物，不手改
│   ├── app/
│   │   ├── app.go
│   │   ├── port/                ★ 全部接口定义（零实现）
│   │   └── usecase/               按用例族分子包
│   │       ├── work/              创建工作、切 worktree、状态推进
│   │       ├── plan/              规划、重规划、冻结
│   │       ├── unit/              契约冻结、执行、审查、验收
│   │       ├── memory/            候选、审核、失效
│   │       └── system/            版本、更新 prepare、恢复
│   ├── domain/
│   │   ├── model/               ★ 一个聚合一个文件，充血
│   │   └── policy/                跨聚合策略（决策等级、DAG 校验、注入选择）
│   ├── constant/                ★ 按主题分文件
│   ├── util/                    ★ 纯函数 + INDEX.md
│   ├── acp/
│   │   ├── protocol/  jsonrpc/  session/  capability/
│   │   ├── runtime/               注册表 + 多版本管理
│   │   ├── adapter/
│   │   │   ├── base.go            ★ 共同实现，被 claude/codex 嵌入
│   │   │   ├── claude/
│   │   │   └── codex/
│   │   └── fake/                ★ Fake Runtime（测试地基）
│   ├── store/                     实现 port 的持久化
│   │   ├── migration/
│   │   └── query/                 复杂查询
│   ├── fsstore/                   .acpflows 下 md 文件读写
│   ├── gitx/  ghx/  eventbus/  platform/
└── tests/                       ★ 跨包测试
    ├── contract/  integration/  fixtures/  testutil/
```

**每个带 `AGENTS.md` 的层级都要说明「不负责什么」** —— 边界比职责更能防止乱放代码。

---

## 6. 前端的对应做法

前端的"继承"是**组合**，不是嵌入。

| 想要的效果 | React 做法 |
|---|---|
| 共享行为 | 自定义 hook |
| 共享结构 + 定制局部 | `children` / render props / 复合组件 |
| 共享 Props 契约 | `interface XProps extends BaseProps` |
| 多态渲染 | **注册表**（`Record<Type, Renderer>`），不是 `switch` |
| 横切能力 | Provider + hook，不是 HOC 套娃 |

```ts
// 13 类事件：注册表，加第 14 类只加一个文件 + 登记一行
export const eventRenderers: Record<EventType, EventRenderer> = {
  message_chunk: MessageChunkRow,
  tool_call:     ToolCallRow,
  // …
}
```

**禁止**：一个 `.tsx` 里五个组件、`switch (event.type)` 的巨型渲染函数、
超过两层的 HOC 嵌套。

---

## 7. 每次动手前的三个问题

1. **这段逻辑该放哪一层？** 说不清就是分层没想明白，回去读 `architecture.md`
2. **我是不是在复制？** 是 → 停下，用 §4 的手段抽出来
3. **加第 3 个同类东西时要改几个文件？** 超过一个 → 用注册表重构

## 8. 禁止清单

- ✗ 接口和它唯一的实现放在同一个包（接口定义在使用方）
- ✗ 上帝接口（超过 5 个方法且调用方只用其中两个）
- ✗ `switch` 分发同类实现（用注册表）
- ✗ 复制粘贴复用（用嵌入 / 模板方法 / 装饰器）
- ✗ 包内平铺超过 10 个文件
- ✗ 按技术种类切文件（`models.go` `utils.go` `handlers.go`）
- ✗ 只有一个实现就抽接口（过早抽象）
- ✗ 在 `cmd/` 之外做依赖装配
- ✗ 把日志/重试/指标塞进业务实现（用装饰器）
