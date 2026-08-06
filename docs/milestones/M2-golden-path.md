# M2 · 主链路垂直切片

> 体系与编号规则见 [`README.md`](README.md)。动手前必读
> [`../domain-model.md`](../domain-model.md)（**主要规格来源**，115 条不变量）、
> [`../architecture.md`](../architecture.md) §4（13 类事件封闭枚举）、
> [`../frontend-guide.md`](../frontend-guide.md) §9（渲染器注册表）、
> [`../i18n.md`](../i18n.md)、[`../open-questions.md`](../open-questions.md) P2。

## 目标

**一条端到端跑通的真实工作流：从纳管一个本地 git 仓库，到一个 `Unit` 被独立会话审查验收、
自动提交到工作分支并落 `Checkpoint`。**

```
创建项目 → 新建工作(切 worktree) → 需求澄清 → 计划冻结 → 单元契约冻结
→ Codex 执行 → 权限裁决 → 证据采集 → 独立审查 → 验收 → 自动提交 → 检查点落盘
```

这条链上的每一步都必须**由应用的状态机驱动**，不是由 Agent 的自述驱动。
Agent 只产生 diff 与消息；「跑到哪一步了」「算不算过」全部由 `duetd` 判定。

## 完成标志

```bash
make check                              # 全绿
make -C backend cover                   # domain/model + domain/policy ≥ 90%
cd backend && go test -tags=integration ./tests/...
pnpm -C frontend test --run
pnpm -C e2e test                        # 黄金路径 spec 绿
make check-event-enum                   # 13 类事件四处一致
make check-evidence-collector           # 不存在以 Agent 文本创建 Evidence 的路径
make check-no-auto-memory               # 不存在自动写入 Memory 的路径
make check-i18n
```

## 为什么这个顺序

**契约先行是本里程碑唯一能让前后端并行的手段。** M2 是第一个前后端都要动的里程碑：
后端 8 个子计划、前端 4 个子计划。它们唯一的交汇点是 `api/openapi.yaml`。
spec 一冻结，两条链的写入路径就完全不重叠（`backend/**` vs `frontend/**`），
前端对着由 spec 生成的 MSW handler 开发，不等后端。
反过来做——先写 handler 再补 spec——两条链立刻串行，M2 的工期翻倍。这是铁律 2 的直接后果。

后端链内部的顺序由**数据依赖**决定，不是由「先做简单的」决定：
没有 `Project` 就没有 worktree；没有冻结的 `UnitContract` 就没有合法的 `Attempt`；
没有 `Attempt` 就没有 `Evidence`；没有 `Evidence` 就没有验收。
链条上任何一环允许「先绕过去、以后补」，验收就会退化成 Agent 自评——
那正是本产品要消灭的东西。

事件投影（S2.8）排在证据与验收之后：13 类事件是**领域动作的投影**，
领域动作没定完就去定投影，等于让展示层反过来定义业务语义。

## 依赖

**M0 全部单元 `✓`**。M2 直接建立在 M0 的 Fake Runtime、会话生命周期、两段式取消、
能力矩阵、`Work` 状态机骨架、`PlanVersion` append-only 骨架、SSE 骨架之上。

M1 与 M2 无依赖关系，交集只有 `POST /v1/system/update/prepare` 的完整语义
（M1 1.7 只做「无进行中工作时直接放行」，暂停与落检查点在本里程碑的 U2.7.4 补全）。

## 全局停止条件

触发任一条 **立刻停下来上报**，不要自行扩大范围：

- `open-questions.md` **P0 的 Q1（`initializing` / `initializing_failed` 未进术语表）
  或 Q3（`executing` 与 `waiting_user` 能否共存）在 M0 收尾时仍未裁决** → M2 不开工
- `open-questions.md` **P2 的 Q7–Q13** 中，当前单元 `stop_conditions` 引用的任一条未裁决 → 停
- 前端撞上 **Q14–Q24** 的设计缺口 → 停（铁律 3：设计规范找不到条目就不许实现）
- 需要新增**第 14 类事件** → 停，按 `frontend-guide.md` §9.5 走四处流程，并判定为 `contract_revision`
- 需要改 `api/openapi.yaml`，而当前单元不属于 S2.1 → 停
- 需要让 Agent 的文本成为 `Evidence` 的内容来源 → 停（无例外，见 U2.7.1）
- 需要自动创建或晋升 `Memory` → 停（无例外，见 U2.13.2）
- 需要引入未经批准的第三方依赖
- `domain` 层需要 import 基础设施包或做 IO

---

## 子计划 DAG

```
                                M0 全部单元 ✓
                                      │
                                      ▼
                    ┌──────────────────────────────────┐
                    │  S2.1 · 契约先行  ◆               │  ◆ 唯一改 api/openapi.yaml
                    │  openapi.yaml → make gen         │     的子计划
                    └────────┬───────────────┬─────────┘
                             │               │
     ═══ 后端链 ■ ═══════════╡               ╞═══════════ 前端链 ▲ ═══
                             │               │
        ┌──────────┬─────────┴──┐            ▼
        ▼          ▼            ▼       S2.9 前端地基 ▲
   S2.2 项目管理  S2.4 角色与  S2.5 需求快照·   布局·客户端·SSE·i18n
        ■         Runtime ■   计划族·契约 ■         │
        │           │            │        ┌─────────┼─────────┐
        ▼           │            │        ▼         ▼         ▼
   S2.3 worktree ■  │            │    S2.10 ▲   S2.11 ▲   S2.12 ▲
        │           │            │    对话页+    计划面板·  项目与
        │           │            │    13 类      契约抽屉·  新建工作
        └───────────┴─────┬──────┘    渲染器     证据抽屉
                          ▼               │         │         │
              S2.6 执行编排与权限裁决 ■     └────┬────┴─────────┘
                          │                     │
              ┌───────────┴───────────┐         │
              ▼                       ▼         │
   S2.7 证据·审查·验收·提交 ■   S2.8 事件投影 ■   │
              └───────────┬───────────┘         │
                          │                     │
                          └──────────┬──────────┘
                                     ▼
                    S2.13 · E2E 黄金路径与常驻守卫 ●
```

**图例**：`◆` 契约先行（两条链的共同前提）· `■` 纯后端 · `▲` 纯前端 · `●` 跨端

### 哪些可以并行

| 关系 | 说明 |
|---|---|
| **后端链 ↔ 前端链** | S2.1 冻结后完全并行。写入路径不重叠：后端只碰 `backend/**` `scripts/check-*.sh`，前端只碰 `frontend/**`。前端对着 U2.1.3 由 spec 生成的 MSW handler 开发，**不等后端实现** |
| S2.2 · S2.4 · S2.5 | 三条从 S2.1 起可并行：分别落在 `domain/model/project.go`、`domain/model/role.go`、`domain/model/{plan,subplan,unit,unit_contract}.go`，无文件交集 |
| S2.7 · S2.8 | 从 S2.6 起可并行：证据与验收落 `domain/model/{evidence,checkpoint}.go` + `gitx/`，事件投影落 `eventbus/`，无文件交集 |
| S2.10 · S2.11 · S2.12 | 从 S2.9 起可并行：各自独占一个 `frontend/src/features/` 子目录 |

**不可并行**：S2.3 依赖 S2.2（没有 `Project` 就没有 worktree 的落点）；
S2.6 依赖 S2.3 + S2.4 + S2.5（派发一个 `Attempt` 同时需要 worktree、`Role`、冻结的 `UnitContract`）。

### 与 `roadmap.md` M2 清单的对应

| roadmap | 拆成 |
|---|---|
| — | S2.1（契约先行，roadmap 未单列，但它是并行的前提） |
| 2.1 项目管理 | S2.2 |
| 2.2 worktree 生命周期 | S2.3 |
| 2.3 角色与 Runtime 绑定 | S2.4 |
| 2.4 计划 / 子计划 DAG / 单元契约 | S2.5 |
| 2.5 对话页 + 13 类事件渲染器 + 过滤器 | S2.9 · S2.10 |
| 2.6 计划面板、契约抽屉、证据抽屉 | S2.11 |
| 2.7 决策 D1–D3 | S2.6（后端）· S2.11（前端 D2/D3 卡片） |
| 2.8 证据采集 + 检查点 + 自动提交 | S2.7 · S2.8 |
| 2.9 E2E 黄金路径 | S2.13 |
| — | S2.12（项目与新建工作的界面，roadmap 2.1/2.2 只写了后端） |

---

## 被待拍板问题挡住的单元

**下表每一行都是硬阻塞。** 撞上就停，不要猜——猜错会让整条链上的测试建立在错误前提上。

### P0（M0 收尾时必须清空，否则 M2 不开工）

| # | 问题 | 挡住 |
|---|---|---|
| Q1 | `initializing` / `initializing_failed` 未进术语表 | U2.1.1 · U2.3.1 · U2.12.2 |
| Q3 | 同一个 `Work` 能否同时是 `executing` 和 `waiting_user` | U2.6.2 · U2.6.3 · U2.11.3 |

### P2 · 领域侧（Q7–Q13）

| # | 问题 | 挡住 |
|---|---|---|
| Q7 | `acceptance_criteria` 是不是 `UnitContract` 的字段 | U2.1.1 · U2.5.3 · U2.7.3 · U2.11.2 |
| Q8 | `contract_revision` 期间 `Work` 处于哪个状态 | U2.1.1 · U2.5.3 · U2.7.2 |
| Q9 | `UnitContract` 谁产出：计划架构师还是单元设计师 | U2.4.1 · U2.5.3 |
| Q10 | `Unit` / `Subplan` 的完整状态集 | U2.1.1 · U2.5.2 |
| Q11 | `superseded` 同时是 `Attempt` 状态和 `Evidence` 状态 | U2.1.2 · U2.6.1 · U2.7.1 |
| Q12 | `Attempt` 缺「已结束待审查」状态 | U2.1.2 · U2.6.1 · U2.7.2 |
| Q13 | Runtime 是二值枚举还是注册表 | U2.1.1 · U2.4.2 |

### P2 · 设计缺口（Q14–Q24，挡住前端）

| # | 缺口 | 挡住 |
|---|---|---|
| Q14 | 设置页 tab 选中态与规范第 08 节矛盾 | U2.12.1 |
| Q15 | 文本输入框 / 文本域无条目 | U2.10.3 · U2.12.2 |
| Q16 | 表格无条目 | U2.12.1 |
| Q17 | 空状态 / 骨架屏 / 错误态 / SSE 断线提示无条目 | U2.9.2 · U2.10.3 · U2.12.1 |
| Q18 | 数据图表无条目 | **M2 内无落点**（报表页在 M4 4.1）。撞上即说明范围跑偏 |
| Q19 | toast / 内联反馈无条目也无 z-index 层 | U2.9.1（Web 降级依赖它） |
| Q20 | 动效 / 过渡全文零规定 | U2.11.1 · U2.11.2 · U2.11.3（抽屉「右侧滑入」、加载圆环） |
| Q21 | 搜索 / 命令面板（⌘K）无面板设计 | U2.9.1（窗口栏已有该按钮） |
| Q22 | 敏感值掩码无规格 | **M2 内无落点**（GitHub 账号页在 M4 4.3） |
| Q23 | 2px/3px/6px 圆角无令牌；`--color-accent-500`/`-200` 未列入用法表 | U2.9.1 · U2.10.3 |
| Q24 | 面包屑无视觉条目 | U2.12.1 |

