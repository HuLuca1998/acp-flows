# AGENTS.md · Duet 工作总纲

> 本文件是本仓库对 AI 协作者的**单一真源**。Codex 直接读本文件；Claude Code 通过 `CLAUDE.md` 指向本文件。
> 关键目录下另有分域 `AGENTS.md`，**就近优先**。
>
> **本文是常驻上下文，刻意保持精简。** 细节全部下沉到 `docs/`，这里只留铁律与路由。
> **不知道该走哪条路、该读哪份文档 → 看 [`docs/ai-playbook.md`](docs/ai-playbook.md)（2.5k，可整篇读）。**

---

## 0. 这是什么

**Duet** —— ACP 多智能体协作编程 App。把 `claude-agent-acp` 与 `codex-acp` 当作可编排的
执行体，用「**计划 → 子计划 → 单元契约 → 尝试 → 证据 → 检查点**」的状态机驱动它们协作写代码，
人只在 D2/D3 决策点介入。

| | |
|---|---|
| 仓库 | `HuLuca1998/acp-flows`（公开） |
| 形态 | macOS 桌面应用（Tauri 壳 + Go sidecar），同一份代码可作为纯 Web 运行 |
| 数据目录 | `~/.acpflows`（用户真实）· `~/.duet-dev`（开发态，测试只许碰这个） |
| 设计真源 | `design/Duet Spec.dc.html`（规范）· `design/ACP Duet 1a.dc.html`（原型） |

```
Tauri v2 (Rust 壳) ──sidecar──▶ duetd (Go, HTTP+SSE) ──stdio──▶ ACP Runtime 子进程
        └── WebView ◀── React 18 + TS + Vite ──▶ HTTP/SSE
                  契约真源：api/openapi.yaml (OpenAPI 3.1)
```

技术选型已定，不要重新讨论 —— 见 [`docs/adr/`](docs/adr/)。

---

## 1. 前提：没有人类逐行审阅

本项目由 Claude 与 Codex 共同维护。默认**没有人类会逐行读你的 diff**。因此：

- **规则必须可被命令验证。** 写在文档里却没有检查手段的规则等于无效规则——补检查，别指望"注意一下"
- **每句结论都要有命令支撑。** 跑不出来就不要写
- **拿不准就停下来问人。** 猜错的代价由后面所有轮次承担
- **"测试全绿"不等于"验证过"。** 不断言真实行为的测试比没有测试更危险

---

## 2. 六条铁律

违反任何一条，改动一律回滚重做。

| # | 铁律 | 一句话 | 详见 |
|---|---|---|---|
| 1 | **测试先行** | 先写会失败的测试，**跑一次确认它是红的**，再写实现。提交说明必须回答「哪个测试先红了」 | [`testing-strategy.md`](docs/testing-strategy.md) |
| 2 | **契约先行** | 改接口的顺序永远是 `改 openapi.yaml → make gen → 改实现` | [`api/AGENTS.md`](api/AGENTS.md) |
| 3 | **设计合规** | UI 必须能在设计规范找到条目；找不到先加条目。**但设计稿管「形」，实测管「值」**（见下） | [`frontend-guide.md`](docs/frontend-guide.md) |
| 4 | **不扩大边界** | 只改允许改的范围。需要越界就**停下来上报** | — |
| 5 | **证据优先** | 结论必须附 diff / 测试输出 / 命令记录。转述不算 | — |
| 6 | **不碰用户真实数据** | 测试禁止读写 `~/.acpflows`、真实仓库、真实令牌。用 `t.TempDir()` / Fake Runtime | [`testing-strategy.md`](docs/testing-strategy.md) |

### 铁律 3 的边界：设计稿管「形」，实测管「值」

| 归设计稿管 | 归实测管 |
|---|---|
| 布局、组件形态、交互流程、文案 | **配置项 id 与取值域、协议字段、能力清单、档位名、枚举成员** |

**冲突时以实测为准**，同时回设计稿修正，不在代码里迁就它。
字段与取值见 [`runtime-config.md`](docs/runtime-config.md)，实测结论见
[`acp-field-notes.md`](docs/acp-field-notes.md) §7.1。

---

## 3. 每个任务的路径

```
① 定位 → ② 写下边界 → ③ 写失败测试（★确认红）→ ④ 最小实现
       → ⑤ 验证 → ⑥ 自查 → ⑦ 提交 → ⑧ make tidy
```

完整说明与路由表在 [`docs/ai-playbook.md`](docs/ai-playbook.md)。**开工前读它。**

**触发以下任一条立刻停，不要自行扩大范围**：需要改公开接口 / schema / OpenAPI 而未授权 ·
需要改边界外的文件 · 发现架构假设错误 · 需要新第三方依赖 · 撞上
[`open-questions.md`](docs/open-questions.md) 里的未决项。

### 撞上烂代码：不要在屎山上堆屎

最危险的一句话是「**跟现有代码保持一致**」——与错误的模式保持一致是在扩散错误。
**三选一，没有第四种**：挡路的先铲平（独立提交）· 路过的登记债务表 + 开 issue · 不确定就问。
**"照着抄一遍然后什么都不说"不在选项里。** 判定标准见 [`tech-debt.md`](docs/tech-debt.md)。

