# 待拍板的问题

> **这里汇总所有需要人做决定的事。** AI 不许替这些问题拍板——
> 撞上了就停下来，指向本文对应编号。
>
> 明细在各专题文档里，本文只做索引与优先级。**决定了就在这里划掉并注明结论与出处。**

---

## 怎么用

| 你是 | 怎么用 |
|---|---|
| AI | 动手前扫一眼与你任务相关的行。撞上未决问题 → **停，不要猜** |
| 人 | 按「什么时候必须定」排序，逐个拍板；定了写进对应文档并在这里标 ✅ |

---

## P0 · 挡住 M0 开工

| # | 问题 | 出处 |
|---|---|---|
| Q1 | `initializing` / `initializing_failed` 两个状态没进术语表——不定就写不出状态枚举的穷举测试 | `domain-model.md` OPEN-3 |
| Q2 | 人类可读 ID（`work-08`）与事件 ULID，谁是主键？ | `domain-model.md` OPEN-1 |
| Q3 | **同一个 Work 能否同时是 `executing` 和 `waiting_user`？** 设计稿两处自相矛盾，直接决定「D2 是否阻塞 executing」这条核心语义 | `domain-model.md` CONF-1 |
| Q4 | ACP 协议的真实消息结构 —— `acp-integration.md` §16 里 10 条标注为「待验证」的假设，必须用真实 Runtime 逐条核实 | `acp-integration.md` §16 |

### 设计稿与真实 ACP 规范的冲突（已核实，需设计侧修正）

对着 `agentclientprotocol.com/protocol/v1/*` 与两个 adapter 的公开源码核实后，
发现设计稿里有**事实性错误**。完整 13 条对照表在 `acp-integration.md` §2.2，以下四条要命：

| # | 冲突 | 影响 |
|---|---|---|
| ~~Q4a~~ | ~~设计稿的 `mem-188` 是错的~~ **← 已撤回，判定过重** | 见下 |
| Q4b | ✅ **已真机证实**：`auto` 只存在于 claude 的 6 个档里，codex 只有 `read-only`/`agent`/`agent-full-access`。设计稿角色表把 codex 绑到 `auto` 是错的，需设计侧修正 | 见 `acp-field-notes.md` §7.1 |
| Q4c | **`session/set_mode` 官方已挂废弃告示**，将被 Session Config Options 取代（codex-acp 已同时暴露两套） | 影响 `AGENTS.md` §8 术语表「会话模式 = `session/set_mode`」的表述 |
| Q4d | **客户端 MUST 用 `cancelled` 应答所有 pending 的 `session/request_permission`** —— 设计稿完全没提。漏了会导致每次取消都超时、`prepare` 永远返回 `blocked` | 直接影响 M1 的自动更新能否工作 |

> ⚠️ 还有一个高危同名陷阱：**ACP 的 `plan` 更新是 Agent 的 TODO 清单，不是 Duet 的 `PlanVersion`。**
> 误映射会污染只增不改的计划版本链。`acp-integration.md` 里单列了一条测试防这个。

#### Q4a 的撤回说明（文档演进的一次实例）

原判定：「设计稿的 `mem-188` 说 codex 默认档不询问是**错的**」，依据是源码里
`AgentMode.Agent` 的 `approvalPolicy` 是 `on-request`。

**这个判定过重，已撤回。** 拿到本机实测记录后（[`acp-field-notes.md`](acp-field-notes.md) §2），
两者其实一致：

| 档位 | `request_permission` 次数 | 文件建了吗 |
|---|---|---|
| `agent`（codex 默认）+ 客户端**全拒** | **0** | ✅ **建了** |
| `read-only` + 客户端全拒 | 2 | ❌ 没建 |

`agent` 档 = `workspace-write` 沙箱 + `on-request` 审批。**沙箱内的写操作根本不需要审批**，
所以观测到 0 次是正确行为；`on-request` 只对越出沙箱的操作生效。
**`mem-188` 的实用结论（默认档不问、必须收权）是对的。**

> **教训**：源码阅读（C 级）不能直接推翻实测（B 级）。
> 两者冲突时，多半是**语义层次不同**，不是谁错了。
> 权威性分级见 [`acp-field-notes.md`](acp-field-notes.md) 开头。

---

## P1 · 挡住 M1（发布与自动更新）

| # | 问题 | 出处 |
|---|---|---|
| Q5 | Apple 开发者证书与公证要不要做（$99/年）？现策略是 ad-hoc，首次安装需手动放行 | `adr/0002` |
| Q6 | minisign 私钥的离线备份放哪、谁保管？**丢了 = 所有已安装客户端永远收不到更新** | `release-and-update.md` §4 |

---

## P2 · 挡住 M2（主链路）