### P3（可按推定实现，撞上冲突再停）

| # | 问题 | 相关单元 | 本里程碑的推定 |
|---|---|---|---|
| Q30 | `~/.duet/worktrees` 与 `~/.acpflows` 两个顶层目录 | U2.3.1 | 按 `domain-model.md` §4 取 `~/.duet/worktrees`，路径来自全局设置项 `worktree_root` |
| Q31 | 多 `Work` 并发上限 | U2.3.1 · U2.6.1 | 上限作为配置项存在，默认值取 M0 U0.2.2 R5 的会话上限 |
| Q28 | 项目 Skill 与全局 Skill 同名优先级 | U2.2.3 | 本里程碑不做全局 Skill 库，同名场景不可达 |

---

## 不变量分配（`domain-model.md` §17 的 115 条）

**M2 覆盖 90 条，M3 覆盖 22 条，M0 已覆盖 3 条。** 每条不变量必须能指认出唯一的收口单元；
指不出来说明单元还没拆对。

| 不变量 | 条数 | 收口单元 |
|---|---|---|
| INV-PRJ-1 | 1 | **→ M3**（记忆注入清单的检索边界） |
| INV-PRJ-2 · -3 | 2 | U2.2.1 |
| INV-PRJ-4 · -5 | 2 | U2.2.2 |
| INV-PRJ-6 | 1 | U2.2.3 |
| INV-WORK-1 | 1 | 守卫按迁移号分布于 U2.3.1（#1–#3）· U2.6.1（#8）· U2.6.3（#5 #7 #10 #17 #19–#22）· U2.7.3（#9 #13–#16）；**穷举断言在 U2.7.3 收口** |
| INV-WORK-2 | 1 | U2.7.3 |
| INV-WORK-3 | 1 | U2.7.3 |
| INV-WORK-4 | 1 | U2.6.1 |
| INV-WORK-5 | 1 | U2.6.3 |
| INV-WORK-6 | 1 | U2.6.4 |
| INV-WORK-7 | 1 | U2.7.4 |
| INV-WT-1 · -2 · -6 | 3 | U2.3.1 |
| INV-WT-3 · -4 | 2 | U2.3.2 |
| INV-WT-5 | 1 | U2.7.3 |
| INV-REQ-1..4 | 4 | U2.5.1 |
| INV-PLAN-1..7 | 7 | U2.5.4 |
| INV-SUB-2 | 1 | U2.5.1（与 INV-REQ-3 同一条断言） |
| INV-SUB-1 · -3..-6 | 5 | U2.5.2 |
| INV-UNIT-1 | 1 | U2.5.3 |
| INV-UNIT-4 | 1 | U2.5.2 |
| INV-UNIT-2 · -3 | 2 | U2.6.1 |
| INV-UNIT-5 | 1 | U2.7.3 |
| INV-CTR-1..5 · -10 | 6 | U2.5.3 |
| INV-CTR-6 | 1 | U2.6.1 |
| INV-CTR-7 | 1 | U2.7.2 |
| INV-CTR-8 | 1 | U2.6.3 |
| INV-CTR-9 | 1 | **→ M3**（`inject` 引用的 Memory/Skill 必须 `active`；M2 内 `inject` 恒为空数组） |
| INV-ATT-1..3 · -6 | 4 | U2.6.1 |
| INV-ATT-5 | 1 | U2.6.4 |
| INV-ATT-4 · -7 · -8 · -9 | 4 | U2.7.2 |
| INV-EVD-1 · -2 · -3 · -5 · -6 | 5 | U2.7.1 |
| INV-EVD-4 | 1 | U2.7.3 |
| INV-EVD-7 | 1 | U2.8.1（后端投影）+ U2.11.3（前端断言） |
| INV-DEC-6 | 1 | U2.6.1（prompt 组装载荷不含令牌串） |
| INV-DEC-1..5 · -7..-10 | 9 | U2.6.3（D3 相关的界面断言在 U2.11.3） |
| INV-CKP-1..3 | 3 | U2.7.3 |
| INV-CKP-4 · -5 | 2 | U2.7.4 |
| INV-CKP-6 | 1 | U2.5.4 |
| INV-MEM-1..12 | 12 | **→ M3** |
| INV-SKL-1 · -6 | 2 | U2.2.3（导入即 `draft` · 源目录逐字节不变） |
| INV-SKL-2..-5 · -7..-10 | 8 | **→ M3** |
| INV-ROLE-2 · -6 | 2 | U2.4.1 |
| INV-ROLE-1 · -3 · -4 | 3 | U2.4.2 |
| INV-ROLE-5 | 1 | U2.7.2（同 INV-ATT-7） |
| INV-RT-3 | 1 | U2.6.1 |
| INV-RT-1 · -2 · -4 | 3 | **M0 已覆盖**（U0.7.1 R2 · U0.8.1 R4 · U0.5.1 R1），M2 不重复 |

**合计**：M2 = 90 · M3 = 22 · M0 = 3 · 总计 115。

> 新增或拆分不变量时，**同步改本表**。本表与 `domain-model.md` §17 的 ID 集合必须完全相等，
> 差集非空视为文档缺陷。

---

## S2.1 · 契约先行 ◆

**阶段交付物**：`api/openapi.yaml` 覆盖 M2 全部端点与 schema，`make gen` 三份生成物落地，
**前后端从此刻起并行**。

> 本子计划是 M2 唯一允许改 `api/openapi.yaml` 的地方。其余任何单元需要改 spec，
> 一律停下来上报并判定为 `contract_revision`。

### ○ U2.1.1 · 主链路资源端点与 schema

| | |
|---|---|
| `goal` | 把「项目 → 工作 → 需求快照 → 计划 → 子计划 → 单元 → 契约 → 尝试 → 证据 → 决策 → 检查点」十一类资源的端点与 schema 写进 spec，使前端在后端零实现的情况下可以开工 |
| `allowed_changes` | `api/openapi.yaml`（`paths` 与 `components.schemas`，**不含 `Event`**）· `api/AGENTS.md` |
| `forbidden_changes` | 不写任何 Go / TS 实现；不改 `Event` schema（那是 U2.1.2）；不改 `/v1/system/*` 与 `/v1/runtimes*`（M0/M1 已定）；不新增第 14 类事件 |
| `stop_conditions` | Q7（`acceptance_criteria` 是否契约字段）· Q8（`contract_revision` 期间的状态词）· Q10（`Unit`/`Subplan` 状态集）· Q13（Runtime 是否封闭枚举）任一未裁决；发现某个字段在 `domain-model.md` 里带 `†` 且无裁决 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 十一类资源各有 schema，字段名与 `domain-model.md` §2–§13 的字段表**逐字一致** | 脚本提取两侧字段名集合，对称差为空 |
| R2 | `WorkState` 枚举与 `AGENTS.md` §8 的九个状态词一字不差，外加 Q1 裁决后的入口态 | 断言枚举取值集合 == `backend/internal/constant/state.go` 的取值集合 |
| R3 | 决策等级只有四级；审查结论只有四类 | 断言 `enum: [D0,D1,D2,D3]` 与 `enum: [accepted, implementation_fix, contract_revision, global_replan]` |
| R4 | 错误一律 RFC 9457 `Problem`，`type` 是 `snake_case` 错误码 | 断言所有 `default` 响应 `$ref` 到 `Problem`；所有 `Problem.type` 取值匹配 `^[a-z][a-z0-9_]*$` |
| R5 | 后端不返回用户可见文案 | 断言除 `Problem.detail` 外，无响应 schema 含自由文本字段；`grep` 断言 `paths` 下的 `examples` 无中日韩字符 |
| R6 | `make gen` 通过，生成物可编译 | `make check-gen` 绿 |
| R7 | 新增的每个错误码在两个 locale 文件里都有词条 | `make check-i18n` 绿 |

> **R1 是本单元的核心。** 字段名对不上时，后面所有「按 spec 并行」的前提就没了——
> 前端按 spec 写的映射会在联调那天全部返工。

### ○ U2.1.2 · 13 类事件 payload schema 与四处一致性守卫

| | |
|---|---|
| `goal` | 给 13 类事件各定义 payload schema，并把「新增一类必须同改四处」变成一条可执行的检查 |
| `allowed_changes` | `api/openapi.yaml`（`components.schemas.Event` 及各 payload）· `scripts/check-event-enum.sh` · `Makefile`（新增 `check-event-enum` 目标）· `.github/workflows/ci.yml` |
| `forbidden_changes` | **不新增第 14 类事件**；不改前端注册表（S2.10）；不改 `design/`（只读） |
| `stop_conditions` | Q11（`superseded` 双语义）· Q12（`Attempt` 已结束待审查状态）——两者决定 `evidence` 与 `state_change` 的 payload 取值域 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | `type` 枚举长度恰为 13，取值与 `architecture.md` §4 表逐字一致 | 断言 `len(enum) == 13` 且集合相等 |
| R2 | 8 类 `app` 事件的 payload 各含一个可下钻 id，5 类 `acp` 事件不含 | 穷举 13 类断言 payload 必填字段 |
| R3 | `check-event-enum` 比对**四个源**：spec 的 `type` 枚举 · `architecture.md` §4 表 · `design/Duet Spec.dc.html` 第 07 节 · 前端 `EVENT_RENDERERS` 键集合；前端源尚不存在时脚本以退出码 2 报「源缺失」，**不是跳过** | 三个已存在的源两两相等；缺失分支有独立用例 |
| R4 | 故意在 spec 里加第 14 类 → 检查红 | 制造违规，断言退出码非 0 且信息指出缺的是哪三处 |
| R5 | 故意改 `architecture.md` §4 表删一行 → 检查红 | 同上手法 |
| R6 | CI 每个 PR 跑 `check-event-enum` | workflow 含该步骤，且删掉该步骤时 `make check` 红 |

**测试**：R4–R6 都是「**故意制造违规，确认检查会红**」——检查脚本自己要能被测。

### ○ U2.1.3 · 生成物落地：Go ServerInterface · TS client · MSW handler

| | |
|---|---|
| `goal` | 让同一份 spec 同时产出三样东西，前端在后端零实现时对着 MSW handler 开发 |
| `allowed_changes` | `scripts/gen-api.sh` · `Makefile` · `backend/internal/api/gen/**` · `frontend/src/api/**` · `frontend/tests/msw/**` |
| `forbidden_changes` | 不手写任何 handler 接口；不手改生成物；不在 `api` 层写业务判断 |
| `stop_conditions` | 生成器产出的接口违反分层要求（同 M0 U0.10.1）；MSW handler 无法从 spec 生成 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | `make gen` 产出三份生成物，二次运行零 diff | `make check-gen` 绿 |
| R2 | 手改生成物会被发现 | 故意改一行，`make check-gen` 红 |
| R3 | MSW handler 与 spec 同源：改 spec 的字段名，未同步的前端测试变红 | 改一个字段名跑 `pnpm test`，断言有测试失败 |
| R4 | 生成物被 lint 忽略但被 `tsc` 覆盖 | `pnpm typecheck` 绿；`pnpm lint` 不报生成物 |
| R5 | 前端零手写 HTTP 调用 | ESLint 拦 `features/` 下的 `fetch(`，故意写一行验证红 |

