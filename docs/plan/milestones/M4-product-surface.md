# M4 · 完整产品面

> 体系与编号规则见 [`README.md`](README.md)。开工前必读
> [`../../spec/frontend-guide.md`](../../spec/frontend-guide.md)（组件与设计合规，**§16 的缺口表挡着本里程碑一半的单元**）、
> [`../../rules/i18n.md`](../../rules/i18n.md)（英文版的全部规则）、
> [`../../spec/domain-model.md`](../../spec/domain-model.md) §12（D3 逐次授权）与
> [`../open-questions.md`](../open-questions.md)。

> **读法**：本文 ~8k token，**不要整篇读**。标准读法是三段：
>
> ```
> ① 目标 + 完成标志 + 全局停止条件      必读
> ② 子计划 DAG                         确认前置做完了
> ③ 你自己那一个 S4.x                   只读这一节
> ```
>
> ```bash
> grep -n '^## S4' docs/plan/milestones/M4-product-surface.md
> ```
>
> | 子计划 | 一句话 | 状态 |
> |---|---|---|
> | S4.1 ★ | 报表指标口径与聚合 | U4.1.2 ⛔ 被 OPEN-7 挡住 |
> | S4.2 | 报表页（指标卡 · 驳回原因表 · 导出 CSV） | U4.2.2 图表 ⛔ 被 Q18 挡住 |
> | S4.3 ★ | 凭据保管（令牌绝不进 Agent 上下文） | 可开工 |
> | S4.4 ★ | 远端操作 = D3 逐次授权 | 依赖 S4.3 |
> | S4.5 | 设置页全量 | 依赖 S4.2 / S4.4 |
> | S4.6 | 英文版 `en-US` | **必须最后做** |
> | ~~S4.7~~ | 启动恢复 —— **已挪到 M1 `U1.8.4`**（`adr/0006` Q40） | — |

## 目标

**报表页给出四项指标、设置页六个分区全量可用、GitHub 集成把令牌关在应用里、
界面能整体切到英文。**

## 完成标志

```bash
make check                                          # 全绿，覆盖率门槛达标
go test -tags=integration ./tests/integration/... -run 'TestReportMetrics|TestGitHubD3|TestResume'
grep -rn 'ghp_\|GITHUB_TOKEN\|GH_TOKEN' backend/internal/{acp,app,domain}   # 必须为空
make check-i18n                                     # 两个 locale 的 key 集合完全相同
pnpm -C e2e test --grep 'push 逐次授权|English UI'
```

## 为什么 M4 是这个顺序

三条链，各自有一条不能颠倒的内部顺序。

**凭据先于远端操作。** 令牌的保管方式定错了，后面每一次 `push` 都在扩大同一个敞口；
而「令牌不进 Agent 上下文」这条约束一旦在某个单元被破一次，
再想收回来要翻遍所有已经写好的会话参数与环境变量拼装代码。

**指标口径先于报表页。** 「一次通过率 68%」的分母是什么，`obsolete` 的单元算不算，
重试算不算——口径没定就画图，等于把一个猜测画成结论摆在用户面前。
这正是设计规范「结论不带证据入口」要挡的东西。

**英文版排最后。** `make check-i18n` 校验「无未使用词条」与「无缺失词条」两个方向，
界面没铺完就做全量词条，这个检查会一直红，而红着的检查很快就会被绕过。

> 这是铁律 5 的另一面：**先把证据链和口径钉死，再把结论呈现出来。**

## 依赖

**M2**（主链路垂直切片）。报表的四项指标全部从 Attempt / 审查结论 /
`OperationInvocation` 聚合，这些对象在 M2 落地。

**与 M3 可并行**（不同 worktree）：M4 写 `features/{report,settings}/`，
M3 写 `features/{memory,skill}/`，后端交集只有 `api/openapi.yaml` 与
`internal/store/migration/` 的编号。两者都要改 `openapi.yaml` 时**串行合并**。

启动恢复已挪进 M1（`U1.8.4`），M4 不再依赖它。

## 全局停止条件

触发任一条 **立刻停下来上报**，不要自行扩大范围：

- 撞上 `open-questions.md` **Q18**（报表页的数据图表在设计规范里无条目，且与「禁止自绘 SVG」字面冲突）
- 撞上 `domain-model.md` **OPEN-7**（`一次通过率` / `平均单元耗时` / 进度分母的定义均未给出）——
  这一条 `open-questions.md` **尚未收录**，需要人先补一行编号再拍板
- 撞上 **Q14**（设置页 tab 选中态与设计规范第 08 节冲突）/ **Q15**（文本输入框无条目）/
  **Q16**（表格无条目）/ **Q17**（空状态与错误态无条目）/ **Q19**（toast 无条目）/
  **Q22**（敏感值掩码无规格）/ **Q13**（Runtime 是二值枚举还是注册表）
