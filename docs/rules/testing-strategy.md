# 测试策略 · 测试先行

> **本仓库没有人类逐行审阅 diff。测试是唯一还在把关的东西。**
> 一个不断言真实行为的测试比没有测试更危险——它制造虚假的安全感，
> 让后面每一轮 AI 都以为这块已经被验证过了。

---

## 1. 强制流程：五步，顺序不可调换

```
① 读验收标准        —— 逐条列出，每条要能变成一句可执行断言
② 写测试            —— 断言绑定标准编号，不写实现
③ 跑，确认是红的    —— ★ 关键步骤。绿了说明测试没测到东西，回②
④ 最小实现          —— 只写到变绿为止，不顺手重构、不提前抽象
⑤ 再跑，确认是绿的  —— 贴出实际输出
```

**第 ③ 步不能跳。** 一个从没红过的测试，你无法证明它在测东西。

提交信息里必须写出**「先红的测试」**是哪一个，见 [`git-workflow.md`](git-workflow.md) §2。
写 `不适用` 而 type 又不是 `test`/`docs`/`chore`/`ci` 的，CI 会拦下。

### 验收标准 → 断言的映射

```go
// R3: 连续取消两次只发送一次协议取消请求
func TestSessionCancel_R3_IsIdempotent(t *testing.T) { ... }

// R4: 取消后 diff 与最后事件游标可读取
func TestSessionCancel_R4_CursorReadableAfterCancel(t *testing.T) { ... }
```

测试名里带标准编号。这样"哪条标准还没有证据"一眼可查——
这正是产品里 `Evidence` 要做的事，我们先在自己身上做到。

---

## 2. 测试分层与位置

### Go：单测就地，集成测试进 `backend/tests/`

```
backend/
├── internal/
│   ├── domain/model/
│   │   ├── work.go
│   │   └── work_test.go          ← 单元测试与源码同目录、同包
│   └── acp/
│       ├── session.go
│       └── session_test.go
└── tests/                        ★ 独立测试包，只放跨包的测试
    ├── contract/                 OpenAPI 契约测试
    ├── integration/              跨包集成测试（对着 Fake Runtime + 临时 SQLite）
    ├── fixtures/                 测试夹具：临时 git 仓库、种子数据、golden 文件
    └── testutil/                 测试专用辅助（★ 不是 internal/util）
```

**为什么分开：**

| | `internal/**/​*_test.go` | `backend/tests/` |
|---|---|---|
| 测什么 | 单个包的契约 | 多个包组装起来的行为 |
| 依赖 | 无 IO（domain）或手写 fake | 真实 SQLite、真实 git、Fake ACP Runtime |
| 速度 | 毫秒级，每次改动都跑 | 秒级，可加 `//go:build integration` |
| 包名 | 同包（可测未导出）或 `xxx_test` 外部包 | 一律外部包，**只走导出 API** |

**`backend/tests/testutil` 与 `backend/internal/util` 是两回事，不要混：**

| | 用途 | 谁能 import |
|---|---|---|
| `internal/util` | 生产代码的纯工具函数 | 生产代码 |
| `tests/testutil` | 建夹具、起临时服务、断言辅助 | **只有测试** |

生产代码 import `tests/testutil` 会被 `depguard` 拦下。

### 分层门槛

| 层 | 形态 | 覆盖率门槛 |
|---|---|---|
| `domain/model` `domain/policy` | 表驱动单测，真实实例真实数据，**零 mock** | **≥ 90%** |
| `app` | 用例测试，port 用**手写 fake** | ≥ 75% |
| `acp` | 对着 Fake Runtime 的集成测试 + 协议 golden JSON | ≥ 80% |
| `store` `fsstore` | 临时 SQLite / `t.TempDir()` | ≥ 70% |
| `api` | 契约测试：`kin-openapi` 对着 `api/openapi.yaml` 校验请求与响应 | — |
| 前端组件 | Vitest + Testing Library（**按行为查询，不按实现**） | ≥ 70% |
| 端到端 | Playwright，跑真实 `duetd` + Fake Runtime | 黄金路径必覆盖 |

> **覆盖率是下限，不是目标。** 90% 覆盖率配一堆恒真断言，等于 0%。
> 覆盖率只用来发现"完全没测的地方"，不用来证明"测得好"。

