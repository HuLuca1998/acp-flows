# 代码规范

> 本文件规定命名、文件组织与写法。**这些规则尽可能由 lint 强制**；
> 靠自觉的规则在无人类审阅的仓库里等于不存在。
> 与根 [`AGENTS.md`](../../AGENTS.md) 的铁律冲突时，以铁律为准。

---

## 1. 三个专用文件夹

每个子项目都有三个固定用途的文件夹。**放错地方是最常见的规范违规。**

| 用途 | Go | TypeScript | Rust |
|---|---|---|---|
| 模型 | `backend/internal/domain/model/` | `frontend/src/models/` | `shell/src-tauri/src/models/` |
| 常量 | `backend/internal/constant/` | `frontend/src/constants/` | `shell/src-tauri/src/constants/` |
| 工具 | `backend/internal/util/` | `frontend/src/utils/` | `shell/src-tauri/src/utils/` |

### 1.1 模型文件夹

**放什么：** 领域类型的定义，以及**只依赖该类型自身状态**的方法。

- 一个聚合一个文件：`work.go` `plan.go` `subplan.go` `unit.go` `unit_contract.go`
  `attempt.go` `evidence.go` `decision.go` `memory.go` `skill.go` `checkpoint.go`
- 模型是**充血的**：状态流转、不变量校验、自身派生属性都写在模型上。
  `Work.Transition(to State) error` 属于模型，不属于 service。
- 值对象与枚举类型跟着它所属的聚合走，不单独开文件。

**不放什么：**

- ✗ 需要访问数据库、文件系统、网络的方法 → 那是 `app` 层用例
- ✗ 跨多个聚合的编排逻辑 → 放 `domain/policy/`
- ✗ API 的请求/响应 DTO → 那是 `api/gen` 生成物，不是领域模型
- ✗ 数据库行结构体 → 那是 `store` 的私有类型

> **贫血模型是失败信号。** 如果 `model/` 里全是纯字段的 struct，
> 所有逻辑都跑到 service 里去了，说明分层做反了，停下来重新设计。

### 1.2 常量文件夹

**放什么：** 跨包共享的常量、枚举取值、固定配置。

按主题分文件，不要一个 `constants.go` 装全部：

```
internal/constant/
├── state.go        # 状态机取值：clarifying / executing / waiting_user …
├── event.go        # 13 类事件 type 取值
├── decision.go     # D0–D3 等级
├── path.go         # .acpflows 下的固定路径片段
├── acp.go          # ACP 方法名：session/new · session/prompt · session/cancel …
└── limit.go        # 超时、重试次数、体积上限
```

**规则：**

- 枚举一律**具名类型 + 常量**，不用裸字符串：
  ```go
  type WorkState string
  const (
      WorkStateClarifying WorkState = "clarifying"
      WorkStateExecuting  WorkState = "executing"
  )
  ```
- 每个枚举必须配一个 `IsValid()` 和 `String()`，以及一个**穷举测试**
  （新增取值忘了处理时测试要红）。
- **不放什么：** 只在一个文件里用到的常量——就地定义，别污染共享命名空间。

### 1.3 工具文件夹

**这是最容易变成垃圾桶的地方。以下是硬性准入规则：**

一段代码要进 `util/`，必须**同时**满足：

1. **纯函数** —— 无 IO、无全局状态、无时间/随机源（要用就注入）
2. **零业务语义** —— 不认识 Work / Unit / Contract 这些概念
3. **已有 ≥ 2 处真实调用方** —— 只有一个调用方就放在调用方旁边
4. **有单元测试** —— 无测试的工具函数不许合并

按主题分文件，文件名即主题：

```
internal/util/
├── slice.go      # 切片操作
├── ptr.go        # 指针与零值
├── strx.go       # 字符串处理
├── pathx.go      # 路径规范化（纯计算，不碰文件系统）
└── retry.go      # 重试策略（时钟注入）
```

**禁止：**

- ✗ 名为 `util.go` `helper.go` `common.go` `misc.go` 的文件 —— 名字不说明内容 = 垃圾桶
- ✗ 带业务语义的工具（`util.FormatUnitID()` → 应该是 `model.UnitID.String()`）
- ✗ 碰 IO 的"工具"（`util.ReadConfig()` → 应该在 `platform` 或 `fsstore`）

> lint 规则会拦下 `util.go` / `helper.go` / `common.go` / `misc.go` 这四个文件名。

### 1.4 工具库的抽取节奏与索引

代码会越写越大。**不主动抽取，工具逻辑就会散落在各处并被反复重写**——
这是 AI 维护的代码库里最常见的浪费，因为下一轮的 AI 不知道上一轮写过什么。

#### 写新工具前：先查索引