- 令牌需要出现在 Agent 上下文的**任何一处**：提示词、环境变量、MCP 参数、`session/new` 载荷
- 出现任何形式的 D3「始终允许」持久化记录（违反 INV-DEC-3）
- 需要依赖本机 `gh` 的登录态，或需要调用 `gh` 命令行
- 需要改 `api/openapi.yaml` 而当前单元没授权

---

## 子计划 DAG

```
S4.3 凭据保管 ★              S4.1 报表指标口径与聚合 ★
  │                            │
  ▼                            ▼
S4.4 远端操作 = D3 ★         S4.2 报表页 ⛔ Q18
  └──────────────┬─────────────┘
                 ▼
          S4.5 设置全量
                 ▼
          S4.6 英文版 en-US
```

**可并行**：`S4.3 → S4.4` 与 `S4.1 → S4.2` 两条链互不相交，从第一天就能并行开。
两条链在 `S4.5` 汇合（设置页要展示 GitHub 绑定），`S4.6` 必须最后做——
它要把前面所有界面的词条一次性补齐并校对。

> 原来的第三条链「S4.7 启动恢复」已按 `adr/0006` Q40 挪进 M1（`U1.8.4`）。

---

## S4.1 · 报表指标口径与聚合 ★

**阶段交付物**：四项指标各有一个写死的口径定义与一条可复现的聚合查询。

> **这个子计划的产物一半是事实、一半是裁定。** 口径没定死之前，
> 后面每一张图都是把猜测画成结论。

### ○ U4.1.1 · `OperationInvocation` 计数与 Runtime 使用统计

| | |
|---|---|
| `goal` | Runtime 使用量按 `OperationInvocation` 次数统计，**重试不计数**，口径可复现 |
| `allowed_changes` | `backend/internal/domain/model/operation_invocation.go` 及其测试 · `backend/internal/store/query/report.go` · `backend/internal/app/report/runtime_usage_usecase.go` · `backend/tests/integration/report_metrics_test.go` |
| `forbidden_changes` | 不做前端渲染（S4.2）；不在 `store` 里写业务规则——「什么算一次调用」在 `domain` |
| `stop_conditions` | 发现同一个 Attempt 可能横跨两个 Runtime——`usage_count` 的归属规则需要人定 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | **重试产生的 Attempt 不增加计数**（INV-RT-3） | 造 1 次成功 + 2 次重试的夹具，断言 `usage_count == 1` |
| R2 | 一次 Attempt 内多轮 `session/prompt` 只算一次调用 | 用 Fake Runtime 跑 3 轮，断言计数 +1 |
| R3 | 按 Runtime 分组的计数之和等于总调用数 | 断言 `sum(byRuntime) == total`，两端各造数据 |
| R4 | 统计可复现：同一数据集连查两次结果相同 | 连查 10 次断言逐次相等（无时间参与、无随机排序） |
| R5 | Attempt 必须记录 `runtime_name` + `runtime_version` + `set_mode`（INV-ATT-6） | 缺任一项时断言该 Attempt 无法进入 `succeeded`，因而也不进统计 |
| R6 | 覆盖率 ≥ 90%（`domain` 部分） | `make cover` |

**测试**：R1 是这条口径唯一的意义所在——**把重试算进去，这个数字就变成了「系统有多不稳定」而不是「用了多少」**。

### ○ U4.1.2 · 三项过程指标 ⛔ 口径待定

| | |
|---|---|
| `goal` | 一次通过率、平均单元耗时、驳回原因分布三项指标各有一个写死的分子分母定义与聚合查询 |
| `allowed_changes` | `backend/internal/domain/policy/report_metric.go` 及其测试 · `backend/internal/store/query/report.go` · `backend/internal/app/report/metrics_usecase.go` · `api/openapi.yaml` 的 `/v1/reports/*` |
| `forbidden_changes` | **不自行定义分母**；不把「驳回原因」当成第五类审查结果（它是 `review_result` 的下钻维度） |
| `stop_conditions` | **开工即撞 `domain-model.md` OPEN-7**：`一次通过率 68%` 与 `平均单元耗时 6m12s` 的定义均未给出，`obsolete` 的单元是否计入分母也未定。`open-questions.md` 尚未收录此条，需要人先补编号再裁定 |

**驳回原因 → 审查结论的映射（`domain-model.md` §10.3 已定，不待裁定）**

| 驳回原因 | 映射到 |
|---|---|
| diff 越出写入边界 | `implementation_fix` |
| 测试未真正验证 | `implementation_fix` |
| 契约不清 → 改版本 | `contract_revision` |
| 架构假设错误 → 重规划 | `global_replan` |