---

## 3. 假测试图鉴 ★

以下每一种都出现过、都会被打回。**写完测试后对着这张表自查一遍。**

### 3.1 mock 喂 mock —— 断言的是自己刚设的值

```go
// ✗ 这个测试永远绿，它只证明了 mock 库能工作
repo := NewMockWorkRepo()
repo.On("FindWork", "work-08").Return(&model.Work{State: "executing"}, nil)
w, _ := repo.FindWork(ctx, "work-08")
assert.Equal(t, "executing", string(w.State))
```

```go
// ✓ 用真实实例测真实规则
w := model.NewWork("work-08", model.WorkStateExecuting)
err := w.Transition(model.WorkStateCompleted)
assert.ErrorIs(t, err, model.ErrMustReviewBeforeComplete)   // 测的是业务规则
```

### 3.2 恒真断言

```go
// ✗ 这些断言不可能失败
assert.NotNil(t, result)
assert.NoError(t, err)          // 唯一断言时等于只测了"没崩"
assert.True(t, len(list) >= 0)
```

```go
// ✓ 断言具体的值和具体的错误
assert.Equal(t, 3, len(list))
assert.Equal(t, []string{"unit-011", "unit-012", "unit-013"}, ids)
assert.ErrorIs(t, err, model.ErrContractFrozen)
```

### 3.3 只运行不断言

```go
// ✗ 跑了一遍没炸，然后呢？
_, err := svc.FreezeContract(ctx, "unit-012")
require.NoError(t, err)
```

```go
// ✓ 跑完要验证状态真的变了
got, err := svc.FreezeContract(ctx, "unit-012")
require.NoError(t, err)
assert.Equal(t, 3, got.Version)
assert.True(t, got.IsFrozen())
stored, _ := repo.LoadContract(ctx, "unit-012")
assert.Equal(t, got.Version, stored.Version)   // 真的落盘了
```

### 3.4 测私有琐碎 helper，不测接口契约

覆盖率好看，但没有验证任何对外承诺。**测导出的行为，不测内部实现细节。**
内部实现改了测试就红，说明测试绑在实现上，不是绑在契约上。

### 3.5 只有 happy path

每个测试至少覆盖：**正常 / 边界 / 错误 / 幂等（若适用）**。

```go
func TestSessionCancel(t *testing.T) {
    tests := []struct{
        name string
        setup func(*fake.Runtime)
        wantErr error
    }{
        {"正常取消", nil, nil},
        {"连续取消两次只发一次请求", nil, nil},              // 幂等
        {"Runtime 不回 stopReason 时超时", fake.NeverStops, acp.ErrCancelTimeout},
        {"reviewing_unit 状态下拒绝取消", nil, model.ErrCancelNotAllowed},  // 边界
    }
    ...
}
```

### 3.6 为了让 CI 变绿而动测试

**这是最严重的违规，等同于伪造验收。**

- ✗ 删掉红的测试
- ✗ 加 `t.Skip()`
- ✗ 把 `assert.Equal(3, n)` 放宽成 `assert.True(n > 0)`
- ✗ 把断言注释掉

测试红了只有两种正确处理：**修实现**，或者**证明测试写错了并说明理由**。

### 3.7 断言在错误的抽象层

```go
// ✗ 断言 SQL 行数——绑死了持久化细节
assert.Equal(t, 1, countRows(db, "SELECT ... FROM units"))
```

```go
// ✓ 断言领域行为
units, _ := repo.ListUnits(ctx, "subplan-03")
assert.Len(t, units, 1)
```

---

## 4. Fake ACP Runtime —— 整套测试策略的支点

`backend/internal/acp/fake/` 提供一个**可编排的假 Agent**，实现完整 ACP 协议。
没有它，上层任何测试都得依赖真实 `claude-agent-acp` / `codex-acp`——
那意味着慢、不确定、要账号、要网络，AI 自测直接废掉。

**它必须是 M0 的第一个交付物。**

能力要求：