| 位置 | 索引 |
|---|---|
| Go | [`backend/internal/util/INDEX.md`](../../backend/internal/util/INDEX.md) |
| TypeScript | [`frontend/src/utils/INDEX.md`](../../frontend/src/utils/INDEX.md) |

**动手写任何工具函数之前，先在对应索引里搜一遍。** 索引存在的唯一目的就是这个。
搜到了就用，别重写；搜到相近的就扩展它，别并列一个新的。

#### 什么时候抽取

| 触发条件 | 动作 |
|---|---|
| 同一段逻辑出现**第 2 个**调用方 | 立刻抽取 |
| 两个调用方在**同一个包**内 | 抽到该包内的私有函数，**不进 `util/`** |
| 跨包 + 零业务语义 + 纯函数 | 进 `util/`，**同时登记索引** |
| 单文件超过 **400 行** | 先检查有没有可抽的工具，再考虑拆分 |
| 一个函数超过 **60 行** | 大概率内部藏着可抽的纯逻辑 |

> **别提前抽象。** 只出现一次的逻辑不许进 `util/`——猜测中的复用是负债，不是资产。
> 铁律是「第 2 次才抽」，不是「预感会复用就抽」。

#### 每个任务收尾时做一次抽取检查

这是 §3「7 步流程」第 6 步自查的一部分：

- [ ] 这次新写的代码里，有没有和 `INDEX.md` 里已有工具重复的？→ 换成调用已有的
- [ ] 有没有同一段逻辑写了两遍？→ 抽出来
- [ ] 新进 `util/` 的函数，索引登记了吗？测试写了吗？
- [ ] 有没有把带业务语义的东西塞进 `util/`？→ 挪回 `model/` 或 `app/`

#### 索引格式

一行一个导出函数，按文件分组：

```markdown
| 函数 | 文件 | 签名 | 一句话说明 |
|---|---|---|---|
| `Chunk` | `slice.go` | `Chunk[T any](s []T, n int) [][]T` | 把切片按固定大小分批 |
```

#### 强制手段

```bash
make check-util-index
```

脚本会把 `util/` 里**实际导出的函数**与索引表逐一比对：

- 导出了但没登记 → 红
- 登记了但已删除 → 红
- 索引里的签名与代码不一致 → 红

CI 在每个 PR 上跑。**索引不是文档，是被校验的清单。**

---

## 2. 文件命名

| 语言 / 类型 | 规则 | 示例 |
|---|---|---|
| Go 源文件 | `snake_case.go` | `unit_contract.go` `fake_runtime.go` |
| Go 测试 | `<被测文件>_test.go` | `unit_contract_test.go` |
| Go 集成测试 | `<主题>_integration_test.go` + `//go:build integration` | `session_integration_test.go` |
| React 组件 | `PascalCase.tsx`，与默认导出同名 | `EventStream.tsx` `UnitContractDrawer.tsx` |
| TS 非组件 | `kebab-case.ts` | `use-event-stream.ts` `format-duration.ts` |
| TS 类型定义 | `kebab-case.ts`，放 `models/` | `unit-contract.ts` |
| CSS Module | `PascalCase.module.css`，与组件同名同目录 | `EventStream.module.css` |
| Rust | `snake_case.rs` | `sidecar_guard.rs` |
| 文档 | `kebab-case.md` | `release-and-update.md` |
| SQL 迁移 | `NNNN_<动词>_<对象>.sql`，编号只增不复用 | `0002_add_checkpoint_commit.sql` |

**一个文件一个主要导出。** 一个 `.tsx` 里塞五个组件会让 AI 定位困难，
也让 diff 变得难读。子组件超过 30 行就拆出去。

**文件长度上限 400 行**（不含生成物与测试）。超了说明职责不单一，拆。

---

## 3. Go 命名规范

### 3.1 包名

- 全小写、单个词、无下划线、无复数：`store` `gitx` `eventbus` ✓ / `git_utils` `models` ✗
- 包名是调用方的前缀，**不要在标识符里重复包名**：
  `store.NewStore()` ✗ → `store.New()` ✓；`model.WorkModel` ✗ → `model.Work` ✓
- 与标准库同名时加后缀 `x`：`gitx` `strx` `pathx`

### 3.2 类型

| 种类 | 规则 | 示例 |
|---|---|---|
| 结构体 | 名词，PascalCase | `UnitContract` `PlanVersion` |
| 接口 | 能力用 `-er`；port 接口用 `<名词>+Repo/Gateway/Port` | `Freezer` `WorkRepo` `RuntimeGateway` |
| 枚举底层类型 | 具名类型，不用裸 `string`/`int` | `type WorkState string` |
| 错误类型 | `<名词>Error` | `ContractFrozenError` |
| 哨兵错误 | `Err<名词>` | `ErrNotFound` `ErrContractFrozen` |