**验收标准**（OPEN-7 裁定后才可动工，这里先钉死与口径无关的结构性约束）

| # | 标准 | 断言 |
|---|---|---|
| R1 | 驳回原因分布的四个桶穷举覆盖三类非 `accepted` 结论 | 表驱动，断言每条 `rejected` 的 Attempt 恰好落进一个桶，无遗漏、无重复 |
| R2 | 新增一个驳回原因而未映射时测试红 | 穷举测试覆盖映射表 |
| R3 | 分布计数之和等于非 `accepted` 的审查结论总数 | 断言 `sum(buckets) == count(review_result != accepted)` |
| R4 | 一次通过率与平均单元耗时的口径以**具名常量**表达，不是查询里的魔法条件 | `grep` 断言 `store/query/report.go` 里不出现裸的状态字符串字面量 |
| R5 | OPEN-7 未裁定时，两项指标的端点返回明确错误而不是一个数 | 断言返回 `ErrMetricDefinitionUndefined`，错误信息指向 `domain-model.md` OPEN-7 |
| R6 | 响应通过 `openapi.yaml` schema 校验 | `kin-openapi`，接进 `backend/tests/contract/` |

**测试**：R5 是**本单元在 OPEN-7 裁定前唯一允许交付的行为**。
一个口径不明的百分比会被用户当成质量指标去做决定，而它可能只是分母选错了。

---

## S4.2 · 报表页 ⛔ 设计缺口阻塞

**阶段交付物**：报表页的非图表部分可用；图表部分在 Q18 裁定前不动工。

### ○ U4.2.1 · 指标卡与驳回原因表

| | |
|---|---|
| `goal` | 报表页的四张指标卡与驳回原因分布表可用，每个数字都有下钻入口 |
| `allowed_changes` | `frontend/src/features/report/**`（不含图表组件）· `frontend/src/i18n/locales/{zh-CN,en-US}.json` 的 `report.*` 词条 · `frontend/tests/INDEX.md` |
| `forbidden_changes` | **不画任何 SVG**；不在 `features/` 里定义设计令牌；不硬编码用户可见文本；不为找不到条目的元素临时发明样式（铁律 3） |
| `stop_conditions` | 撞上 Q16（表格无条目——驳回原因分布是一张真表）/ Q17（空状态与骨架屏无条目）；撞上 OPEN-7（数字本身没有口径，见 U4.1.2） |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 每个指标数字有下钻入口（设计规范：结论不带证据入口是反例） | 断言四张卡各有一个可点元素，点击后进入对应的 Attempt / Unit 列表 |
| R2 | 等宽数据不走 i18n（`i18n.md` §5 例外） | 断言 `6m12s` / `1,204` / `68%` 一类文本未调 `t()`，且用 `--font-mono` |
| R3 | 卡片标题与说明全部走 `t()` | `pnpm -C frontend lint` 绿（ESLint 的中日韩字面量规则） |
| R4 | 口径未裁定时展示明确的「口径待定」态，**不展示占位数字** | 后端返回 `ErrMetricDefinitionUndefined` 时，断言页面不出现任何数字字符 |
| R5 | 组件测试按行为查询 | `grep -rn 'data-testid\|querySelector' frontend/src/features/report` 结果为空 |
| R6 | 词条中英同进同退 | `make check-i18n` 绿 |

### ○ U4.2.2 · 数据图表 ⛔ 阻塞，不动工

| | |
|---|---|
| `goal` | 报表页的折线面积图与环形图 |
| `allowed_changes` | —（Q18 裁定前**没有**允许改动的路径） |
| `forbidden_changes` | 全部。设计规范第 05 节没有数据图表条目，且 `AGENTS.md` §9 写着「禁止自绘 SVG 图标」，两者字面冲突 |
| `stop_conditions` | **本单元开工条件就是停止条件**：`open-questions.md` **Q18** / `frontend-guide.md` §16 **G1** 未裁定前不动工。需要设计侧先写清三件事：①「图标 ≠ 数据图表」的界线在哪 ② 图表用哪几阶强调色（原型用了 `--color-accent-500`，而强调阶用法表只列 900/700/600/400/300，见 Q23 / G9）③ 图表是自绘还是引入库（引库要走「未经批准的第三方依赖」审批） |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | Q18 裁定前本单元保持 `○`，仓库里不存在图表组件 | `grep -rn '<svg' frontend/src/features/report` 结果为空，接进 CI |

**测试**：R1 是一条**防止绕过裁定**的断言，跟 M0 那些「故意制造违规确认检查会红」的检查同类。
把它接进 `pnpm -C frontend lint` 的自定义规则，比写在文档里靠人记住可靠。

