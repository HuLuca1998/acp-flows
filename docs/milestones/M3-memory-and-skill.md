# M3 · 记忆与 Skill

> 体系与编号规则见 [`README.md`](README.md)。开工前必读
> [`../domain-model.md`](../domain-model.md) §14 §15（规格）、
> [`../acp-field-notes.md`](../acp-field-notes.md) §4（**隔离与注入的已跑通方案**）与
> [`../open-questions.md`](../open-questions.md)（P2 / P3 里有多条挡着本里程碑）。

## 目标

**L2 项目记忆、L3 跨项目记忆、L4 Skill 三层完整可用：能落盘、能对账、能人工审核、
能按会话注入两端 Runtime、每一次注入都可点开追溯。**

## 完成标志

```bash
make check                                          # 全绿，覆盖率门槛达标
go test ./internal/domain/model/... -race -run 'TestMemory|TestSkill'
go test -tags=integration ./tests/integration/... -run 'TestMemoryReconcile|TestSkillInjection'
go test -tags=realruntime ./tests/contract/...      # 注入契约对 claude / codex / fake 同时成立
make check-i18n                                     # 记忆页 / Skill 页词条中英齐全
pnpm -C e2e test --grep '记忆候选审核|Skill 发布与回滚'
```

## 为什么 M3 是这个顺序

**记忆和 Skill 本质是同一件事：把东西塞进 Agent 的上下文。** 塞进去的前提是有一条
跑得完的执行链路，所以依赖 M2；三个子链路的内部顺序则由两条硬约束决定。

第一条：**md 是内容，DB 只存索引与状态。** 对账（`S3.1`）必须排在状态机（`S3.2`）之前。
索引与文件不一致时，后面每一条状态迁移都建立在一个说谎的索引上——
`active` 的记忆正文可能已经被人删了，而系统还在往上下文里塞它的 ID。

第二条：**绝不自动写入。** 人工审核闸门（`S3.2`）必须排在注入通道（`S3.5` / `S3.6`）之前。
先通道后闸门的顺序里，中间那段时间系统是「能自动写、只是还没接审核」的状态，
而这条约束在 `AGENTS.md` §9、设计规范第 07 节、`architecture.md` §4 被写死了三遍。

> 这是铁律 5 在存储层面的体现：**先保证每条结论都能回到它的依据，再让结论进入上下文。**

## 依赖

**M2**（主链路垂直切片）。注入发生在 Attempt 上，Attempt 与 Evidence 的采集在 M2 完成。

**与 M4 可并行**（不同 worktree）：M3 写 `features/{memory,skill}/`，M4 写
`features/{report,settings}/`，后端交集只有 `api/openapi.yaml` 与 `internal/store/migration/` 的编号。
两者都要改 `openapi.yaml` 时**串行合并**，不要并行改同一份 spec。

## 全局停止条件

触发任一条 **立刻停下来上报**，不要自行扩大范围：

- 撞上 `open-questions.md` **Q29**（L3 去标识化的具体脱敏规则未定）
- 撞上 **Q25**（记忆计数口径）/ **Q26**（`confidence` `sensitivity` 取值域）/
  **Q27**（`cand-07 → mem-203` 是否改写 ID）/ **Q28**（项目 Skill 与全局 Skill 同名优先级）
- 撞上 `domain-model.md` **OPEN-17**（三个隔离开关的默认值）或 **OPEN-8**（「Work 工作记忆」是不是
  第三种存储）—— 这两条 `open-questions.md` **尚未收录**，需要人先补一行编号再拍板
- 需要写 `~/.claude` 或 `~/.codex` 任何一个字节，或需要改动目标项目已有的 skill 目录
  （违反 `acp-field-notes.md` §4 三条红线）
- 需要给 Memory 或 Skill 加一条物理删除路径（违反 INV-MEM-6）
- 需要改 `api/openapi.yaml` 而当前单元没授权
- 需要引入未经批准的第三方依赖（frontmatter 解析、YAML 处理均属此列）

---

## 子计划 DAG

```
S3.1 记忆落盘与 DB↔文件对账 ★              S3.4 Skill 库与版本化
  │                                          │
  ▼                                          ▼
S3.2 记忆状态机与人工审核 ★                S3.5 Skill 注入通道与隔离开关 ★
  │                                          │
  ▼                                          │
S3.3 跨项目记忆 L3                           │
  └────────────────────┬─────────────────────┘
                       ▼
              S3.6 注入清单与追溯
                       ▼
              S3.7 记忆页与 Skill 页
```

**可并行**：`S3.1 → S3.2 → S3.3` 与 `S3.4 → S3.5` 是两条独立链，
文件不重叠（`domain/model/memory.go` 对 `domain/model/skill.go`，
`fsstore/memory*.go` 对 `fsstore/skill*.go`），从第一天就能并行开。
两条链在 `S3.6` 汇合，`S3.6` 之后不再并行。

---

## S3.1 · 记忆落盘与 DB ↔ 文件对账 ★

**阶段交付物**：记忆正文只存在于 md 文件、DB 只存索引；启动时能发现人手改动并上报差异。

> **这个子计划决定后面所有状态迁移可不可信。** 索引与文件不一致却静默过去，
> 等于把「这条记忆存在」建立在一个没人验证过的前提上。

### ○ U3.1.1 · 记忆 md 读写与 frontmatter 解析