**接口定义在使用方，不在实现方。** `app` 定义 `WorkRepo`，`store` 去实现它。

### 3.3 函数与方法

**动词表 —— 语义固定，不许混用：**

| 前缀 | 含义 | 签名约定 |
|---|---|---|
| `New` | 构造，可能校验失败 | `New(...) (T, error)` 或 `New(...) T` |
| `Must` | 失败即 panic | **仅允许**在 `init()` 与测试里调用 |
| `Find` | 查询，**可能不存在** | `(T, error)`，未找到返回 `ErrNotFound` |
| `Load` | 读取，**必须存在**，不存在即错误 | `(T, error)` |
| `List` | 返回集合 | `([]T, error)`，空集合返回空切片**不是 nil** |
| `Count` | 计数 | `(int, error)` |
| `Create` | 新建并持久化 | `(T, error)` |
| `Update` | 更新已有 | `error` |
| `Delete` | 删除 | `error` |
| `Save` | 存在则更新，否则插入 | `error` |
| `Ensure` | **幂等**保证某状态成立 | `error` |
| `Is` / `Has` / `Can` / `Should` | 判定 | `bool`，**无副作用** |
| `Validate` | 校验 | `error`，**不修改入参** |
| `Freeze` `Supersede` `Accept` `Reject` | 领域动词，语义见 domain-model.md | |

**禁止：**

- ✗ `Get` 前缀。无参访问器直接用字段名：`w.State()` 而不是 `w.GetState()`
- ✗ `Handle` / `Process` / `Do` / `Run` 这类无信息动词（HTTP handler 除外）
- ✗ `Manager` / `Helper` / `Util` / `Service` 结尾的类型名 —— 说明职责没想清楚
- ✗ 同一概念用不同动词（一处 `Fetch` 一处 `Find`）

### 3.4 变量

- **作用域越短，名字越短。** 循环里 `i` `w` 可以；跨 30 行的变量必须是完整词。
- 缩写统一（团队词汇表，不许自创）：

  | 概念 | 缩写 | 概念 | 缩写 |
  |---|---|---|---|
  | context | `ctx` | error | `err` |
  | request | `req` | response | `resp` |
  | configuration | `cfg` | repository | `repo` |
  | identifier | `id` | database | `db` |
  | UnitContract | `contract` | PlanVersion | `pv` |

- **禁止**：`data` `info` `obj` `tmp` `res` `val` 这类不说明内容的名字
- 布尔变量用 `is` / `has` / `can` 开头
- 方法接收者：1–2 个字母，同一类型全仓库统一（`func (w *Work)` 就到处都是 `w`）

### 3.5 错误处理

```go
// ✓ 包装时带上下文，用 %w 保留链
if err := repo.Save(ctx, w); err != nil {
    return fmt.Errorf("save work %s: %w", w.ID, err)
}
```

- 错误信息**小写开头、无标点结尾**，可拼接
- 领域错误定义在 `domain/model/errors.go`，`api` 层统一映射成 `Problem`
- **禁止** `panic` 传播业务错误；`panic` 只用于"不可能发生"的程序 bug
- **禁止**吞错误（`_ = err`）。确实要忽略必须写注释说明为什么

### 3.6 注释

- 导出标识符必须有文档注释，以标识符名开头：`// Work 表示一次完整的开发任务。`
- 注释解释**为什么**，不解释**是什么**。代码已经说了是什么。
- **禁止**：`// TODO` 不带 issue 链接；被注释掉的死代码（用 git 历史）
- 中文注释可用，术语按根 `AGENTS.md` §8 术语表

---

## 4. TypeScript / React 规范

### 4.1 命名

| 种类 | 规则 | 示例 |
|---|---|---|
| 组件 | PascalCase，名词 | `EventStream` `PlanPanel` |
| Hook | `use` + 动词/名词，camelCase | `useEventStream` `useWorkState` |
| 普通函数 | camelCase，动词开头 | `formatDuration` `parseContract` |
| 类型 / 接口 | PascalCase，**不加 `I` 前缀** | `UnitContract` 而非 `IUnitContract` |
| 常量 | `SCREAMING_SNAKE_CASE` | `MAX_SIDEBAR_WIDTH` |
| 枚举取值 | 用 `as const` 对象 + 联合类型，**不用 `enum`** | 见下 |
| 布尔 | `is` / `has` / `can` / `should` 开头 | `isFrozen` `hasEvidence` |
| 事件回调 props | `on` + 事件 | `onSelect` `onDismiss` |
| 事件处理实现 | `handle` + 事件 | `handleSelect` |