### ○ U4.2.3 · 导出 CSV

| | |
|---|---|
| `goal` | 报表页的「导出 CSV」可用：导出当前筛选范围（`近 30 天 · 全部项目` 一类）下的明细，Tauri 与 Web 两种形态走同一条下载路径 |
| `allowed_changes` | `backend/internal/api/report/export.go` · `backend/internal/app/report/export_usecase.go` · `api/openapi.yaml` · `frontend/src/features/report/**` 的按钮接线 · `frontend/src/platform/download.ts` · 对应测试与索引 |
| `forbidden_changes` | **前端不拼 CSV**——内容由后端产出（前端拼一份就是第二个真源，且转义规则必然与后端不一致）；不引入前端 CSV 库；不在导出里塞任何未经 `U4.1.2` 定义口径的指标 |
| `stop_conditions` | **口径未裁定的指标一律不导出**（OPEN-7 / Q18）——导出一列口径不明的数字比不导出更糟，因为它会被拿去做决策；导出的列清单若在设计稿找不到，按 `frontend-guide.md` §16 登记缺口 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | CSV 由后端产出 | `grep -rn 'csv\|CSV' frontend/src` 不出现任何拼接逻辑；断言前端只发请求 + 触发下载 |
| R2 | 导出范围与页面当前筛选一致 | 筛选到「近 7 天 · 项目 A」后导出，断言响应体行数与页面上的明细条数相等 |
| R3 | ★ 转义正确：含逗号、双引号、换行、中文的字段不破坏结构 | 造一条驳回原因含 `他说,"不行"\n换行` 的记录，导出后用 CSV 解析器读回，断言字段逐字节相等 |
| R4 | ★ 口径未裁定的指标**不出现在导出里** | 断言表头列集合等于「已裁定口径」白名单常量；加一个未裁定指标而不改白名单时测试必须红 |
| R5 | 表头是机器可读的稳定列名，不随界面语言变 | 中英两版各导出一次，断言表头逐字节相同 |
| R6 | Tauri 与 Web 走同一条下载路径 | 断言两种形态都调 `platform/download`，`grep` 断言 `features/report` 里不出现 `@tauri-apps` |
| R7 | 大结果集不把整个表读进内存 | 造 10 万行，断言导出过程中进程内存增长有上界（流式写出），且响应带 `Content-Disposition` |
| R8 | 全程不碰 `~/.acpflows` | 铁律 6 守卫断言 |

> R4 与 U4.2.1 的 R4 是同一条原则的两面：**界面上不显示口径不明的数字，
> 导出里更不能有**——CSV 会被下载、转发、贴进汇报，脱离了「口径待定」的上下文。

---

## S4.3 · 凭据保管 ★

**阶段交付物**：GitHub PAT 加密存放在 `~/.acpflows/credentials`，且有一条常驻检查证明它进不了 Agent 上下文。

### ○ U4.3.1 · 加密存取与「不进上下文」守卫

| | |
|---|---|
| `goal` | PAT 加密写入 `~/.acpflows/credentials`，**不写入任何项目目录**，且任何进入 Agent 的载荷里都不可能出现它 |
| `allowed_changes` | `backend/internal/ghx/credential.go` 及其测试 · `backend/internal/platform/keychain.go` · `backend/internal/acp/runtime/env.go` 的拒绝清单 · `scripts/check/check-naming.sh` |
| `forbidden_changes` | 不实现远端操作（S4.4）；令牌不得进入日志、`Problem.detail`、事件 payload、SSE；不得透传 `GITHUB_TOKEN` / `GH_TOKEN` |
| `stop_conditions` | 加密需要引入未经批准的第三方依赖；macOS keychain 不可用时的降级路径需要人定 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 凭据文件落在 `~/.acpflows/credentials`，权限 `0600` | 断言文件模式；路径经 `platform.Paths` 注入到 `t.TempDir()` |
| R2 | **不写入任何项目目录** | 绑定一个夹具项目后对项目目录做递归哈希，断言与绑定前相等 |
| R3 | 落盘内容不含令牌明文 | 断言文件字节里不出现夹具令牌串 |
| R4 | **spawn ACP 子进程时不透传 `GITHUB_TOKEN` / `GH_TOKEN`**（`acp-integration.md` §9.4） | 断言子进程环境变量表里两个键都不存在；宿主环境故意设上这两个变量再断言一次 |
| R5 | 令牌不进日志与错误 | 触发一次鉴权失败，断言日志与 `Problem.detail` 里都不含令牌串，且 `Problem.type` 是机器可读错误码（`i18n.md` §3） |
| R6 | **常驻检查**：全仓库禁止令牌字面量 | `grep -rn 'ghp_\|GITHUB_TOKEN\|GH_TOKEN' backend/internal/{acp,app,domain}` 为空，接进 `scripts/check/check-naming.sh` 与 CI |
| R7 | 测试不碰真实令牌（铁律 6） | 全部用夹具串；HTTP 全部拦截，`testing-strategy.md` §5「永不出网」 |

