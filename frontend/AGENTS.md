# AGENTS.md · frontend

> **就近优先**：与根 [`AGENTS.md`](../AGENTS.md) 冲突时以本文件为准。

## 负责什么

Duet 的全部界面。React 18 + TypeScript + Vite，**一份代码同时跑在 Tauri WebView 和浏览器里**。

## 不负责什么

- **不写业务逻辑。** 状态机、编排、持久化全在 `duetd`。
  前端只做：展示、收集输入、调 API、渲染事件流。
- **不直接 `fetch`。** 一律走 `src/api/` 的生成客户端 + TanStack Query。
- **不认识 Tauri。** 除 `src/platform/` 外，任何文件 import `@tauri-apps/*` 都会被 lint 拦下。

## 目录

```
src/
├── app/          路由、providers、布局骨架（窗口栏 42 / 左栏 252 / 主区 / 右栏 300）
├── design/       tokens.css（Nocturne 令牌）+ duet.css（产品层令牌）
├── ui/           设计系统原语：Button Tag Card Drawer Dialog Dropdown …
├── features/     按页面分：conversation plan memory skill report settings roles project
├── api/          openapi 生成的 client + SSE 订阅封装（gen/ 下不手改）
├── i18n/       ★ 中英双语词条（locales/zh-CN.json · en-US.json）
├── platform/   ★ 平台适配层，唯一可以 import @tauri-apps/* 的地方
├── store/        Zustand（UI 状态）
├── models/     ★ 领域类型
├── constants/  ★ 常量与枚举（as const 对象，禁用 enum）
└── utils/      ★ 纯函数工具 + INDEX.md
```

`tests/` 放跨 feature 的测试基建（MSW handlers、事件序列 fixtures、setup）；
单个组件的测试与组件同目录。

## 依赖方向

| | |
|---|---|
| 允许 | `features` → `ui` → `design`；任何层 → `models` `constants` `utils` `api` |
| 禁止 | `ui` → `features`（原语不许认识业务）；`features/a` → `features/b`（跨 feature 要走 `ui` 或 `models`）；任何非 `platform` 文件 → `@tauri-apps/*` |

由 ESLint `no-restricted-imports` 强制。

## 状态管理分工

| 状态种类 | 用什么 |
|---|---|
| 服务端状态（列表、详情、变更） | TanStack Query |
| 实时事件流 | 单条 SSE → Zustand event slice |
| 纯 UI 状态（栏宽、折叠、抽屉栈、过滤器） | Zustand，持久化到 localStorage |

**不要把 SSE 事件塞进 TanStack Query 缓存** —— 两套失效模型混在一起必炸。

## 检查命令

```bash
make -C .. lint-frontend      # tsc --noEmit + ESLint + Stylelint（含设计合规规则）
make -C .. test-frontend      # vitest --run
make -C .. dev-web            # duetd + vite，浏览器打开 http://localhost:5173
```

## 改这里之前必读

| 改什么 | 读什么 |
|---|---|
| **任何组件** | [`../docs/frontend-guide.md`](../docs/frontend-guide.md) + [`../design/Duet Spec.dc.html`](../design/Duet%20Spec.dc.html) |
| 命名、文件组织 | [`../docs/coding-standards.md`](../docs/coding-standards.md) §4 |
| 抽象与复用 | [`../docs/design-principles.md`](../docs/design-principles.md) §6 |
| 事件流 | [`../docs/architecture.md`](../docs/architecture.md) §4（13 类封闭枚举） |
| **任何用户可见文案** | [`../docs/i18n.md`](../docs/i18n.md) —— 中英双语，禁止硬编码 |
| 写测试 | `web-ui-test` skill |

**新组件必须能在 `design/Duet Spec.dc.html` 找到对应条目。找不到 → 先在设计规范新增条目，再实现。**

## 本域特有的坑

- **浮层被裁。** 输入区卡片、下拉容器**不得** `overflow:hidden`，否则向上展开的浮层被裁掉。
  长列表容器一律 `min-height:0 + overflow-y:auto`。
- **悬浮计划面板必须是纯 overlay**：显示与否不得改变对话布局，对话列顶部留白恒定。
- **状态词不要翻译。** `waiting_user` 原样等宽显示，不写成「等待用户」。
- **纯图标按钮必须有中文 tooltip**（`title` + `data-tt`），漏了会被走查抓到。
- **Web 降级必须真的可用。** `platform` 的 Web 实现不能是空函数——
  `dev-web` 是 AI 自测的主要通道，降级坏了等于自测通道坏了。
- **13 类事件是封闭枚举。** 加第 14 类要同时改四处（openapi / architecture.md / Duet Spec / 渲染器注册表）。
- **别用 `switch (event.type)`。** 用注册表，见 design-principles.md §6。
- **文案硬编码。** 中文字面量出现在 JSX 里会被 ESLint 拦；英文字面量拦不住，
  靠 `make check-i18n` 反查 + 审查兜底。写文案就是 `t('key')`，没有例外。
- **状态词不走 i18n。** `executing` 在中英两版里都是 `executing`，等宽显示。
  把它塞进词条文件是错的。
