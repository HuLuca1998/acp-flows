# M3 · 能管住 AI

> 对应验收点 **V7 · V8**（[`../acceptance.md`](../acceptance.md)）。

## 目标

**AI 要动用户文件之前先问他；用户随时能喊停，停下来之后已经做的部分还在。**

## 完成标志

用户自己做这两件事，全部成功：

1. 让 AI 改一个它不该改的文件 → **弹出卡片等他点**「允许一次 / 拒绝」→ 他不点就一直等着
2. AI 正在跑时点「取消」→ 它停了，**并且改了哪些文件、跑了什么命令都还看得见**；连点两次不出乱子

## 已经就绪的地基

| 已完成 | 提交 | M3 用它来做什么 |
|---|---|---|
| Fake Runtime 脚本回放与请求记录 | `1999457` | 断言「只发了一次协议取消」 |
| `protocol` 的权限应答类型 | `8b3b422` | `cancelled` / `selected` 两种应答可判别 |

## 全局停止条件

1. 需要给权限请求加超时——**协议层不设超时**，挂几小时等用户回来是产品语义
2. 需要改公开接口签名而当前单元未授权
3. 发现取消后 Agent 仍在改文件（说明杀进程那步漏了）
4. 撞上 [`../open-questions.md`](../open-questions.md) 未决项

---

## S3.1 · 动我的文件要先问我

### ✓ U3.1.1 · Fake 能主动发权限请求

| | |
|---|---|
| `goal` | 让权限相关的所有上层测试脱离真实 Agent |
| `allowed_changes` | `backend/internal/acp/fake/**` |
| `forbidden_changes` | `fake` import `protocol` 以外的任何包；Fake 自己去重或纠正收到的消息 |
| `stop_conditions` | — |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 主动发权限请求并阻塞等应答 | 断言未应答前这一轮不结束 |
| R2 | 应答 `cancelled` 时正确解除阻塞 | 断言这一轮以 `cancelled` 收尾 |
| R3 | 声明的能力**表现为真实协议行为** | 声明不支持会话模式 → 建会话响应里**真的没有**那个字段 |
| R4 | `optionId` 原样回传，不按类别匹配 | Fake 发一组 id 与类别语义不一致的选项，断言收到的是原样 id |
| R5 | 权限请求**没有超时** | 断言等待 2s 后这一轮仍未结束 |

> **2026-08-08 已完成。** 脚本里加一种步骤 `Ask`：发一条 `session/request_permission`
> 并**阻塞等应答**。Fake 侧新增 `permission.go`（pending 表 + 反向请求 id 序列），
> `frameWriter` 加 `request`，`dispatch` 先认出「没有 method 但有 id」的入站响应——
> 不先认的话它会掉进 default 被回一个 -32601，而等应答的那一轮永远挂着。
>
> **R5 的「没有超时」是刻意的**，不是漏写：真 Agent 会一直等用户（他可能去泡咖啡了）。
> Fake 自作主张地超时，上层「等用户裁决」的逻辑就测不出来。唯一的解除途径是
> ctx 结束或 `Close`（`abortAll`），那时安静收场而不是当成「用户同意了」。
>
> **造了四个负例**：不阻塞 → R1/R5 红；cancelled 照脚本收尾 → R2 红；
> Fake 按 kind 重排 optionId → R4 红；只处理第一次请求 → 两次请求那条红。

### ✓ U3.1.2 · 三种裁决策略与阻塞语义

| | |
|---|---|
| `goal` | 用户能按角色配置「每次都问 / 自动允许只读 / 一律拒绝」，且等他决定时应用其余部分照常可用 |
| `allowed_changes` | `backend/internal/acp/session/permission.go` · `backend/internal/app/**` 的裁决入口 |
| `forbidden_changes` | 在选项集合不认识时替用户拍板 |
| `stop_conditions` | 某个 Agent 给出的选项无法匹配任何已知策略 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 自动允许只读**只对读类工具生效** | 写类工具断言仍走询问 |
| R2 | 选项集合不认识时走保守分支 | 断言交给用户决定，**绝不猜一个 id** |
| R3 | 阻塞的只是这一轮 | 断言等待期间其他工作的事件流照常 |
| R4 | 裁决理由被记录 | 断言事件载荷里含机器可读的理由码 |