**测试**：R4 是最容易漏的一条——**环境变量也是上下文**（`acp-integration.md` §9.4 原文）。
R6 把这条约束从「记得别写」变成「写了就红」。

### ○ U4.3.2 · 按 remote 匹配账号

| | |
|---|---|
| `goal` | 一个项目按它的 git `remote` 匹配到对应的 GitHub 账号与读写档，多账号不串号，**不依赖本机 `gh` 登录态** |
| `allowed_changes` | `backend/internal/ghx/account.go` 及其测试 · `backend/internal/domain/model/project.go` 的 `github_account` 字段 · `backend/internal/app/project/bind_usecase.go` |
| `forbidden_changes` | 不调用 `gh` 命令行、不读 `~/.config/gh/`；不实现远端操作（S4.4）；`remote` 为空的项目不得回退到「任意一个账号」 |
| `stop_conditions` | 同一个 remote 匹配到多个账号——优先级规则需要人定 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 按 remote 精确匹配账号 | 造两个账号 × 三个 remote 的夹具，表驱动断言每个项目拿到的账号 ID |
| R2 | **多账号不串号** | 断言账号 A 的令牌永不被用于账号 B 绑定的 remote：对 B 的请求断言携带的是 B 的凭据句柄 |
| R3 | `remote` 为空时不绑定任何账号 | 断言 `github_account` 为空，且远端操作端点返回 `ErrNoGitHubAccount` |
| R4 | **不依赖本机 `gh`** | `grep -rn '"gh"\|gh auth\|config/gh' backend/internal` 为空；把 `PATH` 清空后跑全套测试仍绿 |
| R5 | 令牌无效时给出可执行提示 | 断言错误码为 `github_token_invalid`，且前端词条含重新绑定的入口（词条在 `en-US` 与 `zh-CN` 都存在） |
| R6 | 读写档只有两种：`只读` / `可写（需逐次授权）` | 穷举断言取值集合；第三个值断言拒绝 |

**测试**：R2 造「同一台机器上两个 GitHub 账号」的夹具。
**串号一次的后果是把 A 的代码 push 到 B 的仓库**，且发生时没有任何报错。

---

## S4.4 · 远端操作 = D3 ★

**阶段交付物**：`push` / 发 PR / 删除远端资源全部走 D3 逐次授权，且由应用代为执行。

### ○ U4.4.1 · D3 逐次授权

| | |
|---|---|
| `goal` | 每一次远端动作都产生一条独立的 D3 Decision，**即使令牌是可写档**；系统里不存在任何 D3「始终允许」 |
| `allowed_changes` | `backend/internal/domain/policy/decision_level.go` 的远端动作判定及其测试 · `backend/internal/app/github/authorize_usecase.go` · `api/openapi.yaml` 的 D3 授权端点 |
| `forbidden_changes` | 不新增 Decision 等级；不提供「记住这次选择」的持久化；不下调等级（INV-DEC-7） |
| `stop_conditions` | 出现一种远端动作无法归入「push / 发布 / 删除远端 / 付费资源」四类 —— 判定规则要先扩 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | `push` / 发 PR / 删除远端各产生一条 `D3` Decision | 三个用例，各断言 Decision 的 `level == "D3"` 与 `authorized_action` 为具体动作串 |
| R2 | **令牌为可写档时 `push` 仍返回「需 D3 授权」**（INV-DEC-5） | 绑定可写账号后断言 `ErrD3AuthorizationRequired` |
| R3 | **同一动作再次发生必须重新授权**（INV-DEC-4） | 授权一次并执行 → 再次发起同一动作，断言又产生一条新 Decision，且未复用前一条 |
| R4 | **系统中不存在 D3「始终允许」记录**（INV-DEC-3） | 穷举 `store` 的 Decision 相关列名，断言不存在 `always_allow` / `remember` 一类字段；穷举 API 断言无该参数 |
| R5 | 一次 D2 选择不授予任何 D3 权限（INV-DEC-10） | 先确认一个 D2 再发起 `push`，断言仍需 D3 授权 |
| R6 | D3 未决期间 Work 处于 `waiting_user` 且不派发新 Attempt（INV-DEC-2） | 断言 Work 状态与 Attempt 计数 |
| R7 | D3 授权对话框不可点遮罩关闭（`domain-model.md` §12.3） | Playwright：点遮罩后断言对话框仍在 |