---

## S2.2 · 项目管理 ■

**阶段交付物**：一个本地 git 仓库能被纳管成 `Project`，且**纳管本身不启动任何工作**。

### ○ U2.2.1 · 添加本地仓库与 `.acpflows` 初始化

| | |
|---|---|
| `goal` | `AddLocalRepo` 建 `<project>/.acpflows/`、把 `.acpflows/runs/` 追加进 `.gitignore`、按 remote 记录 GitHub 账号推荐，并且不产生任何 `Work` |
| `allowed_changes` | `backend/internal/domain/model/project.go` · `backend/internal/app/project.go` · `backend/internal/app/port/project.go` · `backend/internal/fsstore/project.go` · `backend/internal/store/migrations/**` · `backend/internal/store/project.go` · `backend/tests/` 下对应测试 |
| `forbidden_changes` | 不做 GitHub 令牌存取与远端操作（M4 4.3）；不创建 worktree（U2.3.1）；不实现 Skill 导入（U2.2.3）；不读写 `~/.acpflows` 与用户真实仓库（铁律 6） |
| `stop_conditions` | 目标目录已存在非本应用写入的 `.acpflows/`；`.gitignore` 不可写 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | INV-PRJ-2：创建后 `.gitignore` 含 `.acpflows/runs/` | 读取 `.gitignore` 断言含该行 |
| R2 | `.gitignore` 已含该行时不重复追加 | 连续调用两次，断言该行出现次数 == 1 |
| R3 | INV-PRJ-3：创建后该项目 `Work` 数为 0，且未创建任何 worktree | 断言 `len(works) == 0` 且 `worktree_root` 下无该项目目录 |
| R4 | 目录不含 `.git` 时拒绝并给出错误码 | 断言返回 `Problem.type == "not_a_git_repository"`，且未建 `.acpflows/` |
| R5 | `.acpflows/` 结构完整：`skills/` · `memory/` · `project.yaml` · `runs/` | 穷举断言四项存在 |
| R6 | 全部测试跑在 `t.TempDir()` 的夹具仓库上 | 隔离守卫（M0 U0.1.2 R3）不触发 |

### ○ U2.2.2 · 环境探测与移除语义

| | |
|---|---|
| `goal` | 环境缺 `git` 或缺 worktree 支持时不得创建 `Work`；移除项目只有两种语义，没有第三种 |
| `allowed_changes` | `backend/internal/domain/model/project.go` · `backend/internal/app/project.go` · `backend/internal/platform/envprobe.go` · `backend/tests/` 下对应测试 |
| `forbidden_changes` | 不按语言工具链（`cargo` 等）阻断 `Work` 创建——那只影响单个 Skill 的可用性；不物理删除用户仓库中的任何非 `.acpflows` 文件 |
| `stop_conditions` | 探测出的 git 版本不支持 `worktree`，而 `domain-model.md` INV-PRJ-5 未说明降级路径 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | INV-PRJ-5：缺 `git` 或缺 worktree 支持时 `CreateWork` 返回错误且不产生 `Work` | 注入探测结果为不可用，断言错误码 + `len(works) == 0` |
| R2 | 语言工具链缺失**不**阻断 `CreateWork` | 注入 `cargo` 缺失，断言 `CreateWork` 成功 |
| R3 | INV-PRJ-4：`Remove(unlink_only)` 后 `.acpflows/` 下文件数不变 | 前后对比文件数与内容哈希 |
| R4 | INV-PRJ-4：`Remove(purge_acpflows)` 后 `.acpflows/` 不存在，仓库其余文件逐字节不变 | 断言目录不存在 + `git status` 无变化 |
| R5 | 不存在第三种移除语义 | 穷举 `mode` 取值，非法值构造失败 |
| R6 | 探测结果含**可执行的**修复提示 | 断言错误 `params` 含具体命令字符串 |

### ○ U2.2.3 · 导入已有 Skill 目录（只做复制与 `draft`）

| | |
|---|---|
| `goal` | 扫描项目内 `**/skills`，复制到 `.acpflows/skills/` 并标记 `draft`，**原目录逐字节不变** |
| `allowed_changes` | `backend/internal/app/skill_import.go` · `backend/internal/fsstore/skill.go` · `backend/internal/domain/model/skill.go`（**只加 `status` 与标识字段**）· `backend/tests/` 下对应测试 |
| `forbidden_changes` | 不实现 Skill 版本化 / 校验 / 发布 / 回滚 / 注入（全部属 M3）；不建全局 Skill 库；不执行 `scripts/` 下任何文件 |
| `stop_conditions` | 扫描到同名 Skill 需要决定优先级 → Q28 / OPEN-15 未裁决，停 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | INV-PRJ-6 / INV-SKL-6：导入后源目录内容逐字节不变 | 导入前后对源目录做递归哈希，断言相等 |
| R2 | INV-SKL-1：导入产物状态为 `draft` | 断言 `status == "draft"` |
| R3 | 扫描跳过 `node_modules` 与 `target` | 在两处各放一个 `skills/`，断言未被导入 |
| R4 | 同名 Skill 出现时返回明确错误，不静默取一边 | 断言 `Problem.type == "skill_name_conflict"`（INV-SKL-10 在裁决前的形态） |
| R5 | 导入不执行任何脚本 | 放一个会写标记文件的 `scripts/*.sh`，断言标记文件不存在 |

---

## S2.3 · worktree 生命周期 ■

**阶段交付物**：每个 `Work` 独占一个 worktree 与一个分支；创建失败**永不回落到原仓库目录**。

### ○ U2.3.1 · worktree 与分支创建，失败进 `initializing_failed`

| | |
|---|---|
| `goal` | `CreateWork` 建 `<worktree_root>/<project>/<work>` 与分支 `duet/work-NN`，成功进 `clarifying`、失败进 `initializing_failed`，`baseline` 创建后不可变 |
| `allowed_changes` | `backend/internal/gitx/worktree.go` · `backend/internal/domain/model/work.go`（迁移 #1–#3 的守卫）· `backend/internal/app/work.go` · `backend/internal/constant/state.go` · `backend/tests/` 下对应测试 |
| `forbidden_changes` | 不在原仓库工作目录执行任何写操作；不改 M0 U0.9.1 已冻结的状态枚举取值；不做 diff / commit（S2.7） |
| `stop_conditions` | Q1（两个入口态未进术语表）未裁决；Q30（`~/.duet/worktrees` 与 `~/.acpflows` 两个顶层目录）与实现冲突；Q31（并发上限）需要具体数值 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | INV-WT-1：两个 `Work` 不共享 `worktree_path`，也不共享 `branch` | 并发创建 2 个 `Work`，断言两组值互不相等；构造同名分支时返回 `ErrBranchInUse` |
| R2 | INV-WT-2：worktree 创建失败后 `Work` 为 `initializing_failed`，**且原仓库 `git status` 无变化** | 注入创建失败（目标路径已被占用），断言状态 + 原仓库 `git status --porcelain` 输出为空 |
| R3 | `initializing_failed` 是终态 | 穷举 `Transition(to)`，全部返回 `ErrInvalidTransition` |
| R4 | INV-WT-6：`baseline` 创建后任何写操作返回错误 | 反射断言无 setter；调用修改路径返回 `ErrBaselineImmutable` |
| R5 | 迁移 #2 的守卫：分支名被占用或目标路径非空时不得进 `clarifying` | 两个用例各断言拒绝 |
| R6 | 分支名与 worktree 路径格式与 `domain-model.md` §4 一致 | 断言 `branch == "duet/work-08"`、路径 == `<worktree_root>/<project>/<work>` |

> **R2 是本单元的核心。** 「创建失败就在原目录凑合跑」是最容易被下一轮 AI 加进来的
> 「贴心降级」，它会直接污染用户的工作区。**断言必须落在原仓库的 `git status` 上**，
> 只断言 `Work` 状态挡不住这件事。

### ○ U2.3.2 · Agent 写入边界守卫与 worktree 清理

| | |
|---|---|
| `goal` | Agent 的写路径不在 `worktree_path` 前缀下时必须触发 `request_permission` 或被拒；附加的外部文件只读进入本轮，不写入 worktree、不沉淀为记忆；`CleanWorktrees` 清理已结束 `Work` |
| `allowed_changes` | `backend/internal/gitx/worktree.go` · `backend/internal/app/boundary.go` · `backend/internal/app/attachment.go` · `backend/tests/` 下对应测试 |
| `forbidden_changes` | 不实现审查判定（U2.7.2）；不创建任何 `Memory`；不删除未结束 `Work` 的 worktree |
| `stop_conditions` | 发现某类写操作既不落在 worktree 内也无法被 `request_permission` 拦住 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | INV-WT-3：写路径不在 `worktree_path` 前缀下时触发 `request_permission` 或被拒 | 表驱动：worktree 内 / 兄弟目录 / 原仓库 / `$HOME` 四种路径，断言后三种被拦 |
| R2 | 路径前缀判定对符号链接与 `..` 生效 | 用 `symlink` 与 `../` 各构造一个越界路径，断言被拦 |
| R3 | INV-WT-4：附加的外部文件不写入 worktree | 附加后断言 worktree 中不出现该文件 |
| R4 | INV-WT-4：本轮结束后引用清空，且不产生任何 `Memory` candidate | 断言引用列表为空 + `Memory` 表行数不变 |
| R5 | `CleanWorktrees` 只清理已结束 `Work` | 混合终态与非终态各一个，断言只有终态的 worktree 被移除 |
| R6 | 清理不删除分支上的 commit | 清理后断言分支仍可 `rev-parse` |

---

## S2.4 · 角色与 Runtime 绑定 ■

**阶段交付物**：8 个预置角色可用，`Role` 换绑 Runtime 不改变任何状态机结果。

### ○ U2.4.1 · 8 个预置角色与 11 个 AI 操作

| | |
|---|---|
| `goal` | 落地 8 个预置 `Role` 与 11 个 AI 操作，`Role` 的四要素是字段而非文案，`Role` 不含模型相关字段 |
| `allowed_changes` | `backend/internal/domain/model/role.go` · `backend/internal/constant/role.go` · `backend/internal/constant/operation.go` · `backend/tests/` 下对应测试 |
| `forbidden_changes` | `Role` 不得出现 `model` / `reasoning_effort` 字段；不得在 `domain` / `app` / `api` 出现 runtime 名字（M0 U0.7.1 R4 的常驻检查） |
| `stop_conditions` | Q9（`UnitContract` 谁产出）未裁决 —— 它决定 `unit_contract` 操作挂在哪个角色上；OPEN-16（8 个角色的英文标识）未裁决 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 8 个预置角色与 `domain-model.md` §16.1 表逐行一致 | 表驱动断言 `display_name` / `operations` / `permission_policy` 三列 |
| R2 | 11 个 AI 操作是封闭枚举 | 穷举断言 `len(Operations) == 11`；新增一个而未被任何角色覆盖时测试红 |
| R3 | INV-ROLE-2：`Role` 结构体不含 `model` / `reasoning_effort` | 反射穷举字段名，断言两者不存在 |
| R4 | 四要素 `duty` / `personality` / `boundary` / `output` 均为非空字段 | 8 个角色各断言四项非空 |
| R5 | INV-ROLE-6：存在无 `Role` 承担的 AI 操作时，`CreateWork` 返回明确错误 | 删掉一个角色，断言 `Problem.type == "operation_uncovered"`，不静默兜底（OPEN-18 裁决前的形态） |
| R6 | 上层零品牌判断 | `grep -rn 'codex\|claude' backend/internal/{app,domain,api}` 为空 |

