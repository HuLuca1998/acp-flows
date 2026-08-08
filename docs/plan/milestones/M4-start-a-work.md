# M4 · 开一个工作

> ★ **2026-08-08 里程碑按引用关系重排**（见 `design/DEPENDENCIES.md`）。
> 旧的 `M4`（值得信任）连同它的 `U4.*` 编号已归档到
> [`archive/M4-trust.md`](archive/M4-trust.md)——里面做完的会话恢复、检查点、
> 计划模型没有白做，落位见 [`roadmap.md`](../roadmap.md) 的对照表。
>
> 本文件的 `U4.*` 是**重排后的新编号**。

## 目标

**用户在一个项目下开一个「工作」，Duet 给它拉一条独立的 worktree，
从此 AI 干的活都在那条分支上，用户的主工作区一个字节都不动。**

★ 「工作」是 Duet 的核心单位：一条需求 → 一份计划 → 若干单元 → 一次验收，
全都挂在它下面。没有它，对话就只是聊天。

## 完成标志

用户自己做这七件事，全部成功：

1. 点项目下的「＋ 新建对话」→ **弹出「新建工作」对话框**
2. 看到「已检测到 `.git` · 无未提交改动」，或**如实报告有几个文件改了**
3. 能选基线分支：`main` / `develop` / 「指定 commit…」
4. 看到将创建的分支名、worktree 路径、基线 commit 短号
5. 点「创建 worktree 并开始」→ 工作出现在项目下，**磁盘上真的多了 worktree**
6. 右栏出现「工作区」：分支名、领先几个 commit、未提交改动、本次工作的 commit 列表
7. 创建失败时工作进入 `initializing_failed`，**绝不退回原目录执行**

★ 第 7 条是硬的：worktree 建不出来时**宁可不干活**，也不能在用户的主工作区里跑。
那是三层防线里唯一还立着的一层。

## 已经就绪的地基

| 已完成 | 提交 | M4 用它来做什么 |
|---|---|---|
| worktree 切换（后端） | `U2.4.1` 旧编号 | 弹层确认后调它 |
| `Work` 十一态状态机 | `1da80e9` | `initializing` / `initializing_failed` 就在里面 |
| 左栏项目树的「新建对话」 | `120f019` 之后 | 入口已经在，差弹层 |

## 全局停止条件

1. worktree 创建失败而代码里出现「回落到项目根目录」的分支 —— 立刻停
2. 需要在用户的主工作区执行任何写操作（含 `git stash`）—— 停下来问
3. 基线分支的语义与 `docs/spec/` 里的 `Work` 定义冲突

---

## S4.1 · 工作初始化（后端）

### ○ U4.1.1 · git 状态探测与基线选择

| | |
|---|---|
| `goal` | 开工之前，用户看得到自己仓库现在是什么状态 |
| `allowed_changes` | `backend/internal/platform/git/**` · `backend/internal/app/work/**` |
| `forbidden_changes` | 任何写操作（`stash` / `checkout` / `commit`）；`domain` 里出现 git 调用 |
| `stop_conditions` | 仓库处于 rebase / merge 中途 —— 如实报告并拒绝开工 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 未提交改动**分得清已跟踪与未跟踪** | 造两种脏，断言分别报出（合成一条会漏报新文件） |
| R2 | 列出本地分支并标出当前分支 | 断言 HEAD 所在分支被标记 |
| R3 | **一个字节都不写** | 探测前后断言 `git status --porcelain` 输出完全一致 |
| R4 | rebase / merge 中途**拒绝开工** | 造中途态，断言返回明确错误 |
| R5 | 空仓库（无 commit）如实报告 | 断言不 panic，返回可读错误 |
| R6 | 覆盖率 ≥ 85% | `make cover` |

### ○ U4.1.2 · worktree 创建与失败处置

| | |
|---|---|
| `goal` | 每个工作有自己的 worktree，建不出来就不开工 |
| `allowed_changes` | `backend/internal/app/work/**` · `backend/internal/platform/git/**` |
| `forbidden_changes` | ★ 出现「worktree 失败则用项目根目录」的回落分支 |
| `stop_conditions` | 磁盘空间或权限导致无法创建 —— 报错，不降级 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | worktree 路径在 `~/.acpflows/worktrees/<workID>` | 断言路径前缀，**且不在用户项目内** |
| R2 | 分支名可预测且不撞车 | 造同名分支，断言换名或明确报错，**不覆盖** |
| R3 | ★ 失败 → `initializing_failed`，**不回落** | 注入创建失败，断言状态正确**且执行 cwd 从未被设为项目根** |
| R4 | 删除工作时 worktree 一并清理 | 断言目录消失且 `git worktree list` 里不留幽灵条目 |
| R5 | 基线 commit 被记下来 | 断言工作记录里存了创建时的 commit SHA |

---

## S4.2 · 新建工作弹层与右栏（前端）

### ○ U4.2.1 · 新建工作对话框

| | |
|---|---|
| `goal` | 「＋ 新建对话」按钮活了，开工前把仓库状态和将要做的事说清楚 |
| `allowed_changes` | `frontend/src/features/work/**` · `frontend/src/features/rail/**` · i18n |
| `forbidden_changes` | 编造 git 状态；偏离 `INVENTORY.md` §六的结构 |
| `stop_conditions` | 设计稿里找不到某个字段的展示形态 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 弹层四块齐全（仓库状态 / 基线分支 / 将创建 / 提示） | 断言四个区块对上 `INVENTORY.md` §六 |
| R2 | 有未提交改动时**显示出来**而不是静默放行 | mock 脏状态，断言界面报出文件数 |
| R3 | 「指定 commit…」能输入并校验 | 断言非法 SHA 被拒并给出原因 |
| R4 | 创建中禁用确认按钮 | 断言连点两次只发一次请求 |
| R5 | 失败时显示 `initializing_failed` 与原因 | 断言错误文案含后端返回的原因 |
| R6 | 能切英文 | 断言无硬编码中文 |

### ○ U4.2.2 · 右栏工作区面板

| | |
|---|---|
| `goal` | 用户随时看得到这个工作动了什么、在哪条分支上 |
| `allowed_changes` | `frontend/src/features/inspector/**` · `backend` 的工作区查询端点 |
| `forbidden_changes` | 前端直接跑 git 命令；轮询频率高于 `docs/frontend-guide.md` 的约定 |
| `stop_conditions` | 需要展示的字段后端查不到 —— 先补端点，别在前端算 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 显示分支、领先 commit 数、未提交改动数 | 断言三项都来自后端返回，不在前端推算 |
| R2 | 列出**本次工作产生的** commit | 断言基线之前的 commit 不出现 |
| R3 | 工作不存在时是空态，不是报错弹窗 | 断言显示空态文案 |
| R4 | 数据陈旧时说明「更新于」 | 断言时间戳存在 |
| R5 | 能切英文 | 断言无硬编码中文 |