**测试**：R4 是这条产品主张的结构性防线——**只要 schema 里没有「始终允许」这个字段，
后面任何一轮 AI 都实现不出这个功能**。比在用例里判断可靠得多。

### ○ U4.4.2 · 应用代为执行，Agent 拿不到令牌

| | |
|---|---|
| `goal` | 远端操作由 `duetd` 直接执行，Agent 全程只能请求、拿不到令牌本身（INV-DEC-6） |
| `allowed_changes` | `backend/internal/ghx/remote_op.go` 及其测试 · `backend/internal/app/github/execute_usecase.go` · `backend/tests/integration/github_d3_test.go` |
| `forbidden_changes` | 不把令牌放进任何交给 Runtime 的载荷；不通过 MCP 把远端能力暴露给 Agent；不调用 `gh` |
| `stop_conditions` | 某个远端动作只能由 Agent 侧完成 —— 说明能力边界画错了，停下来 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | **Agent 永远拿不到令牌**（INV-DEC-6） | 录下一整轮会话发给 Runtime 的全部字节（`session/new` + 全部 `session/prompt` + 环境变量表），断言其中不含夹具令牌串 |
| R2 | 远端操作在 `duetd` 进程内完成 | 断言 HTTP 拦截器记录到的请求来自 `ghx`，且 Runtime 子进程零出网（进程级断言：白名单环境不含代理变量） |
| R3 | 授权被拒时不执行任何远端调用 | 断言拦截器记录的请求数为 0 |
| R4 | 执行结果作为 Evidence 由**应用直接采集** | 断言 Evidence 的来源是 `app` 而非 Agent 转述（`AGENTS.md` §9） |
| R5 | 永不出网 | 全套测试在 HTTP 被拦截的前提下通过；解除拦截器后测试**必须红**（证明拦截真的在生效） |

**测试**：R1 的做法是**把发给 Runtime 的整条字节流录下来做子串断言**——
比逐个字段检查可靠，新增一个字段时不会漏。R5 的反向验证同 M0 的「故意制造违规」手法。

---

## S4.5 · 设置全量

**阶段交付物**：设置页六个分区可用；三张真表与全部输入控件有设计依据。

### ○ U4.5.1 · 环境检测与 Runtime 分区

| | |
|---|---|
| `goal` | 展示已安装 Runtime、版本、路径、登录态、探针结果与多版本切换，未通过项显式降级 |
| `allowed_changes` | `frontend/src/features/settings/runtime/**` · `frontend/src/i18n/locales/{zh-CN,en-US}.json` 的 `settings.runtime.*` 词条 · `frontend/tests/INDEX.md` |
| `forbidden_changes` | 不在前端做能力判断——只渲染后端给的能力矩阵；不出现按角色设模型一类 ACP 不支持的设置项（`AGENTS.md` §9） |
| `stop_conditions` | 撞上 Q13（Runtime 是二值枚举还是注册表——设置页出现了第三项 `acp-sidecar`）；撞上 Q14（tab 选中态冲突）/ Q16（表格无条目） |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 探针未通过项显示为能力降级，**不显示为可用**（INV-RT-1） | 用 `11/12 通过` 的夹具，断言对应能力标记为不可用且相关操作按钮禁用 |
| R2 | 未安装 / 未登录时提示含**可执行的具体命令** | 断言文本含 `codex login` 一类命令，且命令用 `--font-mono` 不翻译 |
| R3 | 多版本切换不删旧版本 | Playwright：切换后断言旧版本行仍在列表里 |
| R4 | 前端零品牌硬编码分支 | `grep -rn "=== 'codex'\|=== 'claude'" frontend/src/features/settings` 为空 |
| R5 | 版本号、路径、`protocol_version` 等标识符不翻译 | 中英两版各断言这些文本相同 |
| R6 | 词条中英同进同退 | `make check-i18n` 绿 |

### ○ U4.5.2 · 项目管理分区