| | |
|---|---|
| `goal` | 记忆正文以 md 落在 `<project>/.acpflows/memory/<id>.md`，frontmatter 与正文可无损往返，DB 侧不出现正文字段 |
| `allowed_changes` | `backend/internal/fsstore/memory_file.go` · `backend/internal/fsstore/frontmatter.go` 及其测试 · `backend/internal/store/entity/memory.go` · `backend/internal/store/mapper/memory.go` · `backend/internal/store/memory_repo.go` · `backend/internal/store/migration/NNNN_create_memory_index.sql` · `backend/internal/domain/model/memory.go` 的字段定义 |
| `forbidden_changes` | 不写状态机迁移（U3.2.1）；不碰 `~/.claude` `~/.codex`；不在 DB 里增加任何存正文的列 |
| `stop_conditions` | 撞上 Q26（`confidence` / `sensitivity` 取值域）——字段落库形态取决于它是枚举还是数值；frontmatter 解析需要新增第三方依赖 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | frontmatter 至少含 `id` / `kind` / `scope`，缺任一项读取失败 | 三个缺字段夹具，各断言 `ErrInvalidFrontmatter` 且错误信息含缺失字段名 |
| R2 | 正文往返无损 | 写入含中文、代码块、`---` 分隔线的正文，读回后逐字节相等 |
| R3 | **DB 索引结构中不存在正文字段**（INV-MEM-8） | 对 `memory` 表 `PRAGMA table_info` 取列名集合，断言与白名单常量完全相等；白名单里没有 `content` / `body` / `text` |
| R4 | `kind` 只接受 `constraint` / `experience` / `fact` | 穷举测试；传第四个值断言 `ErrInvalidKind` |
| R5 | 文件路径由注入的 `Paths` 解析，测试不触碰 `$HOME/.acpflows` | 用 `t.TempDir()` 跑全部用例；U0.1.2 的隔离守卫不触发 |
| R6 | 记忆没有物理删除路径（INV-MEM-6） | 反射断言 `fsstore` 与 `store` 的导出方法名集合里不含 `Delete` / `Remove` / `Purge` 前缀 |

**测试**：R2 用 golden 文件；R3 的列名白名单写成 `internal/constant` 里的常量，
**加一列而不改白名单时测试必须红**。

### ○ U3.1.2 · 启动对账与自愈上报

| | |
|---|---|
| `goal` | 启动时逐条比对 md 文件与 DB 索引，四类差异各有确定处置，且**任一侧都不被静默丢弃** |
| `allowed_changes` | `backend/internal/fsstore/reconcile.go` 及其测试 · `backend/internal/app/memory/reconcile_usecase.go` · `backend/tests/integration/memory_reconcile_test.go` |
| `forbidden_changes` | 对账**只读文件**：不改写、不删除任何 md；不自动补建缺失的 md 文件 |
| `stop_conditions` | 出现第五类差异（文件与索引都在但 `id` 冲突之外的情况）——说明模型漏了一种状态 |

**四类差异与处置**

| 差异 | 处置 |
|---|---|
| 有 md、无索引 | 建索引，状态取 frontmatter，产出一条 `orphan_file` 差异记录 |
| 有索引、无 md | 索引标记为不可注入，产出一条 `missing_file` 差异记录，**不删索引** |
| 两侧都有、frontmatter 与索引字段不一致 | 以 **md 为准**更新索引，产出一条 `drift` 差异记录 |
| 两个 md 声明同一个 `id` | 两条都不可注入，产出一条 `duplicate_id` 差异记录 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 四类差异各产出一条可读差异记录（INV-MEM-12） | 四个夹具场景，各断言差异记录的 `kind` 与受影响的 `id` |
| R2 | **索引不被静默丢弃**：`missing_file` 后索引仍可查 | 删掉 md → 跑对账 → 断言 `FindMemory(id)` 仍返回记录，且 `injectable == false` |
| R3 | **md 不被改写**：对账前后项目目录逐字节不变 | 对账前后对 `.acpflows/memory/` 做递归哈希，断言相等 |
| R4 | `drift` 以 md 为准 | 手改 md 的 `kind` → 对账 → 断言 DB 的 `kind` 跟着变，且差异记录含新旧值 |
| R5 | 对账是幂等的 | 连跑两次，断言第二次差异记录数为 0，且索引内容不变 |
| R6 | 对账失败不阻止 `duetd` 启动，但差异必须出现在系统状态里 | 制造 `duplicate_id` → 断言 `duetd serve` 正常起来且 `GET /v1/system/health` 的响应里带该差异计数 |

**测试**：整个子计划的测试跑在 `t.TempDir()` 里的临时项目 + 临时 SQLite（**不用 `:memory:`**，
见 `testing-strategy.md` §5）。R3 是这个单元最重要的一条——**对账是只读的**，
一个会「顺手修好文件」的对账等于把用户手改的内容悄悄覆盖掉。

---

## S3.2 · 记忆状态机与人工审核 ★

**阶段交付物**：候选记忆必须经过一次人的动作才能生效；失效的记忆不再注入，但历史仍可追溯。

### ○ U3.2.1 · Memory 聚合与四条迁移

