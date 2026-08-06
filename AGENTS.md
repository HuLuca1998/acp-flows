# AGENTS.md · Duet 工作总纲

> 本文件是本仓库对 AI 协作者的**单一真源**。Codex 直接读本文件；Claude Code 通过 `CLAUDE.md` 指向本文件。
> 关键目录下另有分域 `AGENTS.md`，**就近优先**：在 `backend/` 下工作时，`backend/AGENTS.md` 覆盖本文件的同名条款。

---

## 0. 这是什么

**Duet** 是一个 **ACP 多智能体协作编程 App**。它把 `claude-agent-acp` 与 `codex-acp` 两个
ACP Runtime 当作可编排的执行体，用「**计划 → 子计划 → 单元契约 → 尝试 → 证据 → 检查点**」
的状态机驱动它们协作写代码，人只在 D2/D3 决策点介入。

| | |
|---|---|
| 仓库 | `HuLuca1998/acp-flows`（公开） |
| 产品名 | Duet |
| 形态 | macOS 桌面应用（Tauri 壳 + Go sidecar），同一份代码可作为纯 Web 运行 |
| 数据目录 | `~/.acpflows`（全局）· `<project>/.acpflows`（项目内） |
| 设计真源 | `design/Duet Spec.dc.html`（规范）· `design/ACP Duet 1a.dc.html`（界面原型） |

技术栈已定，不要重新讨论（决策记录见 `docs/adr/`）：

```
Tauri v2 (Rust 壳)  ──sidecar──▶  duetd (Go, HTTP API + SSE)  ──stdio──▶  ACP Runtime 子进程
        │                              │
        └── WebView ◀── React 18 + TypeScript + Vite ──▶ HTTP/SSE
                            契约真源：api/openapi.yaml (OpenAPI 3.1)
```

---

## 1. 本仓库的特殊前提：没有人类逐行审阅

本项目由 Claude 与 Codex 共同维护。默认**没有人类会逐行读你的 diff**。因此：

- **规则必须可被命令验证。** 写在文档里却没有检查手段的规则等于无效规则——发现了就补测试或 lint，别指望"注意一下"。
- **每句结论都要有命令支撑。** 你在提交说明或回复里写的结论，必须能指出是哪条命令的哪段输出支撑它。跑不出来就不要写。
- **拿不准就停下来问人。** 不要猜着实现。猜错的代价由后面所有轮次承担。
- **"测试全绿"不等于"验证过"。** 一个不断言真实行为的测试比没有测试更危险，因为它制造了虚假的安全感。

---

## 2. 六条铁律

违反其中任何一条，改动一律回滚重做。

### 铁律 1 · 测试先行

先写**会失败的测试**，跑一次**确认它是红的**，再写实现。
提交说明必须能回答一个问题：**"哪个测试先红了？"**
详见 [`docs/testing-strategy.md`](docs/testing-strategy.md)。

### 铁律 2 · 契约先行

`api/openapi.yaml` 是前后端接口的唯一真源。改接口的顺序**永远**是：

```
改 openapi.yaml  →  跑代码生成  →  改 Go 实现 / TS 调用
```

禁止先改 handler 或前端调用、再回头补 spec。前后端靠 spec 并行开发，spec 落后于代码就等于并行开发被破坏。

### 铁律 3 · 设计合规

所有 UI 必须能在 `design/Duet Spec.dc.html` 里找到对应条目。
**找不到 → 先在设计规范里新增条目，再实现。** 禁止临时发明样式。
颜色 / 间距 / 圆角 / 字号一律走 `var(--token)`；硬编码 hex 与裸 px 会被 lint 拦下。

### 铁律 4 · 不扩大边界

只改任务描述里允许改的文件与范围。需要越界时**停下来上报**，不要"顺手改一下"。

> 这条同时是产品自身的核心机制（`UnitContract.forbidden_changes`）。
> 我们要求 Duet 管住它调度的 Agent，就得先自己做到。

### 铁律 5 · 证据优先

结论必须附证据入口：diff / 测试输出 / 命令记录。