| | |
|---|---|
| `goal` | 添加本地仓库、设置默认分支、导入 Skill、清理 worktree、移除项目（两种语义）在界面上可完成 |
| `allowed_changes` | `frontend/src/features/settings/project/**` · `frontend/src/i18n/locales/{zh-CN,en-US}.json` 的 `settings.project.*` 与 `project.*` 词条 · `frontend/tests/INDEX.md` |
| `forbidden_changes` | 不在前端做路径存在性校验（后端的事）；不硬编码用户可见文本；不为找不到条目的元素临时发明样式 |
| `stop_conditions` | 撞上 Q15（文本输入框无条目——Web 降级下手输路径要用真 `input`）/ Q16（项目管理是一张真表）/ Q17（空状态无条目）/ Q19（toast 无条目——「复制路径到剪贴板 + toast」是 Web 降级的依赖） |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 移除项目**只有两种语义**（INV-PRJ-4） | 断言选项恰为 `仅解除索引` 与 `连同 .acpflows 记忆一并清除` 两项，无第三种 |
| R2 | 破坏性动作的按钮文案写清后果（`AGENTS.md` §8） | 断言「连同记忆清除」的按钮可访问名含被删除对象的数量 |
| R3 | 选择文件夹走平台适配层，Web 降级真的可用 | 断言 `features/` 下不 import `@tauri-apps/*`（ESLint 规则）；Web 模式下断言手输路径后校验请求被发出 |
| R4 | 导入 Skill 后原目录不变（INV-PRJ-6 的界面侧） | Playwright + 夹具目录哈希，断言导入前后相等 |
| R5 | 路径、分支名不翻译 | 中英两版断言相同 |
| R6 | 词条中英同进同退 | `make check-i18n` 绿 |

### ○ U4.5.3 · GitHub 账号与按仓库绑定分区

| | |
|---|---|
| `goal` | 绑定 / 解绑 GitHub 账号、查看按仓库的绑定与读写档、发起一次 D3 授权在界面上可完成 |
| `allowed_changes` | `frontend/src/features/settings/github/**` · `frontend/src/i18n/locales/{zh-CN,en-US}.json` 的 `settings.github.*` 词条 · `frontend/tests/INDEX.md` |
| `forbidden_changes` | **令牌明文不得进入前端状态、localStorage、URL 或任何请求以外的地方**；不实现「记住授权」；不调 `gh` |
| `stop_conditions` | 撞上 Q22（敏感值掩码无规格：位数、可否展开、复制行为）/ Q15（PAT 输入需要真的 `<input>`）/ Q16（按仓库绑定是一张真表） |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 令牌提交后前端不再持有明文 | 提交后断言 Zustand store 与 `localStorage` 里不含令牌串 |
| R2 | 展示为掩码 | 断言渲染文本不等于原令牌；Q22 裁定前**不实现展开** |
| R3 | 读写档展示为 `只读` / `可写（需逐次授权）` 两种 | 断言两种档位的文案，且可写档旁边有「仍需逐次授权」的说明 |
| R4 | 界面上不存在任何「始终允许」入口（INV-DEC-3） | 断言 DOM 里无该选项；`grep` 断言词条文件里没有对应 key |
| R5 | D3 授权对话框不可点遮罩关闭 | Playwright，同 U4.4.1 R7 |
| R6 | 词条中英同进同退 | `make check-i18n` 绿 |

**测试**：R4 同时断言 DOM 与词条文件。**词条里有这个 key 就说明有人打算做这个功能**——
在这个产品里，「始终允许」不是待实现，是明令禁止。

---

## S4.6 · 英文版 `en-US`

**阶段交付物**：两个 locale 的 key 集合完全相同，英文版排版校对通过，不翻译清单被机器守住。

### ○ U4.6.1 · 全量词条与强制检查

| | |
|---|---|
| `goal` | `en-US.json` 补齐全部 key，`make check-i18n` 的四项校验全绿，且错误码与词条一一对应 |
| `allowed_changes` | `frontend/src/i18n/locales/en-US.json` · `frontend/src/i18n/locales/zh-CN.json`（仅补缺失 key）· `frontend/src/i18n/resources.d.ts`（由生成器产出）· `scripts/check/check-i18n.sh` |
| `forbidden_changes` | **不翻译状态词、标识符、命令、路径、ID**（`i18n.md` §2）；不改组件逻辑；不新增中文原文当 key；不动态拼 key |
| `stop_conditions` | 发现某个界面文案在后端硬编码返回（违反 `i18n.md` §3）——修后端属于跨单元改动，停下来 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 两个 locale 的 key 集合完全相同 | `make check-i18n` 第一项；故意删一个 key 断言检查变红 |
| R2 | 无未使用词条、无缺失词条 | `make check-i18n` 第二、三项；各故意造一个违规断言检查变红 |
| R3 | `openapi.yaml` 的错误码枚举**全部**有词条 | `make check-i18n` 第四项；新增一个错误码不加词条断言变红 |
| R4 | **不翻译清单被机器守住** | 对 `en-US.json` 全文断言：不出现 `zh-CN.json` 里那些状态词、审查结论、`D0`–`D3`、ACP 方法名的译文；用 `i18n.md` §2 的表生成断言集合 |
| R5 | 术语中英对照固定 | 断言 `en-US.json` 里 `Work` / `Subplan` / `Unit Contract` / `Attempt` / `Evidence` / `Checkpoint` 等 14 个术语用的是 `i18n.md` §2 的译名，无第二种译法 |
| R6 | 复数与日期走 `Intl` / i18next 后缀，无手拼 | `grep` 断言 `en-US.json` 里的计数文案用 `_one` / `_other`；组件里无手拼日期格式 |

