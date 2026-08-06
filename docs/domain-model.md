# 领域模型

> **本文是 `backend/internal/domain/` 的规格说明，改领域逻辑前必读。**
>
> 读者：Claude / Codex / 人。本文只描述**模型与规则**，不含实现代码。
> 模型落地位置与写法见 [`coding-standards.md`](coding-standards.md) §1.1（一个聚合一个文件、充血模型）；
> 分层与依赖方向见 [`architecture.md`](architecture.md) §3；术语以根 [`AGENTS.md`](../AGENTS.md) §8 为准。

---

## 0. 怎么读这份文档

### 0.1 约定

| 约定 | 含义 |
|---|---|
| 状态词 | **一律英文原值、不翻译**：`clarifying` `planning` `ready` `executing` `reviewing_unit` `waiting_user` `paused` `completed` `failed` |
| 字段名 | 一律用设计稿里的原值（`allowed_changes` / `valid_from_commit` / `supersedes` …），不另起中文别名 |
| `†` | 该字段/取值**设计稿里没有直接出现**，是本文为闭合模型而推定的，必须在实现前确认，逐条列在 §17 开放项 |
| `INV-xx-n` | 不变量编号。每条都写成一句可直接变成断言的话，汇总见 §16 |
| `OPEN-n` | 明确的开放项，**不要在实现里替它做主张**，见 §17 |

### 0.2 一手依据

本文的字段名、状态词、契约结构、记忆 DB 列、角色表全部来自设计稿
（`design/ACP Duet 1a.dc.html` 界面原型 + `design/Duet Spec.dc.html` 规范）。
凡设计稿与本文冲突，**以设计稿为准并回来修本文**；凡设计稿自相矛盾，已列入 §17。

### 0.3 这份文档是测试先行的输入

按 [`testing-strategy.md`](testing-strategy.md) 的五步流程，§17 的每条不变量必须先变成一个**会红的测试**，
再有实现。`domain/model` 与 `domain/policy` 的覆盖率门槛是 **≥ 90%，零 mock，真实实例真实数据**。

---

## 1. 聚合总览

### 1.1 关系图

```
                       ┌─────────────────────────────────────────┐
   全局 ~/.acpflows    │  Runtime      一个 ACP adapter 进程       │
                       │  Role         8 个预置角色 + 自定义        │
                       │  Memory(L3)   跨项目记忆                  │
                       │  Skill(全局)  ~/.acpflows/skills/         │
                       └───────────────┬─────────────────────────┘
                                       │ Role ──绑定(n:1)──▶ Runtime
                                       │
┌──────────────────────────────────────┴───────────────────────────────────────┐
│ Project    本地 git 仓库 + <project>/.acpflows                                │
│            (skills/ · memory/ · project.yaml · runs/[gitignored])            │
└───┬──────────────────────┬───────────────────────┬───────────────────────────┘
    │ 1..n                 │ 1..n                  │ 1..n
    ▼                      ▼                       ▼
┌────────┐          ┌────────────┐          ┌────────────┐
│  Work  │          │ Memory(L2) │          │ Skill(L4)  │
└───┬────┘          └────────────┘          └────────────┘
    │  1:1  worktree 路径 + branch(duet/work-NN) + baseline(commit)
    │
    ├──1:0..1──▶ RequirementSnapshot †聚合归属待定(OPEN-2)   R1..Rn
    │
    ├──1:n────▶ PlanVersion   append-only · v1..vN · 恰一个 current
    │              │
    │              └──1:n──▶ Subplan      DAG(depends_on) · 无环 · 无范围空洞
    │                           │
    │                           └──1:n──▶ Unit    DAG(depends_on)
    │                                        │
    │                                        ├──1:n──▶ UnitContract   版本化 · 冻结后只读
    │                                        │             │
    │                                        │             ├──1:n──▶ AcceptanceCriterion
    │                                        │             └──n:m──▶ Memory / Skill@ver  (inject)
    │                                        │
    │                                        └──1:n──▶ Attempt   attempt 1..N
    │                                                     │
    │                                                     ├──1:n──▶ Evidence  4 类 · 应用采集
    │                                                     └──1:n──▶ OperationInvocation
    │                                                                 (Role + Runtime + set_mode)
    ├──1:n────▶ Decision     D0–D3
    └──1:n────▶ Checkpoint   绑定 commit hash

                 AcceptanceCriterion ──n:m──▶ Evidence   （显式绑定，无绑定即「无证据」）
                 Attempt ──审查结论──▶ accepted | implementation_fix | contract_revision | global_replan
```

### 1.2 基数表

| 关系 | 基数 | 说明 |
|---|---|---|
| Project → Work | 1 : n | 一个项目可有多个工作；同时活动数受 OPEN-11 限制 |
| Work → worktree | 1 : 1 | **独占**，不共享，不复用（INV-WT-1） |
| Work → PlanVersion | 1 : n | append-only，n ≥ 1，恰一个 `current` |
| Work → RequirementSnapshot | 1 : 0..1 | 冻结后计划才能开始；快照本身也版本化（`requirement v2`） |
| PlanVersion → Subplan | 1 : n | 每个版本持有一份完整 DAG 快照 |
| Subplan → Unit | 1 : n | 一个 Unit 只属于一个 Subplan |
| Unit → UnitContract | 1 : n | 契约版本 `contract_version` 1..N，至多一个 `frozen` 的 current |
| Unit → Attempt | 1 : n | `attempt 1..N`，任一时刻至多一个 `running` |
| Attempt → Evidence | 1 : n | 四类证据 |
| UnitContract → AcceptanceCriterion | 1 : n | 冻结时必须 ≥ 1 条 |
| AcceptanceCriterion → Evidence | n : m | 显式绑定；设计稿实例：3 条标准 / 2 条有证据 |
| Work → Decision | 1 : n | D0–D3 |
| Work → Checkpoint | 1 : n | `ck-05` `ck-06` `ck-07` … |
| Project → Memory(L2) | 1 : n | **不跨项目检索**（INV-PRJ-1） |
| Project → Skill(L4) | 1 : n | 另有全局 Skill 库 |
| Role → Runtime | n : 1 | 一个 Runtime 可承担多个 Role；一个 Role 任一时刻恰绑一个 Runtime |
| Attempt → OperationInvocation | 1 : n | 报表口径：`Runtime 使用 · 按 OperationInvocation 次数统计，不含重试` |

### 1.3 文件映射（`backend/internal/domain/model/`）

一个聚合一个文件，充血模型。

```
project.go      work.go        requirement_snapshot.go †
plan.go         subplan.go     unit.go        unit_contract.go
attempt.go      evidence.go    decision.go    checkpoint.go
memory.go       skill.go       role.go        runtime.go
```

跨聚合的规则（DAG 无环校验、决策等级判定、注入选择、计划重规划处置校验）放
`backend/internal/domain/policy/`，**不要塞进任何单个模型文件**。

### 1.4 标识符格式

| 聚合 | 展示 ID | 作用域 | 例 |
|---|---|---|---|
| Work | `work-NN` | Project 内单调递增 | `work-08` `work-09` |
| PlanVersion | `v<N>` | Work 内，从 1 连续 +1 | `plan v5` |
| Subplan | `subplan-NN` | Work 内单调递增，**不复用** | `subplan-01` `subplan-03` `subplan-04` |
| Unit | `unit-NNN` | Work 内单调递增，跨 Subplan 连续 | `unit-009` … `unit-013` |
| UnitContract | `v<N>` | Unit 内，从 1 连续 +1 | `契约 v3` |
| Attempt | `attempt <N>` | Unit 内，从 1 连续 +1 | `attempt 2` |
| Evidence | `ev-NNN` | Work 内单调递增 | `ev-438` `ev-440` `ev-441` |
| Checkpoint | `ck-NN` | Work 内单调递增 | `ck-05` `ck-06` `ck-07` |
| Memory | `mem-NNN` / 候选 `cand-NN` | Project（L2）或全局（L3） | `mem-203` `cand-07` |
| Skill | `<name>@<version>` | Project 或全局 | `rust-test-first@2.1` |

> **注意**：SSE 事件信封用的是 ULID（`evt_01J...`，见 `architecture.md` §4），
> 与上表的人类可读序号不是一回事。哪个是主键、序号是否持久化，见 **OPEN-1**。

---

## 2. Project

**职责**：把一个本地 git 仓库纳入 Duet 管理，并为它提供项目级的记忆库、Skill 库与配置。

### 2.1 关键字段

| 字段 | 说明 / 取值 |
|---|---|
| `name` | 项目名，取自目录名。例：`acp-engine` |
| `path` | 本地仓库绝对路径。例：`~/work/acp-engine` |
| `default_branch` | 默认基线分支。例：`main` / `develop` |
| `remote` † | git remote URL；可为空（`duet-app` 未配置 remote） |
| `github_account` † | 按 `remote` 匹配到的 GitHub 账号；权限为 `只读` 或 `可写（需逐次授权）` |
| `acpflows_dir` † | `<project>/.acpflows/`：`skills/` · `memory/` · `project.yaml` · `runs/` |
| `active_worktree_count` | 派生：该项目下当前活动的 worktree 数（`2 个活动`） |
| `skill_count` / `memory_count` | 派生：`4 / 12` |

### 2.2 不变量

| ID | 不变量 |
|---|---|
| INV-PRJ-1 | **项目记忆永远不会被其他项目检索。** 给定 `scope=project(P1)` 的 Memory，对属于 P2 的任何 Attempt 计算注入清单，结果不包含它 |
| INV-PRJ-2 | 创建项目会写 `<project>/.acpflows/`，并把 `.acpflows/runs/` 追加进 `.gitignore`；**运行记录不入库** |
| INV-PRJ-3 | 创建项目**不会开始任何工作**——不切 worktree、不起 Runtime 会话 |
| INV-PRJ-4 | 移除项目只有两种语义：`仅解除索引`（不删除任何文件）/ `连同 .acpflows 记忆一并清除`；不存在第三种 |
| INV-PRJ-5 | 环境缺 `git` 或缺 worktree 支持时**不得创建 Work**（硬要求）；语言工具链（如 `cargo`）按 Skill 的 `compatibility` 字段检查，缺失只影响该 Skill 可用性 |
| INV-PRJ-6 | 导入已有 Skill 目录时**复制**到 `.acpflows/skills/` 并标记 `draft`，**原目录保持不变** |

### 2.3 允许的操作

| 操作 | 前置 | 结果 |
|---|---|---|
| `AddLocalRepo` | 目录存在且含 `.git` | 建 `.acpflows/`、写 `.gitignore`、按 remote 推荐 GitHub 账号 |
| `SetDefaultBranch` | 分支存在 | 影响后续新建 Work 的默认基线 |
| `BindGitHubAccount` | 令牌有效 | 记录 remote → 账号绑定与读写档 |
| `ImportSkills` | 扫描 `**/skills`（跳过 `node_modules`、`target`） | 复制为 `draft` |
| `CleanWorktrees` | — | 清理该项目下已结束 Work 的 worktree |
| `Remove(mode)` | `mode ∈ {unlink_only, purge_acpflows}` | 见 INV-PRJ-4 |

---

## 3. Work

**职责**：一次完整的开发任务，独占一个 git worktree，并持有该任务的全部计划、执行、证据与决策历史。

### 3.1 关键字段

