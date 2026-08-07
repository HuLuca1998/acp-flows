# M2 · 能提需求，看得见 AI 在干什么

> 对应验收点 **V4 · V5 · V6**（[`../acceptance.md`](../acceptance.md)）。

## 目标

**用户把自己的项目加进来，说一句需求，AI 追问清楚后开始干活，而他全程看得见它在做什么。**

## 完成标志

用户自己做这三件事，全部成功：

1. 「创建项目」选一个本地代码文件夹 → 项目出现在左栏
2. 「新建对话」输入一句需求 → AI **追问细节**（不是直接开写），文字**边说边显示**
3. 中间区域像一条时间线：AI 说了什么、读了哪个文件、跑了什么命令，都看得见

## 已经就绪的地基

| 已完成 | 提交 | M2 用它来做什么 |
|---|---|---|
| ACP 传输层（ndjson 分帧 / 双向路由） | `29a45c1` | 和 Agent 收发消息 |
| `protocol` 线格式（13 类事件全集） | `8b3b422` | 认得 Agent 发来的每一类事件 |
| Fake Runtime 脚本回放 | `1999457` | 上层测试不必依赖真实 Agent |
| `Work` 十一态状态机 | `1da80e9` | 工作的生命周期 |

## 全局停止条件

1. 需要新增第 14 类事件类型——它是封闭枚举，新增要同时改四处
2. 需要在用户的项目目录里直接写文件（必须走隔离的工作区）
3. 发现某个 Agent 的行为与 [`../../notes/acp-field-notes.md`](../../notes/acp-field-notes.md) 的实测冲突 → 先复验并裁定
4. 撞上 [`../open-questions.md`](../open-questions.md) 未决项

---

## S2.1 · 把项目加进来

### ◐ U2.1.1 · 添加本地项目与隔离的工作区  ·  `983ba6b`（**R4 工作区隔离未做**）

| | |
|---|---|
| `goal` | 用户能选一个本地代码文件夹加进来，且 Duet 的任何产物都不污染他的项目 |
| `allowed_changes` | `backend/internal/app/project/**` · `backend/internal/gitx/**` · `frontend/src/features/project/**` |
| `forbidden_changes` | 往用户项目目录写任何 Duet 自己的文件；碰 `~/.acpflows` 之外的全局路径 |
| `stop_conditions` | 选中的目录不是 git 仓库而产品语义未定 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 添加后项目出现在列表且可持久化 | 重启 duetd 后断言项目仍在 |
| R2 | **不污染用户项目** | 断言添加前后用户目录的 `git status` 无变化 |
| R3 | 选了非 git 目录时给出可操作提示 | 断言提示含「初始化仓库」或明确拒绝理由 |
| R4 | 每个工作有独立工作区，互不干扰 | 断言两个工作的文件改动互不可见 |

> **2026-08-08：R1–R3 完成，R4 没做。**
> 「添加项目」整条链路（契约 → 领域 → gitx 探测 → 用例 → 落库 → 端点 → 界面）
> 已打通并真机验过。**worktree 隔离没做**——它依赖「工作」这个概念，
> 而 Work 的创建要等 `U2.2.x`。现在硬做会造出一个没有工作可放的空目录。
>
> 接手的人：把 R4 并进第一个真正创建 Work 的单元，别单独做。

> **「添加项目」这个动作要一个字节都不写。** 顺手初始化一下 `.acpflows/`
> 目录结构是很自然的想法，但那会让 R2 直接失败——用户刚把自己的仓库加进来，
> `git status` 就多了一堆没见过的东西，这是最快失去信任的方式。
>
> 两处存放位置不要搞混（`architecture.md` §数据落位 与 `open-questions.md` Q30）：
>
> | 放什么 | 放哪 | 什么时候创建 |
> |---|---|---|
> | worktree | `~/.acpflows/worktrees/`（**用户主目录**） | 真正开一个工作时 |
> | 记忆 / Skill / 项目配置 | `<project>/.acpflows/`（**用户项目里**） | M3，用户主动创建第一条时 |
> | 运行记录 | `<project>/.acpflows/runs/` | 同上，且要写进 `.gitignore` |

---

## S2.2 · 真的把 Agent 跑起来

### ✓ U2.2.1 · 子进程生命周期与 stderr 采集  ·  `ba73425`

| | |
|---|---|
| `goal` | 能可靠地拉起、关闭真实 Agent 进程，出错时说得清为什么 |
| `allowed_changes` | `backend/internal/acp/runtime/process.go` |
| `forbidden_changes` | 不实现 Runtime 发现与版本管理（那是 M1 的 S1.3） |
| `stop_conditions` | 发现需要按进程组杀孙进程而当前抽象做不到 |