```ts
// ✓ 枚举用 as const，可与后端生成的字符串联合直接对齐
export const WORK_STATE = {
  clarifying: 'clarifying',
  executing: 'executing',
} as const
export type WorkState = (typeof WORK_STATE)[keyof typeof WORK_STATE]
```

`enum` 会生成运行时代码且与生成的 OpenAPI 联合类型对不上，**禁用**。

### 4.2 组件

- **一个文件一个组件**（加同名 `.module.css`）。子组件超过 30 行就拆文件。
- Props 类型名 = `<组件名>Props`，定义在组件文件内。
- 优先函数组件 + hooks，不用 class 组件。
- 副作用只在 `useEffect` 里，且**必须写清依赖**；`eslint-plugin-react-hooks` 强制。
- **禁止**在组件里直接 `fetch` —— 一律走 `src/api/` 的生成客户端 + TanStack Query。

### 4.3 样式

- 只用 CSS Modules + 设计令牌，见 [`frontend-guide.md`](../spec/frontend-guide.md)
- **禁止**：硬编码 hex、裸 px、内联 `style` 里写颜色、CSS-in-JS
- 类名 camelCase（CSS Modules 惯例）：`.eventRow` `.unitBadge`

### 4.4 导入顺序

```ts
// 1. 外部依赖
import { useMemo } from 'react'
// 2. 内部绝对路径（@/ 别名）
import { useEventStream } from '@/features/conversation/use-event-stream'
import { WORK_STATE } from '@/constants/state'
// 3. 相对路径
import { EventRow } from './EventRow'
// 4. 样式
import styles from './EventStream.module.css'
```

由 `eslint-plugin-import` 的 `order` 规则强制，不靠手动排。

---

## 4.5 数据库与 GORM

表名、列名、索引命名、GORM 实体定义、迁移文件命名，全部在
[`database.md`](database.md)。**这里不重复**，只记三条最容易犯的：

| ✗ | ✓ |
|---|---|
| 领域模型挂 `gorm:"..."` 标签 | 实体在 `store/entity/`，中间隔一个 `mapper/` |
| `Updates(entity.Work{State: s})` | `Updates(map[string]any{"state": s})` —— struct 形式零值静默丢更新 |
| `Find(&e)` 查单条 | `First(&e)` —— `Find` 记录不存在时 `err` 是 nil |

---

## 5. Rust 规范（`shell/`）

遵循 `rustfmt` 默认与 `clippy` 建议，此外：

- 文件 `snake_case.rs`，模块与文件同名
- 函数 `snake_case`，类型 `PascalCase`，常量 `SCREAMING_SNAKE_CASE`
- Tauri command 函数名与前端调用名一致，前缀 `cmd_`：`cmd_pick_directory`
- **禁止** `unwrap()` / `expect()` 出现在非测试代码（`clippy::unwrap_used` 设为 deny）
- 壳里不写业务逻辑，见 [`../shell/AGENTS.md`](../../shell/AGENTS.md)

---

## 6. 强制手段

| 规则 | 由什么强制 |
|---|---|
| Go 命名与常见坑 | `golangci-lint`（`revive` `stylecheck` `errcheck` `depguard`） |
| Go 分层依赖方向 | `depguard` 规则，见 `backend/.golangci.yml` |
| 禁止 `util.go`/`helper.go`/`common.go`/`misc.go` | `scripts/check-naming.sh`（CI） |
| 文件行数上限 | `scripts/check-naming.sh` |
| TS 命名与导入顺序 | ESLint（`@typescript-eslint` `import` `react-hooks` `unicorn/filename-case`） |
| 禁止硬编码颜色/px | Stylelint + ESLint `no-restricted-syntax`，规则源自 `design/_ds/*/\_adherence.oxlintrc.json` |
| 禁止前端越层 import `@tauri-apps/*` | ESLint `no-restricted-imports`（仅 `src/platform/` 例外） |
| Rust | `cargo clippy -- -D warnings` |

**规则改了但没有强制手段 = 规则没改。** 加规范的同时必须加检查。

---

## 7. 速查：常见违规

| ✗ | ✓ |
|---|---|
| `internal/util/helper.go` | `internal/util/slice.go` |
| `func (s *Store) GetWork(id)` | `func (s *Store) FindWork(id)` |
| `type WorkManager struct` | `type WorkRepo interface` |
| `data := ...` | `contract := ...` |
| `const state = "executing"` | `constant.WorkStateExecuting` |
| `<div style={{color:'#9184d9'}}>` | `className={styles.accent}` |
| `enum EventType {}` | `const EVENT_TYPE = {} as const` |
| `if err != nil { return err }` 无上下文 | `fmt.Errorf("freeze contract %s: %w", id, err)` |
| 一个 `.tsx` 里 5 个组件 | 一文件一组件 |
| `time.Now()` 出现在 domain | 注入 `Clock` |