| 字段 | 说明 / 取值 |
|---|---|
| `id` | `work-08` |
| `project_id` | 所属项目 |
| `title` | `取消运行中的 Agent turn` |
| `state` | 见 §3.2 状态表 |
| `worktree_path` | `~/.duet/worktrees/acp-engine/work-09`（显示为 `wt/work-08`） |
| `branch` | `duet/work-08` |
| `baseline` | 基线 commit：`main@7c1de90`。**创建后不可改** |
| `current_plan_version` | `5` |
| `current_unit_id` | `unit-012`；无执行中单元时为空 |
| `accepted_units` / `total_units` | 派生进度：`3/7` |
| `last_event_seq` † | 最后事件游标；取消/暂停后必须可读（R4） |
| `queued_messages` † | 执行中收到的补充消息队列，见 §3.4 |
| `work_memory` † | 「写入 Work 工作记忆，作为后续子计划输入」的短期记忆，**与 L2/L3 Memory 不是一回事**（OPEN-8） |

### 3.2 状态表

**9 个状态词是 `AGENTS.md` §8 的封闭枚举**，外加两个入口态（见 OPEN-3）。

| 状态 | 语义 | 是否终态 | 谁在动 |
|---|---|---|---|
| `initializing` † | 正在创建 worktree 与分支 | 否 | 应用 |
| `initializing_failed` † | worktree/分支创建失败，**不会退回原目录执行** | **是** | — |
| `clarifying` | 把模糊描述变成可验证的需求快照 | 否 | 需求分析师 |
| `planning` | 出/改 PlanVersion、重排 Subplan DAG、出新契约版本 | 否 | 计划架构师 · 单元设计师 |
| `ready` | 计划已冻结、就绪单元契约已冻结，等待派发 | 否 | 应用 |
| `executing` | 某个 Unit 的 Attempt 正在跑 | 否 | 实现工程师 · 测试执行者 |
| `reviewing_unit` | Attempt 已结束，独立会话正在审查 diff 与证据 | 否 | 实现审查员 |
| `waiting_user` | 存在阻塞性决策（D2/D3）或权限裁决，等人 | 否 | 人 |
| `paused` | 已落检查点并停止，可恢复（更新前/用户主动） | 否 | — |
| `completed` | 计划内全部单元 accepted | **是** | — |
| `failed` | 不可恢复失败或用户放弃 | **是** | — |

### 3.3 迁移表

**只允许下表列出的迁移，其余一律拒绝**（INV-WORK-1）。

| # | from | to | 触发条件 | 守卫 |
|---|---|---|---|---|
| 1 | *(新建)* | `initializing` | 用户在「新建工作」对话框点 `创建 worktree 并开始` | 已选本地仓库 + 基线分支/commit；环境检测通过（INV-PRJ-5） |
| 2 | `initializing` | `clarifying` | worktree + 分支创建成功 | 分支名未被占用；worktree 目标路径不存在或为空 |
| 3 | `initializing` | `initializing_failed` | 创建 worktree 或分支失败 | 不得回落到原仓库工作目录执行任何写操作 |
| 4 | `clarifying` | `planning` | RequirementSnapshot 冻结（`requirement v2 已冻结`） | 待确认事实清单为空 |
| 5 | `clarifying` | `waiting_user` | 需求分析师提出待确认事实并升级为 D2（含「无法判定等级时上调 D2」） | Decision 已落库且 `level ≥ D2` |
| 6 | `planning` | `ready` | PlanVersion 冻结 | DAG 无环（INV-SUB-1）+ 无范围空洞（INV-SUB-2）+ 就绪单元契约已冻结（INV-CTR-6） |
| 7 | `planning` | `waiting_user` | 规划期产生 D2/D3 | 同 #5 |
| 8 | `ready` | `executing` | 派发就绪单元给实现工程师 | 该 Unit 全部前驱 `accepted`；契约 `frozen`；无未决 D2/D3 |
| 9 | `executing` | `reviewing_unit` | Attempt 结束（收到 `stopReason` 或 Agent 报告完成） | 该 Attempt 的证据采集已完成并落盘 |
| 10 | `executing` | `waiting_user` | ① `request_permission` 需人裁决 ② 触发契约 `stop_conditions` ③ 产生 D2/D3 | Decision / 权限请求已落库 |
| 11 | `executing` | `paused` | 用户暂停 或 `POST /v1/system/update/prepare` | 两段式 cancel 完成（发请求 → 等 `stopReason`）+ 证据与游标已采集 + checkpoint 已落盘 |
| 12 | `executing` | `failed` | 不可恢复错误（Runtime 崩溃且无法重连） | — |
| 13 | `reviewing_unit` | `executing` | 审查结论 `implementation_fix` | 契约版本不变；新建 `attempt N+1` |
| 14 | `reviewing_unit` | `ready` | 审查结论 `accepted` 且计划内仍有未完成单元 | 每条 `acceptance_criteria` 至少一条有效 Evidence（INV-EVD-4）；已自动提交并落 checkpoint |
| 15 | `reviewing_unit` | `completed` | 审查结论 `accepted` 且计划内全部单元 `accepted` | 同 #14，且全部需求条目已映射且有证据 |
| 16 | `reviewing_unit` | `planning` | 审查结论 `contract_revision` 或 `global_replan` | `contract_revision` → 必须产出新的 `contract_version`；`global_replan` → 必须产出新的 PlanVersion 且声明已验收工作处置（INV-PLAN-4） |
| 17 | `reviewing_unit` | `waiting_user` | 审查把问题升级为 D2（设计稿实例：越界改 `EngineEvent` 公开枚举） | 同 #5 |
| 18 | `reviewing_unit` | `paused` | `update/prepare` | 当前 attempt 标记 `superseded` + checkpoint 落盘 |
| 19 | `waiting_user` | `executing` | 用户确认选项，且不需要改计划与契约 | Decision 已落库且不可变 |
| 20 | `waiting_user` | `ready` | 用户确认选项，产生新契约版本 | 新 `UnitContract` 已冻结 |
| 21 | `waiting_user` | `planning` | 用户选择需要重规划的选项（设计稿实例：`C · 回退到 ck-07 并重新规划`） | 新 PlanVersion 已声明已验收工作处置 |
| 22 | `waiting_user` | `paused` | 用户暂停 或 `update/prepare` | checkpoint 落盘（`waiting_user` 下直接落，保留待决策项） |
| 23 | `paused` | `ready` / `executing` / `waiting_user` | `POST /v1/system/resume/{workId}` 或启动时自动恢复 | 恢复到 checkpoint 记录的 `resume_state` †；worktree HEAD 与 checkpoint 的 commit hash 一致 |
| 24 | `paused` | `failed` | 用户丢弃该工作（按钮写清后果：`丢弃 2:14 工作`） | 需二次确认 |
| 25 | 任一非终态 | `failed` | 用户放弃 | 需二次确认；已采集证据保留 |

**迁移表的读法**：第 3 列是「什么事发生了」，第 4 列是「不满足就必须拒绝迁移」。
守卫失败返回领域错误（如 `ErrMustReviewBeforeComplete`、`ErrContractNotFrozen`），
**不要静默停留在原状态**。

### 3.4 执行中的补充消息（队列）

用户在 `executing` 期间发消息时，**不弹窗打断**（设计规范第 10 节反例），改为入队并给三个处理方式：

| 处理方式 | 语义 | 对状态机的影响 |
|---|---|---|
| `单元结束后评估影响`（默认） | 不打断 Codex，attempt 完成后由 Claude 判断是否改契约 | 无；可能在 `reviewing_unit` 后触发 `contract_revision` |
| `立即打断当前单元` | 发协议 `cancel`，**保留现场证据**，attempt 标记 `superseded` | `executing` → `reviewing_unit` 或 `planning`（OPEN-6） |
| `仅记录，不影响本次计划` | 写入 Work 工作记忆，作为后续子计划输入 | 无 |

### 3.5 不变量

| ID | 不变量 |
|---|---|
| INV-WORK-1 | 只允许 §3.3 迁移表列出的迁移；其余组合一律返回 `ErrInvalidTransition`，**不得静默停留在原状态** |
| INV-WORK-2 | `completed` / `failed` / `initializing_failed` 是终态，不可再迁出 |
| INV-WORK-3 | 进入 `completed` 要求：计划内全部 Unit 为 `accepted`，且每条 `acceptance_criteria` 至少一条有效 Evidence |
| INV-WORK-4 | 一个 Work 任一时刻至多一个 `running` 的 Attempt |
| INV-WORK-5 | 存在未决 D2/D3 时不得迁入 `executing`（阻塞语义见 §12.3） |
| INV-WORK-6 | 补充消息一律入队，**不得用弹窗打断执行中的单元**；三种处理方式之外没有第四种 |
| INV-WORK-7 | `paused` 恢复后必须回到 Checkpoint 记录的 `resume_state`，不得回到任意状态 |

### 3.6 允许的操作

`Transition(to)` · `Enqueue(message, disposition)` · `AttachReference(file|dir|image)` ·
`Pause()` · `Resume(checkpoint)` · `Abandon(reason)`

---

## 4. Work 与 worktree

**职责**：把「一次工作」在文件系统上物理隔离，使并行工作互不干扰，且原工作区永不受影响。

```
项目仓库 ~/work/acp-engine        基线 main@7c1de90
        │
        │ git worktree add
        ▼
~/.duet/worktrees/acp-engine/work-09      分支 duet/work-09
   └─ Agent 的**全部**写操作只能落在这里
```

| 字段 | 说明 |
|---|---|
| `worktree_root` | 全局设置项，默认 `~/.duet/worktrees`（注意：与数据目录 `~/.acpflows` 不是一处，见 OPEN-10） |
| `worktree_path` | `<worktree_root>/<project>/<work>` |
| `branch` | `duet/work-NN` |
| `baseline` | `<branch>@<commit>`，例 `main@7c1de90`；也可「指定 commit…」 |
| `ahead_count` | 派生：`领先 3 commit` |
| `dirty_count` | 派生：`未提交改动 · 3` |

### 4.1 不变量

| ID | 不变量 |
|---|---|
| INV-WT-1 | 一个 Work 独占一个 worktree 与一个分支；同一 worktree 路径 / 分支名不得同时属于两个 Work |
| INV-WT-2 | worktree 创建失败 → Work 进入 `initializing_failed`，**且不得在原仓库目录执行任何写操作** |
| INV-WT-3 | Agent 的写操作只能落在本 Work 的 worktree 内；越界写必须走 `request_permission` 并由审查判定越界 |
| INV-WT-4 | 附加的外部文件 / 只读挂载文件夹 / 粘贴的图片**只读进入本轮上下文**，不写入 worktree，**也不会自动沉淀为记忆**，且仅本轮有效 |
| INV-WT-5 | 单元验收通过后自动提交到工作分支（`验收后自动提交`）；**push / 发 PR / 删除远端属于 D3**，需逐次授权 |
| INV-WT-6 | `baseline` 创建后不可变；重规划不改基线，只能新增 PlanVersion 或回退到 Checkpoint |

---

## 5. RequirementSnapshot †（设计稿有，聚合归属待定）

**职责**：把用户的模糊描述固化成一份可验证、可映射到证据的需求条目清单。

设计稿证据：`requirement v2 已冻结`、`Requirement Snapshot v2 · 待确认事实清单`、
右栏「需求 → 证据」矩阵 `R3 / R4 / R5`、计划面板统计 `需求 6 · 已映射 6`。

| 字段 | 说明 |
|---|---|
| `work_id` | 所属 Work |
| `version` | `v2`；与 PlanVersion 一样只增不改 † |
| `items[]` | `R1..Rn`，每条一句可验证的陈述。例：`R3 取消必须幂等` |
| `open_facts[]` | 待确认事实清单；非空时不得冻结 |
| `frozen_at` † | 冻结时间 |

