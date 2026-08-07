# AI 工作手册：走哪条路、读哪份文档

> **这份文档解决两件事**：
> ① 你现在处于什么情形 → 该走哪条路
> ② 上下文有限 → 该读什么、不该读什么
>
> 本文自己也遵守规则：**整篇 < 2500 token，可以整篇读。**

---

## 1. 上下文预算：为什么不能全读

| 层 | 内容 | 体量 |
|---|---|---|
| **L0 常驻** | 根 `AGENTS.md` + `CLAUDE.md` | ~6k tok |
| **L1 阶段加载** | 1 个 skill + 1 份目录 `AGENTS.md` | ~1.5k tok |
| **L2 按需检索** | 34 份专题文档 | **~183k tok** |

**全部加起来 216k，超过 200k 上下文窗口。** 一次全读物理上做不到，
就算做到也没有余量干活了。

所以规则是硬的：

| 层 | 怎么读 |
|---|---|
| L0 | 自动在上下文里，**不用主动读** |
| L1 | 开工时读**一个** skill + **一份**就近的目录 `AGENTS.md` |
| L2 | **永远 grep 定位到章节，绝不整篇读** |

### L2 的正确读法

```bash
# ✗ 一次吃掉 27k token，占 13% 上下文
Read docs/acp-integration.md

# ✓ 先定位，再只读那一段
grep -n "两段式取消" docs/acp-integration.md     # → 行号
Read docs/acp-integration.md offset=420 limit=80
```

**超过 8k token 的文档顶部都有「读法」块**，写明章节索引与定位方式。
看到那个块就说明：不要整篇读。

---

## 2. 路由表：你在哪，走哪条路

| 你的情形 | 走这条 | 要读的（就这些，别多读） |
|---|---|---|
| **接到一个开发任务** | `tdd-unit` skill | 该 skill + 目标目录 `AGENTS.md` |
| 写 Go 测试 | `go-unit-test` skill | 该 skill |
| 写前端组件 / 测试 | `web-ui-test` skill | 该 skill + `frontend-guide.md` 的**相关章节** |
| 建表 / 写 GORM / 迁移 | — | `database.md`（grep 到你要的 §） |
| **改 ACP 协议层** | — | `acp-integration.md` + `acp-field-notes.md` 的**相关章节** |
| 配 Runtime / 权限档 | — | `runtime-config.md`（整篇 3k，可全读） |
| **要把应用跑起来** | `run-services` skill | 该 skill |
| **排查问题 / 加日志** | `debug` skill | 该 skill |
| 查数据库 | `db-operate` skill | 该 skill |
| **撞上烂代码** | — | `tech-debt.md` §2 的信号表 |
| 设计抽象 / 分包 | — | `design-principles.md`（grep 到你要的 §） |
| 开 issue | `create-issue` skill | 该 skill |
| 提交 / 开 PR / 合并 | `gh-commit` `gh-pr` `gh-pass` | `git-workflow.md` §2 §3 |
| 审查别人的改动 | `review-diff` skill | 该 skill |
| **发版** | — | `release-and-update.md` + `adr/0007` |
| **撞上「文档没说清楚」** | ↓ 见 §4 | `open-questions.md`（整篇 2k，可全读） |
| **接手上一轮没做完的活** | ↓ 见 §5 | — |

---

## 3. 一个任务的标准路径

```
① 定位     读 tdd-unit skill + 目标目录 AGENTS.md
              └ 需要细节时才 grep 专题文档的对应章节
② 边界     写下：允许改什么 / 禁止改什么 / 什么情况停下来
③ 写失败测试  跑一次，确认是红的 ★ 这步不能跳
④ 最小实现  只写到测试变绿
⑤ 验证     make check  +  make dev 起服务实测（用完 make dev-stop）
⑥ 自查     债务体检 · 抽取检查 · 索引登记
⑦ 提交     Conventional Commits，写明「先红的测试」
⑧ 合并后   make tidy
```

细节在 `tdd-unit` skill 里，**不要为了看这 8 步去读它**——这里就是全部。

---

## 4. 卡住了怎么办

按这个顺序，**不要跳过前面直接猜**：

1. **查 `open-questions.md`** —— 这个问题是不是已经有裁定了？
   （整篇 ~2k，可以全读）
2. **查 `adr/`** —— 是不是有 ADR 解释了为什么这么定？`ls docs/adr/` 看标题就够
3. **查前一个项目** —— `~/work/ai-workflows` 是同一作者的前作，形态相同。
   **它是 A 级证据**，「以前做过」的事优先看它，别从零推导
4. **能不能用实测代替推理？** —— `make probe`（ACP 行为）、写个探针、跑一次看看
5. **还是不确定 → 停下来问人。** 把问题加进 `open-questions.md` 的「仍需拍板」

**不许做的**：猜一个做法然后当成事实往下写。猜错的代价由后面所有轮次承担。

---

## 5. 接手上一轮没做完的活

```bash
git log --oneline -5          # 上一轮做到哪
git status                    # 有没有未提交的改动
make check                    # 现在是绿的吗
make dev-status               # 有没有留下的服务进程
gh pr list                    # 有没有开着的 PR
```

然后读 **最近一次提交的 message** —— 本仓库要求提交信息写明
「先红的测试」与「验证命令」，那就是上一轮的交接单。

**如果 `make check` 是红的**：先修绿再干新活。带着红继续做，
下一轮 AI 分不清哪些红是你造成的。

---

## 6. 交给另一个 AI

**传契约，不传聊天记录**（格式见 `ai-workflow.md` §2）：

```yaml
goal: 一句话
allowed_changes: [具体路径]
forbidden_changes: [具体禁止项]
acceptance_criteria: [每条能变成一句断言]
stop_conditions: [什么情况必须停]
context: [该读哪份文档的哪一节 ← 不要说「读 docs/」]
```

`context` 那行是**上下文预算的关键**：指到章节，不指到文件。

---

## 7. 文档体量的硬规矩

| 层 | 上限 | 超了怎么办 |
|---|---|---|
| L0（根 `AGENTS.md` + `CLAUDE.md`） | **6k tok** | 把细节下沉到专题文档，只留路由 |
| 目录 `AGENTS.md` | **1.5k tok** | 拆子目录，或把详细规格挪进 `docs/` |
| skill | **2k tok** | 同上 |
| L2 专题文档 | 无上限，但 **>8k 必须有「读法」块** | 加章节索引 |

```bash
make check-doc-budget    # CI 会跑
```

**写文档时问一句：这份会在什么时候被读？** 答不上来就说明它放错层了。