- ✗ 「我跑了测试，通过了」
- ✓ 「`go test ./internal/domain/... -run TestPlanVersionAppendOnly` → `ok ... 0.03s`，退出码 0」

### 铁律 6 · 不碰用户真实数据

开发与测试**禁止**读写 `~/.acpflows`、用户真实 git 仓库、真实 GitHub 令牌、真实 ACP Runtime 账号。
一律用 `t.TempDir()` / 临时 SQLite / Fake ACP Runtime / 测试夹具仓库。
CI 上这条由测试守卫强制执行，本地绕过它等于给自己埋雷。

---

## 3. 每个任务的 7 步

> 这套流程本身就是 Duet 要产品化的东西。我们先在自己身上跑通它。

1. **定位** —— 读任务涉及目录的 `AGENTS.md`，以及 §7 表格里对应的文档。
   *不读文档就动手是本仓库最常见的失败模式。*
2. **确认边界** —— 写下：这次允许改哪些文件 / 禁止改什么 / 什么情况必须停下来。
3. **写失败测试** —— 按验收标准逐条写断言，跑一次，**确认是红的**。红不了说明测试没测到东西。
4. **最小实现** —— 只写到测试变绿为止。不顺手重构、不提前抽象、不改公开接口。
5. **验证** —— 跑本域完整检查命令（§6），**贴出实际输出**。
6. **自查** —— 过 §5 清单。
7. **提交** —— Conventional Commits，见 [`docs/git-workflow.md`](docs/git-workflow.md)。

### 撞上历史遗留问题时：不要在屎山上堆屎

最危险的一句话是「**跟现有代码保持一致**」——
与错误的模式保持一致，是在扩散错误。你写的每一行都会被下一轮 AI 当作"现有模式"照抄。

看见 [`docs/tech-debt.md`](docs/tech-debt.md) §2 里能指认的信号时，**三选一，没有第四种**：

| 处境 | 动作 |
|---|---|
| 挡路：不铲就得复制粘贴 / 就得违反分层 / 就写不出真测试 | **先铲平**：补测试锁住现状 → 重构 → **独立提交** → 再做原本的事 |
| 路过：和本次改动没有调用关系 | **登记进债务表 + 开 issue**，按原样继续，并在 PR 里说明 |
| 不确定算不算问题 | **停下来问**，不要自己拍板扩大重构 |

**"照着抄一遍然后什么都不说"不在选项里。**

铲平也有边界：只铲挡路的那一块，不顺手重写整个包；
要动公开接口 / schema / OpenAPI → 停下来，判定为 `contract_revision`。
**铲不动就别铲一半**——新旧两套模式并存比原来更烂。

完整判定标准与债务登记表见 [`docs/tech-debt.md`](docs/tech-debt.md)。

**触发停止条件时立刻停**，不要自行扩大范围继续做：

- 需要修改公开接口 / 数据库 schema / OpenAPI 契约，而任务没授权
- 需要改动任务边界之外的文件
- 发现架构假设错误
- 需要引入新的第三方依赖

---

## 4. 目录地图 —— 改什么去哪里

```
acp-flows/
├── AGENTS.md  CLAUDE.md        ← 你在读的东西
├── api/                        ★ OpenAPI 3.1 契约（前后端唯一真源）
├── backend/                    Go：duetd 后端进程
│   ├── cmd/duetd/              进程入口
│   └── internal/
│       ├── api/                HTTP 传输层（实现 openapi 生成的接口）
│       ├── app/                用例编排 / 事务边界
│       ├── domain/
│       │   ├── model/        ★ 领域模型（一个聚合一个文件，充血）
│       │   └── policy/         跨模型的业务策略
│       ├── constant/         ★ 常量与枚举（按主题分文件）
│       ├── util/             ★ 工具函数（纯函数，准入规则见代码规范）
│       ├── acp/              ★ ACP 协议层 + Fake Runtime（M0 重点）
│       ├── store/              SQLite 持久化 + migrations
│       ├── fsstore/            .acpflows 下的 md 文件读写
│       ├── gitx/               worktree / diff / commit / branch
│       ├── ghx/                GitHub 令牌与远端操作
│       ├── eventbus/           领域事件 → SSE 广播
│       └── platform/           路径、keychain、进程探测
├── frontend/                   React 18 + TS + Vite
│   └── src/
│       ├── app/ design/ ui/ features/ api/ platform/ store/
│       ├── models/           ★ 领域类型
│       ├── constants/        ★ 常量与枚举
│       └── utils/            ★ 工具函数
├── shell/                      Tauri v2（Rust）：窗口、原生能力、sidecar 守护、自动更新
├── e2e/                        Playwright 端到端（跑真实 duetd + Fake Runtime）
├── design/                     ★ 设计稿与设计系统（只读参考，不要改）
├── docs/                       架构与规范文档
│   └── adr/                    架构决策记录
├── scripts/                    构建、代码生成、本地起服务
└── .github/workflows/          CI 与自动发版
```