| ID | 不变量 |
|---|---|
| INV-REQ-1 | `open_facts` 非空时不得冻结；未冻结时 Work 不得离开 `clarifying` |
| INV-REQ-2 | 冻结后不可改写，修订只能产生新版本（与 PlanVersion 同规则） |
| INV-REQ-3 | **无范围空洞**：PlanVersion 冻结时，每条 `items[]` 至少映射到一个 Unit（`需求 6 · 已映射 6`） |
| INV-REQ-4 | 需求条目的完成判定看证据，不看 Agent 自述：`R5 · 无证据` 即未完成 |

> **归属未定**：RequirementSnapshot 是独立聚合，还是 Work 的一个值对象，见 **OPEN-2**。

---

## 6. PlanVersion

**职责**：以 append-only 的方式记录这个 Work 的每一次规划与重规划，使「为什么变成现在这样」永远可追溯。

### 6.1 关键字段

| 字段 | 说明 / 取值 |
|---|---|
| `work_id` | 所属 Work |
| `version` | 从 `1` 起连续 +1，例 `5` |
| `reason` | 重规划理由，**非空**。例：`架构假设错误：取消需两段协议 → subplan-03 重拆` |
| `based_on_requirement_version` † | 依据的需求快照版本 |
| `dispositions[]` | v ≥ 2 必填：对创建时点**每一项已验收工作**的处置，见 §6.3 |
| `subplans[]` | 该版本的完整 Subplan DAG 快照 |
| `is_current` | 派生：`version == max(version)` |
| `created_at` | 创建时间 |

### 6.2 append-only 规则

> 设计稿原文（重规划记录面板）：
> **「计划只能新增版本，不能改写：任何重规划都要说明哪些已验收工作『仍有效 / 需补充 / 需回滚 / 已废弃』。」**

```
v1  初版计划
v2  需求快照 v2 冻结后重算受影响子计划
v3  D2 决策：Runtime 抽象层先行 → 子计划顺序调整
v4  架构假设错误：取消需两段协议 → subplan-03 重拆
v5  用户补充「取消要覆盖 reviewing_unit」→ 新增 unit-013      ← 当前
```

规则：

1. **写路径只有一条：`AppendVersion`。** 模型上**不存在**任何修改已有 PlanVersion 的方法。
2. 版本号连续、无空洞、无重复；`current` 恒等于最大版本号。
3. 每个版本必带 `reason`（上面五条都是真实的 `reason`）。
4. 旧版本被新版本取代**不等于**被删除：`v1..v4` 永远可读，「查看差异」按 `v4 → v5` 呈现。
5. 冻结的 `UnitContract.based_on_plan_version` 指向创建时的版本号，**不随新版本改写**。

### 6.3 已验收工作的处置（`dispositions[]`）

任何 v ≥ 2 的版本，都必须对创建时点**每一项已验收的工作**（已 `accepted` 的 Unit / Subplan）
给出且只给出一个处置：

| 中文（界面） | 标识 † | 语义 | 后果 |
|---|---|---|---|
| 仍有效 | `still_valid` | 已验收成果不受本次重规划影响 | 无；对应 Checkpoint 继续有效 |
| 需补充 | `needs_supplement` | 成果保留，但需要追加单元 | 新版本必须包含至少一个补充 Unit |
| 需回滚 | `needs_rollback` | 成果必须撤销 | 必须指向一个回退目标 Checkpoint |
| 已废弃 | `obsolete` | 成果作废但不删除历史 | 相关 Unit 不再计入进度分母 †（OPEN-7） |

设计稿实例（D2 选项 C）：`回退到 ck-07 并重新规划 —— 丢弃当前 attempt，创建 plan v6；已验收的 subplan-01 保持有效。`
→ 即 `subplan-01: still_valid`。

### 6.4 不变量

| ID | 不变量 |
|---|---|
| INV-PLAN-1 | PlanVersion 一经创建**任何字段不可变**；对已存在版本的写操作返回 `ErrPlanVersionImmutable` |
| INV-PLAN-2 | `version` 从 1 开始连续 +1，无空洞、无重复 |
| INV-PLAN-3 | 每个 Work 任一时刻**恰有一个** `current` PlanVersion，且等于最大版本号 |
| INV-PLAN-4 | `version ≥ 2` 时，创建时点每一项已验收工作都必须有且只有一个 `disposition`；缺一条即拒绝创建 |
| INV-PLAN-5 | `reason` 非空 |
| INV-PLAN-6 | 新增版本不得删除任何历史版本，也不得改写旧版本引用的 Subplan/Unit 快照 |
| INV-PLAN-7 | `disposition = needs_rollback` 必须携带一个存在的 `Checkpoint` 作为回退目标 |

### 6.5 允许的操作

`AppendVersion(reason, dispositions, subplans)` · `Current()` · `Diff(fromVersion, toVersion)`
**没有** `Update` / `Delete`。

---

## 7. Subplan

**职责**：计划下的阶段，构成有向无环图，带阶段交付物与需求映射。

### 7.1 关键字段

| 字段 | 说明 / 取值 |
|---|---|
| `id` | `subplan-01` `subplan-03` `subplan-04` |
| `plan_version` | 归属的 PlanVersion |
| `title` | `ACP Runtime 抽象层` / `取消与现场保留` / `恢复与检查点` |
| `deliverable` | 阶段交付物：`统一 Adapter 接口 + 能力探针` |
| `requirement_refs[]` | 需求映射：`R1 R2` |
| `depends_on[]` | 前驱 Subplan ID 列表 |
| `state` | `blocked` † / `ready` † / `executing` / `accepted`（见 OPEN-4） |
| `accepted_units` / `total_units` | 派生进度：`3/3` `0/2` |

### 7.2 DAG 规则

```
 subplan-01 ✓accepted 3/3 ──▶ subplan-03 executing 0/2 ──▶ subplan-04 blocked 0/2
   Runtime 抽象层                取消与现场保留                恢复与检查点
```

1. **无环校验**：`depends_on` 构成的有向图必须无环。校验在 `domain/policy`，冻结 PlanVersion 时执行。
2. **无范围空洞**：需求快照的每条 `R*` 至少被一个 Unit 覆盖；否则拒绝冻结（INV-REQ-3）。
3. **依赖阻塞**：只要存在任一前驱 Subplan 未 `accepted`，本 Subplan 恒为 `blocked`，不得进入 `executing`。
4. Subplan 编号在 Work 内单调递增且**不复用**——重规划中被废弃的编号留下空洞
   （设计稿 v5 的 DAG 里 `subplan-02` 缺席即此现象，**这是推断，见 OPEN-5**）。

### 7.3 不变量

| ID | 不变量 |
|---|---|
| INV-SUB-1 | `depends_on` 图无环；构造含环的计划返回 `ErrSubplanCycle` |
| INV-SUB-2 | PlanVersion 冻结时不存在范围空洞：每条需求条目至少被一个 Unit 覆盖 |
| INV-SUB-3 | 存在未 `accepted` 前驱时，Subplan 状态恒为 `blocked`，任何迁往 `executing` 的尝试被拒 |
| INV-SUB-4 | Subplan 只属于一个 PlanVersion；跨版本靠 `id` 追溯，内容按版本快照 |
| INV-SUB-5 | Subplan ID 在 Work 内唯一且不复用 |
| INV-SUB-6 | `accepted` 要求其全部 Unit 均 `accepted`（`3/3`） |

---

## 8. Unit

**职责**：最小可执行工作单位，一个 Unit 对应一份冻结契约与一串 Attempt。

### 8.1 关键字段

| 字段 | 说明 / 取值 |
|---|---|
| `id` | `unit-012` |
| `subplan_id` | `subplan-03` |
| `title` | `取消幂等性与现场保留` |
| `depends_on[]` | 前驱 Unit：`unit-013 依赖 unit-012` |
| `state` † | `draft`（契约未冻结）/ `ready` / `executing` / `reviewing_unit` / `accepted` / `obsolete`（OPEN-4） |
| `current_contract_version` | `3`；未冻结时为空（`unit-013 · 契约未冻结`） |
| `attempt_count` | `2` |

### 8.2 不变量

| ID | 不变量 |
|---|---|
| INV-UNIT-1 | 一个 Unit 属于且仅属于一个 Subplan |
| INV-UNIT-2 | **契约未冻结的 Unit 不得进入 `executing`**；远期单元可以留白，但执行前必冻结 |
| INV-UNIT-3 | 存在未 `accepted` 的前驱 Unit 时不得进入 `executing` |
| INV-UNIT-4 | Unit ID 在 Work 内唯一且不复用；编号跨 Subplan 连续 |
| INV-UNIT-5 | `accepted` 要求：审查结论为 `accepted` **且**每条 `acceptance_criteria` 至少一条有效 Evidence |

### 8.3 允许的操作

`AttachContract(contract)` · `Freeze()`（委托契约）· `StartAttempt()` · `Accept(reviewResult)` ·
`Supersede(reason)`

---

## 9. UnitContract

**职责**：把交接写成契约而不是 Prompt——冻结的交接规格，含写入边界、测试策略、停止条件与验收标准。

### 9.1 契约结构（照抄设计稿抽屉里的 YAML）

```yaml
unit_id: unit-012
subplan_id: subplan-03
contract_version: 3
based_on_plan_version: 5

goal: "用户能取消正在运行的 Agent turn，并保留现场证据"

allowed_changes:
  - runtime cancellation module
  - 相关测试

forbidden_changes:
  - 不改变数据库公开 schema
  - 不改变 EngineEvent 公开枚举        # ← attempt 1 命中

test_strategy:
  - 状态机单元测试
  - Fake ACP 延迟响应集成测试

stop_conditions:
  - 需要扩大写入范围
  - 发现公开接口必须变化

acceptance_criteria:                    # 抽屉里独立渲染为「验收标准」列表
  - id: R3
    text: 连续取消两次只发送一次协议取消请求
    evidence: [ev-441]                  # ✓
  - id: R4
    text: 取消后 diff 与最后事件游标可读取
    evidence: [ev-440]                  # ✓
  - id: R5
    text: reviewing_unit 状态下取消被拒绝
    evidence: []                        # ○ 无证据

inject:
  - skill:rust-test-first@2.1
  - mem-203
  - mem-188
```

抽屉页眉的元信息：`v3 当前 / v2 / v1` · `冻结于 ck-07` · `由单元设计师产出` ·
`已冻结 · 写入边界 2 项 / 禁止 2 项`。

> **注**：`acceptance_criteria` 在设计稿的 YAML 代码块里没有直接出现，而是在同一抽屉中作为
> 「验收标准」列表渲染并与 `ev-*` 绑定。本文把它归入契约字段——它是验收判定的唯一依据，
> 不能游离在契约之外。见 **OPEN-9**。

### 9.2 冻结机制

```
draft ──Freeze()──▶ frozen ──(不可改)
                       │
                       │ 需要改动 → 只能出新版本
                       ▼
                  contract_version + 1 → 新的 draft → Freeze()
```