### ○ U2.4.2 · `Role` → Runtime 绑定、`set_mode` 校验、权限裁决策略

| | |
|---|---|
| `goal` | `Role` 先定义再绑定 Runtime；`set_mode` 必须属于该 Runtime 的 `available_modes`；权限裁决策略决定如何应答 `request_permission` |
| `allowed_changes` | `backend/internal/domain/model/role.go` · `backend/internal/domain/policy/binding.go` · `backend/internal/app/role.go` · `backend/internal/store/role.go` · `backend/tests/` 下对应测试 |
| `forbidden_changes` | 不实现权限裁决的运行时流程（U2.6.2）；不改 M0 的能力矩阵；不按 runtime 名字分支 |
| `stop_conditions` | Q13（Runtime 是二值枚举还是注册表）未裁决 —— 它决定 `runtime_name` 的类型；Q4b（codex 无 `auto` 档）导致设计稿角色表绑不上 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | INV-ROLE-1：换绑 Runtime 前后，同一组状态机输入产生相同迁移结果 | 同一批迁移用例在两种绑定下逐条对比，结果相等 |
| R2 | INV-ROLE-3：一个 `Role` 恰绑一个 Runtime；一个 Runtime 可承担多个 `Role` | 断言 `runtime_name` 非空且单值；构造 3 个 `Role` 绑同一 Runtime 成功 |
| R3 | INV-ROLE-4：`set_mode` 不在 `available_modes` 内时保存失败 | 用 M0 探针产出的 `available_modes` 夹具，断言越界取值返回 `ErrModeNotAvailable` |
| R4 | 权限裁决策略只有两种取值 | 穷举断言 `逐条询问` / `自动允许读` 之外的取值构造失败 |
| R5 | 「恢复推荐绑定」回到 §16.1 表的那一行 | 改绑后恢复，断言三列与表一致 |
| R6 | 绑定不引入品牌判断 | `grep` 断言同 U2.4.1 R6 |

---

## S2.5 · 需求快照 · 计划族 · 单元契约 ■

**阶段交付物**：需求可冻结、计划可 append、契约可冻结与修订，且三者全部落盘可追溯。

> 本子计划把 M0 S0.9 的**骨架**补成完整聚合。M0 只验证了内存中的规则，
> 本子计划补齐守卫、持久化与跨聚合校验。

### ○ U2.5.1 · `RequirementSnapshot` 冻结与无范围空洞

| | |
|---|---|
| `goal` | 需求快照冻结后不可改，`open_facts` 非空时不得冻结；`PlanVersion` 冻结时每条需求条目至少映射到一个 `Unit` |
| `allowed_changes` | `backend/internal/domain/model/requirement_snapshot.go` · `backend/internal/domain/policy/coverage.go` · `backend/internal/store/requirement.go` · `backend/tests/` 下对应测试 |
| `forbidden_changes` | `domain` 不得 IO；不实现需求澄清的会话流程（S2.6）；不改 `Work` 状态枚举 |
| `stop_conditions` | OPEN-2（`RequirementSnapshot` 是独立聚合还是 `Work` 的值对象）—— 本单元按**独立聚合**实现，撞上冲突即停 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | INV-REQ-1：`open_facts` 非空时 `Freeze()` 返回错误；未冻结时 `clarifying → planning` 被拒 | 两个用例各断言错误码 |
| R2 | INV-REQ-2：已冻结的快照任何字段写入返回错误 | 反射穷举 setter，全部返回 `ErrSnapshotFrozen` |
| R3 | INV-REQ-3 / INV-SUB-2：存在未被任何 `Unit` 覆盖的需求条目时，`PlanVersion` 冻结被拒 | 构造 6 条需求 5 条被覆盖，断言拒绝且错误 `params` 指出缺的是哪一条 |
| R4 | INV-REQ-4：无绑定 `Evidence` 的需求条目完成判定为 `false` | 注入一段声称完成的 Agent 文本，断言判定仍为 `false` |
| R5 | 快照版本化：修订产生 `v2`，`v1` 仍可读 | 断言两个版本都能取回且内容不同 |
| R6 | 覆盖率 ≥ 90% | `make cover` |

### ○ U2.5.2 · `Subplan` DAG 与依赖阻塞

| | |
|---|---|
| `goal` | `depends_on` 构成的图无环；存在未 `accepted` 前驱时 `Subplan` 恒为 `blocked`；`Subplan` / `Unit` ID 在 `Work` 内唯一且不复用 |
| `allowed_changes` | `backend/internal/domain/model/subplan.go` · `backend/internal/domain/model/unit.go` · `backend/internal/domain/policy/dag.go` · `backend/tests/` 下对应测试 |
| `forbidden_changes` | 不实现契约冻结（U2.5.3）；不实现派发（U2.6.1）；`domain` 不得 IO |
| `stop_conditions` | Q10（`Unit` / `Subplan` 的完整状态集）未裁决；OPEN-5（废弃编号是否留空洞、是否计入进度分母）未裁决 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | INV-SUB-1：含环的 `depends_on` 返回 `ErrSubplanCycle` | 自环 / 二元环 / 三元环三个用例 |
| R2 | INV-SUB-3：前驱未 `accepted` 时 `Subplan` 为 `blocked`，迁往 `executing` 被拒 | 断言状态 + 迁移错误 |
| R3 | INV-SUB-6：存在非 `accepted` 的 `Unit` 时，`Subplan` 迁往 `accepted` 被拒 | 断言错误码 |
| R4 | INV-SUB-5 / INV-UNIT-4：已使用过的 ID 不会被重新分配 | 废弃一个 `subplan-02` 后新建，断言分配到 `subplan-04` 而不是 `02` |
| R5 | INV-SUB-4：同一 `subplan-NN` 在不同 `PlanVersion` 下内容可不同，归属版本唯一 | 断言两个版本的快照内容不同且各自 `plan_version` 唯一 |
| R6 | 新增 `Subplan` / `Unit` 状态而未处理时测试红 | 穷举测试覆盖 `IsValid()` |

### ○ U2.5.3 · `UnitContract` 六项必填 · 冻结 · 修订

| | |
|---|---|
| `goal` | 契约六项必填、冻结后只读、修订只能出新版本、写入边界与禁止边界不得相交 |
| `allowed_changes` | `backend/internal/domain/model/unit_contract.go` · `backend/internal/domain/policy/boundary.go` · `backend/internal/store/contract.go` · `backend/tests/` 下对应测试 |
| `forbidden_changes` | 不校验 `inject` 中的 Memory / Skill（INV-CTR-9 属 M3，本里程碑 `inject` 恒为空数组）；不实现审查（U2.7.2） |
| `stop_conditions` | Q7（`acceptance_criteria` 是否契约字段）· Q8（`contract_revision` 期间 `Work` 处于哪个状态）· Q9（谁产出契约）任一未裁决 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | INV-CTR-4：`goal` / `allowed_changes` / `forbidden_changes` / `test_strategy` / `stop_conditions` / `acceptance_criteria` 六项任缺其一时 `Freeze()` 返回错误 | 六个用例各缺一项 |
| R2 | INV-CTR-1：`frozen` 契约的任何 setter 返回 `ErrContractFrozen` | 反射穷举导出方法 |
| R3 | INV-CTR-2：连续 `Revise()` 得到 `contract_version` 1,2,3…，无空洞无重复 | 连续修订 5 次断言序列 |
| R4 | INV-CTR-3：冻结后 `frozen_at_checkpoint` 与 `based_on_plan_version` 非空且可解析 | 断言两字段指向存在的对象 |
| R5 | INV-CTR-5：`allowed_changes` 与 `forbidden_changes` 相交时 `Freeze()` 返回错误 | 构造相交路径，断言拒绝且错误指出相交项 |
| R6 | INV-CTR-10：每条 `acceptance_criteria` 的 `id` 在契约内唯一且非空 | 构造重复 `id` 与空 `id` 各一个用例 |
| R7 | INV-UNIT-1：`Unit` 的 `subplan_id` 唯一且非空 | 断言构造失败 |
| R8 | 旧版本永远可读 | 修订到 v3 后断言 v1 / v2 仍可取回且内容不变 |
| R9 | 覆盖率 ≥ 90% | `make cover` |

### ○ U2.5.4 · 计划族持久化与 `PlanVersion` 差异

| | |
|---|---|
| `goal` | `PlanVersion` / `Subplan` / `Unit` / `UnitContract` 落 SQLite，append-only 在存储层同样成立，`Diff(v4, v5)` 可读 |
| `allowed_changes` | `backend/internal/store/plan.go` · `backend/internal/store/migrations/**` · `backend/internal/app/plan.go` · `backend/internal/domain/model/plan.go` · `backend/tests/` 下对应测试 |
| `forbidden_changes` | 不在 `store` 写业务规则；不提供任何 `UPDATE` / `DELETE` 已冻结版本的语句 |
| `stop_conditions` | Q2 / OPEN-1（人类可读 ID 是不是主键）与 schema 设计冲突 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | INV-PLAN-1：对已存在的 `PlanVersion` 调用任何 setter 返回 `ErrPlanVersionImmutable` | 反射穷举 |
| R2 | INV-PLAN-2 / -3：版本序列 1,2,3… 无空洞无重复；任意时刻 `count(is_current) == 1` 且等于 `max(version)` | 连续 append 5 次逐次断言 |
| R3 | INV-PLAN-4：`v ≥ 2` 且缺任一已验收工作的 `disposition` 时 `AppendVersion` 返回错误 | 构造一个已 `accepted` 的 `Subplan`，缺处置时断言拒绝 |
| R4 | INV-PLAN-5：`reason` 为空时返回错误 | 断言错误码 |
| R5 | INV-PLAN-6：新增版本后旧版本的 `Subplan` / `Unit` 快照逐字段不变 | 快照前后逐字段对比 |
| R6 | INV-PLAN-7：`disposition = needs_rollback` 未给回退 `Checkpoint` 时返回错误 | 断言错误码 |
| R7 | INV-CKP-6：回退到某 `Checkpoint` 后 `max(plan_version)` 比回退前大 1 | 断言版本号差值 == 1 |
| R8 | SQL 层不存在改写已冻结版本的语句 | `grep` 断言 `store/plan.go` 无 `UPDATE plan_versions` / `DELETE FROM plan_versions` |
| R9 | `Diff(v4, v5)` 产出可读差异 | 断言差异含新增 `Unit` 的 id 与变更的 `reason` |

---

## S2.6 · 执行编排与权限裁决 ■

**阶段交付物**：一个冻结的契约能被派发成一次 `Attempt`，权限裁决与 D2/D3 决策真的阻塞。

### ○ U2.6.1 · 派发单元：`Attempt` 记账与 `OperationInvocation`