**依赖方向是单向的，反向依赖一律拒绝：**

```
api  →  app  →  domain
         │
         └──▶ (port 接口) ◀── store / acp / gitx / ghx / fsstore
```

`domain` 不 import 任何基础设施包，也不 import 标准库之外的 IO。这是整套测试策略的地基。

---

## 4.1 文档自生长：新目录必须自带 AGENTS.md + CLAUDE.md

目录会越来越多。**每个"关键目录"都必须有成对的 `AGENTS.md` + `CLAUDE.md`；
缺失时，谁在这个目录里干活谁补上——这是任务的一部分，不是可选的收尾工作。**

### 什么算"关键目录"（可机器判定）

满足**任一**条件即为关键目录：

1. 顶层目录（`backend/` `frontend/` `shell/` `api/` `e2e/` `design/` `docs/` `scripts/`）
2. `backend/internal/` 下的任一一级子包
3. `frontend/src/` 下的任一一级子目录，以及 `frontend/src/features/` 下的任一一级子目录
4. 该目录**直接包含 ≥ 3 个源文件**（`.go` `.ts` `.tsx` `.rs`），无论层级多深

### 什么时候补

在**第一次**往一个关键目录里写文件时就补，和业务改动放同一个提交。
不要攒着以后统一补——攒着的结果永远是不补。

### 怎么补

```bash
make docs-scaffold DIR=backend/internal/store     # 从模板生成两份骨架
```

模板在 [`docs/templates/`](docs/templates/)。生成后**必须填实**下列内容，
留着占位符不算完成：

- **这个目录负责什么 / 不负责什么**（边界比职责更重要）
- **依赖方向**：允许 import 谁，禁止 import 谁
- **本域的检查命令**（怎么测、怎么 lint）
- **本域特有的坑**：踩过的、容易犯的错
- **改这里之前必读的文档**

`CLAUDE.md` 保持三行薄壳，指向同目录 `AGENTS.md`，**不要复制内容**——
两份内容一旦重复就必然漂移。

### 强制手段

```bash
make check-docs      # 列出所有缺失 AGENTS.md / CLAUDE.md 的关键目录
```

CI 在每个 PR 上跑这条。缺失即**红**，PR 合不进去。

### 内容过期也是缺陷

如果你发现某个目录的 `AGENTS.md` 与实际代码已经对不上（描述了不存在的文件、
写着已经废弃的规则），**当场修正**，不要绕开。文档滞后按缺陷处理，与代码 bug 同等对待。

---

## 4.1.1 文档演进：发现问题就地改

**这套文档是活的，不是一次性写完就冻结的。**
第一版必然有想不到的地方——你在干活时撞上的每一个坑，都是文档的缺口。

### 触发条件 —— 撞上任一条就要动文档

| 你遇到了 | 动作 |
|---|---|
| 文档描述与实际代码对不上 | **当场修文档**，和业务改动同一个提交 |
| 文档没说清楚，你猜了一个做法 | 把你的判断写进去，**并标注这是新增的裁定** |
| 踩了坑（看起来对但其实错） | 写进该目录 `AGENTS.md` 的「本域特有的坑」 |
| 被审查打回 | 打回理由如果是通用的 → 补进规范或禁止清单 |
| 两份文档说法不一致 | 立刻合并到其中一处，另一处改成链接 |
| 规则没有强制手段，你发现有人（包括你）违反了 | **补检查脚本**，见 `docs/adr/0003` |
| 设计稿里找不到对应条目 | 登记进 `docs/frontend-guide.md` 的设计缺口表 |