| 规则 | 说明 |
|---|---|
| 冻结时机 | 契约在派发给实现工程师**之前**冻结；冻结时记录 `frozen_at_checkpoint`（`冻结于 ck-07`）与 `based_on_plan_version` |
| 冻结后 | 任何字段写入返回 `ErrContractFrozen`。界面上只有「修订契约版本」按钮，没有「编辑」 |
| 修订触发 | 审查结论 `contract_revision`，或 D2 决策产生新契约（`决定会写入 Decision 记录并生成新的契约版本`） |
| 版本可见性 | 旧版本永远可读（`v3 当前 / v2 / v1`），不删除 |
| Attempt 绑定 | 每个 Attempt 绑定一个**已冻结**的契约版本；`attempt 2 · 契约 v3` |

### 9.3 不变量

| ID | 不变量 |
|---|---|
| INV-CTR-1 | `frozen` 契约的任何字段写入返回 `ErrContractFrozen` |
| INV-CTR-2 | 修订只能产生新版本；`contract_version` 从 1 连续 +1，无空洞、无重复 |
| INV-CTR-3 | 冻结时必须记录 `frozen_at_checkpoint` 与 `based_on_plan_version`，二者非空且指向存在的对象 |
| INV-CTR-4 | 冻结要求 `goal` / `allowed_changes` / `forbidden_changes` / `test_strategy` / `stop_conditions` / `acceptance_criteria` **六项均非空**；`inject` 可空 |
| INV-CTR-5 | `allowed_changes` 与 `forbidden_changes` 的范围不得相交 |
| INV-CTR-6 | Attempt 只能引用 `frozen` 的契约版本；引用 `draft` 返回 `ErrContractNotFrozen` |
| INV-CTR-7 | diff 落在 `allowed_changes` 之外时，审查结论**不得**为 `accepted`（只能是 `implementation_fix` 或 `contract_revision`） |
| INV-CTR-8 | 命中任一 `stop_conditions` 时 Attempt 必须停止并上报，**不得自行扩大范围继续做** |
| INV-CTR-9 | `inject` 中引用的 Memory 必须为 `active`、Skill 必须为 `active` 且 `compatibility` 满足；否则冻结失败 |
| INV-CTR-10 | 每条 `acceptance_criteria` 必须有稳定 `id`（`R3` `R4` `R5`），Evidence 按 `id` 绑定 |

---

## 10. Attempt 与审查

**职责**：对一个 Unit 的一次执行，是证据的产生者与 Runtime 使用的记账单位。

### 10.1 关键字段

| 字段 | 说明 / 取值 |
|---|---|
| `unit_id` | `unit-012` |
| `attempt_no` | `2`（Unit 内从 1 连续 +1） |
| `contract_version` | `3` |
| `state` | `running` / `succeeded` / `superseded` / `rejected` |
| `started_at` / `elapsed` | 界面显示 `执行中 2:14` |
| `role_id` † | 承担本次执行的角色（`implementer`） |
| `runtime_name` / `runtime_version` / `set_mode` | **本次实际使用的** Runtime 与模式，写进执行报告以便追溯 |
| `start_checkpoint` † | 起始检查点（执行报告模板要求：`契约版本 / 起始检查点 / diff hash`） |
| `diff_hash` † | 本次 diff 的哈希 |
| `review_result` | `accepted` / `implementation_fix` / `contract_revision` / `global_replan`；未审查时为空 |

### 10.2 Attempt 状态机

```
                    ┌──────────┐
   派发 ───────────▶│ running  │
                    └────┬─────┘
        审查 accepted    │    审查 implementation_fix / contract_revision / global_replan
          ┌──────────────┼──────────────┐
          ▼              │              ▼
    ┌───────────┐        │        ┌──────────┐
    │ succeeded │        │        │ rejected │
    └───────────┘        │        └──────────┘
                         │ 被打断（用户「立即打断当前单元」/ update prepare）
                         ▼
                   ┌────────────┐
                   │ superseded │   现场证据保留
                   └────────────┘
```

| 状态 | 语义 | 终态 |
|---|---|---|
| `running` | 正在执行 | 否 |
| `succeeded` | 审查结论为 `accepted` | 是 |
| `superseded` | 被打断/被取代，现场证据保留 | 是 |
| `rejected` | 审查判定不通过 | 是 |

### 10.3 审查四类结果

审查由**实现审查员**在**独立会话**中做（不延续规划会话、不修改代码、不伪造通过）。

| 结果 | 语义 | Attempt | Work 迁移 | 必须产出 |
|---|---|---|---|---|
| `accepted` | diff 在边界内，每条验收标准都有证据 | `succeeded` | → `ready` / `completed` | Checkpoint + 自动提交 |
| `implementation_fix` | 实现问题，契约不变 | `rejected` | → `executing`（`attempt N+1`） | 驳回理由 + 证据引用 |
| `contract_revision` | 契约不清或边界需调整 | `rejected` | → `planning` | 新的 `contract_version` |
| `global_replan` | 架构假设错误 | `rejected` | → `planning` | 新的 PlanVersion + 已验收工作处置 |

设计稿实例：`attempt 1 · implementation_fix —— attempt 1 的 diff 越出写入边界（改动了 EngineEvent
公开枚举），已驳回并触发 attempt 2；这条同时升级为 D2 决策等你确认。`

报表页给出驳回原因的分布（这是审查结论的下钻维度，不是第五类结果）：

| 驳回原因 | 计数 | 映射到 |
|---|---|---|
| diff 越出写入边界 | 11 | `implementation_fix` |
| 测试未真正验证 | 7 | `implementation_fix` |
| 契约不清 → 改版本 | 5 | `contract_revision` |
| 架构假设错误 → 重规划 | 2 | `global_replan` |

### 10.4 不变量

| ID | 不变量 |
|---|---|
| INV-ATT-1 | `attempt_no` 在 Unit 内从 1 连续 +1，无空洞、无重复 |
| INV-ATT-2 | 一个 Unit 任一时刻至多一个 `running` 的 Attempt；一个 Work 任一时刻至多一个 `running` 的 Attempt |
| INV-ATT-3 | `running` 是唯一非终态；`succeeded` / `superseded` / `rejected` 不可再迁出 |
| INV-ATT-4 | `state = succeeded` **当且仅当** `review_result = accepted` |
| INV-ATT-5 | 被打断的 Attempt 一律标 `superseded`，其已采集的 Evidence **必须保留** |
| INV-ATT-6 | Attempt 必须记录本次实际使用的 `runtime_name` + `runtime_version` + `set_mode`；缺任一项不得进入 `succeeded` |
| INV-ATT-7 | 审查必须在**独立会话**中进行：审查的会话 id 不得等于产生该 Attempt 的会话 id |
| INV-ATT-8 | 实现方不得审查自己：`review_result` 的产出角色不得是本 Attempt 的执行角色 |
| INV-ATT-9 | 「全绿」不构成 `accepted`：存在任一 `acceptance_criteria` 无有效 Evidence 时，`accepted` 被拒（`无证据即未通过`） |

---

## 11. Evidence

**职责**：为每条结论提供可核对的入口。**证据由应用直接采集，不是 Agent 的转述。**

### 11.1 硬约束（这条不可商量）

> 根 `AGENTS.md` §9 禁止清单：**「✗ 把 Agent 的转述当证据（证据必须由应用直接采集）」**
> 设计稿证据抽屉页眉：**「由应用直接采集，非 Agent 转述」**

含义，逐条落到实现上：

| 证据类型 | **唯一合法采集者** | 绝不接受 |
|---|---|---|
| Git diff | `internal/gitx` 对 worktree 直接算 diff | Agent 消息里贴的 diff 文本 |
| 测试输出 | 应用起的进程的 stdout/stderr + exit code | Agent 说「测试通过了」 |
| 命令记录 | 应用记录的 argv / cwd / exit code / 耗时 / 输出 | Agent 复述的命令与结果 |
| 审查意见 | 独立审查会话的结构化结论（含引用的 `ev-*`） | 实现方自评 |

**判定方法（可测）**：Evidence 有 `collector` 字段，取值只能是这四个采集器之一；
不存在任何路径能让 ACP `message_chunk` / `thought_chunk` 的文本成为 Evidence 的内容来源。

### 11.2 关键字段

| 字段 | 说明 / 取值 |
|---|---|
| `id` | `ev-440` `ev-441` `ev-438` |
| `attempt_id` | 产生它的 Attempt |
| `kind` | `git_diff` † / `test_output` † / `command_record` † / `review_note` †（界面：Git diff · 测试输出 · 命令记录 · 审查意见） |
| `collector` | 采集器标识，见 §11.1 |
| `status` † | `valid` / `superseded`（`ev-438 attempt 1 越界 · 已废弃 · superseded`） |
| `summary` | `cancel.rs +64 −12 · 边界内` / `cancel_idempotent.rs +118 · 新增测试` |
| `in_boundary` † | diff 类专有：是否落在 `allowed_changes` 内（`均在写入边界内`） |
| `exit_code` † | 命令/测试类专有：`cargo test exit 0` |
| `criteria_refs[]` | 绑定的验收标准 id（`R3` → `ev-441`） |

### 11.3 不变量

| ID | 不变量 |
|---|---|
| INV-EVD-1 | Evidence 只能由四类采集器创建；**任何以 Agent 文本为内容来源的创建路径都不存在** |
| INV-EVD-2 | Evidence **不可删除**，只能标记 `superseded`；被 `superseded` 的证据仍可读 |
| INV-EVD-3 | 验收标准与 Evidence 的绑定是**显式**的；没有绑定就是「无证据」，不做任何推断 |
| INV-EVD-4 | 单元验收要求每条 `acceptance_criteria` 至少一条 `valid` 的 Evidence；否则 `accepted` 被拒 |
| INV-EVD-5 | `command_record` / `test_output` 类必须含 `exit_code`；缺失即无效证据 |
| INV-EVD-6 | `git_diff` 类必须能判定 `in_boundary`；无法判定视为越界 |
| INV-EVD-7 | 界面上每条结论都必须能点开到 Evidence（`结论不带证据入口` 是设计反例） |

---

## 12. Decision（D0–D3）

**职责**：把「谁能拍板、什么时候必须停下来问人」写成可判定的等级，并留下不可篡改的决定记录。

### 12.1 等级定义

| 等级 | 名称 | 语义 | 是否阻塞 | 典型 |
|---|---|---|---|---|
| `D0` | 自动 | 应用/Agent 直接执行，不留决策记录 | 否 | 契约边界内的读文件、跑测试 |
| `D1` | 记录 | 自动执行但必须留 Decision 记录 | 否 | 边界内的可逆改动；实现工程师「D1 以上只报告」 |
| `D2` | 需确认 | **必须由人从给定选项中选一个**才能继续 | **是** | 改公开接口 / 越出写入边界 / 架构假设变更 |
| `D3` | 逐次授权 | **每一次**外部动作单独授权 | **是** | `push` · 发布 · 删除远端 · 付费资源 |

### 12.2 判定规则

1. **升级优先**：拿不准就上调。需求分析师边界写死「无法判定等级时上调 D2」；
   决策顾问「给选项与代价，不替用户拍板，拿不准就上调 D2」。**任何情况下不得下调等级。**
2. **越界即 D2**：diff 越出 `allowed_changes`、命中 `stop_conditions`、需要改公开接口或外部行为 → 至少 D2。
3. **外部动作即 D3**：`push` / 发布 / 删除远端资源 / 付费资源，**即使令牌是可写的**也必须逐次授权。
4. **权限裁决不是 Decision**：ACP `request_permission` 是协议层的一轮阻塞
   （`允许一次` / `拒绝`），由 Role 的「权限裁决」策略应答；它可能**升级**成一个 D2 Decision，但两者是不同对象。

### 12.3 D2 / D3 阻塞语义