> **2026-08-08 已完成。** `session/permission.go`：`Decide` 是**纯函数**
> （策略 + 工具类别 + 选项集合 → 选哪个 / 交给用户），没有 IO 也没有等待，
> 所以能穷举所有组合；等待在 `handlePermission` 里。
>
> **只读是白名单**（read / search / think），不是黑名单。反过来列危险的话，
> Agent 新增一类工具会默认落进「自动允许」——用户以为自己开的是
> 「让它随便看」，实际开的是「让它随便改」，而他不会发现，直到文件被改了。
> `R1` 那条测试穷举 `protocol.AllToolKinds()`，新增一类时会逼人做一次决定。
>
> **优先 `once` 而非 `always`**（超出验收标准，但同源）：替用户做的决定要
> 尽量小。选 always 的话，一次自动裁决会永久改变后续所有请求的处理方式，
> 而用户根本不知道发生过这件事。
>
> **造了六个负例**：白名单改黑名单 → R1 红；猜第一个选项 → R2 红；
> 优先 always → 红；认不出的策略默认放行 → 红；理由码塞人话 → R4 红；
> 给裁决加全局锁 → R3 红。
>
> ★ **R3 那条测试第一版是无效的**：让 B 会话跑一个没有权限请求的脚本，
> 于是加一把全局锁它照样绿。改成 B 也发权限请求（策略能自动裁决）才测到东西。
> 教训写进了 `session/AGENTS.md`。

### ✓ U3.1.3 · 权限卡片界面

| | |
|---|---|
| `goal` | 用户看得懂 AI 想干什么，并能一键允许或拒绝 |
| `allowed_changes` | `frontend/src/features/permission/**` · `frontend/src/i18n/locales/*.json` |
| `forbidden_changes` | 硬编码文案；用弹窗打断非阻塞信息 |
| `stop_conditions` | 设计稿里找不到对应条目 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 文案与设计稿一致 | 断言出现「请求写入」「写入边界外」「允许一次」「拒绝」 |
| R2 | 点了之后卡片消失且这一轮继续 | 断言应答发出后卡片不再渲染 |
| R3 | 未应答时卡片**不会自己消失** | 断言 5s 后卡片仍在 |
| R4 | 拒绝真的生效 | 断言拒绝后没有发出允许类应答 |

> **2026-08-08 已完成。** `PermissionCard`（一张卡片）+ `PermissionDock`
> （管「哪些还没应答」）。四条标准全部落地，造了五个负例。
>
> **不是弹窗**：卡片长在时间线里，用户能看到「它为什么要动这个文件」的上下文。
>
> **按钮照 Agent 给的 `options` 渲染**，不自己造一套。Agent 可能给三个
> （「这个目录以后都允许」），自己造两个的话那个就消失了。
>
> **R3 的「不自动消失」是刻意的**：没有超时、没有自动关闭。用户去倒杯水
> 回来，卡片没了、AI 也停着——他不知道刚才发生过什么，更不知道该做什么。

---

### ✓ U3.1.4 · 权限请求接线：从 Agent 到界面再回去

> **2026-08-08 补登记。** `U3.1.1`–`U3.1.3` 做完之后发现 `S3.1` 缺了一环：
> Fake 会问了、策略会裁了、卡片画好了，**但没有人把请求送到界面、
> 也没有人把用户的选择送回 Agent**。少了它，前三个单元都停在半空。
>
> 这一环跨契约、后端、前端三处，所以单独成一个单元而不是塞进 `U3.1.3`
> （那个单元的 `allowed_changes` 只有前端）。

| | |
|---|---|
| `goal` | 用户在界面上点的那一下，真的到达 Agent |
| `allowed_changes` | `api/openapi.yaml` · `backend/internal/api/**` · `backend/internal/app/**` · `backend/internal/acp/agent/**` · `frontend/src/features/**` |
| `forbidden_changes` | 前端自己造 `optionId`；后端替用户超时拍板；把待裁决状态只放在内存里而不发事件 |
| `stop_conditions` | 契约改动影响到已发布的端点语义 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 权限请求作为事件到达界面 | 断言 SSE 收到 `type=request_permission` 且载荷含 `options` 与 `toolCallId` |
| R2 | 用户的选择原样回到 Agent | 断言 Fake 收到的 `optionId` 与界面上点的那个逐字相同 |
| R3 | 应答后这一轮继续 | 断言应答之后收到 `turn_end` |
| R4 | 重复应答同一条请求被拒 | 断言第二次返回错误码，且**不再向 Agent 发第二条应答** |
| R5 | 工作被取消时 pending 的请求全部收到 `cancelled` | 断言 Fake 侧每条 pending 都被应答，没有一条挂着 |