| | |
|---|---|
| `goal` | `Memory` 聚合与状态机，只有 `candidate→active` · `candidate→discarded` · `active→invalid` · `active→obsolete` 四条迁移可达 |
| `allowed_changes` | `backend/internal/domain/model/memory.go` 及其测试 · `backend/internal/constant/memory.go` |
| `forbidden_changes` | `domain` 不得 import 任何内部包；不得出现 `context.Context` / `time.Now()`；不得提供任何 `Delete` 方法 |
| `stop_conditions` | 撞上 Q26（取值域）、Q27（候选 ID 是否改写）；撞上 `domain-model.md` OPEN-8（`scope` 枚举是否要加 `work`） |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 状态取值与 `domain-model.md` §14.3 一字不差 | 穷举测试对照 `constant.MemoryStatus*`，断言集合等于 `{candidate, active, invalid, obsolete, discarded}` |
| R2 | **穷举 status 组合，只有四条迁移返回 nil**（INV-MEM-4） | 5×5 表驱动，断言 25 个格子里恰好 4 个为 nil，其余为 `ErrInvalidTransition` 且错误含 from/to |
| R3 | **不存在把 Memory 直接创建为 `active` 的路径**（INV-MEM-2） | 反射遍历 `model` 包全部导出构造函数，断言返回值的 `Status()` 恒为 `candidate` |
| R4 | `source_refs` 为空或不指向 Evidence/Unit 时创建失败（INV-MEM-3） | 三个用例：空、指向 `work-08`、指向不存在的 `ev-999`，各断言 `ErrInvalidSourceRefs` |
| R5 | `Deprecate("")` 失败；`supersedes` 指向自身或构成环时失败（INV-MEM-7） | 三个用例，各断言对应错误；环的用例构造 `A→B→A` |
| R6 | **不存在 `DeleteMemory`**（INV-MEM-6） | 反射断言导出方法名集合，加一个 `Delete*` 方法时测试红 |
| R7 | 覆盖率 ≥ 90% | `make cover` |

**测试**：R2 是这个单元的核心——**新增一个状态而没有补迁移表时，25 个格子会变 36 个，测试必须红**。

### ○ U3.2.2 · 候选产出与人工确认

| | |
|---|---|
| `goal` | 记忆候选由证据驱动产出并落成 `memory_candidate` 事件；`candidate → active` 必须携带用户主体，不存在任何自动晋升路径 |
| `allowed_changes` | `backend/internal/app/memory/curate_usecase.go` 及其测试 · `backend/internal/api/handler/memory.go` · `api/openapi.yaml` 的 `Memory` 与 `/v1/projects/{projectId}/memories*` 部分 · `backend/tests/integration/memory_review_test.go` |
| `forbidden_changes` | 不实现晋升 L3（U3.3.2）；不在 `api` 层写状态判断；不改事件信封 schema |
| `stop_conditions` | 撞上 Q27——若候选 ID 需要改写为 `mem-*`，`source_refs` 与历史 `injection` 的引用重定向规则未定，不许自己拍板 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | **`Confirm()` 必须带用户主体**（INV-MEM-2） | 不传主体调用断言 `ErrActorRequired`；传空字符串同样失败 |
| R2 | **不存在自动晋升路径** | 用 Fake Runtime 跑完一整轮 Attempt（含 `end_turn`），断言该项目下 `active` 记忆条数不变、`candidate` 条数 +N |
| R3 | 聊天原文不得成为记忆正文（INV-MEM-3） | 把一段 `message_chunk` 原文当正文提交，断言 `ErrSourceRefsRequired`，且候选未落盘 |
| R4 | 产出候选时发出一条 `memory_candidate` 事件 | 订阅 SSE，断言收到的事件 `type == "memory_candidate"`、`source == "app"`、payload 含候选 ID 与 `source_refs` |
| R5 | 每次状态变更追加一条变更历史，历史无 setter、无 delete（INV-MEM-10） | 连做 `Confirm` → `MarkInvalid`，断言历史条数从 1 变 3；反射断言历史类型无导出 setter |
| R6 | `valid_from_commit` / `last_verified_commit` 不可解析时写入失败（INV-MEM-11） | 在夹具仓库里传一个不存在的 hash，断言 `ErrCommitNotFound` |
| R7 | 响应通过 `openapi.yaml` schema 校验 | `kin-openapi` 校验，接进 `backend/tests/contract/` |

**测试**：R2 是这条产品主张的唯一防线，**必须跑在集成层**（真实用例编排 + 临时 SQLite），
单测层的 R1 挡不住「用例编排里自己调了一次 `Confirm`」。

### ○ U3.2.3 · 失效、废弃与注入边界

| | |
|---|---|
| `goal` | `invalid` / `obsolete` 的记忆不进入**新** Attempt 的注入清单，但历史 Attempt 的 `injection` 记录仍能解析出它 |
| `allowed_changes` | `backend/internal/domain/policy/injection_scope.go` 及其测试 · `backend/internal/store/memory_repo.go` · `backend/tests/integration/memory_lifecycle_test.go` |
| `forbidden_changes` | 不实现完整注入选择策略（U3.6.1）——本单元只做「哪些进不去」；不删除任何历史记录 |
| `stop_conditions` | 发现历史 `injection` 记录只存了 ID 而解析不出当时的正文快照——说明 M2 的注入记录形态需要修，属于跨里程碑改动 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 置 `invalid` 后新注入清单不含它（INV-MEM-5） | 注入清单里断言该 ID 不出现；同一项目其他 `active` 记忆仍在 |
| R2 | **历史 Attempt 的 `injection` 仍返回它**（失效 ≠ 删除） | 先跑一次 Attempt 让它被注入 → 置 `invalid` → 查该 Attempt 的 `injection`，断言仍解析出该 ID 与当时的版本 |
| R3 | `obsolete` 同 R1 / R2 | 同样两条断言 |
| R4 | **项目记忆永不跨项目**（INV-MEM-1 / INV-PRJ-1） | 建 P1 / P2 两个项目夹具，对 P2 的 Attempt 计算注入清单，断言结果不含任何 `scope=P1` 的 ID |
| R5 | 注入统计随每次注入递增，且不因失效而清零 | 注入两次 → 置 `invalid` → 断言计数仍为 2 |

