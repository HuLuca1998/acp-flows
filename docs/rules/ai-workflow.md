# Claude × Codex 协作规范

> 本仓库由两个 AI 共同维护，**默认没有人类逐行审阅**。
> 这份文档定义谁做什么、怎么交接、冲突时听谁的。
>
> 有意思的地方：**我们在自己身上跑的这套流程，正是 Duet 要产品化的东西。**
> 所以流程里的每个概念（契约、边界、证据、审查四结论）都和产品的领域模型一一对应。
> 自己都跑不通的流程，不该做成产品卖给别人。

---

## 1. 分工

不按"谁更强"分，按**角色隔离**分——同一个 AI 不能既当运动员又当裁判。

| 阶段 | 主责 | 说明 |
|---|---|---|
| 需求澄清、方案设计 | **Claude** | 拆任务、定契约、写验收标准 |
| 契约冻结（任务描述） | **Claude** | 写死边界与停止条件，见 §2 |
| 实现 | **Codex** 或 Claude | 谁实现都行，但实现方定下来后不能兼任审查 |
| 审查 | **另一方** | 实现方不得审查自己的改动 |
| 决策拍板 | **人** | D2/D3 等级的事一律等人 |

**核心约束：实现方 ≠ 审查方。** 见 [`git-workflow.md`](git-workflow.md) §3。

| 实现 | 审查 |
|---|---|
| Claude | Codex |
| Codex | Claude |
| 人 | 任一 AI |

---

## 2. 交接格式：任务契约

跨 AI 交接时，**不要传"聊天记录"或"大概意思"，传一份结构化契约**。
格式照抄产品的 `UnitContract`：

```yaml
goal: "用户能取消正在运行的 Agent turn，并保留现场证据"

allowed_changes:
  - backend/internal/acp/session.go
  - backend/internal/acp/session_test.go

forbidden_changes:
  - 不改 api/openapi.yaml
  - 不改任何公开接口签名
  - 不新增第三方依赖

test_strategy:
  - 状态机单元测试
  - 对着 Fake Runtime 的延迟响应集成测试

acceptance_criteria:
  - R1: 连续取消两次只发送一次协议取消请求
  - R2: 取消后 diff 与最后事件游标可读取
  - R3: reviewing_unit 状态下取消被拒绝

stop_conditions:
  - 需要扩大写入范围
  - 发现公开接口必须变化
  - 发现架构假设错误

context:
  - docs/acp-integration.md §两段式取消
  - backend/internal/acp/AGENTS.md
```

**每条 `acceptance_criteria` 都要能变成一句可执行断言**——
实现方会照着它写测试，写不出断言说明标准没定清楚，退回重写。

---

## 3. Claude 怎么调 Codex

用 `codex-collab` skill。三种模式，**不要混用**：

| 目的 | 模式 | 权限 |
|---|---|---|
| 让 Codex 审查改动 | 只读审查 | **只读**，不许改文件 |
| 问 Codex 意见（选型、命名、方案） | 多轮对话 | 只读 |
| 让 Codex 实现一个任务 | 执行模式 | 可写，**但必须先给它 §2 的契约** |

### 让 Codex 实现时

1. 先把契约（§2）写好，**边界和停止条件写死**
2. 明确告诉它：**测试先行**，先写失败测试再实现
3. 让它返回执行报告：改了哪些文件、跑了哪些命令、每条验收标准对应哪个测试
4. **Claude 拿到结果后必须 `git diff` 亲自过一遍**，不能只看它的报告
   —— 铁律 5：Agent 的转述不是证据

### 让 Codex 审查时

给它：PR diff + 验收标准 + 相关 `AGENTS.md`。要求返回：

- 四选一结论：`accepted` / `implementation_fix` / `contract_revision` / `global_replan`
- **逐条依据**，每条指向具体文件行号或命令输出

审查流程见 `review-diff` skill。

---

## 4. Codex 怎么找 Claude

Codex 读同一套 `AGENTS.md`（`.agents/skills` 与 `.claude/skills` 软链到同一个 `.skills/`）。

需要 Claude 介入的情况：

- 触发了 `stop_conditions` 里的任何一条
- 需要改契约、改 OpenAPI、改设计规范
- 审查发现 `contract_revision` 或 `global_replan`

**处理方式：停下来，写清楚为什么，不要自行扩大范围继续做。**

---

## 5. 冲突裁决

| 冲突 | 听谁的 |
|---|---|
| 两个 AI 对实现方案有分歧 | **不投票。** 各写一份方案与代价，交给人选 |
| 文档之间冲突 | 就近优先：目录 `AGENTS.md` > 根 `AGENTS.md` |
| 文档与设计稿冲突 | **设计稿为准**（`design/Duet Spec.dc.html`） |
| 文档与代码冲突 | **文档为准**，代码是缺陷，去修代码 |
| 契约与实现冲突 | **契约为准**。契约本身有问题 → `contract_revision`，退回重定，不许边做边改 |
| OpenAPI 与实现冲突 | **spec 为准**（铁律 2） |

---

## 6. 上下文注入规则

给对方 AI 的上下文要**够用且最小**。

**必给：**

- 目标目录的 `AGENTS.md`
- 任务契约（§2）
- 相关专题文档的**具体章节**（不是"读一下 docs/"）

**不要给：**

- 整个仓库的文件列表
- 完整的设计稿 HTML（250KB，用 `docs/spec/frontend-guide.md` 代替）
- 上一轮的聊天记录（传契约，不传对话）

**绝不给：**

- GitHub 令牌、任何凭据
- 用户真实项目的数据

---

## 7. 并行工作

两个 AI 同时干活时**必须用 worktree 隔离**，见 [`git-workflow.md`](git-workflow.md) §4。

```bash
make wt NAME=feat/acp-session-cancel
make wt NAME=feat/ui-event-stream
```

**禁止跨 worktree 改文件。** 这是并行安全的唯一保证。

任务切分要保证**文件不重叠**。切不开就别并行——串行比合并冲突便宜。

---

## 8. 每轮结束时的知识沉淀

发现了「看起来对但其实错」的写法、踩了坑、被审查打回 →
**把它写进对应目录的 `AGENTS.md` 的「本域特有的坑」一节。**

这一节的价值随时间增长，是防止下一轮 AI 重犯的唯一手段。

同理：

- 重复造了轮子 → 说明 `util/INDEX.md` 没被查，检查索引描述是否可检索
- 写了重复的测试 → 说明 `tests/INDEX.md` 的「覆盖行为」列写得太像函数名

---

## 9. 禁止清单

- ✗ 实现方审查自己的改动
- ✗ 用聊天记录当交接材料（用 §2 的契约）
- ✗ 把 Codex 的执行报告当证据，不亲自 `git diff`
- ✗ 触发停止条件后自行扩大范围继续做
- ✗ 两个 AI 投票决定方案（交给人）
- ✗ 并行时不用 worktree
- ✗ 把凭据或用户真实数据传给另一个 AI
- ✗ 踩了坑不写进 `AGENTS.md`