---

## 4. 目录地图

```
acp-flows/
├── api/openapi.yaml          ★ 前后端契约唯一真源
├── backend/                  Go · duetd
│   ├── cmd/                  唯一做依赖装配的地方
│   ├── internal/
│   │   ├── api/ app/         传输层 / 用例编排（app/port 定义全部抽象）
│   │   ├── domain/model/   ★ 领域模型，零 IO、零第三方库
│   │   ├── constant/ util/ ★ 常量 / 纯函数工具（各有 INDEX.md）
│   │   ├── acp/            ★ ACP 协议层 + Fake Runtime
│   │   ├── store/            GORM + SQLite（entity/ mapper/ migration/ 分包）
│   │   └── platform/         路径 · 时钟 · ID · 日志（不确定性的唯一入口）
│   └── tests/                跨包测试 + testutil（★ 与 internal/util 不是一回事）
├── frontend/src/             React（models/ constants/ utils/ i18n/ platform/）
├── shell/                    Tauri v2
├── design/                   ★ 只读，不要改
└── docs/                     规范 · adr/ · milestones/
```

**依赖方向单向，反向一律拒绝**（由 `depguard` 强制，见 `backend/.golangci.yml`）：

```
api → app → domain          基础设施实现 app/port 的接口，不被 app 依赖
```

**新建关键目录必须补 `AGENTS.md` + `CLAUDE.md`**，`make check-docs` 会拦。
规则见 [`doc-system.md`](docs/doc-system.md)。

---

## 5. Skill

`.skills/` 是真源，`.claude/skills` 与 `.agents/skills` 软链过去——Claude 与 Codex 用同一套。

| skill | 什么时候用 |
|---|---|
| `tdd-unit` | 开始任何一个开发单元（**最常用**） |
| `go-unit-test` · `web-ui-test` | 写 Go / 前端测试 |
| `run-services` | **起服务、页面测试** —— 唯一允许的起服务方式 |
| `debug` | **排查问题、查日志、加日志调试** |
| `db-operate` | 连数据库、查数据、改数据 |
| `review-diff` | 审查另一个 AI 的改动（实现方不得自审） |
| `create-issue` | 开 GitHub issue |

---

## 6. 命令速查

```bash
make check          # 提交前必跑：文档 + 索引 + 预算 + lint + 全部测试
make dev            # ★ 起前后端（端口写死 7777/5173，幂等）
make dev-stop       # ★ 用完必须停
make dev LOG=acp=trace   # 按域调日志级别
make logs-db        # 查落库的日志
make probe          # ACP 真机探针（零模型开销）
make tidy           # ★ 合并 PR 后：清理分支 / worktree / 远端残留引用
```

**不要裸跑 `go run` 或 `pnpm dev`** —— 见 `run-services` skill。
命令跑不通时**先修命令**，一个跑不动的检查等于没有检查。

---

## 7. 提交前自查

- [ ] 有一个测试是**先写的、先红过的**，我能说出它的名字
- [ ] 断言的是具体值，不是 `NotNil` / `NoError` 这类恒真式
- [ ] 改了接口 → `openapi.yaml` 已同步且 `make gen` 跑过
- [ ] 改了 UI → 能指出设计规范的对应条目；无硬编码 hex / 裸 px
- [ ] 新建关键目录 → 已补文档；新工具/新测试 → 已登记 INDEX
- [ ] 没碰 `~/.acpflows`；没新增未批准的依赖
- [ ] 这次的改动**会不会成为下一个人的屎山**
- [ ] `make check` 全绿，**输出已贴出**

---

## 8. 最高频的禁止项

完整清单见 [`docs/forbidden.md`](docs/forbidden.md)。这里只列最容易犯的：

- ✗ 先写实现后补测试；恒真断言；mock 喂 mock
- ✗ **为了让 CI 变绿而删测试 / 加 skip / 放宽断言** —— 等同伪造验收
- ✗ 复制粘贴复用；`switch` 分发同类实现（用注册表）
- ✗ **品牌判断离开 adapter** —— `grep -rn 'codex\|claude' internal/{app,domain,api}` 必须为空
- ✗ 领域模型挂 `gorm` 标签；`domain` import 任何第三方库
- ✗ 组件里硬编码用户可见文本（走 `t('key')`）；翻译状态词
- ✗ 测试碰 `~/.acpflows`、真实仓库、真实令牌
- ✗ 写死 hex / 裸 px / emoji 当图标

---

## 9. 术语

固定术语与状态词见 [`docs/glossary.md`](docs/glossary.md)。

**状态词一律英文、不翻译、等宽显示**，共 11 个：
`initializing` `initializing_failed` `clarifying` `planning` `ready` `executing`
`reviewing_unit` `waiting_user` `paused` `completed` `failed`

**语气：工程化、精准、可核对。**
✓「单元 unit-012 契约 v3 已冻结」 ✗「我已经准备好开始写这块了 🎉」

---

## 10. 当前阶段

**M0 进行中**，脚手架四件套已跑通。剩余关键路径是 **Fake ACP Runtime**（S0.4）。

进度见 [`docs/roadmap.md`](docs/roadmap.md)，单元级拆解见 [`docs/milestones/`](docs/milestones/)。