**测试**：R2 与 R4 各写一条集成测试。R4 是 `domain-model.md` 里被登记两次的不变量
（INV-MEM-1 与 INV-PRJ-1），**两处 ID 都要在测试名里出现**。

---

## S3.3 · 跨项目记忆 L3

**阶段交付物**：L3 记忆落在 `~/.acpflows/memory/`，晋升路径必须先去标识化再送人审。

### ○ U3.3.1 · L3 存储与检索边界

| | |
|---|---|
| `goal` | L3 记忆独立落盘与索引，检索时对全部项目可见，且不与 L2 的 `id` 空间冲突 |
| `allowed_changes` | `backend/internal/fsstore/memory_file.go` 的 scope 分派 · `backend/internal/store/memory_repo.go` · `backend/internal/app/memory/scope.go` 及其测试 |
| `forbidden_changes` | 不实现晋升流程（U3.3.2）；L3 目录的读写一律经 `platform.Paths`，不得出现 `os.UserHomeDir()` |
| `stop_conditions` | 撞上 `domain-model.md` OPEN-13（L0 / L1 是什么）——若层级编号还要扩，`scope` 枚举形态会变 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | L3 正文落在 `~/.acpflows/memory/<id>.md`，L2 落在 `<project>/.acpflows/memory/<id>.md` | 两条记忆各写一次，断言实际路径（`Paths` 注入到 `t.TempDir()`）与预期相等 |
| R2 | L3 对全部项目可见 | 建 P1 / P2 两个项目，对两者的 Attempt 各算一次注入清单，断言同一条 L3 记忆都在 |
| R3 | L2 的项目隔离不因 L3 的存在被破坏 | 同 U3.2.3 R4，断言在 L2+L3 混合场景下仍成立 |
| R4 | 测试不触碰真实 `$HOME/.acpflows` | 隔离守卫未触发；跑一遍带 `-race` 的全量用例 |
| R5 | L2 与 L3 出现同一个 `id` 时返回明确错误 | 断言 `ErrMemoryIDConflict` 且错误含两侧路径，**不静默取一边** |

### ○ U3.3.2 · 去标识化送审 ⛔ 阻塞

| | |
|---|---|
| `goal` | `PromoteToCrossProject()` 的送审载荷里不含任何项目标识，且晋升必须经过一次用户确认 |
| `allowed_changes` | `backend/internal/domain/policy/deidentify.go` 及其测试 · `backend/internal/app/memory/promote_usecase.go` · `api/openapi.yaml` 的晋升端点 |
| `forbidden_changes` | **不自行定义脱敏规则**；不在晋升时改写原 L2 记忆的正文；不删除原 L2 记忆 |
| `stop_conditions` | **开工即撞 `open-questions.md` Q29**（`architecture.md` §8 第 3 条）：去标识化的具体脱敏规则未定。项目名、绝对路径、仓库 remote、分支名、commit hash、内部标识符（`unit-012` / `ev-412`）各自是脱敏还是保留，逐项都要人拍板 |

**验收标准**（Q29 裁定后才可动工，这里先钉死与规则无关的结构性约束）

| # | 标准 | 断言 |
|---|---|---|
| R1 | **晋升必须经过一次用户确认**（INV-MEM-9） | 不带用户主体调用断言 `ErrActorRequired`；确认前断言 `~/.acpflows/memory/` 下没有新文件 |
| R2 | 送审载荷里不含项目标识（INV-MEM-9） | 断言载荷序列化后的字符串不含项目名、`path`、`remote`、默认分支名——**四项各一条断言** |
| R3 | 脱敏是纯函数，同一输入产出同一输出 | 表驱动跑 100 次，断言结果逐字节相同（无随机、无时间参与） |
| R4 | 原 L2 记忆不被改写、不被删除 | 晋升后断言原 md 逐字节不变，且 `FindMemory(l2id)` 仍返回 |
| R5 | Q29 未裁定时，晋升端点返回明确错误而不是「尽力脱敏」 | 断言返回 `ErrDeidentifyRuleUndefined`，错误信息指向 `open-questions.md` Q29 |

**测试**：R5 是**本单元在 Q29 裁定前唯一允许交付的行为**。
一个「先按感觉脱敏、以后再补规则」的实现会把用户的项目名散播到全局记忆库里，
而这类泄漏在发生之后无法回收。

---

## S3.4 · Skill 库与版本化

**阶段交付物**：Skill 可导入、可校验、可发布、可回滚，四类文件的语义边界被测试钉死。

### ○ U3.4.1 · 导入与四类文件语义