| | D2 | D3 |
|---|---|---|
| Work 状态 | 产生时 Work 进入 `waiting_user` | 同左 |
| 派发 | 未决期间不得派发新 Attempt | 同左 |
| 选项 | ≥ 2 个选项，至多一个 `recommended`，每个选项写清**代价** | 允许一次 / 拒绝 |
| 「始终允许」 | 允许（由 Role 权限裁决策略配置） | **不适用，永不提供** |
| 遮罩关闭 | 允许点遮罩关闭 | **D3 授权对话框不可点遮罩关闭** |
| 延后 | `稍后决定` | 同左，等价于拒绝本次 |

设计稿的 D2 实例（`扩展 EngineEvent 公开枚举`）：

```
A · 新增 Cancelled 变体            [推荐]
    向后兼容；订阅端需加分支。影响 3 个文件、1 条公开契约，
    需新增 contract v4 与一条迁移说明。
B · 复用 Stopped{reason}
    不动公开契约，语义变模糊；后续恢复逻辑可能需要再拆。
C · 回退到 ck-07 并重新规划
    丢弃当前 attempt，创建 plan v6；已验收的 subplan-01 保持有效。

决定会写入 Decision 记录并生成新的契约版本；
D3（push / 发布 / 删除）仍需逐次授权，不受本次选择影响。
```

### 12.4 关键字段

| 字段 | 说明 / 取值 |
|---|---|
| `work_id` | 所属 Work |
| `level` | `D0` / `D1` / `D2` / `D3` |
| `title` | `扩展 EngineEvent 公开枚举` |
| `options[]` | `{key, title, cost, recommended}`；`A/B/C` |
| `chosen_option` | 用户所选；未决时为空 |
| `origin` † | 触发来源：`review` / `stop_condition` / `permission_escalation` / `role_escalation` |
| `unit_id` / `attempt_id` † | 关联对象 |
| `decided_by` / `decided_at` † | 谁在什么时候决定的 |
| `authorized_action` † | D3 专有：被授权的**具体一次**动作（`cargo publish --dry-run`） |

### 12.5 不变量

| ID | 不变量 |
|---|---|
| INV-DEC-1 | 每个 Decision 恰有一个 `level ∈ {D0, D1, D2, D3}` |
| INV-DEC-2 | 存在未决的 D2/D3 时，Work 处于 `waiting_user` 且不得派发新 Attempt |
| INV-DEC-3 | **D3 逐次授权**：一次 D3 授权只对一个具体动作实例有效；系统中不存在任何形式的 D3「始终允许」持久化记录 |
| INV-DEC-4 | 同一 D3 动作再次发生时必须重新授权，不得复用历史授权 |
| INV-DEC-5 | D3 覆盖 `push` / 发布 / 删除远端 / 付费资源；即使 GitHub 令牌为可写档也必须逐次授权 |
| INV-DEC-6 | **Agent 永远拿不到令牌本身**；远端操作由应用代为执行 |
| INV-DEC-7 | 等级只能上调不能下调；无法判定时判为 D2 |
| INV-DEC-8 | D2 的 `options` 数量 ≥ 2，且 `recommended = true` 的至多一个 |
| INV-DEC-9 | Decision 落库后不可变；改主意 = 新建一条 Decision |
| INV-DEC-10 | 用户做出选择本身**不降低**其他等级的要求：一次 D2 选择不授予任何 D3 权限 |

---

## 13. Checkpoint

**职责**：可恢复点，绑定一个真实 commit hash。

### 13.1 关键字段

| 字段 | 说明 / 取值 |
|---|---|
| `id` | `ck-07` `ck-06` `ck-05` |
| `work_id` | 所属 Work |
| `commit_hash` | `abc123` `9f2ee1` `3f10ab2` |
| `trigger` | `unit-011 accepted` / `subplan-01 accepted` / `update_prepare` † / `user_pause` † |
| `resume_state` † | 恢复后 Work 应回到的状态（`executing` / `ready` / `waiting_user`），见 OPEN-6 |
| `unit_id` † | 恢复接口返回：`{work_id, checkpoint_id, unit_id}` |
| `created_at` | `2 分钟前` / `1 小时前` |

### 13.2 落点时机

| 事件 | 落 Checkpoint |
|---|---|
| Unit `accepted` | 是（验收后自动提交 → 落点，如 `ck-07 · unit-011 accepted · abc123`） |
| Subplan `accepted` | 是（`ck-06 · subplan-01 accepted · 9f2ee1`） |
| UnitContract 冻结 | 不新建，只**引用**当前检查点（`冻结于 ck-07`） |
| `update/prepare` | 是，每个非终态 Work 都必须落点或报 `blocked` |
| 用户暂停 | 是 |

### 13.3 不变量

| ID | 不变量 |
|---|---|
| INV-CKP-1 | Checkpoint 必须绑定一个在该 Work 的分支上**真实存在**的 commit hash |
| INV-CKP-2 | Checkpoint 不可变、不可删除 |
| INV-CKP-3 | 每次 Unit `accepted` 与 Subplan `accepted` 都必须落一个 Checkpoint |
| INV-CKP-4 | 恢复时必须校验 worktree HEAD 与 `commit_hash` 一致；不一致则拒绝恢复并上报 |
| INV-CKP-5 | `update/prepare` 对每个非终态 Work 要么落 Checkpoint 并置 `paused`，要么返回 `blocked` 及原因；**不存在第三种结果** |
| INV-CKP-6 | 回退到某 Checkpoint 必须产生一个新的 PlanVersion（`回退到 ck-07 并重新规划 → 创建 plan v6`），不得原地改写计划 |

---

## 14. Memory（L2 / L3）

**职责**：把被证据支持的项目知识沉淀成可注入、可失效、可追溯的条目。

### 14.1 层级

| 层 | 范围 | 落盘 | 检索边界 |
|---|---|---|---|
| **L2** | 项目记忆 | `<project>/.acpflows/memory/<id>.md` | **只在本项目内检索**（INV-MEM-1） |
| **L3** | 跨项目记忆 | `~/.acpflows/memory/<id>.md` | 全部项目；晋升前**必须去项目标识后送审** |

> L4 是 Skill（§15）。L0/L1 未在设计稿出现，见 OPEN-13。

### 14.2 md 是内容，DB 只存索引与状态

```
<project>/.acpflows/memory/mem-203.md          SQLite (~/.acpflows/duet.db)
┌────────────────────────────────┐            ┌──────────────────────────────┐
│ ---                            │            │ scope                        │
│ id: mem-203                    │            │ kind                         │
│ kind: constraint               │  ◀──索引──▶ │ status                       │
│ scope: acp-engine              │            │ confidence                   │
│ ---                            │            │ sensitivity                  │
│ 集成测试使用临时 SQLite，        │            │ source_refs                  │
│ 不得读写用户真实数据库           │            │ created_by                   │
│                                │            │ created_at / updated_at      │
│ 依据：ev-412 · unit-009        │            │ valid_from_commit            │
└────────────────────────────────┘            │ last_verified_commit         │
   人可读可编辑、可入 git                       │ supersedes                   │
                                              │ 注入统计                      │
                                              └──────────────────────────────┘
```

**DB 不存正文。** 启动时执行 DB ↔ 文件对账：文件被人手改过或删过，索引要能自愈并上报差异。

### 14.3 关键字段（照抄设计稿「系统数据库记录 · SQLite」）

| 字段 | 取值 / 说明 |
|---|---|
| `id` | `mem-203`；候选用 `cand-07`（见 OPEN-14） |
| `scope` | 项目名（L2）或跨项目（L3） |
| `kind` | `constraint` / `experience` / `fact` |
| `status` | `candidate` / `active` / 已失效 † `invalid` / 废弃 † `obsolete` |
| `confidence` | 置信度（取值域未定义，OPEN-12） |
| `sensitivity` | 敏感度，设计稿实例 `internal`（取值域未定义，OPEN-12） |
| `source_refs` | 依据的证据/单元引用，例 `ev-412 · unit-009` |
| `created_by` | 产出者角色，设计稿实例 `memory_curator（记忆管理员）` |
| `created_at` / `updated_at` | 时间戳 |
| `valid_from_commit` | 从哪个 commit 起有效 |
| `last_verified_commit` | 最近一次重新校验通过的 commit（`随 abc123 重新校验通过`） |
| `supersedes` | 指向被本条取代的记忆 |
| `注入统计` | 被注入次数等派生统计 |

md 侧 frontmatter 至少含：`id` / `kind` / `scope`；正文之后是「依据：`source_refs`」。

### 14.4 状态机

```
                 用户确认（必须有人的动作）
   candidate ─────────────────────────────▶ active
       │                                     │
       │ 用户否决 †                            ├──「标记失效」──▶ invalid †
       ▼                                     │
   discarded †(OPEN-14)                      └──「废弃（保留历史）」──▶ obsolete †
                                                     必须给理由，可指向 supersedes
```

**绝不自动写入。** 这条在三处被写死：
`AGENTS.md` §9 反例「✗ 自动把聊天原文或一次成功经验写成长期记忆」、
设计规范第 07 节事件表「记忆写入候选 · App candidate · **绝不自动写入**」、
`architecture.md` §4 事件枚举 `memory_candidate`。

**失效不等于删除**（设计稿原文）：
> 「标记失效后新 Attempt 不再注入，但历史运行仍可追溯当时用过它；废弃需说明原因并可指向 `supersedes`。」

变更历史（不可删）示例：

```
2026-08-04 11:20   由 unit-009 证据创建 · candidate
2026-08-04 15:02   用户确认 → active
2026-08-06 09:41   随 abc123 重新校验通过
```

### 14.5 不变量

| ID | 不变量 |
|---|---|
| INV-MEM-1 | `scope` 为项目 P1 的记忆，永不出现在属于 P2 的任何注入清单中 |
| INV-MEM-2 | **绝不自动写入**：Memory 只能以 `candidate` 创建；`candidate → active` 必须伴随一次用户确认动作，不存在任何自动晋升路径 |
| INV-MEM-3 | 聊天原文、单次成功经验不得直接成为 Memory 正文；`source_refs` 必须指向 Evidence 或 Unit |
| INV-MEM-4 | 状态迁移只允许：`candidate→active` · `candidate→discarded` · `active→invalid` · `active→obsolete`；其余拒绝 |
| INV-MEM-5 | `invalid` / `obsolete` 的记忆不进入**新** Attempt 的注入清单；但历史 Attempt 的 `injection` 记录仍能解析出它 |
| INV-MEM-6 | 记忆**不可物理删除**（失效 ≠ 删除）；历史运行的可追溯性不得被任何状态变更破坏 |
| INV-MEM-7 | `obsolete` 必须带理由；`supersedes` 若非空，必须指向存在的记忆，不得指向自身，且 `supersedes` 链无环 |
| INV-MEM-8 | 正文只存在于 md 文件；DB 中不存正文字段 |
| INV-MEM-9 | 晋升为 L3（`提升为跨项目`）必须先去项目标识再送审，且需用户确认 |
| INV-MEM-10 | 每次状态变更追加一条变更历史；历史条目不可删、不可改 |
| INV-MEM-11 | `valid_from_commit` / `last_verified_commit` 必须是该项目仓库中存在的 commit |
| INV-MEM-12 | 启动对账：md 文件与 DB 索引不一致时必须上报差异，**不得静默丢弃任一侧** |

### 14.6 允许的操作