| | |
|---|---|
| `goal` | 把冻结契约派发给绑定的 `Role`，产生 `Attempt` 并记录本次实际使用的 Runtime 与 `set_mode`；同一时刻至多一个 `running` |
| `allowed_changes` | `backend/internal/domain/model/attempt.go` · `backend/internal/app/dispatch.go` · `backend/internal/app/prompt.go` · `backend/internal/store/attempt.go` · `backend/tests/` 下对应测试 |
| `forbidden_changes` | prompt 组装载荷中不得出现任何令牌串；不实现证据采集（U2.7.1）；不按 runtime 名字分支 |
| `stop_conditions` | Q11（`superseded` 双语义）· Q12（`Attempt` 缺「已结束待审查」状态）未裁决；Q31 需要具体并发上限数值 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | INV-UNIT-2 / INV-CTR-6：契约未冻结时 `StartAttempt()` 返回 `ErrContractNotFrozen` | 断言错误码，且未产生 `Attempt` |
| R2 | INV-UNIT-3：前驱 `Unit` 未 `accepted` 时 `StartAttempt()` 被拒 | 断言错误码 |
| R3 | INV-ATT-1：连续 `StartAttempt()` 得到 `attempt_no` 1,2,3… | 断言序列 |
| R4 | INV-ATT-2 / INV-WORK-4：已有 `running` 时再次 `StartAttempt()` 返回错误；一个 `Work` 任一时刻 `running` 数 ≤ 1 | 并发发起 5 次，断言成功 1 次 |
| R5 | INV-ATT-3：终态 `Attempt` 的任何迁移返回错误 | 穷举三个终态 |
| R6 | INV-ATT-6：缺 `runtime_name` / `runtime_version` / `set_mode` 任一项时不得进 `succeeded` | 三个用例 |
| R7 | INV-DEC-6：任何进入 Agent 上下文的载荷中不含令牌串 | 在 keychain 夹具里放一个可识别的假令牌，断言组装后的 prompt 全文不含它 |
| R8 | INV-RT-3：重试产生的 `Attempt` 不增加 `OperationInvocation` 计数 | `attempt 2` 后断言计数未变 |
| R9 | 迁移 #8 的守卫：前驱未全 `accepted` / 契约未 `frozen` / 存在未决 D2 时不得进 `executing` | 三个用例各断言拒绝 |

### ○ U2.6.2 · 权限裁决 `request_permission` → `waiting_user`

| | |
|---|---|
| `goal` | ACP 反向请求按 `Role` 的权限裁决策略应答；需人裁决时 `Work` 进 `waiting_user` 并阻塞派发；取消时用 `cancelled` 应答所有 pending |
| `allowed_changes` | `backend/internal/app/permission.go` · `backend/internal/domain/policy/permission.go` · `backend/internal/domain/model/work.go`（迁移 #10 的守卫）· `backend/tests/` 下对应测试 |
| `forbidden_changes` | 不把权限裁决当成 `Decision`（两者是不同对象）；不改 M0 U0.6.1 的取消实现，只复用；不提供任何形式的 D3「始终允许」 |
| `stop_conditions` | Q3（`executing` 与 `waiting_user` 能否共存）未裁决 —— 它直接决定本单元的状态迁移 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 策略 `自动允许读` 对读类请求自动应答，不进 `waiting_user` | Fake Runtime 发一个读类请求，断言 `Work` 状态不变且已应答 |
| R2 | 策略 `逐条询问` 使 `Work` 进 `waiting_user` 并阻塞派发 | 断言状态 + `StartAttempt()` 被拒 |
| R3 | 未应答前 turn 不结束 | 断言 `session/prompt` 未 resolve（复用 M0 U0.4.2 R1 的 Fake 编排） |
| R4 | 取消时用 `cancelled` 应答**所有** pending 的 `request_permission` | Fake 发 2 个后取消，断言两个都收到 `cancelled`（Q4d，复用 M0 U0.6.1 R3） |
| R5 | 权限裁决可升级为 D2 `Decision`，且两者是不同对象 | 断言升级后存在 1 条 `Decision` 且权限请求记录仍可独立取回 |
| R6 | 「始终允许」只对权限裁决策略可配置，**永不作用于 D3** | 穷举持久化字段，断言不存在 D3 维度的「始终允许」记录 |

### ○ U2.6.3 · `Decision` D0–D3：等级判定与阻塞

| | |
|---|---|
| `goal` | 决策等级可判定、只上调不下调、D2/D3 阻塞 `waiting_user`、D3 逐次授权且不存在「始终允许」 |
| `allowed_changes` | `backend/internal/domain/model/decision.go` · `backend/internal/domain/policy/decision_level.go` · `backend/internal/app/decision.go` · `backend/internal/store/decision.go` · `backend/tests/` 下对应测试 |
| `forbidden_changes` | 不实现 GitHub 远端操作（M4 4.3），只实现「未授权即拒绝」这一侧；不提供任何 D3 授权的持久化复用路径 |
| `stop_conditions` | Q3（`executing` 与 `waiting_user` 能否共存）未裁决 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | INV-DEC-1：`level` 只能取 D0/D1/D2/D3 | 非法值构造失败 |
| R2 | INV-DEC-2 / INV-WORK-5：存在未决 D2/D3 时 `Work` 为 `waiting_user` 且 `StartAttempt()` 被拒 | 断言状态 + 派发错误 |
| R3 | INV-DEC-3：**持久化中不存在任何「始终允许」记录** | 穷举 `decisions` 表与其索引的全部列名，断言无该维度字段 |
| R4 | INV-DEC-4：同一 D3 动作第二次发生时仍返回「需授权」 | 授权一次后再次触发，断言仍需授权 |
| R5 | INV-DEC-5：GitHub 账号为可写档时，`push` 仍返回「需 D3 授权」 | 注入可写档，断言 `Problem.type == "d3_authorization_required"` |
| R6 | INV-DEC-7：判定函数信息不足时返回 D2；不存在返回值低于输入等级的路径 | 表驱动穷举输入，断言 `out >= in`；无信息用例断言 == D2 |
| R7 | INV-DEC-8：D2 的 `options` 长度 ≥ 2 且 `recommended` 数量 ≤ 1 | 两个反例用例 |
| R8 | INV-DEC-9：已 `decided` 的 `Decision` 任何写入返回错误 | 反射穷举 setter |
| R9 | INV-DEC-10：确认一个 D2 后，D3 动作仍需单独授权 | 断言 D3 仍返回需授权 |
| R10 | INV-CTR-8：命中 `stop_conditions` 时 `Attempt` 停止，且必须产生一条 `level ≥ D2` 的 `Decision` | 断言 `Attempt` 非 `running` + `Decision` 计数 +1 |

> **R3 是 D3 语义的落点。** 「始终允许」只要在存储里有一个字段能表达，
> 下一轮就会有人把它接上去。**断言必须打在列名上，不是打在业务方法上。**

### ○ U2.6.4 · 执行中的补充消息队列

| | |
|---|---|
| `goal` | `executing` 期间收到的用户消息一律入队，三种处理方式之外没有第四种，**不得用弹窗打断** |
| `allowed_changes` | `backend/internal/domain/model/work.go`（`queued_messages`）· `backend/internal/app/queue.go` · `backend/internal/store/queue.go` · `backend/tests/` 下对应测试 |
| `forbidden_changes` | 不产生任何阻塞式对话框语义的事件；不实现记忆写入（M3）；被打断的 `Attempt` 的 `Evidence` 不得删除 |
| `stop_conditions` | OPEN-6（打断后 `Work` 落到 `reviewing_unit` 还是 `planning`）未裁决 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | INV-WORK-6：`executing` 期间收到的消息进入 `queued_messages`，`Work` 状态不变 | 断言队列长度 +1 且状态未变 |
| R2 | 三种处理方式之外没有第四种 | 穷举取值，非法值构造失败 |
| R3 | `立即打断当前单元` 走两段式 cancel，`Attempt` 标 `superseded` | 断言只发一次协议 cancel（复用 M0 U0.6.1 R1）+ 状态 |
| R4 | INV-ATT-5：打断后 `Attempt` 的 `Evidence` 集合与打断前一致 | 打断前后对比 `Evidence` id 集合，断言相等 |
| R5 | `仅记录，不影响本次计划` 不改变 `PlanVersion` | 断言 `max(plan_version)` 不变 |
| R6 | 队列不产生阻塞式对话框事件 | 断言投影出的事件类型不含 `decision` 与 `request_permission` |

---

## S2.7 · 证据 · 审查 · 验收 · 自动提交 · 检查点 ■

**阶段交付物**：一个 `Unit` 能被验收，且**验收结论的每一条都能点回到应用自己采集的证据**。

> 这是 M2 的心脏。这个子计划做歪，整个产品就退化成「AI 说它做完了」。

### ○ U2.7.1 · 四类采集器：`Evidence` 由应用直接采集 ★

| | |
|---|---|
| `goal` | `git_diff` / `test_output` / `command_record` / `review_note` 四个采集器落地，并让「不存在任何以 Agent 文本创建 `Evidence` 的构造路径」变成一条常驻检查 |
| `allowed_changes` | `backend/internal/domain/model/evidence.go` · `backend/internal/app/evidence.go` · `backend/internal/gitx/diff.go` · `backend/internal/platform/exec.go` · `backend/internal/constant/evidence.go` · `scripts/check-evidence-collector.sh` · `Makefile` · `backend/tests/` 下对应测试 |
| `forbidden_changes` | **`Evidence` 的任何构造入口不得接受 `internal/acp` 包的类型**；不得为「Agent 说测试通过了」留任何旁路；不提供 `DeleteEvidence` |
| `stop_conditions` | Q11（`superseded` 同时是 `Attempt` 与 `Evidence` 状态）未裁决；某类证据无法由应用直接采集 → 停，不要退回让 Agent 转述 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | INV-EVD-1：`collector` 只能取四类采集器之一 | 穷举构造，第五种取值构造失败 |
| R2 | **不存在任何以 Agent 文本创建 `Evidence` 的构造路径** | 用 `go/ast` 遍历 `backend/internal/{domain,app}`：集合 A =「能构造 `Evidence` 的函数」，集合 B =「签名中出现 `internal/acp` 包类型的函数」，断言 `A ∩ B == ∅`。落进 `scripts/check-evidence-collector.sh` 并接入 CI |
| R3 | 故意加一个接受 ACP chunk 的 `Evidence` 构造函数 → 检查红 | 制造违规，断言退出码非 0 且指出函数名 |
| R4 | `git_diff` 的内容来自 `gitx` 对 worktree 直接计算 | 让 Fake Runtime 在消息里贴一段假 diff，断言采集到的 diff 与 `git diff` 输出一致、与假 diff 不同 |
| R5 | `test_output` 的内容来自应用起的进程的 stdout/stderr + 退出码 | 让 Fake Runtime 声称测试通过而实际进程退出码为 1，断言 `Evidence.exit_code == 1` |
| R6 | INV-EVD-5：`test_output` / `command_record` 缺 `exit_code` 时创建失败 | 两个用例 |
| R7 | INV-EVD-6：无法判定 `in_boundary` 的 `git_diff` 记为 `in_boundary = false` | 构造无法解析的路径，断言字段为 `false` |
| R8 | INV-EVD-2：不存在 `DeleteEvidence`；`Supersede()` 后原内容仍可读 | 反射断言无删除方法 + 读取断言 |
| R9 | INV-EVD-3：未显式绑定的验收标准 `evidence` 为空数组，不做模糊匹配 | 构造文本高度相似但未绑定的 `Evidence`，断言仍为空数组 |