| | |
|---|---|
| `goal` | 导入 Skill 时复制到 `.acpflows/skills/` 并标记 `draft`，**原目录逐字节不变**；`SKILL.md` / `scripts/` / `references/` / `assets/` 四类文件各自的载入语义被区分开 |
| `allowed_changes` | `backend/internal/fsstore/skill_file.go` 及其测试 · `backend/internal/domain/model/skill.go` 的字段定义 · `backend/internal/app/skill/import_usecase.go` |
| `forbidden_changes` | 不实现注入（S3.5）；不移动源目录（复制而非移动，INV-SKL-6）；不改动目标项目已有的 skill 目录 |
| `stop_conditions` | 扫描 `**/skills` 时命中超过阈值的目录数（`node_modules` / `target` 已排除后仍爆量）——说明扫描范围规则需要重定 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 导入后 `status == draft`（INV-SKL-1） | 断言导入返回值与落库的 `status` 都是 `draft` |
| R2 | **导入后源目录内容逐字节不变**（INV-SKL-6） | 导入前后对源目录做递归哈希，断言相等 |
| R3 | 扫描跳过 `node_modules` / `target` | 夹具里在两处各放一个 `skills/`，断言扫描结果不含它们 |
| R4 | 四类文件被分别归类，缺 `SKILL.md` 即失败 | 断言归类结果的四个集合与夹具一致；删掉 `SKILL.md` 断言 `ErrSkillMainDocMissing` |
| R5 | `assets/` 永不进入指令上下文（INV-SKL-7） | 断言归类结果里 `assets` 集合的 `context_role` 恒为 `never` |
| R6 | 引用语法 `skill:<name>@<version>` 可解析、可往返 | 表驱动：合法三例、非法三例（缺 `@`、版本非法、name 含空格），各断言解析结果或错误 |

### ○ U3.4.2 · frontmatter 校验与状态机

| | |
|---|---|
| `goal` | frontmatter 校验不过的 Skill 不能置 `active`，且失败原因可读；`draft → active → deprecated` 三态迁移穷举可测 |
| `allowed_changes` | `backend/internal/domain/model/skill.go` 及其测试 · `backend/internal/constant/skill.go` · `backend/internal/domain/policy/skill_validate.go` |
| `forbidden_changes` | `domain` 不得做 IO；校验器不得读环境（`compatibility` 的实际检测在 U3.4.3）；不得静默拒绝 |
| `stop_conditions` | frontmatter 出现 `name` / `description` / `compatibility` 之外的必需字段——需求扩大，回去改规格 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 缺 `name` 或缺 `description` 时校验失败（INV-SKL-1） | 两个夹具各断言 `Activate()` 返回错误，且 `status` 仍为 `draft` |
| R2 | **校验失败带非空 `validation_error`**（INV-SKL-2） | 断言 `validation_error == "frontmatter 缺 description"`——照 `domain-model.md` §15.1 的原文 |
| R3 | 穷举三态迁移，只有 `draft→active` · `active→deprecated` · `deprecated→active`（回滚）返回 nil | 3×3 表驱动，断言 9 个格子里恰好 3 个为 nil |
| R4 | 新增状态而未处理时测试红 | 穷举测试覆盖 `IsValid()`，加第四个状态时格子数变 16 |
| R5 | 覆盖率 ≥ 90% | `make cover` |

**测试**：R2 的断言值直接抄规格原文。**校验器给不出可读原因等于没有校验**——
用户面对的是「这个 Skill 就是用不了」，无从修。

### ○ U3.4.3 · 版本发布、回滚与兼容性检测

| | |
|---|---|
| `goal` | 同一 `name` 任一时刻至多一个 `active` 版本；发布不删旧版本，回滚 = 把旧版本重新置 `active`；`compatibility` 由环境检测校验 |
| `allowed_changes` | `backend/internal/app/skill/publish_usecase.go` 及其测试 · `backend/internal/store/skill_repo.go` · `backend/internal/platform/toolchain.go` · `backend/tests/integration/skill_version_test.go` |
| `forbidden_changes` | 不删除任何版本目录；环境检测不得执行 Skill 自带的 `scripts/`（那是权限裁决的事，见 U3.5.1） |
| `stop_conditions` | `compatibility` 表达式需要超出「工具名 + 版本比较」的语法（如逻辑组合）——语法要先定 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | **同名 Skill 的 `active` 版本数 ≤ 1**（INV-SKL-3） | 连发 `1.0` → `2.0`，断言查询 `active` 返回恰好 1 条且版本为 `2.0` |
| R2 | 发布新版本后旧版本仍可读（INV-SKL-4） | 断言 `1.0` 的目录仍在，且 `FindSkill(name, "1.0")` 返回记录 |
| R3 | 回滚后旧版本 `active`、新版本非 `active`（INV-SKL-4） | 回滚到 `1.0`，断言两条记录的状态；断言 `2.0` 的目录**未被删除** |
| R4 | `compatibility` 不满足时该 Skill 不进注入清单（INV-SKL-9） | 用 `cargo ≥ 99.0` 的夹具 + 注入的假工具链探测器，断言注入清单不含它 |
| R5 | 环境检测走注入的 port，测试零外部进程 | 断言测试进程未 spawn 任何子进程（`platform.Proc` 的假实现记录调用） |
| R6 | 缺工具链只影响该 Skill，不阻断 Work（INV-PRJ-5） | 断言在 R4 场景下 `CreateWork` 仍成功 |

---

## S3.5 · Skill 注入通道与隔离开关 ★

> **本子计划的方案已经在真机上跑通，直接照抄 [`../acp-field-notes.md`](../acp-field-notes.md) §4，
> 不要重新设计。** 那一节里的每条结论都是 B 级实测（`claude-agent-acp 0.63.0` ·
> `codex-acp 1.1.7`），偏离它就是把已经付过学费的坑再踩一遍。

**阶段交付物**：Skill 通过会话参数注入两端 Runtime，三个隔离开关有确定的实现映射。

### ○ U3.5.1 · skill 分发目录与两端注入