| # | 问题 | 出处 |
|---|---|---|
| Q7 | `acceptance_criteria` 是不是 `UnitContract` 的字段？设计稿 YAML 里没有，但抽屉里独立渲染 | `domain-model.md` OPEN-9 |
| Q8 | `contract_revision` 期间 Work 处于哪个状态？九个状态词里没有对应的 | `domain-model.md` OPEN-21 |
| Q9 | `UnitContract` 到底谁产出？计划架构师还是单元设计师？设计稿两处冲突 | `domain-model.md` CONF-3 |
| Q10 | Unit / Subplan 的完整状态集 | `domain-model.md` OPEN 系列 |
| Q11 | `superseded` 同时用作 Attempt 状态和 Evidence 状态，生命周期不同却共用一个词 | `domain-model.md` CONF-5 |
| Q12 | Attempt 缺「已结束待审查」状态，覆盖不了 Work 处于 `reviewing_unit` 的那一段 | `domain-model.md` CONF-6 |
| Q13 | Runtime 是二值枚举（claude/codex）还是注册表？设置页出现了第三项 `acp-sidecar` | `domain-model.md` OPEN 系列 |

---

## P2 · 设计缺口（挡住前端实现）

设计规范里找不到条目、但界面上确实存在的东西。**每一条都需要设计侧先补条目，前端才能实现**
（铁律 3：找不到条目 → 先在设计规范新增条目，再实现）。

| # | 缺口 | 优先级 |
|---|---|---|
| Q14 | **设置页 tab 选中态与第 08 节的「选中 = accent-900 底 + accent-300 字」直接矛盾** —— 实打实的冲突，需裁定 | 高 |
| Q15 | **文本输入框 / 文本域**无条目（原型里全是 div 模拟）：placeholder 色、focus 态、校验错误态全缺 | 高 |
| Q16 | **表格**无条目，但有三张真表（项目管理 / GitHub 绑定 / 角色与 Runtime） | 高 |
| Q17 | **空状态 / 骨架屏 / 错误态 / SSE 断线提示**四种界面无依据 | 高 |
| Q18 | **数据图表**（报表页折线图 + 环形图）无条目，且与「禁止自绘 SVG」字面冲突——需写清「图标 ≠ 数据图表」 | 中 |
| Q19 | **toast / 内联反馈**无条目也无 z-index 层，但 Web 降级方案依赖它 | 中 |
| Q20 | **动效 / 过渡**全文零规定，但设计稿写着抽屉「右侧滑入」、加载「圆环」 | 中 |
| Q21 | **搜索 / 命令面板**（窗口栏有 `搜索 ⌘K` 按钮）无面板设计 | 低 |
| Q22 | **敏感值掩码**（`ghp_…••••`）无规格：位数、可否展开、复制行为 | 低 |
| Q23 | 2px/3px/6px 圆角无令牌；`--color-accent-500` / `-200` 在用但未列入用法表 | 低 |
| Q24 | **面包屑**只在布局规则里被提一句，无视觉条目（原型有三级） | 低 |

完整明细见 `frontend-guide.md` §16。

---

## P1 · 由 M1 分章拆解时发现

| # | 问题 | 出处 |
|---|---|---|
| Q33 | **「发现新版本」要不要成为第 14 类事件？** `release-and-update.md` 说要发一条 `app` 事件，但 `Event.type` 是封闭枚举共 13 类，没有对应的一类。新增第 14 类要同时改四处。裁定前不实现事件发布 | `M1-release-and-update.md` |
| Q34 | **简化版 `prepare` 遇到「有进行中工作」时该怎么办？** `roadmap.md` 1.7 与 `adr/0002` 都只说「无工作时直接放行」，没说有工作时。M1 分章按**失败安全**补齐为返回 `blocked`，需确认 | `M1-release-and-update.md` U1.7.1 |
| Q35 | **`prepare` 的完整语义横跨 M0 与 M2。** `adr/0002` 说它依赖「M0 的 Work 状态机与 checkpoint」，但 Checkpoint 单元实际排在 M2（2.8），M1 内部无法闭环 | `M1-release-and-update.md` 全局停止条件 |
| Q36 | **品牌判断禁令与契约打架**：`openapi.yaml` 的 `RuntimeName` 是 `enum: [claude, codex]`，生成物必然落在 `internal/api/gen`，与「`grep 'codex\|claude'` 在 `api` 下为空」冲突。M1 分章按「排除 `gen/`」处理，需确认 | `M1-release-and-update.md` U1.9.1 |

## P2 · 由 M3/M4 分章拆解时发现（`domain-model.md` 有编号但此处未收录）

| # | 问题 | 出处 | 挡住谁 |
|---|---|---|---|
| Q37 | **报表指标的口径没定义** —— 「一次通过率 68%」「平均单元耗时 6m12s」的分子分母是什么？`obsolete` 单元计不计入分母？**这比图表没有设计条目更靠前**：图画不了只是画不了，口径没定则数字本身没有意义 | `domain-model.md` OPEN-7 | M4 报表全部 |
| Q38 | **三个隔离开关的默认值**（关闭 Runtime 机器级记忆 / 禁用未授权项目 MCP / 允许 Runtime 内建 Skill） | `domain-model.md` OPEN-17 | M3 U3.5.2 |
| Q39 | **「Work 工作记忆」是第三种存储，还是 `scope=work` 的 Memory？** 影响 `Memory.scope` 的枚举形态 | `domain-model.md` OPEN-8 | M3 记忆落库 |