> **2026-08-08 已完成，真机端到端跑通两遍**（真 `claude-agent-acp` 改 README.md）：
> 点「Reject」→ AI 回「你拒绝了这次编辑，所以 README.md 没有改动」，文件一字未动；
> 点「Allow」→ worktree 里真的多了一行，而用户仓库仍然干净、没有孤儿进程。
>
> `app/permission.Broker` 是中转站：会话线程调 `Ask` 挂住，HTTP 线程调
> `Answer` 叫醒。装配在 `cmd/duetd`——那是唯一能同时看见 acp 层与 app 层的地方。
>
> **走查抓到三个单测测不出的问题**：
>
> | 撞到的 | 根因 | 修在 |
> |---|---|---|
> | 启动失败时 panic，真正的原因（端口被占）被完全盖住 | `LogSink` 关掉后再写 = send on closed channel | `store/log_sink.go` 有界丢弃 |
> | 卡片只写「AI 请求写入」，不说改哪个文件 | ACP 的 `toolCall.locations` / `rawInput` 没取 | `session.pathOf` |
> | 路径是 worktree 绝对路径，撑成两行还暴露内部目录 | 没转成项目内相对路径 | `agent.shortenPath` |
>
> 第一个的代价最典型：一个普通的「address already in use」变成了一堆
> goroutine 栈，排查方向完全被带偏。

---

---

## S3.2 · 我随时能喊停

### ◐ U3.2.1 · 两段式取消与幂等（R6 触发 stop_condition，另立单元）

| | |
|---|---|
| `goal` | 用户点取消后 AI 真的停了，现场证据保留，连点两次不出乱子 |
| `allowed_changes` | `backend/internal/acp/session/cancel.go` 及其测试 |
| `forbidden_changes` | 改任何公开接口签名 |
| `stop_conditions` | 发现必须扩大写入范围；发现公开接口必须变化 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | **连续取消两次只发送一次协议取消** | Fake 记录请求数，断言 `== 1` |
| R2 | 取消后改动与事件游标**可读取** | 断言取消后能取到有效游标与 diff |
| R3 | 取消时用 `cancelled` 应答**所有** pending 的权限请求 | Fake 发 2 个权限请求后取消，断言两个都收到 `cancelled` |
| R4 | Agent 不回应时超时并可诊断 | 断言超时错误含耗时与会话标识 |
| R5 | 超时后**同时**发取消并杀进程 | 断言子进程已退出 |
| R6 | 审查中的工作拒绝取消 | 断言返回明确的拒绝错误 |

> **R3 是规范硬要求且设计稿完全没提。** 漏了会导致每次取消都超时、
> 一键更新的 `prepare` 永远返回 `blocked`——**它直接连着 M1 的 V2**。
>
> **R5 是「界面说已取消、后台还在烧钱改文件」的唯一防线。**

> **2026-08-08：R1–R5 已完成，R6 触发了本单元的 `stop_conditions`。**
>
> `session.CancelWithin` 两段式：先用 `cancelled` 应答所有 pending 的权限请求
> （R3，ACP 硬要求且设计稿完全没提），再发 `session/cancel` **通知**
> （不是请求，发出去就完事），然后等这一轮真的以 `stopReason: cancelled` 收尾。
>
> 幂等靠 `Session.cancelling`。「在跑」的判据是独立的 `inTurn` 标志，
> **不能拿 `onEvent != nil`**——调用方完全可以不关心事件而传 nil，
> 那时用户点了停会毫无反应而 AI 照跑。
>
> 超时返回 `ErrCancelTimeout`，配一个显眼的 `MustKill(err)`——
> 这件事太容易漏，而漏了的后果是「界面说已取消、后台还在烧钱改文件」。
>
> **顺带改了 Fake**（超出 `allowed_changes`，但没有它 R1/R2 根本测不出来）：
> 收到 `session/cancel` 后像真 Agent 那样以 `cancelled` 收尾，
> 而 `NeverStops` 预设升级成「**连取消也不理**」——那才是「Agent 卡死了」
> 的样子，也是 R4/R5 唯一能测的场景。★ Fake **绝不去重**取消通知：
> 去重是被测代码的职责，Fake 替它做了的话 R1 会永远绿。
>
> ---
>
> **R6（审查中的工作拒绝取消）触发 `stop_conditions` 的第一条：必须扩大写入范围。**
>
> 「工作处于什么状态」是 domain / app 的知识，`acp/session` 不知道也不该知道——
> 让调用方传一个「能不能取消」的判断函数进来，等于把业务规则漏进基础设施层。
>
> 已另立 `U3.2.3` 承接它。