> **R4 与 R5 是这条铁律的真正防线。** 只断言「有 `collector` 字段」挡不住任何事——
> 必须让 Fake Runtime **说谎**，然后断言应用采集到的是真相。

### ○ U2.7.2 · 独立会话审查与四类结论

| | |
|---|---|
| `goal` | 审查在独立会话中进行，实现方不得审查自己，结论只有四类，越界 diff 不得判 `accepted` |
| `allowed_changes` | `backend/internal/app/review.go` · `backend/internal/domain/model/attempt.go`（`review_result`）· `backend/internal/domain/policy/review.go` · `backend/tests/` 下对应测试 |
| `forbidden_changes` | 不允许审查会话复用执行会话的 `sessionId`；不允许实现角色产出 `review_result`；不修改被审查的 diff |
| `stop_conditions` | Q8（`contract_revision` 期间 `Work` 处于哪个状态）· Q12（`Attempt` 缺「已结束待审查」状态）未裁决 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | INV-ATT-7 / INV-ROLE-5：审查会话 id ≠ 执行会话 id | 断言两个 id 不等；相等时结论被拒 |
| R2 | INV-ATT-8：`review_result` 的产出角色 ≠ 本 `Attempt` 的执行角色 | 断言相等时返回 `ErrSelfReview` |
| R3 | INV-CTR-7：存在越界 diff `Evidence` 时，`review_result = accepted` 被拒 | 断言只能是 `implementation_fix` 或 `contract_revision` |
| R4 | INV-ATT-9：全部测试通过但存在无证据标准时，`accepted` 被拒 | 注入全绿的 `test_output` + 一条无证据标准，断言拒绝 |
| R5 | INV-ATT-4：`state == succeeded` ⟺ `review_result == accepted`（双向） | 双向各一个用例 |
| R6 | 四类结论各自的 `Work` 迁移与必须产出物 | 表驱动四行：`accepted` → `Checkpoint` + 自动提交；`implementation_fix` → `attempt N+1`；`contract_revision` → 新 `contract_version`；`global_replan` → 新 `PlanVersion` + 处置 |
| R7 | 驳回必须带理由与 `ev-*` 引用 | 断言缺任一项时结论构造失败 |

### ○ U2.7.3 · 验收 → 自动提交 → 检查点落盘

| | |
|---|---|
| `goal` | `Unit` 验收通过后自动提交到工作分支并落 `Checkpoint`；`push` / 发 PR 属于 D3，未授权一律拒绝 |
| `allowed_changes` | `backend/internal/app/accept.go` · `backend/internal/gitx/commit.go` · `backend/internal/domain/model/checkpoint.go` · `backend/internal/store/checkpoint.go` · `backend/tests/` 下对应测试 |
| `forbidden_changes` | 不实现 `push` / PR 的成功路径（M4 4.3），只实现「无 D3 授权即拒绝」；不删除任何 `Evidence`；不改 `baseline` |
| `stop_conditions` | Q7（`acceptance_criteria` 是否契约字段）未裁决 —— 它决定验收判定读哪里 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | INV-EVD-4 / INV-UNIT-5：任一验收标准无 `valid` `Evidence` 时 `Accept()` 被拒 | 断言错误 `params` 指出是哪条标准 |
| R2 | INV-WT-5：`Unit` `accepted` 后工作分支多出一个 commit | 前后 `git rev-list --count` 差值 == 1 |
| R3 | INV-WT-5：调用 `push` 时若无 D3 授权则被拒 | 断言 `Problem.type == "d3_authorization_required"` 且未发起任何远端调用 |
| R4 | INV-CKP-1：`commit_hash` 在该 `Work` 分支上可解析，否则创建失败 | `git cat-file -e` 校验；构造假 hash 断言创建失败 |
| R5 | INV-CKP-2：`Checkpoint` 无 setter、无 delete | 反射穷举 |
| R6 | INV-CKP-3：`Unit` / `Subplan` `accepted` 后 `Checkpoint` 数量 +1 | 两个用例 |
| R7 | INV-WORK-3：进入 `completed` 时全部 `Unit` 为 `accepted` 且每条标准都有 `valid` `Evidence` | 构造一条缺证据的标准，断言 `completed` 被拒 |
| R8 | INV-WORK-2：`completed` / `failed` / `initializing_failed` 下任何 `Transition` 返回错误 | 穷举三个终态 × 全部目标状态 |
| R9 | **INV-WORK-1 穷举收口**：对全部 `(from, to)` 组合调用 `Transition`，只有 §3.3 的 25 条返回 nil | 表驱动 11 × 11 全组合 |
| R10 | 自动提交的提交信息含 `unit-NNN` 与 `contract vN` | 断言 commit message 匹配 |

### ○ U2.7.4 · 暂停 / 恢复 / `update prepare` 补全

| | |
|---|---|
| `goal` | 补全 M1 1.7 的暂停语义：`update/prepare` 对每个非终态 `Work` 要么落 `Checkpoint` 并置 `paused`，要么返回 `blocked`；恢复必须回到 `resume_state` |
| `allowed_changes` | `backend/internal/app/pause.go` · `backend/internal/app/resume.go` · `backend/internal/api/system.go`（`prepare` / `resume` 的实现）· `backend/tests/` 下对应测试 |
| `forbidden_changes` | 不改 `/v1/system/*` 的 spec（M1 已定）；不改 M0 U0.6.1 的两段式取消实现，只复用 |
| `stop_conditions` | 发现存在既不能落点也不能报 `blocked` 的第三种结果 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | INV-CKP-5：结果集合 ⊆ `{prepared, blocked}`，且两者之和 == 非终态 `Work` 数 | 构造 3 个非终态 `Work`（1 个 Runtime 无响应），断言 2 + 1 == 3 |
| R2 | 返回 `blocked` 的 `Work` **不落 `Checkpoint`** | 断言该 `Work` 的 `Checkpoint` 数不变 |
| R3 | INV-WORK-7：`Resume()` 后的状态等于 `Checkpoint` 的 `resume_state` | 三个 `resume_state` 各一个用例 |
| R4 | INV-CKP-4：worktree HEAD 与 `commit_hash` 不一致时 `Resume()` 返回错误 | 手动移动 HEAD 后断言拒绝并上报 |
| R5 | 迁移 #11 的守卫：两段式 cancel 完成 + 证据与游标已采集 + `Checkpoint` 已落盘，缺一不得进 `paused` | 三个用例各缺一项 |
| R6 | `waiting_user` 下暂停保留待决策项 | 恢复后断言未决 `Decision` 仍在 |

---

## S2.8 · 事件投影与 Work 级 SSE ■

**阶段交付物**：领域动作被投影成 13 类事件，前端只靠一条 SSE 就能重建整个界面。

### ○ U2.8.1 · 领域事件 → 13 类事件投影

| | |
|---|---|
| `goal` | 每个领域动作恰好投影到 13 类中的一类；8 类 `app` 事件永远携带可下钻的结构化产物 id |
| `allowed_changes` | `backend/internal/eventbus/projection.go` · `backend/internal/constant/event.go` · `backend/internal/app/**`（只加事件发布调用）· `backend/tests/` 下对应测试 |
| `forbidden_changes` | **不新增第 14 类事件**；`thought_chunk` 投影不得携带完整思维链原文；不投影任何创建 `Memory` 的动作 |
| `stop_conditions` | 某个领域动作在 13 类里找不到归属 → 停，走 `frontend-guide.md` §9.5 的四处流程 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 13 类各有至少一条投影用例 | 表驱动 13 行 |
| R2 | 新增一个领域事件而未映射到 13 类之一时测试红 | 穷举测试覆盖领域事件枚举 |
| R3 | INV-EVD-7：8 类 `app` 事件的 payload 必含可下钻 id | 穷举断言各自的必填 id 字段非空 |
| R4 | 5 类 `acp` 事件不含下钻 id | 穷举断言 |
| R5 | `memory_candidate` 只投递候选，**M2 内不存在创建 `active` `Memory` 的路径** | `scripts/check-no-auto-memory.sh`：AST 断言 `backend/internal/{app,domain}` 中无任何函数产出 `status == "active"` 的 `Memory` |
| R6 | `thought_chunk` 投影只带摘要字段 | 注入含全文的 payload，断言投影结果无全文字段 |
| R7 | `seq` 单调递增且跨重启连续 | 重启后断言不回退、不重复（复用 M0 U0.5.2 R3） |

### ○ U2.8.2 · Work 级 SSE 端点与跨 Work 隔离

| | |
|---|---|
| `goal` | `GET /v1/works/{workId}/events` 推送该 `Work` 的 13 类事件，支持 `Last-Event-ID` 续传，且跨 `Work` 严格隔离 |
| `allowed_changes` | `backend/internal/api/sse/work_events.go` · `backend/internal/eventbus/**` · `backend/tests/` 下对应测试 |
| `forbidden_changes` | 不改事件信封 schema（U2.1.2 已定）；不在 `api` 层写业务判断 |
| `stop_conditions` | — |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 未知 `type` 被拒 | 发一个第 14 类，断言被拒且不进入订阅者 |
| R2 | `Last-Event-ID` 续传只补发 `> N` 的事件 | 断言首条事件的 `seq == N+1` |
| R3 | **跨 `Work` 隔离**：订阅 `work-08` 收不到 `work-09` 的事件 | 两条订阅并发，断言事件集合不相交 |
| R4 | 客户端断开时订阅者被回收 | 断言 `eventbus` 订阅者数归零 |
| R5 | 慢消费者不阻塞其他订阅者 | 一个订阅者不读，断言另一个仍正常收 |
| R6 | 响应通过 `openapi.yaml` 的 `Event` schema 校验 | `kin-openapi` 校验 13 类各一条 |

---

## S2.9 · 前端地基 ▲

**阶段交付物**：布局骨架、生成客户端、SSE 订阅、i18n 全部就位，**三条特性链可以并行开工**。

> 本子计划只依赖 S2.1 的 spec 与 MSW handler，**不等任何后端实现**。

### ○ U2.9.1 · AppShell 三栏骨架 · 窗口栏 · platform 适配层

| | |
|---|---|
| `goal` | 42 / 252 / 主区 / 300 的四区骨架、窗口栏三个折叠开关、Tauri 与 Web 双实现的 platform 适配层 |
| `allowed_changes` | `frontend/src/app/**` · `frontend/src/design/**` · `frontend/src/ui/**` · `frontend/src/platform/**` · `frontend/src/constants/layout.ts` · `frontend/eslint.config.js` · `frontend/.stylelintrc.json` |
| `forbidden_changes` | 除 `src/platform/` 外任何位置 import `@tauri-apps/*`；硬编码 hex / 裸 px；自绘 SVG 图标；引入 tooltip / dropdown / modal 第三方库 |
| `stop_conditions` | Q19（toast 无条目也无 z-index 层，Web 降级依赖它）· Q21（⌘K 面板无设计）· Q23（圆角令牌与 `--color-accent-500`/`-200` 未列入用法表）任一未裁决 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 布局数值只来自 `constants/layout.ts`，CSS 只读 `var(--layout-*)` | Stylelint 拦裸 px；故意写一行验证红 |
| R2 | 三个折叠开关只在 `TitleBar.tsx` 出现一次 | 断言全仓库该组件实例数 == 3 且全部在 `src/app/TitleBar.tsx` |
| R3 | 右栏只在对话页启用，其他页窗口栏图标 `disabled` | 路由派生测试，两个路由各一个用例 |
| R4 | 除 `src/platform/` 外无 `@tauri-apps/*` import | ESLint 规则；在 `features/` 故意写一行验证红 |
| R5 | 五条 Web 降级**真的可用** | 每条一个组件测试，断言产生用户可见反馈（不是空实现） |
| R6 | 长列表容器 `min-height: 0` + `overflow-y: auto` | Stylelint 规则；故意写 `overflow: hidden` 验证红 |
| R7 | 五个交互状态每个可交互组件都写全 | Stylelint 断言每个 `.module.css` 含五个状态选择器 |
| R8 | 纯图标控件必须有中文 tooltip | `IconButtonProps.tooltip` 必填；漏传时 `tsc` 红 |

