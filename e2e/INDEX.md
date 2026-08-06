# 测试索引 · e2e

> Playwright 端到端。**每个 spec 文件一行。**
> 规则见 [`../docs/testing-strategy.md`](../docs/testing-strategy.md) §8。

## E2E 只测黄金路径

端到端测试慢且脆。**只覆盖跨越整个系统才能验证的行为**，
单个模块能测出来的东西一律下沉到单测或集成测试。

跑的是真实 `duetd`（临时数据目录 + Fake ACP Runtime），不是 mock 服务。

## 索引

| spec | 覆盖的路径 | 状态 |
|---|---|---|
| `golden-path.spec.ts` | **必须一直绿。** 当前是 M0 冒烟：应用能加载、窗口栏与主区渲染、后端可达且无 token 返回 401。<br>完整链路（创建项目 → 新建工作切 worktree → 计划冻结 → 单元契约冻结 → 执行 → 权限裁决 → 证据采集 → 独立审查 → 验收 → 检查点落盘）随 M2 各单元逐段追加，**不要另起 spec** | M0 冒烟 |

<!--
登记示例：

| `golden-path.spec.ts` | 创建项目 → 新建工作(worktree) → 计划冻结 → 单元执行 → 权限裁决 → 证据 → 验收 → 检查点 | 必须一直绿 |
| `update-flow.spec.ts` | 检测新版本 → prepare 暂停工作落检查点 → 模拟重启 → 从检查点恢复 | M1 |
-->