### ○ U3.2.2 · 取消按钮与现场保留的界面

| | |
|---|---|
| `goal` | 用户点完取消，立刻看到「停了」，并能翻看已经做了什么 |
| `allowed_changes` | `frontend/src/features/timeline/**` · `frontend/src/features/work/**` |
| `forbidden_changes` | 取消后清空时间线；用「加载中」掩盖取消未完成 |
| `stop_conditions` | 设计稿里找不到取消态的展示形态 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 取消后时间线**保留全部历史** | 断言取消前的事件条数不减少 |
| R2 | 取消进行中有明确反馈 | 断言按钮变为进行中态而不是无响应 |
| R3 | 取消完成后状态词变为终态 | 断言状态显示为等宽英文原值 |
| R4 | 取消失败时如实告知 | 断言超时后出现错误提示而不是假装成功 |

---

### ✓ U3.2.3 · 取消的业务规则：什么时候不许停

> **2026-08-08 补登记。** `U3.2.1` 的 R6 触发了那个单元的 `stop_conditions`
> （必须扩大写入范围）：「工作处于什么状态」是 domain / app 的知识，
> `acp/session` 不知道也不该知道。硬塞进去等于把业务规则漏进基础设施层。

| | |
|---|---|
| `goal` | 取消在该拒绝的时候明确拒绝，用户看得懂为什么 |
| `allowed_changes` | `backend/internal/domain/model/**` · `backend/internal/app/work/**` · `backend/internal/api/**` · `frontend/src/features/**` |
| `forbidden_changes` | 把工作状态的判断塞进 `acp/session`；拒绝时不给理由码 |
| `stop_conditions` | 发现某个状态下「该不该允许取消」没有明确答案 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 审查中的工作拒绝取消 | 断言返回 `work_cancel_not_allowed`，且**没有**向 Agent 发协议取消 |
| R2 | 拒绝的理由是机器可读的码 | 断言错误码在封闭枚举里，界面按它查词条 |
| R3 | 可取消的状态穷举有据 | 断言每个 `WorkState` 都明确表态「能不能取消」，新增状态时测试会红 |
| R4 | 取消成功后工作进入 `paused` 并落检查点 | 断言状态与检查点事件都在 |
| R5 | 取消超时后**杀进程并把工作推到 failed** | 断言子进程已退出、状态是 `failed`、事件里有原因码 |

> **R5 接的是 `U3.2.1` 的 `MustKill`**：那个函数给出了「必须杀」的信号，
> 但真正去杀的是 app 层——它才持有进程句柄。

> **2026-08-08 已完成。** 规则在 `domain/model.CanCancel`（纯判断，穷举全部
> 11 个状态），用例在 `app/work.Cancel`，端点是 `POST /v1/works/{id}/cancel`。
>
> **顺序是「先问规则再动手」**：不行就直接拒、**一次协议取消都不发**。
> 反过来的话，审查中的工作已经被掐掉了才发现不该掐。
>
> **跨层不传哨兵错误**：`port.AgentCanceller.CancelTurn` 返回
> `(mustKill bool, err error)`。app 层不许 import acp（depguard 挡着），
> 拿不到 `session.ErrCancelTimeout`——而「必须杀进程」这件事太重要，
> 不能靠调用方去 `errors.Is` 一个它看不见的类型。
>
> **`ProcessRunner` 顺带获得了取消能力**（超出 `allowed_changes`，
> 但那份「哪个工作对应哪个进程」的映射只有它有）：
> ★ **先 track 进程再握手**——反过来的话，一个连 `initialize` 都不回的 Agent
> 会让 `session.Open` 挂住，而那时 `KillAgent` 找不到它，用户点了停界面转圈
> 而进程一直跑着。造那条真进程测试时撞出来的。
>
> **造了九个负例**，全部会红。