### 什么时候当场改，什么时候开 issue

| | |
|---|---|
| **当场改** | 事实性错误、过期描述、缺一条坑、缺一条禁止项、措辞含糊 |
| **开 issue** | 需要改变架构决策、需要人拍板的取舍、要动 ADR |

**当场改的门槛要低。** 犹豫「这算不算越界」时——改文档不算越界，
铁律 4 管的是代码写入边界，不管文档修正。

### 怎么改

- 加规则的同时问：**这条怎么被 CI 拦？** 拦不了就改成能拦的形式，
  或明确标注「靠自觉，可能失效」
- **一件事只在一处写。** 需要交叉引用用链接，不要复制
- 数值要具体（`400 行`，不是「不要太长」）；不写「应该」「尽量」「建议」
- 架构决策变了 → **写新 ADR**，旧 ADR 开头加 `> 已被 adr/00XX 取代`，不改旧正文

### 每轮收尾必做

- [ ] 这次踩的坑写进对应 `AGENTS.md` 了吗？
- [ ] 这次的改动让哪份文档过期了？改了吗？
- [ ] 有没有发现「规则存在但没人执行」的？补检查了吗？

**文档滞后按缺陷处理，与代码 bug 同等对待。** 攒着以后统一修的结果永远是不修。

---

## 4.2 项目级 Skill

固定流程写成了 skill，放在 [`.skills/`](.skills/)（真源），
`.claude/skills` 与 `.agents/skills` 软链接过去——**Claude 和 Codex 用同一套**。

| skill | 什么时候用 |
|---|---|
| `tdd-unit` | 开始任何一个开发单元时（**最常用**） |
| `go-unit-test` | 写或改任何 Go 测试 |
| `web-ui-test` | 前端测试：组件行为 / E2E / 真实浏览器模拟用户走查 |
| `review-diff` | 审查另一个 AI 的改动 |
| `create-issue` | 开 GitHub issue |

写新 skill 的规则见 [`.skills/README.md`](.skills/README.md)。
**判断标准：反复执行、步骤固定、做错有代价的流程写成 skill；
始终生效的约束写进 AGENTS.md。**

---

## 5. 提交前自查清单

逐条确认，不要跳：

- [ ] 有一个测试是**先写的、先红过的**，我能说出它的名字
- [ ] 改了接口 → `api/openapi.yaml` 已同步，且代码生成跑过
- [ ] 改了 UI → 能在 `design/Duet Spec.dc.html` 指出对应条目；没有硬编码 hex / 裸 px
- [ ] 没有超出任务允许的写入范围（`git status` 里没有意料之外的文件）
- [ ] 新建了关键目录 → 已补上 `AGENTS.md` + `CLAUDE.md` 且内容填实（`make check-docs` 绿）
- [ ] 本次改动让某份 `AGENTS.md` 过期了 → 已同步修正
- [ ] 本域检查命令全绿，**输出已贴出**
- [ ] 没有读写 `~/.acpflows` 或用户真实仓库
- [ ] 没有新增未经批准的第三方依赖
- [ ] 提交信息符合 Conventional Commits，并说明"哪个测试先红了"
- [ ] 中文文案符合术语表（§8），状态词保持英文原值
- [ ] 命名与文件归属符合 [`docs/coding-standards.md`](docs/coding-standards.md)：
      模型在 `model/`、常量在 `constant(s)/`、工具在 `util(s)/`，没有 `helper.go` 这类垃圾桶文件

---

## 6. 命令速查