### ○ U2.9.2 · 生成客户端 · SSE 订阅 · Zustand event slice

| | |
|---|---|
| `goal` | 服务端状态走 TanStack Query，实时事件走单条 SSE → Zustand event slice，两套失效模型不混 |
| `allowed_changes` | `frontend/src/api/**` · `frontend/src/store/**` · `frontend/src/models/event.ts` · `frontend/tests/msw/**` |
| `forbidden_changes` | SSE 事件塞进 TanStack Query 缓存；组件里出现 `fetch`；手写 API 类型（必须来自生成物） |
| `stop_conditions` | Q17（SSE 断线提示无设计条目）未裁决 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 组件里不出现 `fetch` | ESLint 拦；故意写一行验证红 |
| R2 | SSE 事件写入 Zustand event slice，**不进** TanStack Query 缓存 | 注入 10 条事件，断言 query cache 的快照前后相等 |
| R3 | 乱序到达按 `seq` 归位 | 乱序注入 5 条，断言渲染顺序按 `seq` |
| R4 | 断线后用 `Last-Event-ID` 续传，只补 `> N` | Fake `EventSource` 断言重连请求头与补发范围 |
| R5 | 一个 `Work` 只开一条 SSE 连接 | 断言 `EventSource` 实例数 == 1 |
| R6 | 组件卸载时连接被关闭 | 断言 `close()` 被调用 |

### ○ U2.9.3 · i18n 初始化与词条真源

| | |
|---|---|
| `goal` | 所有用户可见文案走 `t()`，状态词与标识符不翻译且等宽，两个 locale 文件同进同退 |
| `allowed_changes` | `frontend/src/i18n/**` · `frontend/src/ui/StatusText.tsx` · `scripts/check-i18n.sh` · `frontend/eslint.config.js` · `Makefile` |
| `forbidden_changes` | 用中文原文当 key；动态拼 key；只更新一个 locale 文件；翻译状态词 / 标识符 / 命令 / 路径 / ID |
| `stop_conditions` | 后端返回了中文文案 → 停，回 S2.1 判定为 `contract_revision` |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | JSX 中出现中日韩字面量 → lint 红 | 故意写一行，`pnpm lint` 红 |
| R2 | `zh-CN.json` 与 `en-US.json` 的 key 集合完全相同 | `make check-i18n`；删一边一条验证红 |
| R3 | 状态词只经 `StatusText` 一个出口，等宽且不翻译 | 断言两个 locale 文件中不存在九个状态词的词条；断言 `StatusText` 用 `--font-mono` |
| R4 | `openapi.yaml` 的 `Problem.type` 错误码全部有词条 | `make check-i18n`；加一个错误码不加词条验证红 |
| R5 | 禁止动态拼 key | ESLint 规则；故意写 `t('status.' + s)` 验证红 |
| R6 | 无未使用词条、无缺失词条 | `make check-i18n` 两项各一个反例验证 |

---

## S2.10 · 对话页与 13 类事件渲染器 ▲

**阶段交付物**：13 类事件全部有渲染器，过滤器由同一张表生成，输入区五段固定。

### ○ U2.10.1 · 渲染器注册表与 13 个组件 ★

| | |
|---|---|
| `goal` | 一张 `EVENT_RENDERERS` 表管住 13 类事件，少一类多一类都 `tsc` 红，**代码中不存在按事件类型分发的 `switch`** |
| `allowed_changes` | `frontend/src/features/conversation/events/**` · `frontend/src/constants/event.ts` · `frontend/src/models/event.ts` · `frontend/eslint.config.js` · `frontend/tests/INDEX.md` |
| `forbidden_changes` | **`switch (event.type)` 与等价的 if-else 链**；另建一份过滤项列表；新增第 14 类；在组件测试里断言 CSS 类名或 DOM 结构 |
| `stop_conditions` | 某类事件在 `design/Duet Spec.dc.html` 第 07 节找不到展示形态条目 → 停（铁律 3） |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | `{ [T in EventType]: EventRenderer<T> }` 少一类 `tsc` 红 | 删一条验证 `pnpm typecheck` 红 |
| R2 | 多一类 `tsc` 红 | 加一条不在枚举里的键验证红 |
| R3 | `source: 'app'` 缺 `openTarget` 编译不过 | 删一个 `openTarget` 验证 `tsc` 红 |
| R4 | **零 `switch` 分发** | ESLint `no-restricted-syntax` 拦 `SwitchStatement` 的 discriminant 为 `event.type`；故意写一个验证红 |
| R5 | 加第 14 类只需加一个组件文件 + 注册表一行 | 加一个测试用事件类型，断言无需改任何既有组件文件 |
| R6 | 13 类各一个渲染测试，断言可见文本与交互后状态 | `getByRole` / `getByText` 查询；断言无 `data-testid`、无 class 断言 |
| R7 | `check-event-enum` 的第四个源纳入比对，脚本不再走「源缺失」分支 | 断言脚本比对 4 个集合且退出码 0 |
| R8 | `thought_chunk` 不渲染完整思维链 | payload 带全文时断言界面只出现摘要 |
| R9 | `request_permission` 阻塞当前轮 | 断言未应答时后续事件行不可交互 |

> **R4 是本单元的存在理由。** `switch (event.type)` 写起来更顺手，
> 但它让「13 类全覆盖」从编译期约束退化成人工纪律——加第 14 类时漏一个分支不会有任何提示。

### ○ U2.10.2 · 过滤器面板由注册表生成

| | |
|---|---|
| `goal` | 过滤器面板的项、中文名、`wire` 标识、默认可见性全部来自 `EVENT_RENDERERS`，不存在第二份列表 |
| `allowed_changes` | `frontend/src/features/conversation/TimelineFilter.tsx` 及其 `.module.css` · `frontend/tests/` 下对应测试 |
| `forbidden_changes` | 另建过滤项常量；把中文名硬编码在组件里 |
| `stop_conditions` | — |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | `filterItems.length === Object.keys(EVENT_RENDERERS).length` | 直接断言 |
| R2 | 每一类都能被开关 | 表驱动 13 行，关闭后断言该类事件行不可见 |
| R3 | 计数来自事件流 | 注入已知条数，断言计数一致 |
| R4 | 中文名走 `t()`，`wire` 等宽且不翻译 | 断言 `wire` 在两个 locale 文件中无词条 |
| R5 | 默认可见性三档各生效 | `shown` / `collapsed` / `hidden` 各一个用例 |

### ○ U2.10.3 · 输入区五段与队列状态条

| | |
|---|---|
| `goal` | 五段顺序写死的输入区，队列为空时整条隐藏，卡片不裁切向上展开的浮层 |
| `allowed_changes` | `frontend/src/features/conversation/Composer*.tsx` · `QueueStrip.tsx` · `ComposerRefs.tsx` · `WorkspaceInfoBar.tsx` 及各自 `.module.css` |
| `forbidden_changes` | 把五段做成可配置 `slots`；输入区卡片 `overflow: hidden`；空动词按钮文案 |
| `stop_conditions` | Q15（文本输入框 / 文本域无设计条目）· Q17（空状态无条目）未裁决 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 五段顺序写死，`ComposerProps` 无 `slots` 类字段 | 类型断言 + 渲染顺序断言 |
| R2 | `queued` 为空时 ① 整条不渲染；`refs` 为空时 ④ 不渲染 | 两个用例断言元素不存在 |
| R3 | 卡片不裁切向上展开的浮层 | 打开过滤器面板，断言面板完整可见（不断言 class，断言可见文本存在） |
| R4 | 队列条文案走 `t()` 且带复数 | 断言 `count=1` 与 `count=3` 两种文案不同 |
| R5 | 引用区文案固定「仅本轮有效」且走 `t()` | 断言词条存在 |
| R6 | 发送按钮 `disabled` 时 `cursor: default`、`opacity: .45` | Stylelint 断言，不用组件测试断样式 |
| R7 | 按钮用动词短语，无「确定」「提交」 | `check-i18n` 扫描 `zh-CN.json` 断言无空动词词条 |

---

## S2.11 · 计划面板 · 契约抽屉 · 证据抽屉 · 决策卡 ▲

**阶段交付物**：每条结论都能点开到结构化产物，D3 授权对话框永不提供「始终允许」。

### ○ U2.11.1 · 悬浮计划面板与子计划 DAG

| | |
|---|---|
| `goal` | 计划面板是纯 overlay，显示与否不改变对话布局；DAG 呈现子计划三态与进度；重规划记录可查看差异 |
| `allowed_changes` | `frontend/src/features/plan/**` · `frontend/tests/` 下对应测试 |
| `forbidden_changes` | 计划面板参与 flex 布局；页面内再放一个折叠开关；自绘 SVG 图标 |
| `stop_conditions` | Q20（动效 / 过渡全文零规定）未裁决 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 纯 overlay：开关面板前后对话列首条消息的位置不变 | 断言 `getBoundingClientRect().top` 相等 |
| R2 | 子计划三态各自渲染 | `blocked` / `executing` / `accepted` 三个用例 |
| R3 | 需求映射统计来自后端派生字段 | 断言渲染值 == MSW 返回值，前端不自行计算 |
| R4 | 重规划记录按版本列出且每条可「查看差异」 | 断言 5 个版本各有入口 |
| R5 | 状态词等宽不翻译 | 断言经 `StatusText` 渲染 |
| R6 | 面板可拖动且范围限制在主区内 | 拖到边界外断言位置被夹紧 |

### ○ U2.11.2 · 契约抽屉：五段 + 验收标准 ↔ 证据绑定

| | |
|---|---|
| `goal` | 契约抽屉呈现五段与验收标准列表，冻结态只有「修订契约版本」，无绑定证据的标准显示为「无证据」 |
| `allowed_changes` | `frontend/src/features/plan/ContractDrawer*.tsx` 及其 `.module.css` · `frontend/tests/` 下对应测试 |
| `forbidden_changes` | 冻结态提供「编辑」入口；对未绑定的证据做模糊匹配推断；抽屉宽度取三档之外的值 |
| `stop_conditions` | Q7（`acceptance_criteria` 是否契约字段）· Q16（表格无条目）· Q20（动效）未裁决 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 抽屉宽度只有 560 / 720 / 980 三档 | 类型层：传第四个值 `tsc` 红 |
| R2 | 冻结态**不存在**「编辑」入口 | 断言该按钮在 DOM 中不存在 |
| R3 | 每条验收标准显示绑定的 `ev-*`；无绑定显示「无证据」 | 三条标准两条有证据的用例，断言第三条显示「无证据」且无任何 `ev-*` |
| R4 | 版本切换 `v3` / `v2` / `v1` 各能取回 | 三个用例断言内容不同 |
| R5 | 写入边界与禁止边界分两栏呈现，条目数与后端一致 | 断言「写入边界 2 项 / 禁止 2 项」 |
| R6 | 页眉元信息含 `冻结于 ck-07` 与产出角色 | 断言可见文本 |

