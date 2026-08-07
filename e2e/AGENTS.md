# AGENTS.md · e2e

> **就近优先**：与根 [`AGENTS.md`](../AGENTS.md) 冲突时以本文件为准。

## 负责什么

Playwright 端到端测试。跑**真实 `duetd`**（临时数据目录 + Fake ACP Runtime）+ 真实前端。

## 不负责什么

**E2E 只测跨越整个系统才能验证的行为。**

单个模块能测出来的一律下沉：组件行为 → Vitest；后端逻辑 → Go 单测/集成测试。
E2E 慢且脆，塞进去的每一条用例都要付出长期维护成本。

## 黄金路径（必须一直绿）

```
创建项目 → 新建工作(切 worktree) → 计划冻结 → 单元执行 → 权限裁决
→ 证据生成 → 单元验收 → 检查点落盘
```

M1 之后追加：

```
检测到新版本 → prepare(暂停工作 + 落检查点) → 模拟重启 → 从检查点恢复
```

## 规则

- **定位一律用 `getByRole` / `getByText`**，禁止 CSS 选择器和 `data-testid` 兜底。
  设计会改，绑死结构的用例只会变成噪音。
- 每个 spec 独占一个临时数据目录，**测试间不共享状态**
- 绝不出网、绝不碰 `~/.acpflows`、绝不起真实 ACP Runtime
- 新增 spec → 登记 [`INDEX.md`](INDEX.md)，**登记前先搜有没有重复**

## 检查命令

```bash
pnpm -C e2e test
pnpm -C e2e test --headed --debug     # 看着它跑
make -C .. test-e2e
```

E2E 在 `main` 上跑，不阻塞 PR（它慢，且依赖构建产物）。

## 改这里之前必读

- [`../docs/rules/testing-strategy.md`](../docs/rules/testing-strategy.md) §6
- `web-ui-test` skill —— 三层测试怎么选、走查清单

## 本域特有的坑

- **等待要用断言，不用 `waitForTimeout`。** `await expect(x).toBeVisible()` 自带重试。
- **Fake Runtime 的脚本要放 `fixtures/`**，不要内联在 spec 里——多个 spec 会复用。
- **别在 E2E 里断言样式。** 视觉还原度靠人工走查（`web-ui-test` skill 第三层）。
- **失败时先看 duetd 日志**，不要只看浏览器截图——多数 E2E 失败根因在后端。