```bash
# 全量检查（提交前必跑）
make check

# 后端
make -C backend test          # go test ./... -race
make -C backend lint          # golangci-lint
make -C backend cover         # 覆盖率 + 门槛校验

# 前端
pnpm -C frontend test         # vitest
pnpm -C frontend lint         # eslint + stylelint（含设计合规规则）
pnpm -C frontend typecheck    # tsc --noEmit

# 契约代码生成（改完 openapi.yaml 必跑）
make gen

# 文档完整性：关键目录是否都有 AGENTS.md + CLAUDE.md
make check-docs
make docs-scaffold DIR=<path>   # 从模板生成缺失的两份文档

# 本地起服务（Web 模式，最方便的调试形态）
make dev-web                  # duetd + vite，浏览器打开 http://localhost:5173

# 端到端
pnpm -C e2e test
```

> 命令若跑不通，**先修命令再干活**——一个跑不动的检查等于没有检查。

---

## 7. 该读哪份文档

| 你要做的事 | 必读 |
|---|---|
| **写任何代码**（命名、文件组织、目录归属） | [`docs/coding-standards.md`](docs/coding-standards.md) |
| **设计抽象**（接口放哪、包怎么分、怎么复用而不复制） | [`docs/design-principles.md`](docs/design-principles.md) |
| **撞上烂代码**（铲还是不铲、怎么铲、债务登记） | [`docs/tech-debt.md`](docs/tech-debt.md) |
| 理解整体架构、进程模型、分层 | [`docs/architecture.md`](docs/architecture.md) |
| 改领域模型、状态机、业务规则 | [`docs/domain-model.md`](docs/domain-model.md) |
| 写任何测试 | [`docs/testing-strategy.md`](docs/testing-strategy.md) |
| 做 ACP 协议层 / Runtime 适配 | [`docs/acp-integration.md`](docs/acp-integration.md)（规格）+ [`docs/acp-field-notes.md`](docs/acp-field-notes.md)（**实测与前一个项目踩过的 10 个坑**） |
| 做发版、CI、客户端自动更新 | [`docs/release-and-update.md`](docs/release-and-update.md) |
| 开分支、写提交、开 PR、合并、worktree | [`docs/git-workflow.md`](docs/git-workflow.md) |
| 改 CI、加检查、CI 变慢了 | [`docs/ci.md`](docs/ci.md) |
| **撞上「设计稿没说 / 两处冲突 / 需要人取舍」** | [`docs/open-questions.md`](docs/open-questions.md) —— **停下来，别猜** |
| 写前端组件、还原设计稿 | [`docs/frontend-guide.md`](docs/frontend-guide.md) |
| **写任何用户可见文案**（中英双语） | [`docs/i18n.md`](docs/i18n.md) |
| Claude 与 Codex 怎么分工协作 | [`docs/ai-workflow.md`](docs/ai-workflow.md) |
| 想知道某个技术选型为什么这么定 | [`docs/adr/`](docs/adr/) |
| 排里程碑、判断某功能是不是现在做 | [`docs/roadmap.md`](docs/roadmap.md) |

---

## 8. 术语表

界面语言是简体中文，**标识符与状态词保留英文原值并等宽显示**。

| 中文 | 英文标识 | 说明 |
|---|---|---|
| 工作 | `Work` | 一次完整的开发任务，独占一个 git worktree |
| 计划 | `Plan` / `PlanVersion` | 只增不改，重规划产生新版本 |
| 子计划 | `Subplan` | 计划下的阶段，构成有向无环图 |
| 单元 | `Unit` | 最小可执行工作单位 |
| 单元契约 | `UnitContract` | 冻结的交接规格，含写入边界与验收标准 |
| 尝试 | `Attempt` | 对一个单元的一次执行 |
| 证据 | `Evidence` | diff / 测试输出 / 命令记录 / 审查意见 |
| 检查点 | `Checkpoint` | 可恢复点，绑定 commit hash |
| 决策等级 | `D0`–`D3` | D0 自动 · D1 记录 · D2 需确认 · D3 逐次授权 |
| 主工作树 | `worktree` | git worktree |
| 会话模式 | `set_mode` | ACP `session/set_mode` |
| 权限裁决 | `request_permission` | ACP `session/request_permission` |
| 角色 | `Role` | 8 个预置角色，先定义再绑定 Runtime |
| 运行时 | `Runtime` | 一个 ACP adapter 进程（claude / codex） |