### ○ U2.11.3 · 证据抽屉与 D2/D3 决策卡

| | |
|---|---|
| `goal` | 每条结论都能点开到 `Evidence`；D2 卡片给选项与代价；**D3 授权对话框不提供「始终允许」，且不可点遮罩关闭** |
| `allowed_changes` | `frontend/src/features/conversation/EvidenceDrawer*.tsx` · `DecisionCard*.tsx` · `D3AuthDialog*.tsx` 及各自 `.module.css` · `frontend/tests/` 下对应测试 |
| `forbidden_changes` | D3 对话框出现「始终允许」控件；D3 对话框可点遮罩关闭；结论渲染不带证据入口；用弹窗打断执行中的单元展示非阻塞信息 |
| `stop_conditions` | Q3（`executing` 与 `waiting_user` 能否共存）· Q20（动效）未裁决 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | INV-EVD-7：每条结论都有证据入口 | 表驱动遍历审查意见 / 验收判定 / `Checkpoint` 三类结论，各断言存在可点击入口 |
| R2 | 四类 `Evidence` 各自的呈现 | `git_diff` / `test_output` / `command_record` / `review_note` 四个用例 |
| R3 | `superseded` 的证据仍可读且标注已废弃 | 断言可见且带标注 |
| R4 | D2 卡片 ≥ 2 个选项，至多一个推荐，每个选项写清代价 | 断言选项数、推荐数、每项代价文本非空 |
| R5 | **D3 对话框不存在「始终允许」控件** | 断言该文案与控件在 DOM 中均不存在 |
| R6 | **D3 对话框点遮罩不关闭**；D2 点遮罩可关闭 | 两个用例各断言关闭回调是否被调用 |
| R7 | `waiting_user` 期间派发类按钮 `disabled` | 断言按钮不可交互 |
| R8 | 破坏性按钮写清后果 | 断言文案含具体后果（`丢弃 2:14 工作`），不是「取消」 |

---

## S2.12 · 项目与新建工作 ▲

**阶段交付物**：黄金路径的前两步在界面上可走通。

### ○ U2.12.1 · 项目管理页

| | |
|---|---|
| `goal` | 添加本地仓库、查看项目列表与派生统计、移除项目（两种语义），路径选择走 platform 适配层 |
| `allowed_changes` | `frontend/src/features/project/ProjectList*.tsx` · `AddRepoDialog*.tsx` 及各自 `.module.css` · `frontend/tests/` 下对应测试 |
| `forbidden_changes` | 组件里 import `@tauri-apps/*`；移除项目提供第三种语义；空动词按钮 |
| `stop_conditions` | Q14（tab 选中态与规范矛盾）· Q16（表格无条目）· Q17（空状态无条目）· Q24（面包屑无条目）任一未裁决 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 移除项目**只有两个选项** | 断言选项数 == 2 且文案为「仅解除索引」/「连同 .acpflows 记忆一并清除」 |
| R2 | 破坏性选项写清后果 | 断言文案含「清除」与目录路径 |
| R3 | 路径选择走 `platform.pickDirectory()`，Web 降级为手输 + 后端校验 | 两个平台实现各一个用例，Web 下断言输入框存在且提交后触发校验请求 |
| R4 | 空状态可见 | 零项目时断言空状态文案存在 |
| R5 | 派生统计来自后端 | 断言 `2 个活动` / `4 / 12` == MSW 返回值 |
| R6 | 全部文案走 `t()` | `make check-i18n` 无缺失词条 |

### ○ U2.12.2 · 新建工作对话框与 `initializing_failed`

| | |
|---|---|
| `goal` | 选仓库与基线、点「创建 worktree 并开始」，环境未过时按钮不可用，创建失败展示原因且**不提供任何在原目录继续的入口** |
| `allowed_changes` | `frontend/src/features/project/NewWorkDialog*.tsx` 及其 `.module.css` · `frontend/tests/` 下对应测试 |
| `forbidden_changes` | 提供「在原目录继续」「跳过 worktree」之类的降级入口；硬编码状态词的中文译名 |
| `stop_conditions` | Q1（`initializing_failed` 未进术语表）· Q15（文本输入框无条目）未裁决 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 主按钮文案为「创建 worktree 并开始」 | 断言可见文本 |
| R2 | 环境检测未通过时按钮 `disabled`，并给出**可执行的**修复命令 | 断言按钮不可交互 + 命令字符串可见且等宽 |
| R3 | `initializing_failed` 展示原因 | 断言原因文本来自后端错误码翻译 |
| R4 | **不存在任何在原目录继续的入口** | 穷举断言：对话框内无任何按钮的 handler 指向不带 worktree 的创建路径 |
| R5 | 基线可选分支或指定 commit | 两个用例 |
| R6 | 状态词等宽不翻译 | 断言经 `StatusText` 渲染 |

---

## S2.13 · E2E 黄金路径与常驻守卫 ●

**阶段交付物**：一条 spec 走完 12 步，四条守卫在 CI 上常驻。

### ○ U2.13.1 · 黄金路径 E2E spec

| | |
|---|---|
| `goal` | 一条 Playwright spec 跑真实 `duetd` + Fake Runtime + 临时数据目录，走完从创建项目到检查点落盘的全部 12 步 |
| `allowed_changes` | `e2e/**` · `e2e/INDEX.md` · `backend/tests/fixtures/e2e/**` · `scripts/dev-web.sh` |
| `forbidden_changes` | 读写 `~/.acpflows` 或用户真实仓库；用 `data-testid` 兜底查询；跳过任何一步 |
| `stop_conditions` | 某一步在界面上无法走通 → 停，回到对应子计划，不要在 E2E 里绕过去 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 一条 spec 走完 12 步，每步有可见的断言点 | 12 个断言，缺一即红 |
| R2 | 跑真实 `duetd` + Fake Runtime + 临时数据目录 | 断言进程真实启动；断言 `$HOME/.acpflows` 未被访问 |
| R3 | 最终 `Checkpoint` 绑定的 commit 在工作分支上真实存在 | 用 `git cat-file -e` 校验 |
| R4 | 权限裁决那一步**真的阻塞** | 不裁决时断言 30s 内流程不推进；裁决后断言继续 |
| R5 | **证据抽屉里的 diff 与 `git diff` 直接算出的一致** | 让 Fake Runtime 在消息里贴一段不同的假 diff，断言界面显示的是真 diff |
| R6 | 界面上不出现任何未走 `t()` 的中文 | 截取全部可见文本，断言每条都能在 `zh-CN.json` 中找到 |
| R7 | 查询按用户可见的东西做 | 断言 spec 中零 `data-testid` |

> **R5 是黄金路径的验收核心。** 走通 12 步只证明流程不崩；
> **让 Fake Runtime 说谎再断言界面显示真相**，才证明这条链是应用驱动的。

### ○ U2.13.2 · 四条常驻守卫接进 CI

| | |
|---|---|
| `goal` | 把 M2 引入的四条规则做成常驻检查：事件枚举四处一致 · `Evidence` 无 Agent 文本路径 · 无 `switch` 分发 · 无自动写入 `Memory` |
| `allowed_changes` | `scripts/check-event-enum.sh` · `scripts/check-evidence-collector.sh` · `scripts/check-no-auto-memory.sh` · `Makefile` · `.github/workflows/ci.yml` · `scripts/AGENTS.md` |
| `forbidden_changes` | 让任一守卫在源缺失时静默跳过；把守卫做成只在本地跑的可选检查 |
| `stop_conditions` | 某条规则找不到可机器判定的形式 → 停，按 `adr/0003` 改成能拦的形式或明确标注「靠自觉」 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 事件枚举四处一致：故意在任一处改动 → 红 | 四个用例，每处各制造一次违规 |
| R2 | `Evidence` 采集器：故意加一个接受 ACP 类型的构造函数 → 红 | 制造违规验证 |
| R3 | 无 `switch` 分发：故意写一个 `switch (event.type)` → 红 | 制造违规验证 |
| R4 | 无自动写入 `Memory`：故意加一条直接产出 `active` `Memory` 的路径 → 红 | 制造违规验证 |
| R5 | 四条都进 `make check` 且进 CI | 删掉任一条 CI 步骤时 `make check` 红 |
| R6 | 守卫脚本自身有测试 | 每条守卫的正例与反例各一个用例 |

---

## M2 验收

**全部单元 `✓` 之外，还要满足：**

| # | 标准 | 怎么验 |
|---|---|---|
| A1 | `make check` 全绿，`domain/model` 与 `domain/policy` 覆盖率 ≥ 90% | CI + `make cover` |
| A2 | **E2E 黄金路径 12 步一次跑通并保持绿** | `pnpm -C e2e test` |
| A3 | **`Evidence` 的四类采集器全部由应用直接采集** —— 让 Fake Runtime 在 diff / 测试结论上说谎，界面显示的仍是真相 | U2.7.1 R4/R5 + U2.13.1 R5 |
| A4 | **不存在任何以 Agent 文本创建 `Evidence` 的构造路径** | `make check-evidence-collector`，AST 断言 `A ∩ B == ∅` |
| A5 | **不存在任何自动写入 `Memory` 的路径** | `make check-no-auto-memory` |
| A6 | 13 类事件封闭：四处一致，新增第 14 类必须同改四处 | `make check-event-enum`，四处各制造一次违规验证会红 |
| A7 | 前端**零 `switch (event.type)`**，13 类少一多一都 `tsc` 红 | ESLint + `pnpm typecheck`，各制造一次违规验证 |
| A8 | `domain-model.md` §17 的 115 条不变量中，**归属 M2 的 90 条各有一个先红过的测试** | 逐条核对本文「不变量分配」表与 `backend/tests/INDEX.md` |
| A9 | `Work` 状态机 25 条迁移的穷举断言通过（全组合，非抽样） | U2.7.3 R9 |
| A10 | D3 无「始终允许」：持久化层无该维度字段，界面无该控件 | U2.6.3 R3 + U2.11.3 R5 |
| A11 | 每个 `Work` 独占一个 worktree 与分支；创建失败后原仓库 `git status` 为空 | U2.3.1 R1/R2 |
| A12 | 界面上不存在未走 `t()` 的中文；状态词在两个 locale 文件里都无词条 | `make check-i18n` + U2.13.1 R6 |
| A13 | `grep -rn 'codex\|claude' backend/internal/{app,domain,api}` 为空 | CI（M0 U0.7.1 R4 的常驻检查在 M2 后仍然绿） |
| A14 | 关键目录文档齐备且填实 | `make check-docs` |
| A15 | 测试索引无遗漏、无死链 | `make check-test-index` |

**A3 与 A4 是 M2 真正的验收标准。** 其余都是过程；
**「结论由应用的证据支撑，不由 Agent 的自述支撑」才是这个产品的存在理由**——
这一条守不住，后面 M3 的记忆、M4 的报表全部建立在假数据上。