`ProposeCandidate(source_refs)` · `Confirm()` · `Reject()` · `EditContent()` ·
`Reverify(commit)` · `PromoteToCrossProject()` · `MarkInvalid()` · `Deprecate(reason, supersedes)` ·
`ExportYAML()`
**没有** `Delete`。

---

## 15. Skill（L4）

**职责**：把可复用的执行方法固化成版本化、可回滚、可按需载入的指令包。

### 15.1 关键字段

| 字段 | 说明 / 取值 |
|---|---|
| `name` | `rust-test-first` |
| `version` | `2.1` / `1.4` / `0.4` / `0.9` |
| `scope` | 项目（`<project>/.acpflows/skills/`）或全局（`~/.acpflows/skills/`） |
| `status` | `draft` / `active` / `deprecated` |
| `hit_count` | `命中 46` / `命中 12` |
| `validation_error` | `校验未通过：frontmatter 缺 description` |
| frontmatter | `name` · `description`（**必需**）· `compatibility`（例 `cargo ≥ 1.80`） |

引用语法：`skill:<name>@<version>`，例 `skill:rust-test-first@2.1`。

### 15.2 四类文件的语义

```
skills/rust-test-first/
├── SKILL.md          主文档 · 必需       常驻指令上下文（何时使用 / 步骤 / 禁止）
├── scripts/          需权限放行         可执行；运行属于工具调用，受权限裁决约束
│   └── run_tests.sh
├── references/       按需载入           只在需要时载入上下文，不随 SKILL.md 常驻
│   └── cargo-matrix.md
└── assets/           输出模板           产出物模板，不进入指令上下文
    └── report-template.md
```

| 目录 | 进不进指令上下文 | 何时读 | 约束 |
|---|---|---|---|
| `SKILL.md` | **进**，常驻 | 注入时 | 必需；缺失即校验失败 |
| `scripts/` | 不进（只进文件清单） | 被显式调用时执行 | **需权限放行**，走 `request_permission` |
| `references/` | 按需进 | Agent 明确需要时 | 不随 `SKILL.md` 常驻，避免上下文膨胀 |
| `assets/` | **不进** | 生成产出物时读取 | 只作模板，不作指令 |

### 15.3 版本化与状态机

```
   新建 / 导入 ──▶ draft ──校验通过 + 发布──▶ active ──▶ deprecated
                    ▲                          │
                    └────── 回滚：把旧版本重新置 active ──┘
```

| 规则 | 说明 |
|---|---|
| 导入 | 复制到 `.acpflows/skills/` 并标记 `draft`，**原目录保持不变**；校验通过才可 `active` |
| 校验 | 至少检查：`SKILL.md` 存在、frontmatter 含 `name` + `description` |
| 发布 | 新版本不覆盖旧版本；同一 `name` 任一时刻至多一个 `active` 版本 |
| 回滚 | 把旧版本重新置 `active`，不删除新版本 |
| 兼容性 | `compatibility` 由环境检测校验（`cargo 之类的语言工具链按项目 Skill 的 compatibility 字段检查`） |

### 15.4 不变量

| ID | 不变量 |
|---|---|
| INV-SKL-1 | 新建/导入的 Skill 一律为 `draft`；未通过校验不得置 `active` |
| INV-SKL-2 | 校验失败必须给出可读原因（`frontmatter 缺 description`），不得静默拒绝 |
| INV-SKL-3 | 同一 `name` 任一时刻至多一个 `active` 版本 |
| INV-SKL-4 | 发布新版本不删除旧版本；回滚 = 重新置旧版本为 `active` |
| INV-SKL-5 | `deprecated` 的 Skill 不进入**新**的 `inject` 清单；历史注入记录仍可解析 |
| INV-SKL-6 | 导入不改动原目录（复制而非移动） |
| INV-SKL-7 | `references/` 不随 `SKILL.md` 进入常驻上下文；`assets/` 永不进入指令上下文 |
| INV-SKL-8 | `scripts/` 的执行必须经过权限裁决，不得静默执行 |
| INV-SKL-9 | `compatibility` 不满足时该 Skill 不得被注入 |
| INV-SKL-10 | 项目 Skill 与全局 Skill 同名时的优先级见 OPEN-15，**不要擅自选一边** |

---

## 16. Role 与 Runtime 绑定

> 设计稿原则（设置页页脚）：**「角色先定义、再绑定 Runtime——同一个角色换成另一端不影响状态机；
> 一个 Runtime 也可以承担多个角色。」**

### 16.1 8 个预置角色

| 角色 | AI 操作 | 性格与提示语气 | Runtime | `set_mode` | 权限裁决 |
|---|---|---|---|---|---|
| 需求分析师 | `clarify` · `snapshot` | 追问式，逐条确认，不放过「大概/应该」 | `claude` | `plan` | 逐条询问 |
| 计划架构师 | `plan` · `subplan_dag` | 结构优先、克制，不写含糊单元 | `claude` | `plan` | 逐条询问 |
| 单元设计师 | `unit_contract` | 把交接写成契约而不是 Prompt，边界写死 | `claude` | `plan` | 逐条询问 |
| 实现工程师 | `implement` | 沉默执行、报告详尽，触发停止条件立即停 | `codex` | `auto` | 逐条询问 |
| 测试执行者 | `test` · `report` | 只跑不改，失败原文照录，不修饰结论 | `codex` | `read-only` | 自动允许读 |
| 实现审查员 | `review_unit` | 怀疑式，「全绿」不等于验证过，无证据即未通过 | `claude` | `default` | 自动允许读 |
| 决策顾问 | `advise_decision` | 给选项与代价，不替用户拍板，拿不准就上调 D2 | `claude` | `plan` | 自动允许读 |
| 记忆管理员 | `curate_memory` | 保守，只提候选；跨项目一律去项目标识后送审 | `claude` | `default` | 自动允许读 |

上表的 Runtime / `set_mode` / 权限裁决三列是**推荐绑定**，用户可改；`恢复推荐绑定` 恢复到这一行。

### 16.2 角色的四要素

对话页的角色卡把每个角色展开成四项——**这四项是 Role 的字段，不是文案**：

| 要素 | 例（实现审查员） |
|---|---|
| `duty` 职责 | 独立会话审查 diff 与证据，判定四类结果之一 |
| `personality` 性格 | 怀疑式，「全绿」不等于验证过；无证据即未通过 |
| `boundary` 边界 | 不延续规划会话、不修改代码、不伪造通过 |
| `output` 产出 | `accepted` / `implementation_fix` / `contract_revision` / `global_replan` |

其余已知的边界（直接来自设计稿，都是可测的约束）：

| 角色 | 边界 |
|---|---|
| 需求分析师 | 不写代码、不定技术方案；**无法判定等级时上调 D2** |
| 计划架构师 | 不实现、不改验收标准；**远期单元可留白但执行前必冻结** |
| 实现工程师 | 不改目标、外部行为、测试标准与写入边界；**D1 以上只报告** |

### 16.3 Role 字段

| 字段 | 说明 |
|---|---|
| `id` † | 角色标识。**唯一被设计稿证实的是 `memory_curator`**（记忆管理员），其余 7 个见 OPEN-16 |
| `display_name` | `实现审查员` |
| `operations[]` | 承担的 AI 操作（下表 11 个之一或多个） |
| `duty` / `personality` / `boundary` / `output` | §16.2 |
| `prompt` | 角色提示词，可编辑 |
| `runtime_name` | 绑定的 Runtime |
| `set_mode` | `plan` / `auto` / `read-only` / `default`（**取值来自 `session/new` 返回的 `availableModes`**） |
| `permission_policy` | `逐条询问` / `自动允许读`（对应如何应答 `session/request_permission`） |
| `is_preset` † | 是否预置 |

**11 个 AI 操作**：`clarify` · `snapshot` · `plan` · `subplan_dag` · `unit_contract` ·
`implement` · `test` · `report` · `review_unit` · `advise_decision` · `curate_memory`

### 16.4 Runtime 字段

| 字段 | 说明 / 取值 |
|---|---|
| `name` | `claude` / `codex` |
| `package` | `claude-agent-acp` / `codex-acp` |
| `version` | `0.63.0` / `1.1.7` |
| `path` | `~/.npm-global/bin/claude-agent-acp` |
| `protocol_version` | `1` |
| `probe_result` | `12/12 通过` / `11/12 通过` |
| `install_state` | `已安装 · 已登录` / `未安装` |
| `process_state` | `就绪` / `运行中` |
| `default_permission_profile` | `codex` 的默认权限档是 `agent`（不询问）——**会话建立后自动 `session/set_mode` 收权** |
| `available_modes[]` | 由 `session/new` 返回 |
| `usage_count` | 报表：`codex 1,204 次` / `claude 620 次`，按 `OperationInvocation` 计数，**不含重试** |

### 16.5 「ACP 不提供、因此这里不设」

> 设计稿原文：**「模型与推理强度不在协议里——由各 Runtime 自身配置决定（`claude` 的 settings、
> `codex` 的 config），应用只记录本次实际使用的 Runtime 版本与模式，写进执行报告以便追溯。」**

因此：

- **Role 不含 `model` / `reasoning_effort` 字段。** 对话角色卡上出现的 `claude · sonnet · 中`、
  `codex · gpt-5-codex · 高` 是**观测到的运行时事实**（写在执行报告里），不是可配置项。
- 设计 ACP 不支持的设置项是 `AGENTS.md` §9 的明令反例。

### 16.6 隔离开关

| 开关 | 语义 |
|---|---|
| `关闭 Runtime 机器级记忆` | 禁止 Runtime 使用自己的跨会话记忆 |
| `禁用未授权项目 MCP Server` | 未授权的项目级 MCP Server 不加载 |
| `允许 Runtime 内建 Skill` | 是否允许 Runtime 自带的 Skill 参与 |

默认值设计稿未给出，见 OPEN-17。

### 16.7 不变量

| ID | 不变量 |
|---|---|
| INV-ROLE-1 | Role 先定义再绑定 Runtime：改绑 Runtime 不改变任何状态机迁移与守卫的结果 |
| INV-ROLE-2 | Role **不含** `model` / `reasoning_effort` 字段；模型信息只作为观测结果记录在 Attempt 上 |
| INV-ROLE-3 | 一个 Role 任一时刻恰绑定一个 Runtime；一个 Runtime 可承担多个 Role |
| INV-ROLE-4 | `set_mode` 取值必须属于该 Runtime `session/new` 返回的 `available_modes`；不在集合内则拒绝保存 |
| INV-ROLE-5 | 审查类角色必须在独立会话执行，不得延续规划会话（同 INV-ATT-7） |
| INV-ROLE-6 | 11 个 AI 操作每个至少被一个已绑定 Runtime 的 Role 覆盖，否则 Work 不可开始 †（OPEN-18） |
| INV-RT-1 | Runtime 探针未通过项必须体现为能力降级，**不得静默当作可用** |
| INV-RT-2 | `codex` 会话建立后必须自动 `session/set_mode` 收权；未收权前不得派发写操作 |
| INV-RT-3 | Runtime 使用统计按 `OperationInvocation` 计数且**不含重试** |
| INV-RT-4 | `protocol_version` 不为 `1` 时的处理未定义（见 `architecture.md` §8.2），**不得擅自兼容** |

---

## 17. 不变量清单（汇总）

**这一节是测试先行的清单。** 每条都写成一句可直接变成断言的话；
写实现前先让它红一次（[`testing-strategy.md`](testing-strategy.md) §1）。