> **2026-08-08：platform/proc.go 没有建，已从 allowed_changes 去掉。**
> 原计划把「起子进程」这件事拆成通用层（platform）与 ACP 层（runtime），
> 但实际做下来**没有第二个调用方**：进程组、SIGTERM 升级、stderr 采集、
> 清嵌套环境变量，每一条都是为 ACP Runtime 的具体形态服务的。
> 拆出去只会得到一个只有一个使用者的抽象，而它的接口形状完全由那个使用者决定。
>
> `stop_conditions` 里那条「按进程组杀孙进程」**确实发生了**——但当前抽象做得到，
> 所以没有停下来找人。

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | **spawn 前清除嵌套会话环境变量** | 断言子进程环境里没有 `CLAUDECODE` / `CLAUDE_CODE_ENTRYPOINT` |
| R2 | stderr 被完整采集，报错时带出来 | 让假子进程往 stderr 写一行，断言错误信息里包含它 |
| R3 | 关闭先 `SIGTERM` 超时再 `SIGKILL` | 用忽略 SIGTERM 的假进程，断言最终被 KILL |
| R4 | 崩溃残留的进程下次启动能清理 | 断言僵尸被回收 |

> R1 对本项目**必踩**：Duet 自己就在 Claude Code 里开发，不清这些变量
> `claude-agent-acp` 会误判嵌套而拒绝服务。

### ◐ U2.2.2 · 一轮完整会话：问清楚需求  ·  `55919b7`（**R3 未做**）

| | |
|---|---|
| `goal` | 用户说一句需求，AI 追问、他补充、直到需求清楚——**文字边说边显示** |
| `allowed_changes` | `backend/internal/acp/session/**` |
| `forbidden_changes` | 不处理取消（M3）；不做 adapter 差异（U2.2.3） |
| `stop_conditions` | 发现 `session/prompt` 的阻塞语义与规格不符 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | **真流式**：第一个字远早于整轮结束 | Fake 首块后延迟 2s 才结束，断言首块在 200ms 内到达 |
| R2 | 结束原因五种取值各有处理，只有 `end_turn` 算正常 | 五个用例，断言 `max_tokens` 不被当成功 |
| R3 | 系统提示词只在首轮发 | 断言第二轮的请求里不含它 |
| R4 | 工作目录必须是已存在的绝对路径 | 传相对路径断言前置校验失败且**未发出**建会话请求 |
| R5 | 13 类事件每类都有去处（可以是显式丢弃） | 穷举测试：新增一类而未处理时必须红 |

> **2026-08-08：R1 R2 R4 R5 完成，R3 没做。**
> 「系统提示词只在首轮发」要等有「多轮」这个概念——本单元只跑通了单轮，
> 现在做的话，判断「是不是首轮」没有可依据的状态。接手的人：
> 把 R3 并进第一个真正做多轮对话的单元。
>
> **越界改动一处**：`acp/jsonrpc` 的通知派发从 `go serveNotification`
> 改成同步按序。session 层修不了它——顺序是在 jsonrpc 的读循环里丢的，
> 而 `agent_message_chunk` 的顺序就是用户看到的字序。
> 实测 200 条通知里第一条到手的是 seq 3。

### ◐ U2.2.3 · 两个 Agent 的差异内化  ·  `68b32af`（**R4 待真 Agent 验证**）

| | |
|---|---|
| `goal` | 上层只表达意图，不认识「这是 Claude 还是 Codex」 |
| `allowed_changes` | `backend/internal/acp/adapter/**` |
| `forbidden_changes` | 上层任何文件出现 runtime 名字 |
| `stop_conditions` | 某个差异无法在 adapter 内填平（应升级为能力查询） |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 配置项**按 `category` 取，不按 `id` 取** | 两端的推理强度都能用 `thought_level` 取到 |
| R2 | **上层零品牌判断** | `grep -rn 'codex\|claude' internal/{app,domain,api}` 为空，接进 CI |
| R3 | 共同实现只有一份 | 断言两个 adapter 各自 < 150 行 |
| R4 | 同一批断言跑遍两个真实 Agent 与 Fake | 测试代码里零 `if impl ==` |
| R5 | 探针读出的能力矩阵 == Fake 声明的能力 | 让 Fake 声明不支持会话模式，断言矩阵对应项为不通过 |

> **2026-08-08：R1 R2 R3 R5 完成，R4 只做了一半。**
> 「同一批断言跑遍两端」已经做到——样例数据是 `acp-field-notes` §7.1 里
> 实测记下的两端真实形态，测试代码里零 `if impl ==`。
> **但还没跑过真 Agent**：那需要一个能同时驱动 claude-agent-acp 与 codex-acp
> 的集成测试，而它依赖 Work 的创建（要有 cwd、要有会话）。
>
> 接手的人：做 Work 创建之后补一个 `//go:build integration` 的集成测试，
> 拿同一批断言跑真 Agent。`make probe` 已经能拉起两端，可以照它的做法。

## S2.4 · 把一次需求变成一个「工作」

**用户拿到什么**：提一个需求，就有一个看得见、停得掉、恢复得了的「工作」。
这是 V5 与 V6 真正连起来的地方。

### ○ U2.4.1 · 新建工作：worktree、会话、时间线接线

> **2026-08-08 新登记。** `M2` 前面四个单元各自留了一笔欠账，
> **全都卡在同一个前提上**：没有「一个工作」这个概念。
> 分开补的话，每一笔都要重新理解同一套上下文，所以并成一个单元。