| | |
|---|---|
| `goal` | 把 `active` 且兼容的 Skill 复制到一个**独立的分发目录**，claude 走 `_meta.claudeCode.options.plugins`、codex 走 `additionalDirectories` 注入 |
| `allowed_changes` | `backend/internal/acp/adapter/claude/**` · `backend/internal/acp/adapter/codex/**` · `backend/internal/acp/adapter/base/**` · `backend/internal/fsstore/skill_dist.go` 及其测试 · `backend/tests/contract/runtime_contract_test.go` |
| `forbidden_changes` | **不用 cwd 隐式项目发现**（`.codex/skills`）—— 实测不稳定，见 §4；不把分发目录指向 Duet 自己的代码目录或 worktree；`internal/{app,domain,api}` 不得出现 runtime 名字 |
| `stop_conditions` | 真机复验发现 `plugins` 或 `additionalDirectories` 已不被接受（对应 M0 U0.3.2 R6 的裁定失效）→ 先更新 `acp-field-notes.md` §7 裁定表再继续 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | claude 侧注入走 `plugins` | 对发出的 `session/new` JSON 断言 `_meta.claudeCode.options.plugins[0].type == "local"` 且 `.path` 等于分发目录绝对路径 |
| R2 | codex 侧注入走 `additionalDirectories` | 断言 `additionalDirectories` 含分发目录与 worktree 路径两项 |
| R3 | **codex 的 `session/new.mcpServers` 恒为空数组**（§4 硬规则 2） | 断言该键存在且长度为 0；非空时 codex-acp 会整体覆盖 thread config 的 `mcp_servers`，禁用条目全部丢失 |
| R4 | **不依赖 cwd 隐式发现** | 断言分发目录不等于 `cwd`，且不是 `cwd` 的子目录 |
| R5 | 分发目录只含 Skill 内容 | 断言目录下的顶层项集合等于 `active` Skill 的 `name` 集合；断言不含 `.git` / `go.mod` / `package.json` |
| R6 | `deprecated` 的 Skill 不出现在新注入清单，历史注入仍可解析（INV-SKL-5） | 两条断言，手法同 U3.2.3 R2 |
| R7 | 上层零品牌判断 | `grep -rn 'plugins\|additionalDirectories' backend/internal --include=*.go` 的命中只落在 `internal/acp/adapter/`，接进 `scripts/check-naming.sh` |
| R8 | 注入契约对 claude / codex / fake 三个实现同时成立 | 扩展 M0 U0.8.2 的表驱动契约测试，**断言里零 `if impl ==`** |
| R9 | `scripts/` 的执行触发一次权限裁决（INV-SKL-8） | 用 Fake Runtime 发起一次 `scripts/run_tests.sh` 调用，断言收到一次 `session/request_permission` |

**测试**：R1–R4 全部对着发出的 JSON 做 golden 断言（`acp-integration.md` §13.2 的手法）。
R8 复用 M0 已建好的一致性契约测试骨架——**注入是最容易在两端悄悄分叉的地方**。

### ○ U3.5.2 · 三个隔离开关的实现映射 ⛔ 默认值待定

| | |
|---|---|
| `goal` | 设置页的三个隔离开关各自映射到确定的进程环境变量与 `session/new` 协议参数，且映射对两端都有测试 |
| `allowed_changes` | `backend/internal/acp/adapter/claude/isolation.go` · `backend/internal/acp/adapter/codex/isolation.go` · `backend/internal/acp/runtime/env.go` · `backend/internal/domain/model/runtime.go` 的开关字段 · 对应测试 |
| `forbidden_changes` | **三条红线**（§4）：不写 `~/.codex` / `~/.claude` 一个字节；不影响用户终端直接用 codex / claude；不修改或覆盖目标项目的 skill。隔离只有两类载体——spawn 时的进程环境变量 + `session/new` 的协议参数 |
| `stop_conditions` | 撞上 `domain-model.md` **OPEN-17**（三个开关的默认值未定，`open-questions.md` 尚未收录此条）；发现 `CODEX_HOME=受控目录` 被重新提出 —— 该方案**已否决**（实测会写满几千个文件） |

**开关 → 实现的映射（照抄 `acp-field-notes.md` §4）**

| 开关 | claude 侧 | codex 侧 |
|---|---|---|
| 关闭 Runtime 机器级记忆 | `settingSources: ["project"]`（不含 `user`）+ 环境变量 `CLAUDE_CODE_DISABLE_AUTO_MEMORY=1` | `CODEX_CONFIG` 的 `skills.config` 逐个 `enabled: false` |
| 禁用未授权项目 MCP Server | `strictMcpConfig: true` | `CODEX_CONFIG` 的 `mcp_servers` 逐个 `enabled: false` |
| 允许 Runtime 内建 Skill | **无法关闭** | **无法关闭** |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 「关闭机器级记忆」开启时，claude 的 `settingSources` 恰为 `["project"]` | 断言数组值；断言**不含** `"user"` |
| R2 | 同上，spawn 环境含 `CLAUDE_CODE_DISABLE_AUTO_MEMORY=1` | 断言子进程环境变量表里该键的值 |
| R3 | 「禁用未授权项目 MCP」开启时 `strictMcpConfig == true` | 对 `session/new` JSON 断言。**这条是安全约束**：project 档下目标项目能用 `enableAllProjectMcpServers: true` 给自己的 MCP 自动放行 |
| R4 | codex 侧 Duet 注入的 MCP 只出现在 `CODEX_CONFIG`，不出现在 `session/new` | 两条断言：`CODEX_CONFIG.mcp_servers.duet` 存在；`session/new.mcpServers` 长度为 0 |
| R5 | `skills.config` **只用 `name` 选择器**，且 `name` 取 SKILL.md frontmatter 的值 | 夹具的目录名与 frontmatter `name` 故意不同，断言下发的是 frontmatter 值；断言载荷里不出现 `path` 选择器 |
| R6 | **`~/.claude` 与 `~/.codex` 逐字节不变** | 起一轮完整会话前后对两个目录做递归哈希，断言相等（跑在临时 `HOME` 下） |
| R7 | 「允许 Runtime 内建 Skill」关闭时**必须显式降级并上报**，界面不得声称已关闭 | 断言返回一条能力降级记录（INV-RT-1），且该开关的状态查询返回 `unsupported` 而不是 `disabled` |
| R8 | 禁用只在本会话生效，不持久化 | 会话结束后重跑一次不带隔离的会话，断言机器级条目重新出现 |