> **与各聚合小节的关系（不是重复）**：各聚合的「不变量」表写的是**规则本身**（为什么成立、约束什么）；
> 本节写的是**同一条规则的断言形式**（怎么验、断言什么）。两处 ID 一一对应，
> **ID 集合必须完全相等**——新增一条不变量而漏登记本节，视为文档缺陷。
> 当前共 **115 条**。

### 17.1 Project

| ID | 断言 |
|---|---|
| INV-PRJ-1 | 对项目 P2 计算注入清单，结果不含任何 `scope` 为 P1 的 Memory |
| INV-PRJ-2 | 创建项目后 `.gitignore` 含 `.acpflows/runs/` |
| INV-PRJ-3 | 创建项目后该项目的 Work 数为 0，且未创建任何 worktree |
| INV-PRJ-4 | `Remove(unlink_only)` 后 `.acpflows/` 下文件数不变；`Remove(purge_acpflows)` 后目录不存在 |
| INV-PRJ-5 | 环境探测缺 git 或缺 worktree 支持时 `CreateWork` 返回错误且不产生 Work |
| INV-PRJ-6 | `ImportSkills` 后原目录内容逐字节不变，新 Skill 状态为 `draft` |

### 17.2 Work 与 worktree

| ID | 断言 |
|---|---|
| INV-WORK-1 | 对全部 (from,to) 组合调用 `Transition`，只有 §3.3 列出的组合返回 nil，其余返回 `ErrInvalidTransition` |
| INV-WORK-2 | `completed` / `failed` / `initializing_failed` 状态下任何 `Transition` 都返回错误 |
| INV-WORK-3 | 进入 `completed` 时，计划内全部 Unit 为 `accepted`，且每条 `acceptance_criteria` 都有 ≥1 条 `valid` Evidence |
| INV-WORK-4 | 一个 Work 任一时刻 `running` 的 Attempt 数 ≤ 1 |
| INV-WORK-5 | 存在未决 D2/D3 时 `ready → executing` 被拒 |
| INV-WORK-6 | `executing` 期间收到的消息进入 `queued_messages`，Work 状态不变，且不产生阻塞式对话框 |
| INV-WORK-7 | `Resume()` 后的状态等于 Checkpoint 的 `resume_state` |
| INV-WT-1 | 两个 Work 不共享 `worktree_path`，也不共享 `branch` |
| INV-WT-2 | worktree 创建失败后 Work 为 `initializing_failed`，且原仓库工作目录 `git status` 无变化 |
| INV-WT-3 | 写路径不在 `worktree_path` 前缀下时，写请求必须触发 `request_permission` 或被拒 |
| INV-WT-4 | 附加外部文件后，worktree 中不出现该文件；本轮结束后引用清空；不产生任何 Memory candidate |
| INV-WT-5 | Unit `accepted` 后工作分支多出一个 commit；调用 push 时若无 D3 授权则被拒 |
| INV-WT-6 | `baseline` 在 Work 创建后任何写操作都返回错误 |

### 17.3 需求与计划

| ID | 断言 |
|---|---|
| INV-REQ-1 | `open_facts` 非空时 `Freeze()` 返回错误；未冻结时 `clarifying → planning` 被拒 |
| INV-REQ-2 | 已冻结的 RequirementSnapshot 任何字段写入返回错误 |
| INV-REQ-3 | 存在未被任何 Unit 覆盖的需求条目时，PlanVersion 冻结被拒 |
| INV-REQ-4 | 无绑定 Evidence 的需求条目，其完成判定为 false（不因 Agent 文本变 true） |
| INV-PLAN-1 | 对已存在的 PlanVersion 调用任何 setter 返回 `ErrPlanVersionImmutable` |
| INV-PLAN-2 | 连续 `AppendVersion` 得到的 `version` 序列为 1,2,3,…，无空洞无重复 |
| INV-PLAN-3 | 任意时刻 `count(is_current) == 1` 且等于 `max(version)` |
| INV-PLAN-4 | v≥2 且缺少任一已验收工作的 `disposition` 时，`AppendVersion` 返回错误 |
| INV-PLAN-5 | `reason` 为空时 `AppendVersion` 返回错误 |
| INV-PLAN-6 | 新增版本后，旧版本的 Subplan/Unit 快照逐字段不变 |
| INV-PLAN-7 | `disposition = needs_rollback` 而未给回退 Checkpoint 时返回错误 |

### 17.4 Subplan / Unit / UnitContract

| ID | 断言 |
|---|---|
| INV-SUB-1 | 含环的 `depends_on` 使 DAG 校验返回 `ErrSubplanCycle` |
| INV-SUB-2 | 同 INV-REQ-3 |
| INV-SUB-3 | 前驱未 `accepted` 时 Subplan 状态为 `blocked`，迁往 `executing` 被拒 |
| INV-SUB-4 | 同一 `subplan-NN` 在不同 PlanVersion 下的内容可不同，但归属版本唯一 |
| INV-SUB-5 | 已使用过的 Subplan ID 不会被重新分配给新的 Subplan |
| INV-SUB-6 | 存在非 `accepted` 的 Unit 时，Subplan 迁往 `accepted` 被拒 |
| INV-UNIT-1 | Unit 的 `subplan_id` 唯一且非空 |
| INV-UNIT-2 | 契约未冻结时 `StartAttempt()` 返回 `ErrContractNotFrozen` |
| INV-UNIT-3 | 前驱 Unit 未 `accepted` 时 `StartAttempt()` 被拒 |
| INV-UNIT-4 | 已使用过的 Unit ID 不会被重新分配 |
| INV-UNIT-5 | 存在无证据的验收标准时 `Accept()` 被拒 |
| INV-CTR-1 | 对 `frozen` 契约的任何 setter 返回 `ErrContractFrozen` |
| INV-CTR-2 | 连续 `Revise()` 得到 `contract_version` 1,2,3,…，无空洞无重复 |
| INV-CTR-3 | 冻结后 `frozen_at_checkpoint` 与 `based_on_plan_version` 均非空且可解析 |
| INV-CTR-4 | 六项必填字段任缺其一时 `Freeze()` 返回错误 |
| INV-CTR-5 | `allowed_changes` 与 `forbidden_changes` 相交时 `Freeze()` 返回错误 |
| INV-CTR-6 | Attempt 引用 `draft` 契约时创建失败 |
| INV-CTR-7 | 存在越界 diff Evidence 时，`review_result = accepted` 被拒 |
| INV-CTR-8 | 命中 `stop_conditions` 时 Attempt 不得继续，且必须产生一条 Decision（等级 ≥ D2） |
| INV-CTR-9 | `inject` 含非 `active` 的 Memory/Skill 时 `Freeze()` 返回错误 |
| INV-CTR-10 | 每条 `acceptance_criteria` 的 `id` 在契约内唯一且非空 |

### 17.5 Attempt / Evidence

| ID | 断言 |
|---|---|
| INV-ATT-1 | 连续 `StartAttempt()` 得到 `attempt_no` 1,2,3,… |
| INV-ATT-2 | 已有 `running` Attempt 时再次 `StartAttempt()` 返回错误 |
| INV-ATT-3 | 终态 Attempt 的任何状态迁移返回错误 |
| INV-ATT-4 | `state == succeeded` ⟺ `review_result == accepted`（双向） |
| INV-ATT-5 | 打断后 Attempt 为 `superseded`，且其 Evidence 集合与打断前一致 |
| INV-ATT-6 | 缺 `runtime_name` / `runtime_version` / `set_mode` 任一项时不得进入 `succeeded` |
| INV-ATT-7 | 审查会话 id ≠ 执行会话 id，否则审查结论无效 |
| INV-ATT-8 | 审查结论的产出角色 ≠ 本 Attempt 的执行角色 |
| INV-ATT-9 | 全部测试通过但存在无证据标准时，`accepted` 被拒 |
| INV-EVD-1 | 不存在任何 API 能以 `message_chunk` / `thought_chunk` 文本创建 Evidence；`collector` 只能取四类采集器之一 |
| INV-EVD-2 | 不存在 `DeleteEvidence`；`Supersede()` 后仍可读取原内容 |
| INV-EVD-3 | 未显式绑定的验收标准，其 `evidence` 为空数组（不做模糊匹配） |
| INV-EVD-4 | 任一验收标准无 `valid` Evidence 时 `Accept()` 被拒 |
| INV-EVD-5 | `test_output` / `command_record` 缺 `exit_code` 时创建失败 |
| INV-EVD-6 | 无法判定 `in_boundary` 的 diff Evidence 视为越界（`in_boundary = false`） |
| INV-EVD-7 | 任一结论对象（审查意见、验收判定、检查点）都能解析出至少一个 Evidence 入口 |

### 17.6 Decision / Checkpoint

| ID | 断言 |
|---|---|
| INV-DEC-1 | `level` 只能取 D0/D1/D2/D3；非法值构造失败 |
| INV-DEC-2 | 存在未决 D2/D3 时 Work 为 `waiting_user` 且 `StartAttempt()` 被拒 |
| INV-DEC-3 | D3 授权后，持久化中不存在任何「始终允许」记录（穷举存储字段断言） |
| INV-DEC-4 | 同一 D3 动作第二次发生时仍返回「需授权」 |
| INV-DEC-5 | GitHub 账号为可写档时，`push` 仍返回「需 D3 授权」 |
| INV-DEC-6 | 任何进入 Agent 上下文的载荷中不含令牌串 |
| INV-DEC-7 | 判定函数在信息不足时返回 D2；不存在返回值低于输入等级的路径 |
| INV-DEC-8 | D2 的 `options` 长度 ≥ 2 且 `recommended` 数量 ≤ 1 |
| INV-DEC-9 | 已 `decided` 的 Decision 任何写入返回错误 |
| INV-DEC-10 | 确认一个 D2 后，D3 动作仍需单独授权 |
| INV-CKP-1 | `commit_hash` 在该 Work 分支上可解析；否则创建失败 |
| INV-CKP-2 | Checkpoint 无 setter、无 delete |
| INV-CKP-3 | Unit/Subplan `accepted` 后 Checkpoint 数量 +1 |
| INV-CKP-4 | worktree HEAD 与 `commit_hash` 不一致时 `Resume()` 返回错误 |
| INV-CKP-5 | `update/prepare` 对每个非终态 Work，结果集合 ⊆ {prepared, blocked}，且两者之和等于非终态 Work 数 |
| INV-CKP-6 | 回退到 Checkpoint 后 `max(plan_version)` 比回退前大 1 |

### 17.7 Memory / Skill