**测试**：R1–R3 的验证手法是**故意制造违规，确认检查会红**——
`scripts/AGENTS.md` 的一贯要求，检查脚本自己要能被测。

### ○ U4.6.2 · 英文排版校对

| | |
|---|---|
| `goal` | 英文界面在真实布局下不溢出、不截断、不换行错位，等宽 ID 与英文正文混排可读 |
| `allowed_changes` | `frontend/src/i18n/locales/en-US.json`（仅调整文案长度）· `e2e/i18n-layout.spec.ts` · `frontend/tests/INDEX.md` |
| `forbidden_changes` | **不为了排版改组件样式**——排版放不下说明文案太长，改文案不改布局；不改 `zh-CN.json` |
| `stop_conditions` | 某处英文无论怎么改都放不下 —— 说明该处布局本身有问题，属于设计缺口，按 `frontend-guide.md` §16 登记后停下来；撞上 `i18n.md` §9 开放项 3（英文版等宽 ID 与中文版排版差异） |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 七个主页面在 `en-US` 下无横向溢出 | Playwright 逐页断言 `document.scrollingElement.scrollWidth <= clientWidth` |
| R2 | 按钮、tab、标签无文本截断 | 逐个断言元素的 `scrollWidth <= clientWidth` |
| R3 | 状态词在英文版里仍是等宽英文原值 | 断言字体族为 `--font-mono` 且文本等于 `zh-CN` 版的同一位置文本 |
| R4 | 语言切换即时生效且可持久 | 切换后断言页面文本变化；重载后断言仍是 `en-US` |
| R5 | 英文缺词条时回退中文而**不显示 key** | 临时删一个 `en-US` 词条，断言页面显示中文原文而不是 `settings.update.available` |
| R6 | 两版的按钮都用动词短语，不用空动词 | 断言 `en-US` 里不出现 `OK` / `Submit` 作为主按钮文案 |

---

## S4.7 · 启动时从检查点恢复 —— **已挪到 M1**

> `adr/0006` **Q40** 裁定：原 `U4.7.1` → **[`M1 的 U1.8.4`](M1-release-and-update.md)**。
>
> 理由：它依赖 M1 自己的 `/system/resume`（U1.8.3）与 Checkpoint 最小版（U1.7.3），
> 放在 M4 会让它在整个里程碑里一直是一条孤立的并行链——
> **依赖都在 M1，交付却排在 M4，中间那段时间它既做不了也验不了。**
>
> 本节保留是为了让照着旧编号来找的人能找到去处。M4 不再有 S4.7。

---

## M4 验收

**全部单元 `✓` 之外，还要满足：**

| # | 标准 | 怎么验 |
|---|---|---|
| A1 | `make check` 全绿，覆盖率达标 | CI |
| A2 | **令牌不进 Agent 上下文**：录下一整轮会话的全部字节与环境变量，不含令牌串 | `go test -tags=integration ./tests/integration/... -run TestGitHubD3` |
| A3 | **不存在 D3「始终允许」**：schema、API 参数、界面、词条四处都没有 | 四条断言各一个测试；`grep` 接进 CI |
| A4 | 全仓库不出现令牌字面量 | `grep -rn 'ghp_\|GITHUB_TOKEN\|GH_TOKEN' backend/internal/{acp,app,domain}` 为空，接进 CI |
| A5 | **不依赖本机 `gh`**：清空 `PATH` 后全套 GitHub 相关测试仍绿 | 本地跑一次 |
| A6 | `make check-i18n` 四项全绿，且四项**各自被故意制造的违规验证过会红** | 四条断言 |
| A7 | 英文版七个主页面无横向溢出、无文本截断 | `pnpm -C e2e test --grep 'English UI'` |
| A8 | 被设计缺口挡住的单元（U4.2.2）**没有绕过裁定偷偷实现** | `grep -rn '<svg' frontend/src/features/report` 为空，接进 CI |
| A9 | 被口径挡住的指标（U4.1.2）**没有摆出一个猜的数字** | 断言 `ErrMetricDefinitionUndefined` 仍是 OPEN-7 裁定前的唯一行为 |

**A2 与 A3 是 M4 真正的验收标准。** 报表和英文版做砸了是体验问题，
令牌漏进 Agent 上下文或者出现一个「始终允许」是安全问题——
后者一旦发生，用户是在事后从 GitHub 的操作记录里发现的。