**测试**：R6 与 R8 跑在集成层，`HOME` 指向 `t.TempDir()`。
R7 直接对应 `acp-field-notes.md` §1 的那条教训——**界面说了一件实现里没有的事，就是硬性错误**。
「允许内建 Skill」这个开关两端都关不掉，做成一个能关的开关等于当面撒谎。

---

## S3.6 · 注入清单与追溯

**阶段交付物**：每一轮 Attempt 注入了哪些记忆与哪些 Skill 可查、可点开、可核对。

### ○ U3.6.1 · 注入选择策略

| | |
|---|---|
| `goal` | 给定一个 Attempt，产出一份确定的、可复现的注入清单，选择规则全部在 `domain/policy` 里 |
| `allowed_changes` | `backend/internal/domain/policy/injection.go` 及其测试 · `backend/internal/app/attempt/inject_usecase.go` |
| `forbidden_changes` | `policy` 不做 IO；**不得为同名冲突静默取一边**（INV-SKL-10）；不得把 `candidate` 状态的记忆放进清单 |
| `stop_conditions` | 撞上 Q28（项目 Skill 与全局 Skill 同名优先级）；撞上 Q26（`confidence` 是否参与注入排序） |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 清单可复现：同一输入产出同一顺序 | 同一夹具跑 100 次，断言 ID 序列逐次相同（无 map 迭代顺序泄漏） |
| R2 | 只有 `active` 记忆进清单 | 造 5 种状态各一条，断言清单长度为 1 且是 `active` 那条 |
| R3 | 只有 `active` 且 `compatibility` 满足的 Skill 进清单 | 四个组合（active/deprecated × 兼容/不兼容），断言只有一个进 |
| R4 | **项目 Skill 与全局 Skill 同名时返回明确错误**（INV-SKL-10） | 断言 `ErrSkillNameConflict`，错误信息含两侧路径并指向 `open-questions.md` Q28；**断言它不是「取了其中一个」** |
| R5 | 跨项目边界（INV-MEM-1）在 policy 层就成立 | 直接给 policy 喂 P1+P2 的记忆，断言输出只含本项目与 L3 |
| R6 | 覆盖率 ≥ 90% | `make cover` |

### ○ U3.6.2 · `injection` 事件与追溯入口

| | |
|---|---|
| `goal` | 每轮注入落一条 `injection` 事件，事件里的每个 ID 都能点开到对应的记忆或 Skill 版本 |
| `allowed_changes` | `backend/internal/app/attempt/inject_usecase.go` 的事件发布 · `api/openapi.yaml` 的 `/v1/attempts/{attemptId}/injection` · `frontend/src/features/conversation/events/InjectionEvent.tsx` 及其测试 |
| `forbidden_changes` | 不新增第 14 类事件（`injection` 已在 13 类枚举内）；不改事件信封 schema；不在事件 payload 里塞记忆正文 |
| `stop_conditions` | 追溯需要的字段在 M2 的 `injection` 记录里不存在 —— 属于跨里程碑改动，停下来 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 每轮注入恰好一条 `injection` 事件 | 跑一轮 Attempt，断言该 `type` 的事件计数为 1 |
| R2 | payload 含记忆 ID 与 `skill:<name>@<version>` 引用，**不含正文** | 断言 payload 的键集合；断言序列化结果不含记忆正文里的特征串 |
| R3 | 每个 ID 可解析到具体对象与版本 | 逐个 ID 调追溯端点，断言返回 200 且版本号与注入时一致 |
| R4 | 记忆事后被置 `invalid` 后，历史事件仍可解析（INV-MEM-5） | 置 `invalid` → 重查该事件的每个 ID，断言仍返回 200 |
| R5 | 前端渲染为灰色单行 + ID 芯片，点击跳记忆页 | Vitest + Testing Library：`getByRole('button')` 取芯片，断言点击后路由为 `memory`，**不断言 class 名** |
| R6 | 响应通过 `openapi.yaml` schema 校验 | `kin-openapi`，接进 `backend/tests/contract/` |

**测试**：R2 是「md 是内容、DB 只存索引」在事件层的延伸——
**事件 payload 里一旦有了正文，正文就有了第二个真源**。

---

## S3.7 · 记忆页与 Skill 页

**阶段交付物**：记忆的审核与失效、Skill 的发布与回滚在界面上可完成。

### ○ U3.7.1 · 记忆页（L2 / L3）