| ID | 断言 |
|---|---|
| INV-MEM-1 | 见 INV-PRJ-1 |
| INV-MEM-2 | 不存在把 Memory 直接创建为 `active` 的路径；`Confirm()` 必须带用户主体 |
| INV-MEM-3 | `source_refs` 为空或不指向 Evidence/Unit 时，`ProposeCandidate` 失败 |
| INV-MEM-4 | 穷举 status 组合，只有四条迁移返回 nil |
| INV-MEM-5 | 记忆置 `invalid` 后新注入清单不含它；查询历史 Attempt 的 `injection` 仍返回它 |
| INV-MEM-6 | 不存在 `DeleteMemory` |
| INV-MEM-7 | `Deprecate("")` 失败；`supersedes` 指向自身或构成环时失败 |
| INV-MEM-8 | DB 索引结构中不存在正文字段（穷举列名断言） |
| INV-MEM-9 | `PromoteToCrossProject` 的送审载荷中不含项目名/路径等项目标识 |
| INV-MEM-10 | 每次状态变更后变更历史条数 +1；历史无 setter、无 delete |
| INV-MEM-11 | `valid_from_commit` / `last_verified_commit` 不可解析时写入失败 |
| INV-MEM-12 | md 文件被外部删除后启动对账产生一条差异报告，索引不被静默丢弃 |
| INV-SKL-1 | 新建/导入后 `status == draft`；校验失败时 `Activate()` 返回错误 |
| INV-SKL-2 | 校验失败的 Skill 带非空 `validation_error` |
| INV-SKL-3 | 同名 Skill 的 `active` 版本数 ≤ 1 |
| INV-SKL-4 | 发布新版本后旧版本仍可读；回滚后旧版本 `active`、新版本非 `active` |
| INV-SKL-5 | `deprecated` 的 Skill 不出现在新注入清单；历史注入仍可解析 |
| INV-SKL-6 | 导入后源目录内容逐字节不变 |
| INV-SKL-7 | 注入载荷含 `SKILL.md` 正文，不含 `references/` 与 `assets/` 内容 |
| INV-SKL-8 | 执行 `scripts/` 触发一次权限裁决 |
| INV-SKL-9 | `compatibility` 不满足时 Skill 不出现在注入清单 |
| INV-SKL-10 | 项目 Skill 与全局 Skill 同名时的选择规则**尚未确定**（OPEN-15）：在裁决前，该场景必须返回明确错误，不得静默取一边 |

### 17.8 Role / Runtime

| ID | 断言 |
|---|---|
| INV-ROLE-1 | 改绑 Runtime 前后，同一组状态机输入产生相同的迁移结果 |
| INV-ROLE-2 | Role 结构体不含 `model` / `reasoning_effort` 字段（穷举字段名断言） |
| INV-ROLE-3 | Role 的 `runtime_name` 非空且唯一；同一 Runtime 可出现在多个 Role 上 |
| INV-ROLE-4 | `set_mode` 不在 `available_modes` 内时保存失败 |
| INV-ROLE-5 | 见 INV-ATT-7 |
| INV-ROLE-6 | 存在无 Role 承担的 AI 操作时，`CreateWork` 的行为**尚未确定**（OPEN-18）：在裁决前必须返回明确错误，不得静默兜底 |
| INV-RT-1 | 探针失败项对应的能力在能力矩阵中标记为不可用，且相关操作被拒绝而非静默降级 |
| INV-RT-2 | `codex` 会话在 `set_mode` 成功前，写类工具调用被拒 |
| INV-RT-3 | 重试产生的 Attempt 不增加 `OperationInvocation` 计数 |
| INV-RT-4 | `protocol_version != 1` 时返回明确错误，不进入任何兼容分支 |

---

## 18. 尚未决定的事

**这些是明确的开放项，不要在实现里替它们做主张。** 需要时提出来。
带 † 的字段与取值全部对应到这里。

| # | 开放项 | 现状 | 影响 |
|---|---|---|---|
| OPEN-1 | **人类可读 ID 是不是主键** | 设计稿全程用 `work-08` / `unit-012` / `ev-441`；`architecture.md` §4 的事件信封用 ULID `evt_01J...` | 决定 store schema 与跨对象引用格式。两套并存时必须定死谁是主键、谁是展示序号 |
| OPEN-2 | **RequirementSnapshot 的聚合归属** | 设计稿有 `requirement v2 已冻结`、`需求 → 证据` 矩阵，但没有独立页面 | 是独立聚合（`requirement_snapshot.go`）还是 Work 的值对象 |
| OPEN-3 | **`initializing` / `initializing_failed` 未收录进术语表** | `AGENTS.md` §8 只列 9 个状态词，设计稿另有 `initializing_failed` | 要么补录进 §8（推荐），要么把 worktree 创建失败表达成 `failed` + 原因。**必须先定，否则状态枚举的穷举测试写不出来** |
| OPEN-4 | **Unit 与 Subplan 的完整状态集** | 设计稿只出现 Subplan 的 `accepted` / `executing` / `blocked`，Unit 的「执行中」「契约未冻结」 | Unit/Subplan 是否需要 `failed` / `obsolete`；是否复用 Work 的状态词 |
| OPEN-5 | **重规划中被废弃的 Subplan/Unit 怎么表示** | v5 的 DAG 只显示 `subplan-01/03/04`，`subplan-02` 缺席；设计稿未说明原因 | 「编号不复用、废弃留空洞」是本文的**推断**。需要确认，并确定废弃项是否计入进度分母 |
| OPEN-6 | **打断后的落点状态** | 「立即打断当前单元 → attempt 标记 `superseded`」，但没说 Work 去 `reviewing_unit` 还是 `planning` | 影响迁移表 #11 与 §3.4 |
| OPEN-7 | **进度分母口径** | `3/7`、`62 / 91`、`一次通过率 68%`、`平均单元耗时 6m12s` 的定义均未给出 | 报表与计划面板的数字含义；`obsolete` 单元是否计入 |
| OPEN-8 | **「Work 工作记忆」是什么** | 「仅记录，不影响本次计划 → 写入 Work 工作记忆，作为后续子计划输入」 | 它与 L2/L3 Memory 的关系：是第三种存储，还是 `scope=work` 的 Memory？前者要新表，后者要扩 `scope` 枚举 |
| OPEN-9 | **`acceptance_criteria` 是不是契约字段** | 设计稿的契约 YAML 里没有它，抽屉里作为独立区块渲染 | 本文把它放进契约（验收判定的唯一依据）。若实际是独立对象，INV-CTR-4 / INV-CTR-10 要改 |
| OPEN-10 | **worktree 根目录与数据目录不同源** | 设置页 `worktree 根目录 ~/.duet/worktrees`，而数据目录是 `~/.acpflows` | 两个顶层目录是有意为之还是笔误。`git-workflow.md` §4 也写的 `~/.duet/worktrees` |
| OPEN-11 | **多 Work 并发上限** | `architecture.md` §8.4 已列为开放项 | 同时能跑几个 worktree / Runtime 进程 |
| OPEN-12 | **`confidence` 与 `sensitivity` 的取值域** | 只见到 `sensitivity: internal`；`confidence` 只有占位符 | 枚举还是数值；`confidence` 是否参与注入排序 |
| OPEN-13 | **L0 / L1 是什么** | 设计稿只出现 L2 / L3 / L4 | 层级编号是否有 L0/L1（如 Runtime 内建记忆、Work 工作记忆），还是编号从 L2 起就是历史遗留 |
| OPEN-14 | **候选记忆的 ID 前缀** | 候选是 `cand-07`，正式是 `mem-203` | 晋升 `candidate → active` 时 ID 是否改写。若改写，`source_refs` 与历史注入记录的引用会断，需要重定向规则；若不改写，界面前缀不一致要修 |
| OPEN-15 | **项目 Skill 与全局 Skill 同名时的优先级** | 设计稿两个库并列（`acp-engine 4 Skill` / `全局 ~/.acpflows · 9`），未说明冲突解决 | 注入选择策略 |
| OPEN-16 | **8 个角色的英文标识** | 只有 `memory_curator` 被证实（Memory 的 `created_by`） | 其余 7 个的 `id`。裁决后一次定全并写进 `internal/constant/role.go` |
| OPEN-17 | **三个隔离开关的默认值** | 关闭 Runtime 机器级记忆 / 禁用未授权项目 MCP Server / 允许 Runtime 内建 Skill | 默认开还是关。前两个默认「开启隔离」更符合本产品的边界主张，但设计稿没说 |
| OPEN-18 | **AI 操作必须全覆盖吗** | 「一键初始化推荐角色集（8 个角色 + 默认提示词 + 推荐绑定）」 | 用户删掉某个角色后，对应的 AI 操作没人承担时是阻止开始工作，还是回退到某个兜底角色 |
| OPEN-19 | **Runtime 是封闭枚举还是注册表** | `AGENTS.md` §8 写「一个 ACP adapter 进程（claude / codex）」；但设置页的 Runtime 检测列表里出现了第三项 `acp-sidecar · runtime 注册表与探测服务 · 未安装` | 若是注册表，Runtime 需要 id/来源/安装方式等字段，`role.runtime_name` 不能是二值枚举 |
| OPEN-20 | **处置与状态的英文标识** | 设计稿只给中文：`仍有效 / 需补充 / 需回滚 / 已废弃`、记忆的 `已失效 / 废弃` | 本文提议 `still_valid` / `needs_supplement` / `needs_rollback` / `obsolete` 与 `invalid` / `obsolete`。需确认，且注意 `obsolete` 在两处语义不同，可能要换名 |
| OPEN-21 | **`contract_revision` 期间 Work 处于哪个状态** | 设计稿没有「正在改契约」的状态 | 本文迁移表 #16 暂取 `planning`（把 `planning` 定义为「计划族活动」）。若不接受，需要新增状态词并同步 `AGENTS.md` §8 |

### 已发现的设计稿自相矛盾（需要设计侧裁决，不要在代码里各选一边）

| # | 矛盾 | 证据 |
|---|---|---|
| CONF-1 | **同一个 Work 同时被标为 `executing` 和 `waiting_user`** | 左栏与报表页：`取消运行中的 Agent turn · executing · 3/7`；同一 Work 的 D2 抽屉与右栏却写「工作已进入 `waiting_user`」，且对话里 unit-012 显示「执行中 2:14」的同时挂着「D2 决策 · 1 待处理」 |
| CONF-2 | **记忆计数对不上** | 筛选条：`全部 12 · active 9 · 候选 2 · 已失效 3` → 9+2+3 = 14 ≠ 12。「全部」的口径（是否含 candidate / 已失效 / 废弃）未定义 |
| CONF-3 | **UnitContract 的产出角色** | 计划架构师的「产出」写着 `PlanVersion · Subplan DAG · UnitContract`，但契约抽屉写「由单元设计师产出」，且 `unit_contract` 是单元设计师的 AI 操作 |
| CONF-4 | **角色卡显示模型与推理强度，而设置页明确不设该项** | 角色卡：`claude · sonnet · 中`、`codex · gpt-5-codex · 高`；设置页：「模型与推理强度不在协议里……应用只记录本次实际使用的 Runtime 版本与模式」。本文按「观测结果而非配置项」处理（§16.5），但界面呈现位置需要设计侧确认 |
| CONF-5 | **`superseded` 同时是 Attempt 状态和 Evidence 状态** | Attempt：「attempt 标记 `superseded`」；Evidence：`ev-438 · attempt 1 越界 · 已废弃 · superseded`。两者语义相近但生命周期不同，共用一个词容易在实现里混 |
| CONF-6 | **Attempt 缺少「已结束待审查」状态** | 状态集是 `running / succeeded / superseded / rejected`，但 Work 有 `reviewing_unit` 这一段；Attempt 在这段时间处于哪个状态没有定义（`running` 已经不准确，`succeeded` 还没定） |

---

## 19. 相关文档

| 你要做的事 | 读 |
|---|---|
| 模型放哪、怎么写（充血、一个聚合一个文件） | [`coding-standards.md`](coding-standards.md) §1.1 |
| 分层、依赖方向、存储边界、13 类事件 | [`architecture.md`](architecture.md) §3 §4 |
| 把本文 §17 的不变量变成测试 | [`testing-strategy.md`](testing-strategy.md) |
| `paused` / Checkpoint / 恢复的完整流程 | [`release-and-update.md`](release-and-update.md) §5 |
| 产品 worktree 与开发 worktree 的区别 | [`git-workflow.md`](git-workflow.md) §4 |
| 术语与状态词的唯一真源 | [`../AGENTS.md`](../AGENTS.md) §8 |