| | |
|---|---|
| `goal` | 用户对一个项目提需求，生成一个工作：有独立 worktree、有 ACP 会话、时间线实时显示 |
| `allowed_changes` | `backend/internal/domain/model/work.go` · `backend/internal/app/work/**` · `backend/internal/gitx/**` 的 worktree · `backend/internal/store/**` 的工作仓储 · `backend/internal/api/**` 的 `/works` · `frontend/src/features/chat/**` · `api/openapi.yaml` 的 `/works` |
| `forbidden_changes` | 往用户项目目录写任何 Duet 自己的文件；worktree 建在项目目录里（必须在 `~/.acpflows/worktrees`，见 `open-questions.md` Q30） |
| `stop_conditions` | 发现 worktree 与用户已有分支冲突而产品语义未定 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 每个工作有独立 worktree，互不干扰 | 断言两个工作的文件改动互不可见（补 `U2.1.1` 的 R4） |
| R2 | worktree 在 `~/.acpflows/worktrees`，**不在用户项目里** | 断言创建前后用户项目的 `git status` 无变化 |
| R3 | 系统提示词只在首轮发 | 断言第二轮的请求里不含它（补 `U2.2.2` 的 R3） |
| R4 | 时间线实时显示这个工作的事件 | 断言 SSE 推来的事件出现在界面上（补 `U2.3.2` 的接线） |
| R5 | 同一批断言跑遍两个真实 Agent | `//go:build integration`，测试代码里零 `if impl ==`（补 `U2.2.3` 的 R4） |
| R6 | 关掉应用再打开，工作还在且能接着看 | 重启后断言工作与它的事件都还在 |

> **R2 是这个单元最要紧的一条。** 用户把自己的代码目录交给 Duet，
> 而 worktree 是 Duet 唯一会大量写文件的地方——写错位置的话，
> 他的仓库里会突然多出一堆分支和目录。
>
> `make probe` 已经能拉起两端真 Agent，R5 可以照它的做法。

---

## S2.3 · 让用户看得见

### ✓ U2.3.1 · 事件实时推到界面  ·  `d3e1957`

| | |
|---|---|
| `goal` | AI 每做一步，界面立刻显示，不是等它全做完才刷出来 |
| `allowed_changes` | `backend/internal/eventbus/**` · `backend/internal/store/**` 的事件仓储与迁移 · `backend/internal/api/**` 的 SSE 端点 |
| `forbidden_changes` | 为性能异步落库（会导致「前端收到了，重启后库里没有」） |
| `stop_conditions` | — |

> **2026-08-08 修正 `allowed_changes`。** 原来只写了 sse 与 eventbus，
> 但 R1 要求「序号**跨重启**连续」、R5 要求「**先落库**再扇出」——
> 两条都必须落库，而 `events` 表还不存在。照原范围开工，第一步建迁移就越界了。
>
> 这是本仓库第二次出现同类疏漏（`U2.1.1` 也漏了 store 与契约）。
> **写 allowed_changes 时对着验收标准逐条问一句「实现它要碰哪些目录」。**

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 事件序号单调递增、无洞、跨重启连续 | 重启后断言序号不回退不重复 |
| R2 | 断线重连只补发没收到的 | 断言从 N 恢复时首条事件序号 > N |
| R3 | 客户端断开时订阅者被回收 | 断言订阅者数归零（**防泄漏**） |
| R4 | 慢消费者不阻塞其他人 | 一个订阅者不读，断言另一个仍正常收 |
| R5 | **先落库再扇出** | 断言落库失败时不扇出 |

### ◐ U2.3.2 · 时间线渲染与过滤  ·  `f38f316`（**未接进对话页**）

| | |
|---|---|
| `goal` | 用户能看懂 AI 在做什么，并能只看自己关心的那几类 |
| `allowed_changes` | `frontend/src/features/timeline/**` · `frontend/src/i18n/locales/*.json` |
| `forbidden_changes` | 用 `switch` 分发事件类型（用注册表）；硬编码文案 |
| `stop_conditions` | 设计稿里找不到某类事件的展示形态 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 13 类事件各有渲染器，**加一类不改既有代码** | 加一个测试用类型，断言只需注册一行 |
| R2 | 过滤器按设计稿分组（ACP 事件 / 应用事件） | 断言分组标题与设计稿逐字一致 |
| R3 | 未知事件类型不白屏 | 喂一个没见过的类型，断言页面仍可用 |
| R4 | 文字流式追加不闪烁 | 断言同一条消息的多个片段合并进同一个气泡 |

> **2026-08-08：四条验收标准全部满足，但组件还没接进对话页。**
> `Timeline` 与 `TimelineFilter` 已可用且测试齐备，缺的是「谁把 SSE 的事件
> 喂给它」——那需要对话页有一个「当前工作」的概念，而 Work 的创建还没做。
>
> 接手的人：做 Work 创建时把这两个组件接上，顺带把 `U2.1.1` 的 R4
> （worktree 隔离）一起做掉——它们依赖的是同一个前提。