| 能力 | 用途 |
|---|---|
| 按脚本回放事件序列（`message_chunk` / `tool_call` / `stopReason` …） | 测事件流与渲染 |
| 可配置延迟、乱序、断流 | 测超时、断点续传、游标 |
| **不回 `stopReason`** | 测取消超时 → `prepare` 返回 `blocked` |
| 主动发 `session/request_permission` | 测权限裁决与阻塞 |
| 记录收到的全部请求 | 断言"只发了一次协议 cancel"（幂等） |
| 可声明能力矩阵（探针 12 项任意组合通过/失败） | 测 Runtime 探测与降级 |

Fake Runtime 本身也要有测试——它是测试的地基，地基歪了上面全歪。

---

## 5. 数据隔离（铁律 6）

**测试禁止读写：** `~/.acpflows` · 用户真实 git 仓库 · 真实 GitHub 令牌 · 真实 Runtime 账号。

| 需要什么 | 用什么 |
|---|---|
| 数据目录 | `t.TempDir()`，通过 `platform.Paths` 注入 |
| 数据库 | 临时 SQLite 文件（**不用 `:memory:`**——测不出 WAL 与并发行为） |
| git 仓库 | `tests/fixtures` 现场 `git init` 一个夹具仓库 |
| ACP Runtime | Fake Runtime |
| GitHub | 拦截 HTTP，**永不出网** |
| 时间 | 注入 `Clock`，禁止 `time.Now()` |
| ID | 注入 `IDGen`，测试里用确定性序列 |

`tests/testutil` 里有一个守卫：测试进程一旦尝试打开 `$HOME/.acpflows` 就直接 `t.Fatal`。
**不要绕过它。**

> 这条来自设计稿里的一条项目记忆：
> `mem-203 · constraint · 集成测试使用临时 SQLite，不得读写用户真实数据库`

---

## 6. 前端测试

| 层 | 工具 | 要点 |
|---|---|---|
| 组件行为 | Vitest + Testing Library | 用 `getByRole` / `getByText` **按用户可见的东西查询**，禁止 `data-testid` 兜底和查 class |
| API mock | MSW，handler **从 `api/openapi.yaml` 生成** | mock 与契约同源，spec 改了 mock 自动跟着变 |
| 事件流 | Fake `EventSource` + 录制的事件序列 | 13 类事件各至少一个渲染测试 |
| 视觉合规 | Stylelint + ESLint 规则 | 硬编码 hex / 裸 px / emoji 图标直接红 |
| 端到端 | Playwright | 跑真实 `duetd`（临时数据目录 + Fake Runtime） |

**禁止**在组件测试里断言 CSS 类名或 DOM 结构——设计稿会改，那些测试只会变成噪音。
断言用户看得见的文本、可访问性角色、以及交互后的状态变化。

### E2E 黄金路径（必须一直绿）

```
创建项目 → 新建工作（切 worktree） → 计划冻结 → 单元执行（Fake Runtime）
→ 权限请求裁决 → 证据生成 → 单元验收 → 检查点落盘
```

M1 之后追加一条：

```
检测到新版本 → prepare（暂停工作 + 落检查点） → 模拟重启 → 从检查点恢复
```

---

## 7. 命令

```bash
make check                    # 提交前全量：docs + lint + test

make -C .. test-backend       # go test ./... -race -count=1
cd backend && go test ./internal/domain/... -run TestPlanVersion -v
cd backend && go test -tags=integration ./tests/...
make cover                    # 覆盖率 + 门槛校验

pnpm -C frontend test --run
pnpm -C e2e test
```

`-count=1` 是为了禁用测试缓存——缓存过的绿色不算验证。

---

## 8. 测试索引 ★

AI 反复写重复测试的根因和重复造工具轮子一样：**它不知道已经测过什么。**
所以测试也要有索引，规则与工具库索引对称。

| 范围 | 索引 |
|---|---|
| 后端全部 Go 测试（含就地单测与 `tests/`） | [`../backend/tests/INDEX.md`](../../backend/tests/INDEX.md) |
| 前端 Vitest | [`../frontend/tests/INDEX.md`](../../frontend/tests/INDEX.md) |
| Playwright 端到端 | [`../e2e/INDEX.md`](../../e2e/INDEX.md) |

### 写新测试前：先查索引

按**行为**搜，不是按函数名搜。问自己两个问题：

1. 这个行为已经被哪个测试覆盖了？→ 覆盖了就**扩展它的用例表**，不要新开一个测试函数
2. 有没有名字不同但实质相同的测试？→ 有就合并，别并列