### 里程碑划分的待确认项

| # | 问题 | 建议 |
|---|---|---|
| Q40 | **启动时从检查点恢复（`U4.7.1`）放在 M4 是否合理？** 它依赖 M1 的 `/system/resume` 与 M2 的 Checkpoint，与报表/设置/GitHub/英文版零交集，在 M4 里是一条孤立的并行链 | 挪到 M1 尾部或 M2 |
| Q41 | **三个隔离开关的归属**：界面在设置页（M4），实现在 adapter 会话参数（M3）。M3/M4 若要并行，**这是唯一一处跨里程碑依赖** | 把开关的设置页 UI 也划进 M3 |
| Q42 | **M2 的 2.5「13 类事件渲染器」是全做还是只做主链路用到的几个？** 影响 M3 的 `injection` 渲染器是新建还是改已有文件 | — |

---

### 已就地修正的（不需要拍板）

| 问题 | 处理 |
|---|---|
| `git-workflow.md` §5 列了四个 required check，与 `ci.md` 规则 2「只能 required 汇总门禁」矛盾 | ✅ 已改为只有 `ci`，并加了警告框说明为什么 |
| `release-and-update.md` §2 说 CI 有 `docs` job，实际文档检查在 `guard` 里 | ✅ 已改 |
| `release.yml` 的构建矩阵两行都是 `macos-14`，与 §4 的图（arm64 + x64 两个 runner）不符 | ✅ 已改注释说明 macos-14 可交叉编译两个 target |
| `database.md` §7 说「行结构+查询+映射全在一起」，与 §1 的 `entity/`+`mapper/` 分包矛盾 | ✅ 已改，并说明为什么这是有理由的例外 |
| `release.yml` 引用了不存在的 `scripts/make-universal.sh` 与 `make-updater-manifest.sh` | 由 M1 的 U1.5.1 / U1.5.3 补 |
| 三个端点只在 `release-and-update.md` §8 的表里，`openapi.yaml` 没有 | 由 M1 对应单元补 |

---

## P3 · 可以先按推定实现，之后再定

| # | 问题 | 出处 |
|---|---|---|
| Q25 | 记忆计数口径（`全部 12` 与 `9+2+3=14` 对不上） | `domain-model.md` CONF-2 |
| Q26 | `confidence` / `sensitivity` 的取值域 | `domain-model.md` OPEN 系列 |
| Q27 | 候选记忆 `cand-07` → `mem-203` 的 ID 是否改写（改写会断掉 `source_refs`） | `domain-model.md` OPEN 系列 |
| Q28 | 项目 Skill 与全局 Skill 同名时的优先级 | `domain-model.md` OPEN 系列 |
| Q29 | 跨项目记忆（L3）的去标识化具体规则 | `architecture.md` §8 |
| Q30 | `~/.duet/worktrees` 与 `~/.acpflows` 两个顶层目录是否有意为之 | `domain-model.md` OPEN 系列 |
| Q31 | 多 Work 并发上限 | `architecture.md` §8 |
| Q32 | 角色卡显示 `claude · sonnet · 中`，但设置页明确「模型不在协议里，这里不设」——呈现位置需确认 | `domain-model.md` CONF-4 |

---

## 已决

| # | 问题 | 结论 | 出处 |
|---|---|---|---|
| ✅ | 应用外壳形态 | Tauri v2 + Go sidecar；**duetd 必须独立可运行，不得走 IPC 绕过 HTTP** | `adr/0001` |
| ✅ | 前端技术栈 | React 18 + TS + Vite | `adr/0001` |
| ✅ | API 契约真源 | OpenAPI 3.1 手写 spec 先行 | `adr/0001` |
| ✅ | 首个里程碑 | ACP 协议层（Fake Runtime 第一个做） | `adr/0001` · `roadmap.md` |
| ✅ | 发布方式 | release-please + tag 触发；**发版由人点**，其余全自动 | `adr/0002` |
| ✅ | 签名策略 | minisign 立刻做；Apple 公证暂缓（CI 预留开关） | `adr/0002` |
| ✅ | 更新语义 | 自动检查、**绝不自动安装**；更新前 `prepare` 暂停工作落检查点 | `adr/0002` |
| ✅ | 仓库归属 | `HuLuca1998/acp-flows`（公开） | — |
| ✅ | 界面语言 | 中英双语，`zh-CN` 默认；**后端只返回错误码，不返回中文文案** | `i18n.md` |

---

## 维护规则

- **新问题从哪来**：写文档、写代码、审查时撞上「设计稿没说 / 两处冲突 / 需要人取舍」→ 加一行
- **AI 不许自己删行**。只有人拍板后才移到「已决」
- 编号只增不复用
- 每个里程碑开工前，把该里程碑对应优先级的问题清空
