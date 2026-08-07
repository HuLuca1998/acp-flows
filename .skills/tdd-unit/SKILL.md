---
name: tdd-unit
description: 开始任何一个开发单元时使用——写新功能、修缺陷、改行为之前都要走这套流程。触发场景：接到一个开发任务、准备动手写代码、用户说「实现 X」「修一下 Y」。本 skill 是本仓库测试先行铁律的执行流程，含五步顺序、验收标准到断言的映射、以及收尾时的抽取与索引检查。纯文档改动、纯配置改动不需要用它。
---

# 一个开发单元的执行流程

> 铁律 1：先写会失败的测试，跑一次确认它是红的，再写实现。
> 完整规范见 [`docs/testing-strategy.md`](../../docs/rules/testing-strategy.md)。

## 步骤

### ① 定位：读文档，不要猜

```
根 AGENTS.md  →  目标目录的 AGENTS.md  →  §7 表格里对应的专题文档
```

**不读文档就动手是本仓库最常见的失败模式。** 尤其是：

- 改领域逻辑 → `docs/domain-model.md`
- 改接口 → `api/AGENTS.md`（**先改 spec**）
- 改 UI → `design/Duet Spec.dc.html` 找对应条目
- 写代码 → `docs/coding-standards.md`（命名、文件归属）

### ② 写下边界

动手前明确写出来（写进 PR 描述，不是想想就算）：

```
允许改动：backend/internal/acp/session.go, session_test.go
禁止改动：任何公开接口、api/openapi.yaml、数据库 schema
停止条件：需要改 EngineEvent 公开枚举 / 需要新增第三方依赖 / 发现架构假设错误
```

### ③ 逐条验收标准 → 断言

先把验收标准列成清单，每条要能变成**一句可执行断言**。变不成的说明标准写得太模糊，回去要清楚。

```go
// R3: 连续取消两次只发送一次协议取消请求
func TestSessionCancel_R3_IsIdempotent(t *testing.T) { ... }
```

测试名带标准编号，这样「哪条标准还没证据」一眼可查。

**写测试前先查索引**（`backend/tests/INDEX.md` / `frontend/tests/INDEX.md`），
按**行为**搜。已经有测试覆盖这个行为 → 扩展它的用例表，**不要新开一个函数**。

### ④ 跑，确认是红的 ★

```bash
cd backend && go test ./internal/acp/... -run TestSessionCancel -count=1 -v
```

**这一步不能跳。** 一个从没红过的测试，你无法证明它在测东西。
如果它一上来就绿了 —— 回 ③，测试没测到东西。

把失败输出**留着**，PR 里要贴。

### ⑤ 最小实现

只写到测试变绿为止。

- 不顺手重构
- 不提前抽象
- 不改公开接口
- 不改 `forbidden_changes` 里的东西

**触发停止条件就停下来上报**，不要自行扩大范围继续做。

### ⑥ 再跑，全绿

```bash
make check          # 文档 + 索引 + lint + 全部测试
```

贴出实际输出。

### ⑦ 收尾：抽取、登记、债务体检

**债务体检**（见 [`docs/tech-debt.md`](../../docs/rules/tech-debt.md)）：

- [ ] 我这次是不是「照着现有的错误模式又写了一遍」？
- [ ] 挡路的烂代码铲了吗？铲平是**独立提交**吗？
- [ ] 路过看见的问题登记进债务表 + 开 issue 了吗？
- [ ] **我这次的改动会不会成为下一个人的屎山？**
      （你写的每一行都会被下一轮 AI 当作"现有模式"照抄）

**抽取与登记**：

- [ ] 新写的代码里有和 `util/INDEX.md` 已有工具重复的？→ 换成调用已有的
- [ ] 同一段逻辑写了两遍？→ 抽出来（规则：**第 2 个调用方出现时才抽**，别提前抽象）
- [ ] 新进 `util/` 的函数 → 登记 `INDEX.md` + 写测试
- [ ] 新测试 → 登记 `tests/INDEX.md`
- [ ] 新建了关键目录 → `make docs-scaffold DIR=...` 并把 TODO 填实
- [ ] 把业务语义的东西塞进 `util/` 了？→ 挪回 `model/` 或 `app/`

### ⑧ 提交

```
<type>(<scope>): <祈使语气的一句话>

<为什么这么改>

先红的测试: TestSessionCancel_R3_IsIdempotent
验证: cd backend && go test ./internal/acp/... -count=1 → ok, 退出码 0
```

格式见 [`docs/git-workflow.md`](../../docs/rules/git-workflow.md) §2。
「先红的测试」是必填项，CI 会校验。

## 自查：最好用的一条

**把实现里的关键一行删掉，这个测试会红吗？**

不会 → 测试是假的，回 ③ 重写。

## 禁止

- ✗ 先写实现后补测试
- ✗ 跳过「确认是红的」这一步
- ✗ 为了让 CI 变绿而删测试 / 加 `skip` / 放宽断言（**最严重的违规**）
- ✗ 顺手改任务边界外的文件