**状态词一律英文、不翻译、等宽显示：**
`clarifying` · `planning` · `ready` · `executing` · `reviewing_unit` · `waiting_user` · `paused` · `completed` · `failed`

**语气：工程化、精准、可核对。**

- ✓ 「单元 unit-012 契约 v3 已冻结」
- ✗ 「我已经准备好开始写这块了 🎉」

按钮用动词短语（`创建 worktree 并开始` / `请求 push 授权`），不用「确定」「提交」这类空动词；
破坏性动作要在按钮上写清后果（`丢弃 2:14 工作`）。

---

## 9. 禁止清单

出现即视为不合规，改动回滚。

**设计与组织**（完整清单见 `docs/design-principles.md` §8）

- ✗ **品牌判断离开 adapter** —— `grep -rn 'codex\|claude' backend/internal/{app,domain,api}` 必须是空的。
  claude 与 codex 的差异一律在 `acp/adapter/` 内部填平，上层只表达意图与查询能力
- ✗ 接口方法名照抄协议方法名（`SetMode` → 应该是 `RestrictPermissions`）—— 协议一变全仓库跟着改
- ✗ 复制粘贴复用 —— 想复制时停下来，用嵌入 / 模板方法 / 装饰器
- ✗ `switch` 分发同类实现 —— 用注册表，加第 3 个实现不该改老代码
- ✗ 包内平铺超过 10 个文件；按技术种类切文件（`models.go` `utils.go` `handlers.go`）
- ✗ 接口和它唯一的实现放同一个包（接口定义在**使用方**）
- ✗ 上帝接口；只有一个实现就抽接口（过早抽象）
- ✗ 单文件 > 400 行、单函数 > 60 行

**工程**

- ✗ 先写实现后补测试
- ✗ 断言恒真、mock 喂 mock、只运行不断言的测试
- ✗ 改了接口不改 `api/openapi.yaml`
- ✗ `domain` 包 import 任何基础设施包或做 IO
- ✗ 测试读写 `~/.acpflows`、用户真实仓库或真实令牌
- ✗ 未经批准新增第三方依赖
- ✗ 把 Agent 的转述当证据（证据必须由应用直接采集）
- ✗ 提交信息里写没有命令输出支撑的结论

**国际化**（完整清单见 `docs/i18n.md` §8）

- ✗ 组件里硬编码用户可见文本 —— 一律 `t('key')`
- ✗ 翻译状态词 / 标识符 / 命令 / 路径 / ID —— 它们在中英两版里都保持英文等宽
- ✗ 只更新 `zh-CN.json` 不更新 `en-US.json` —— 两个语言文件永远同进同退
- ✗ 后端返回中文文案给界面展示 —— 后端只返回错误码，前端负责翻译

**设计**（完整清单见 `design/Duet Spec.dc.html` 第 10 节）

- ✗ 写死 hex 或新造颜色（含语义色，语义色只有 `--color-pass` / `--color-fail` 两个）
- ✗ emoji、Unicode 几何符号当图标，或自绘 SVG 图标（图标只从 Phosphor regular 取）
- ✗ 实心填充的主按钮、彩色渐变按钮
- ✗ 悬浮层影响正文布局，或被父级 `overflow:hidden` 裁切
- ✗ 同类选择器在不同页面位置不一致
- ✗ 纯图标按钮没有中文 tooltip
- ✗ 设计 ACP 不支持的设置项（如按角色设模型——模型不在协议里）
- ✗ 用弹窗打断执行中的单元来展示非阻塞信息
- ✗ 结论不带证据入口
- ✗ 自动把聊天原文或一次成功经验写成长期记忆

---

## 10. 当前阶段

见 [`docs/roadmap.md`](docs/roadmap.md)。当前处于 **M0 · ACP 协议层**，
下一个是 **M1 · 发布与自动更新**（用户指定优先）。

**代码尚未开始编写。** 在 M0 任务被明确指派之前，不要自行开始实现业务代码。