| | |
|---|---|
| `goal` | 记忆列表、筛选、候选审核、标记失效、废弃、晋升 L3 六个动作在界面上可完成，每个动作都有证据入口 |
| `allowed_changes` | `frontend/src/features/memory/**` · `frontend/src/i18n/locales/{zh-CN,en-US}.json` 的 `memory.*` 词条 · `frontend/tests/INDEX.md` |
| `forbidden_changes` | 不在 `features/` 里定义设计令牌；不写死 hex 或裸 px；不硬编码用户可见文本；不为找不到条目的元素临时发明样式（铁律 3） |
| `stop_conditions` | 撞上 Q25（`全部 12` 与 `9+2+3=14` 对不上，筛选条计数口径未定）；撞上 Q15（文本输入框无条目）/ Q16（表格无条目）/ Q17（空状态与骨架屏无条目） |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 候选必须点「确认」才变 `active`，界面上无任何自动路径 | Playwright：进入页面 → 断言候选仍是 `candidate` → 点确认 → 断言状态变 `active`；**中间不刷新页面也不等待** |
| R2 | 状态词原样英文等宽，不翻译（`i18n.md` §2） | 切到 `en-US` 与 `zh-CN` 各断言一次：DOM 里的状态文本恒为 `candidate` / `active` / `invalid` / `obsolete` |
| R3 | 「标记失效」的按钮文案写清后果（`AGENTS.md` §8） | 断言按钮可访问名不是「确定」「提交」这类空动词 |
| R4 | 每条记忆有证据入口 | 断言每行都有指向 `source_refs` 的可点元素，点击后抽屉打开且含 `ev-*` 编号 |
| R5 | 晋升 L3 前展示去标识化后的送审内容 | 断言预览区文本不含项目名与路径（与 U3.3.2 R2 同源） |
| R6 | 词条中英同进同退 | `make check-i18n` 绿 |
| R7 | 组件测试按行为查询 | `grep -rn 'data-testid\|querySelector' frontend/src/features/memory` 结果为空 |

**测试**：R1 走 Playwright（`e2e/`），其余走 Vitest。
**Q25 未裁定前，筛选条的「全部」计数不实现**——显示一个口径不明的数字比不显示更糟。

### ○ U3.7.2 · Skill 页（L4）

| | |
|---|---|
| `goal` | Skill 列表、导入、校验错误展示、发布、回滚、按需查看四类文件在界面上可完成 |
| `allowed_changes` | `frontend/src/features/skill/**` · `frontend/src/i18n/locales/{zh-CN,en-US}.json` 的 `skill.*` 词条 · `frontend/tests/INDEX.md` |
| `forbidden_changes` | 同 U3.7.1；不在前端做 frontmatter 校验（校验是后端的事，前端只展示 `validation_error`） |
| `stop_conditions` | 撞上 Q16（表格无条目）/ Q17（空状态无条目）；撞上 Q28（同名优先级）——冲突时界面要显示什么取决于裁定 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 校验失败的 Skill 展示后端给的 `validation_error` 原文 | 断言页面文本含 `frontmatter 缺 description`；断言前端代码里没有第二份校验逻辑（`grep` 断言 `features/skill` 下不出现 `description` 的存在性判断） |
| R2 | 版本状态词不翻译 | 中英两版各断言文本恒为 `draft` / `active` / `deprecated` |
| R3 | 回滚后旧版本 `active`、新版本仍在列表里 | Playwright：回滚后断言两条版本行都在，状态各自正确 |
| R4 | 四类文件分区展示，`scripts/` 标注需权限放行 | 断言四个分区标题存在；断言 `scripts/` 分区带权限提示文本 |
| R5 | `hit_count` 与 `命中 46` 一类的等宽数据不走 i18n（`i18n.md` §5 例外） | 断言该处未调 `t()`，且用 `--font-mono` |
| R6 | 词条中英同进同退 | `make check-i18n` 绿 |

---

## M3 验收

**全部单元 `✓` 之外，还要满足：**

| # | 标准 | 怎么验 |
|---|---|---|
| A1 | `make check` 全绿，覆盖率达标 | CI |
| A2 | **绝不自动写入**：跑完一整条 E2E 黄金路径，`active` 记忆条数不因任何 Agent 行为增加 | `pnpm -C e2e test`，断言前后计数 |
| A3 | **失效 ≠ 删除**：置 `invalid` 的记忆仍能从历史 Attempt 的 `injection` 解析出来 | `go test -tags=integration ./tests/integration/... -run TestMemoryLifecycle` |
| A4 | **DB ↔ 文件对账**四类差异各有一条集成测试，且对账过程只读 | 递归哈希断言 + `make check-test-index` 确认四条都已登记 |
| A5 | 注入契约对 claude / codex / fake 三个实现同时通过 | `go test -tags=realruntime ./tests/contract/...` 本地跑 |
| A6 | `~/.claude` 与 `~/.codex` 在全量测试前后逐字节不变 | 跑全量测试前后各做一次递归哈希 |
| A7 | `domain-model.md` §17.7 的 **INV-MEM-1..12 与 INV-SKL-1..10 共 22 条，每条都有一个测试名带 ID 的断言** | `grep -c 'INV-MEM-\|INV-SKL-' backend/tests/INDEX.md` 与逐条核对 |
| A8 | 被 open-questions 挡住的单元（U3.3.2）**没有绕过裁定偷偷实现** | 断言 `ErrDeidentifyRuleUndefined` 仍是该端点在 Q29 裁定前的唯一行为 |

**A7 是 M3 真正的验收标准。** 记忆与 Skill 这两个聚合的价值全在不变量上——
一条能被自动写入的记忆、一个能被物理删除的历史，比没有这个功能更糟。
