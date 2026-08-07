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
| `RuntimeSection.test.tsx` | `src/features/settings/` | ★ ACP Runtime 检测的**误导路径**：探测失败时说「检测不出来」而**绝不说「未安装」**（用户会照着去装已经装好的东西，装完还是不行），且这种情况下不给任何命令；整体检测失败显示错误与重试而不是空列表；没登录给的是登录命令不是安装命令；命令原样显示以便选中复制（R2 要求提示含具体命令而非「请检查配置」）|
| `persisted.test.ts` | `src/utils/` | 界面偏好持久化：存进去能读回来、布尔不丢类型；**坏值退回默认不抛**、**写失败静默忽略**——一个存储问题不该让整页白屏 |
| `use-update-flow.test.ts` | `src/features/settings/` | ★★ 一键更新流程的两条命脉：**先 prepare 再下载，顺序不可颠倒**；**prepare 返回 blocked 时绝不发起下载**（装下去会丢掉用户几十分钟的活）。另含 prepare 本身失败也不下载、浏览器形态不碰 prepare、检查失败不留过期状态、下载进度反馈 |
| `platform.test.ts` | `src/platform/` | 运行形态检测**靠壳注入的标记而非 User-Agent**（Tauri 的 WebView UA 与 Safari 极像，猜错就会在浏览器里显示一个点不动的更新按钮）；Web 降级给出真实的发布页地址而不是空函数；浏览器里调自更新明确抛错而不是静默无事发生 |

<!--
登记示例：

| `EventStream.test.tsx`   | `src/features/conversation/` | 13 类事件各自的渲染形态；过滤器开关；折叠态 |
| `use-event-stream.test.ts`| `src/features/conversation/`| SSE 断线重连、Last-Event-ID 续传、乱序事件按 seq 归位 |
| `UpdateCard.test.tsx`    | `src/features/settings/`     | 更新状态机：available → preparing → blocked 时展示被卡住的工作 |
-->