> 典型的重复模式：`TestCancelWorks` / `TestCancelSuccess` / `TestSessionCancelBasic`
> 三个函数测同一件事，只是不同轮次的 AI 各写了一个。索引就是用来挡这个的。

### 索引粒度

| 语言 | 登记什么 | 不登记什么 |
|---|---|---|
| Go | 每个**顶层 `func TestXxx`** 一行 | 表驱动的子用例（在「覆盖行为」列里概述即可） |
| TS | 每个 **`*.test.ts(x)` 文件**一行 | 单个 `it()` |
| E2E | 每个 **`*.spec.ts` 文件**一行 | 单个步骤 |

### 格式

```markdown
| 测试 | 文件 | 层 | 覆盖的行为 / 验收标准 |
|---|---|---|---|
| `TestSessionCancel_R3_IsIdempotent` | `internal/acp/session_test.go` | acp | R3 连续取消只发一次协议请求；含超时与 reviewing_unit 拒绝 |
```

「覆盖的行为」要写**行为**，不要抄函数名。抄函数名的索引没有检索价值。

### 强制手段

```bash
make check-test-index
```

- 有测试但没登记 → 红
- 登记了但测试已删除 → 红

CI 每个 PR 都跑。**索引不是文档，是被校验的清单。**

---

## 9. 自查清单

写完测试，逐条过：

- [ ] 这个测试**先红过**，我看见过它失败
- [ ] 断言的是**具体的值**，不是 `NotNil` / `NoError` 这类恒真式
- [ ] 用的是**真实实例真实数据**，没有 mock 喂 mock
- [ ] 覆盖了正常 / 边界 / 错误 / 幂等，不只有 happy path
- [ ] 测的是**导出的契约**，不是内部实现细节
- [ ] 没有碰 `~/.acpflows`、真实仓库、真实令牌、真实网络
- [ ] 没有 `time.Now()` / 随机数 / 测试间顺序依赖
- [ ] 测试名能说明它在验哪条验收标准
- [ ] 如果我把实现里的关键一行删掉，这个测试**会红**  ← 最好用的自查

---

## ★ 负例：怎么证明一个检查真的在检查

「删掉关键一行会红」是自查的**做法**，这一节讲它的**坑**——
2026-08-07 一天之内在三个地方各栽了一次。

### 坑 1 · 负例选错了，于是"没红"被当成"通过"

给设置页加了「窄窗口下不出横向滚动条」的 e2e，删掉 `min-width: 0` 想看它红——
没红。当时的内容本来就撑不宽，那一行是防御性的。

**换一个一定会触发的负例**（把导航栏宽度改成 2000px），才发现测试**根本无效**：
它查的是 `documentElement` 的溢出，而主区自己有 `overflow`，
内部撑宽被吸收成一条内部滚动条，页面级毫无异常。

> 负例要挑**必然触发**的，不是**可能触发**的。
> 挑了个不该红的负例，你验证的是"它不红"，而这什么也没证明。

### 坑 2 · 检查被自己的说明文字放行

`exec.CommandContext` 必须设 `WaitDelay` 的检查，第一版是「后面 15 行内出现
`WaitDelay` 就算过」。而解释这条规则的**注释里就有这个词**——
把真正的赋值删掉，检查照样绿。

> 匹配代码的检查要**排掉注释行**。写检查时问一句：
> 这条规则的说明文字本身，会不会让它通过？

### 坑 3 · 本地绿、CI 红，而 CI 是对的

提交信息检查在开发机上放过了错误格式。根因是 C locale 下 `grep` 按字节处理中文
（详见 `scripts/check/lib/commit_msg.py` 顶部）。

**一个本地放行、只有 CI 才红的检查，比没有检查更糟**——它让人以为验证过了。
本仓库中文是常态，所以**凡是要匹配中文的判定一律交给 Python，不写进 shell 正则**。

### 一条速查

新加一个检查或不变量测试时，按顺序回答：

1. 我造的负例，**一定**会违反这条规则吗？（不是"大概会"）
2. 它红了吗？**红在我期望的那一行**吗？
3. 规则的说明文字、注释、文档，会不会让检查误判为通过？
4. 本地和 CI 的结果一致吗？（locale、shell 版本、grep 实现都可能不同）
