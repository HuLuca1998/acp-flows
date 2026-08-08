# 测试索引 · frontend

> **写任何新测试前，先在本表里按「行为」搜一遍。**
> 规则见 [`../../docs/rules/testing-strategy.md`](../../docs/rules/testing-strategy.md) §8。
> `make check-test-index` 会逐项比对，不一致即红。

## 登记规则

- 每个 **`*.test.ts` / `*.test.tsx` 文件**一行（不是每个 `it()`）
- 「覆盖的行为」写用户可观察的行为，不写实现细节
- 断言 CSS 类名或 DOM 结构的测试**不许存在**，见 testing-strategy.md §6

## 本目录还放什么

`frontend/tests/` 除索引外，放**跨 feature 共享的测试基建**：

```
frontend/tests/
├── INDEX.md
├── msw/          由 api/openapi.yaml 生成的 mock handlers
├── fixtures/     录制的事件序列、示例契约/证据数据
└── setup.ts      Vitest 全局 setup
```

单个组件的测试与组件同目录（`EventStream.test.tsx` 挨着 `EventStream.tsx`），
**不集中放这里**。

## 索引

| 测试文件 | 位置 | 覆盖的行为 |
|---|---|---|
| `Button.test.tsx` | `src/ui/` | 设计规范 §05/§08 的按钮**行为与可访问性契约**：可访问名称、点击回调、disabled 不触发、**纯图标按钮必须同时有 title 与 data-tt**、带快捷键时 tooltip 拼上快捷键、带文字的按钮不加 tooltip、type 默认 button 不误提交表单。<br>纯视觉规则（主按钮永不实心等）由 stylelint + 人工走查保证，不在这里测 |
| `schema.contract.test.ts` | `src/api/` | ★ 生成物与 `api/openapi.yaml` 的**编译期**契约：六个端点齐备、枚举没退化成 `string`、`required` 没被丢、`Runtime.name` 不是封闭枚举（注册表可扩展）。<br>断言靠 `@ts-expect-error` —— 类型一放宽，tsc 就报「未使用的 directive」而**编译不过**。`vitest` 跑绿不代表通过，真正的门是 `make lint-frontend` |
| `App.test.tsx` | `src/app/` | ★ 应用骨架的**信息架构契约**：左栏导航**恰好 5 项且不含对话与计划**（对话是主区本身、计划是悬浮面板——这条曾经做错过）；五个导航页都打得开不白屏；右栏只在对话出现；当前页高亮唯一；窗口栏收纳三个折叠开关；计划面板由窗口栏唤出；**骨架占位里不含任何数字**（编造数据比空白更糟）；非法页面标识规整回对话 |
| `UpdateSection.test.tsx` | `src/features/settings/` | ★ 更新区的**失败路径**：检查失败时明确报错且**绝不出现「已是最新版本」**（网络断了却说已是最新，用户永远不知道自己在用旧版）；已是最新时不显示更新按钮；Web 形态给「前往下载」而不是点不动的「一键更新」；当前版本在任何状态下都显示 |
| `SettingsNav.test.tsx` | `src/features/settings/` | ★ 设置页的**结构契约**：左栏恰好六项、名字与副标题逐字照设计稿且顺序一致；**副标题里没有编造的数字**（设计稿的「3 个项目」是示意数据）；同一时刻只渲染一个分区；**来回切分区不重新发起检测请求**（探测要拉子进程，重复探测会让设置页点一下卡一下）|
| `event-registry.test.ts` | `src/features/timeline/` | ★ 事件注册表的**结构契约**：契约里 13 类事件每类都有渲染器（断言「**不是兜底**」而不是「defined」——只看 defined 的话漏注册时兜底会接住它，照样绿）；未知类型有兜底不返回 undefined；过滤器分组与设计稿一致且每类都被某项管到 |
| `Timeline.test.tsx` | `src/features/timeline/` | ★ 渲染行为：**文本流合并进同一气泡**（每片一个气泡的话打字过程中界面会疯狂重排）、**不同的工具调用各占一张卡片**（合并会让用户以为只动了一个文件）、未知类型照样渲染且不影响其余、载荷没有 text 时不崩<br>★★ 工具调用（真机走查补的）：同一个 `toolCallId` 的 `tool_call` + 若干 `tool_call_update` **归并成一张卡片**（不归并的话四条一模一样，用户以为动了四个文件）；卡片上显示 Agent 给的 title，没有就退到文件路径；状态取最后一次更新（停在中间态会让人以为还在跑）；**摘要不许降级覆盖但同档要覆盖**——ACP 先给泛称「Read File」再补具体的「Read README.md」，两侧各有一个负例卡着 |
| `use-event-stream.test.ts` | `src/features/timeline/` | ★★ **带 Authorization 头**——真机撞到的：第一版用 `EventSource`，它带不了自定义头，`/v1/events` 一路 401，而假 EventSource 的单测全绿。改用 fetch + ReadableStream，假的只剩网络<br>★ **token 不进 URL**（URL 会进浏览器历史与访问日志，而这个 token 等于驱动 Agent 改代码的权限）<br>★★ **首次连接带 `Last-Event-ID: 0` 把历史要回来**（不带的话用户重开应用后时间线是空的）；重连带最后收到的 seq（从头补会让整条时间线重放一遍）<br>★ 按 `work_id` 过滤（事件流是全局的）；按 seq 去重；卸载与切换工作时中止请求；坏消息与心跳跳过不断流；跨网络分片的消息能拼回来 |
| `Rail.test.tsx` | `src/features/rail/` | ★ 左栏 Runtime 栏显示**真实检测结果**（原本写死「尚未检测」，而后端返回两个 ready——界面说谎比界面简陋糟得多）：列出名称与版本、**区分就绪与未登录**（后者意味着用户得去登录）、一个都没有时给提示、**查询失败不让左栏白掉**（后端没起来时用户最需要的正是这条左栏）、折叠成 48px 图标条时不显示 |
| `ChatPage.test.tsx` | `src/features/chat/` | ★ 对话页的**用错路径**：没项目时引导去加而不是给一个点了没用的输入框；空需求不发请求（发出去后端会拒，用户看到一句莫名其妙的错误）；建工作失败**要说出来**（静默的话他不知道是没点上、在转圈、还是失败了）；建好之后才订阅事件流 |
| `ProjectSection.test.tsx` | `src/features/settings/sections/` | ★ 项目管理的**措辞与降级**：按钮写「移除」而**不是「删除」**（用户交出的是自己的代码目录，写「删除」会让他以为文件没了）；非 git 目录照样列出并给 `git init` 而不是拒绝；拿不到目录选择能力时给手填输入框而不是一个点了没反应的按钮（浏览器的 showDirectoryPicker 只给句柄不给路径）；出错时说读不到而不是伪装成「一个都没有」|
| `RuntimeSection.test.tsx` | `src/features/settings/` | ★ ACP Runtime 检测的**误导路径**：探测失败时说「检测不出来」而**绝不说「未安装」**（用户会照着去装已经装好的东西，装完还是不行），且这种情况下不给任何命令；整体检测失败显示错误与重试而不是空列表；没登录给的是登录命令不是安装命令；命令原样显示以便选中复制（R2 要求提示含具体命令而非「请检查配置」）|
| `persisted.test.ts` | `src/utils/` | 界面偏好持久化：存进去能读回来、布尔不丢类型；**坏值退回默认不抛**、**写失败静默忽略**——一个存储问题不该让整页白屏 |
| `use-update-flow.test.ts` | `src/features/settings/` | ★★ 一键更新流程的两条命脉：**先 prepare 再下载，顺序不可颠倒**；**prepare 返回 blocked 时绝不发起下载**（装下去会丢掉用户几十分钟的活）。另含 prepare 本身失败也不下载、浏览器形态不碰 prepare、检查失败不留过期状态、下载进度反馈 |
| `platform.test.ts` | `src/platform/` | 运行形态检测**靠壳注入的标记而非 User-Agent**（Tauri 的 WebView UA 与 Safari 极像，猜错就会在浏览器里显示一个点不动的更新按钮）；Web 降级给出真实的发布页地址而不是空函数；浏览器里调自更新明确抛错而不是静默无事发生 |
| `PermissionCard.test.tsx` | `src/features/permission/` | M3 U3.1.3 R1/R4：★★ 说清楚**谁要干什么、动哪个文件**（「codex 请求写入 `crates/…/events.rs` · 写入边界外」），照设计稿<br>★★ 按钮用 **Agent 给的 options 一个不漏**——自己造「允许/拒绝」两个的话，Agent 提供的第三种选项消失而用户不知道<br>★★ 点「拒绝」发的是拒绝那个 `optionId`，绝不发允许类（搞反 = 用户点拒绝而 Agent 收到允许）<br>★ 提交中禁用全部按钮（挡连点）；不越界时不说「写入边界外」（乱说会让用户对所有提示脱敏）；按工具类别说人话；认不出的类别用兜底文案不露原始码；没选项时说清楚而不是显示空卡片 |
| `PermissionDock.test.tsx` | `src/features/permission/` | M3 U3.1.3 R2/R3：★★ **点了才消失**（不消失的话用户会再点一次，第二条应答会被 Agent 当成不认识的请求）<br>★★ **不点它 5 秒后仍在**，没有超时没有自动关闭——用户去倒杯水回来，卡片没了、AI 也停着，他不知道刚才发生过什么<br>★ 提交失败时卡片留下并给提示（静默移除 = 用户以为处理完了而 AI 还在等）；多条各自独立；提交中挡住第二次点击；上游撤回时消失；一条都没有时什么都不渲染 |
| `use-permissions.test.ts` | `src/features/permission/` | M3 U3.1.4：从事件流里挑出待裁决的请求。★★ 契约的**蛇形字段转成驼峰**（`option_id` → `optionId`）——转错的话按钮上的 id 是 undefined，用户点了什么都发不出去<br>★★ **已应答的自己记**（事件流不撤回历史事件，不记的话用户点完一刷新同一张卡片又回来）；**提交失败时不算已应答**（记了的话卡片没了而 AI 还在等）<br>★ 载荷缺字段跳过这一条而不是白屏；**零选项的请求照样交给界面**（悄悄跳过的话 AI 挂着而用户完全不知道）；切工作时重新开始<br>★ 「提交失败」那条第一版无效：用 `await expect(act(...)).rejects` 的话 act 抛出结束、React 没 flush，断言看的是旧状态——造负例时发现的 |
| `WorkStatus.test.tsx` | `src/features/work/` | M3 U3.2.2：★★ 状态词显示**英文原值不翻译**（术语表硬要求——用户在文档、日志、界面上看到的必须是同一个词）<br>★★ 审查中**不给按钮并说清楚为什么**（只变灰的话用户以为是应用卡了）<br>★★ 提交中变进行中态并禁用（R2）；**失败如实告知不假装成功**（R4——静默失败的话用户以为停住了而 AI 照跑）<br>★ 「现在不能停」与「停失败了」文案分得开——都说「失败」的话，用户会一直重试一个注定被拒的操作<br>★ 认不出的状态也显示原值，不留空白 |
| `ProjectTree.test.tsx` | `src/features/rail/` | ★★ **M2 完成标志第 1 条**（用户流程第一步：创建项目 → 创建对话 → 观测对话）。2026-08-08 用户打开应用第一句话是「为什么菜单没有显示项目列表和对话记录」——那时这里是骨架占位，而 `/v1/projects` 明明有数据<br>★ 列出真实项目；项目下挂**它的**工作与状态；**工作按路径归位**（挂错的话用户在 A 项目下看到 B 的工作）；「新建对话」能点且带对项目路径；点工作能打开；项目能折叠（开着五六个时不折叠左栏会长到看不见底）<br>★★ **查询失败要说出来，不装作「还没有项目」**——装作没有的话用户以为自己的项目丢了，而实际是后端没起来 |

<!--
登记示例：

| `EventStream.test.tsx`   | `src/features/conversation/` | 13 类事件各自的渲染形态；过滤器开关；折叠态 |
| `use-event-stream.test.ts`| `src/features/conversation/`| SSE 断线重连、Last-Event-ID 续传、乱序事件按 seq 归位 |
| `UpdateCard.test.tsx`    | `src/features/settings/`     | 更新状态机：available → preparing → blocked 时展示被卡住的工作 |
-->
