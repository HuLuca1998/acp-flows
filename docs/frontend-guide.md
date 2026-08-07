# 前端实现指南

> 本文是 `frontend/` 的**实现规格**，写任何组件前必读；设计真源是
> [`design/Duet Spec.dc.html`](../design/Duet%20Spec.dc.html)，**本文与它冲突时以设计稿为准**。
>
> 界面原型（真实文案与结构）在 [`design/ACP Duet 1a.dc.html`](../design/ACP%20Duet%201a.dc.html)。
> `design/` 是**只读**的，不要改；发现缺口按 §14 走登记流程。

> **读法**：本文 ~17k token，**不要整篇读**。写一个组件通常只需要
> 「§7 的那一行 + §1 令牌 + §13 lint 规则」三处。规则见
> [`ai-playbook.md`](ai-playbook.md) §1。
>
> | 你要做的事 | 读哪一节 | 定位 |
> |---|---|---|
> | 用颜色 / 间距 / 字号 → 该写哪个令牌 | §1 §3 §4 §5 | `grep -n '^## 1\.\|^## 3\.' docs/frontend-guide.md` |
> | 一个面板该用第几层底色 | §2 | `grep -n '^## 2\.' docs/frontend-guide.md` |
> | 加图标 | §6 | `grep -n '^## 6\.' docs/frontend-guide.md` |
> | **实现某个具体组件** | §7 查那一行即可 | `grep -n '组件名' docs/frontend-guide.md` |
> | 页面骨架 / 三栏布局 | §8 | `grep -n '^## 8\.' docs/frontend-guide.md` |
> | **写事件渲染器** | §9 ★ | `grep -n '^## 9\.' docs/frontend-guide.md` |
> | hover / focus / disabled 怎么写 | §10 | `grep -n '^## 10\.' docs/frontend-guide.md` |
> | 文案、状态词展示 | §11 | `grep -n '^## 11\.' docs/frontend-guide.md` |
> | Tauri / Web 双形态差异 | §12 | `grep -n '^## 12\.' docs/frontend-guide.md` |
> | **lint 报错了，想知道为什么** | §13 | `grep -n '^## 13\.' docs/frontend-guide.md` |
> | 设计稿没有这个组件 | §14 §16 | `grep -n '^## 14\.\|^## 16\.' docs/frontend-guide.md` |
> | 想确认某个写法是不是反例 | §15 | `grep -n '^## 15\.' docs/frontend-guide.md` |

---

## 0. 本文的位置与边界

| 你要找的东西 | 去哪 |
|---|---|
| 命名、文件组织、导入顺序、`enum` 禁用、Props 类型名、CSS Modules 类名 | [`coding-standards.md`](coding-standards.md) §4 |
| 前端目录职责、平台适配层接口、状态管理分工、13 类事件枚举 | [`architecture.md`](architecture.md) §4–§5 |
| 组合优于继承、注册表代替 switch、文件层级上限 | [`design-principles.md`](design-principles.md) §6 |
| 词条 key、什么翻译什么不翻译、`Intl` 用法 | [`i18n.md`](i18n.md) |
| 术语表、状态词、语气、按钮文案、禁止清单 | [`../AGENTS.md`](../AGENTS.md) §8 §9 |
| 组件测试怎么写、E2E 黄金路径 | [`testing-strategy.md`](testing-strategy.md) §6 |
| **令牌、层级、组件规格、布局数值、事件渲染器、设计合规 lint** | **本文** |

本文**不重复** `coding-standards.md` §4 的任何内容。凡是命名与写法问题，回去读那份。

本文里的所有数值都来自设计规范，**不许四舍五入、不许"差不多"**。
写「适当的间距」这种话的代码评审一律驳回。

---

## 1. 令牌分层

### 1.1 两层，两个文件

```
frontend/src/design/
├── tokens.css     Nocturne 基础层 —— 从设计系统同步，禁止手改
└── duet.css       Duet 产品层 —— 只在这里定义 --s-* / --z-* / --font-mono / --layout-*
```

- **Nocturne 是纯 CSS 令牌**：无构建步骤、无 JS、无 mixin、无 SCSS。
  它就是一份 `:root{}` 声明，`tokens.css` 原样承载。
- `duet.css` **只允许**做两件事：定义产品层新令牌、覆写 `--color-bg`。
  在 `duet.css` 里写组件样式一律驳回。
- 两份文件在 `src/app/` 的根入口按 `tokens.css → duet.css` 顺序 import，顺序不可调换。

### 1.2 Nocturne 基础层（不要在本仓库重新定义）

| 组 | 令牌 |
|---|---|
| 语义色 | `--color-bg` `--color-surface` `--color-text` `--color-accent` `--color-divider` |
| 中性阶 | `--color-neutral-100` … `--color-neutral-900` |
| 强调阶 | `--color-accent-100` … `--color-accent-900` |
| 第二强调阶 | `--color-accent-2-100` … `--color-accent-2-900` |
| 字体 | `--font-heading` `--font-body` `--font-heading-weight` |
| 间距 | `--space-1` `--space-2` `--space-3` `--space-4` `--space-6` `--space-8` |
| 圆角 | `--radius-sm` `--radius-md` `--radius-lg` |
| 阴影 | `--shadow-sm` `--shadow-md` `--shadow-lg` |

字体只有 **Inter** 一种。`--font-heading` 与 `--font-body` 都指向 Inter，
差别在 weight，不在 family。**不许引入第二种西文字体。**

`--color-accent-2-*` 在 Duet 里**当前没有用途**。要用它必须先在设计规范里立条目（§14）。

### 1.3 Duet 产品层（定义在 `duet.css`）

| 令牌 | 值 | 用途 |
|---|---|---|
| `--s-canvas` | `#0e0f18` | 窗口外画布 |
| `--color-bg` | `#161826` | 主内容区（**覆写** Nocturne 同名令牌） |
| `--s-rail` | `#12141f` | 侧栏 / 抽屉 |
| `--s-raised` | `#1b1d2b` | 窗口栏 / 浮层 / 输入卡 |
| `--s-inset` | `#191b28` | 卡片与列表项的内嵌底 |
| `--s-claude-edge` | `#2f2b47` | Claude 角色徽标边缘 |
| `--color-pass` | `var(--color-accent-300)` | 通过 / 新增行 / 有效 |
| `--color-fail` | `oklch(0.66 0.125 25)` | 失败 / 删除行 / 失效 |

**本文新增、设计稿里以字面量出现的三个令牌**（设计稿是独立 HTML 文档，写死了字面量；
我们的实现不许照抄字面量，必须落成令牌）：

| 令牌 | 值 | 设计稿里的字面量出处 |
|---|---|---|
| `--font-mono` | `ui-monospace, SFMono-Regular, Menlo, monospace` | 全文 `font-family:ui-monospace,Menlo,monospace` |
| `--s-scrim` | `rgb(14 15 24 / .72)` | 抽屉 / 对话框遮罩 `rgba(14,15,24,.72)` |
| `--z-handle` `--z-titlebar` `--z-panel` `--z-dropdown` `--z-drawer` `--z-tooltip` | 见 §5.3 | z-index 数字散落在原型各处 |

### 1.4 一律 `var()`，禁止写死

```css
/* ✗ 回滚重做 */
.card { background: #191b28; box-shadow: inset 0 0 0 1px #23262f; }
/* ✓ */
.card { background: var(--s-inset); box-shadow: inset 0 0 0 1px var(--color-neutral-900); }
```

**唯一允许出现颜色字面量的文件是 `src/design/duet.css`**（令牌定义处本身）。
其他任何 `.css` / `.tsx` 里出现 hex、`rgb()`、`hsl()`、`oklch()`、颜色关键字，
由 §13 的 lint 直接拦下。

---

## 2. 四层底色的层级语义

**层级靠底色深浅表达，不靠阴影堆叠。** 由深到浅：

| 令牌 | 用在哪 | 具体位置 |
|---|---|---|
| `--s-canvas` | 窗口外画布 | `body` 背景；Web 模式下窗口四周 |
| `--color-bg` | 主内容区 | 对话列、各页面正文区 |
| `--s-rail` | 侧栏 / 抽屉 | 左栏、右栏、抽屉主体、下拉面板、输入区引用段 |
| `--s-raised` | 窗口栏 / 浮层 | 窗口栏、悬浮计划面板、输入卡、行内 code 芯片、引用芯片 |
| `--s-inset` | 卡片与列表项的内嵌底 | 所有卡片、列表项、代码块 |

### 卡片的唯一配方

```css
.card {
  background: var(--s-inset);
  box-shadow: inset 0 0 0 1px var(--color-neutral-900);
  border-radius: var(--radius-md);
}
```

- **描边用 `box-shadow: inset`，不用 `border`** —— border 会改变盒模型尺寸，
  在密集布局里逐像素累积成偏差。
- **禁止 `box-shadow` 堆叠做层次**（`0 1px 2px …, 0 4px 8px …`）。层次是底色的事。
- `--shadow-lg` 只有一个用途：下拉浮层（§7.7）。`--shadow-sm` 只有一个用途：输入卡。
  除此之外全站不用投影。
- **禁止用 `--color-text` 作描边**（刺眼白边），禁止纯白 `#fff` 与纯黑 `#000`。

---

## 3. 中性阶 · 强调色 · 语义色 · 角色色

### 3.1 中性阶用法（写死，不许自由发挥）

| 阶 | 用途 |
|---|---|
| `--color-neutral-300` | 行内代码、需要强调的标识符 |
| `--color-neutral-400` | **正文** |
| `--color-neutral-500` | 次要说明 |
| `--color-neutral-600` | 弱化文字、meta 行、时间与计数 |
| `--color-neutral-700` | 分组标题（10px uppercase）、代码注释 |
| `--color-neutral-800` | **描边**、进度轨道、滚动条拇指 |
| `--color-neutral-900` | **分隔线**、卡片内嵌描边 |

`--color-neutral-100` / `--color-neutral-200` 在 Duet 里**没有用途**。用它先立条目。

### 3.2 强调色：只做线与光，不做大面积填充

| 阶 | 用途 |
|---|---|
| `--color-accent-900` | 选中底（列表项、下拉选中项、激活的图标按钮） |
| `--color-accent-700` | 描边（主按钮、可切换的下拉触发器、决策卡、权限卡） |
| `--color-accent-600` | 进度条填充 |
| `--color-accent-400` | 图标 / 激活态 / 分段进度已完成段 / D0–D3 角标实底 |
| `--color-accent-300` | 正文强调、主按钮文字、代码关键字 |
| `--color-accent-200` | **仅**开关打开态的圆点 |

**全站唯一允许的大面积实底是 D0–D3 决策等级角标**（`--color-accent-400` 底 + `--color-bg` 字）。
其余任何"实心填充的强调色块"都是反例。

### 3.3 语义色只有两个

| 令牌 | 含义 |
|---|---|
| `--color-pass` | 通过 / 新增行 / 有效 / `active` |
| `--color-fail` | 失败 / 删除行 / 失效 / 越界 |

**不得引入第三种语义色。** 没有 warning 色、没有 info 色。
需要表达"注意"时用 `--color-accent-700` 描边卡承载，不是新颜色。

### 3.4 角色标识

| 角色 | 主色 | 徽标底 | 徽标描边 |
|---|---|---|---|
| Claude | `--color-accent-400` | `--color-accent-900` | `--color-accent-700`（外缘 `--s-claude-edge`） |
| Codex | `--color-neutral-300` | `--color-neutral-900` | `--color-neutral-700` |

**不要给 Agent 分配新颜色。** 未来接入第三个 Runtime 时，先在设计规范立条目。

角色色在代码里只经过一个出口：

```ts
// src/constants/role.ts
export const ROLE_TONE = { claude: 'claude', codex: 'codex' } as const
export type RoleTone = (typeof ROLE_TONE)[keyof typeof ROLE_TONE]
```

CSS 侧用 `data-role="claude|codex"` 属性选择器取色，**不在 TS 里拼颜色字符串**。

---

## 4. 字体

### 4.1 Inter（界面与叙述）

| 字号 / 字重 | 用途 | 行高 |
|---|---|---|
| 20 / 500 | 页面主标题（`h4`） | 默认 |
| 15 / 500 | 区块标题、渲染后的 md 二级标题 | 默认 |
| 13.5 / 400 | **对话正文** | 1.7 |
| 12.5 / 400 | 列表项主文本、表格单元 | 默认 |
| 11.5 / 400 | 按钮、芯片、说明文字 | 默认 |
| 10 / uppercase / `letter-spacing:.09em` | 分组标题 | 默认 |

### 4.2 `--font-mono`（标识符与数据）

| 字号 | 用途 | 示例 |
|---|---|---|
| 12.5 | 领域对象 ID | `unit-012` `mem-203` `plan v5` |
| 11.5 | 路径、分支、命令、commit hash | `~/work/acp-engine` `duet/work-08` `abc123` |
| 11 | 行内状态词 | `executing` `waiting_user` |
| 10.5 | 角标、时间、计数 | `+64` `−12` `2:14` `12 条` |

### 4.3 判定规则（唯一一条，机械执行）

> **凡是能被复制粘贴到终端或代码里的字符串，一律等宽。**

包括但不限于：ID、路径、分支名、commit hash、命令、退出码、状态词、
YAML/JSON 片段、`session/set_mode` 这类协议方法名、版本号、`ghp_…` 令牌掩码。

中文叙述一律 Inter。**中英混排时英文与数字两侧不加额外空格**，交给字体处理——
在 JSX 里手动插 `{' '}` 或 `&nbsp;` 调间距的写法一律驳回。

---

## 5. 间距 · 圆角 · z-index

### 5.1 间距（密集为默认）

| 场景 | 值 |
|---|---|
| 列表项内 | `gap: 7–9` |
| 卡片内 | `padding: 9–14` |
| 区块之间 | `gap: 14` |
| 页面留白 | `padding: 22 28` |
| 对话消息之间 | `gap: 24` |

Nocturne 的 `--space-*` 阶（1/2/3/4/6/8）**覆盖不到这套密集数值**。
落地规则：能对上 `--space-*` 的用令牌；对不上的走 `duet.css` 里的 `--gap-*` 别名，
**不许在组件 CSS 里写裸 px**：

```css
/* duet.css */
:root {
  --gap-row: 8px;      /* 列表项内 7–9 的代表值 */
  --pad-card: 10px 12px;
  --gap-block: 14px;
  --pad-page: 22px 28px;
  --gap-message: 24px;
}
```

### 5.2 圆角

| 令牌 | 值 | 用在哪 |
|---|---|---|
| `--radius-sm` | 4px | 图标按钮、类型标签、D0–D3 角标 |
| `--radius-md` | 8px | 卡片、列表项、按钮、下拉面板与其条目 |
| `--radius-lg` | 14px | 输入卡、浮层、窗口 |
| `999px` | — | 筛选胶囊、开关 |

设计稿里还出现 2px / 3px（进度条圆角）与 6px（tooltip）。这两档 Nocturne 没有令牌，
在 `duet.css` 里补 `--radius-xs: 2px`、`--radius-tip: 6px`，**不要写裸 px**。
（这是一处设计稿缺口，已登记在 §16。）

### 5.3 z-index

只有六层，全部落成令牌，**组件里禁止出现裸数字 `z-index`**：

| 令牌 | 值 | 承载 |
|---|---|---|
| `--z-handle` | 5 | 拖拽手柄 / 悬浮胶囊（区间 4–5） |
| `--z-titlebar` | 6 | 窗口栏 |
| `--z-panel` | 9 | 可拖动的计划面板 |
| `--z-dropdown` | 14 | 页内下拉（区间 13–16） |
| `--z-drawer` | 24 | 抽屉 / 对话框（区间 20–26） |
| `--z-tooltip` | 60 | tooltip 与输入框浮层 |

需要在同一层内做前后关系时，用同层内的 DOM 顺序，不要发明 `z-index: 7`。

---

## 6. 图标

**只用 Phosphor regular（单一线重）。** 四档尺寸，没有第五档：

| 尺寸 | 用在哪 |
|---|---|
| 16 | 主导航与窗口栏 |
| 15 | 按钮内 |
| 13 | 行内动作 |
| 12 | 芯片内 |

### 6.1 已登记的图标映射

图标名集中在 `src/constants/icon.ts`，**组件里不许直接写 `ph-*` 字符串**：

| 语义 | Phosphor | 语义 | Phosphor |
|---|---|---|---|
| 折叠侧栏 | `ph-sidebar-simple` | 分支 | `ph-git-branch` |
| 新建工作 | `ph-note-pencil` | 改动 | `ph-file-diff` |
| 搜索 | `ph-magnifying-glass` | 权限 | `ph-shield-check` |
| 计划 | `ph-list-checks` | 过滤 | `ph-funnel` |
| 报表 | `ph-chart-bar` | 附加 | `ph-paperclip` |
| 记忆 | `ph-brain` | 检查点 | `ph-flag` |
| Skill | `ph-book-open-text` | 契约 | `ph-file-lock` |
| 角色 | `ph-users-three` | 证据 | `ph-check-circle` |
| 设置 | `ph-gear` | GitHub | `ph-github-logo` |
| 文件引用 | `ph-file-text` | 发送 | `ph-arrow-up` |

```ts
// src/constants/icon.ts
export const ICON = {
  collapseSidebar: 'ph-sidebar-simple',
  newWork: 'ph-note-pencil',
  // …
} as const
export type IconName = (typeof ICON)[keyof typeof ICON]
```

挑不到合适的图标 → **先在设计规范里登记新条目**（§14），不要临时挑一个近似的。

### 6.2 禁止

- ✗ emoji
- ✗ 自绘 SVG 图标
- ✗ 用 Unicode 几何符号（`◧ ⧉ ⚑ ◑ ◔ ▤ ◫ ▾`）代替图标

**唯一例外是这五个文本符号**——它们是控件的组成部分，不是独立图标：

| 符号 | 用途 |
|---|---|
| `⌄` | 下拉指示 |
| `✕` | 关闭 / 移除 |
| `⠿` | 拖动手柄 |
| `✓` `✗` | 正反例标记、验收标准命中与否 |

---

## 7. 组件清单与实现映射

`src/ui/` 是**设计系统原语**：无业务语义、不认识 Work / Unit / Contract、不发请求。
带业务语义的组合体属于对应 `src/features/<name>/`。

一文件一组件 + 同名 `.module.css`，见 [`coding-standards.md`](coding-standards.md) §4.2。

| 设计规范条目 | 文件 | 备注 |
|---|---|---|
| 按钮 | `src/ui/Button.tsx` | 四种 variant |
| 按钮（纯图标） | `src/ui/IconButton.tsx` | `tooltip` 必填 |
| 图标 | `src/ui/Icon.tsx` | 四档尺寸 |
| 标签与芯片 | `src/ui/Tag.tsx` `src/ui/DecisionBadge.tsx` `src/ui/StatusText.tsx` `src/ui/RefChip.tsx` | 四个独立组件 |
| 卡片与列表项 | `src/ui/Card.tsx` `src/ui/CardHeader.tsx` `src/ui/ListItem.tsx` | 三态固定 |
| 进度与选择控件 | `src/ui/SegmentedProgress.tsx` `src/ui/BarProgress.tsx` `src/ui/Switch.tsx` `src/ui/Checkbox.tsx` `src/ui/Radio.tsx` | 全部自绘 |
| 下拉与浮层 | `src/ui/Dropdown.tsx` | 触发器 + 面板同一组件 |
| 抽屉 / 对话框 | `src/ui/Drawer.tsx` `src/ui/Dialog.tsx` `src/ui/Scrim.tsx` | 共用 Scrim |
| 加载中 | `src/ui/Spinner.tsx` | 圆环，不是旋转图标 |
| 代码与 Markdown | `src/ui/CodeBlock.tsx` `src/ui/DiffView.tsx` `src/ui/Markdown.tsx` | 五色高亮 |
| 拖拽 | `src/ui/ResizeHandle.tsx` `src/ui/DragHandle.tsx` | 分栏 / 浮层 |
| tooltip 属性 | `src/ui/tooltip-props.ts` | 返回 `{ title, 'data-tt' }` |
| 输入区（五段） | `src/features/conversation/Composer*.tsx` | **feature 组合体，不进 `ui/`** |

### 7.1 按钮 `Button.tsx`

```ts
export interface ButtonProps {
  variant: 'primary' | 'secondary' | 'ghost' | 'danger'
  icon?: IconName
  loading?: boolean
  disabled?: boolean
  onClick: () => void
  children: ReactNode          // 动词短语，见 §11
}
```

| variant | 描边 | 文字 | 底 |
|---|---|---|---|
| `primary` | `inset 0 0 0 1px var(--color-accent-700)` | `--color-accent-300` | **透明** |
| `secondary` | `inset 0 0 0 1px var(--color-neutral-800)` | `--color-neutral-400` | 透明 |
| `ghost` | 无 | `--color-neutral-500` | 透明，hover 出 `--color-surface` |
| `danger` | `inset 0 0 0 1px var(--color-neutral-800)` | `--color-fail` | 透明 |

**硬约束：**

- **主按钮永不填充实心。** 没有 `background: var(--color-accent-*)` 的按钮。
- **禁止彩色渐变按钮。**
- 危险动作用 `--color-fail` **文字色**，不是红底。
- 尺寸：高 24–26，`padding: 4px 11px`，`border-radius: var(--radius-md)`，字号 11.5–12。
- 按钮内图标固定 15。

### 7.2 图标按钮 `IconButton.tsx`

```ts
export interface IconButtonProps {
  icon: IconName
  tooltip: string              // 必填，中文；带快捷键写成「折叠侧栏 ⌘B」
  active?: boolean
  disabled?: boolean
  size?: 16 | 15 | 13 | 12     // 默认 15
  onClick: () => void
}
```

**`tooltip` 是必填 prop——「纯图标按钮必须有中文 tooltip」这条规则由 `tsc` 强制，
不靠人记。** 忘了写 → `pnpm typecheck` 红。

| 态 | 底 | 文字 |
|---|---|---|
| 默认 | 透明 | `--color-neutral-500` |
| hover | `--color-surface` | `--color-accent-300` |
| 激活 | `--color-accent-900` | `--color-accent-300` |

尺寸 28×24，`border-radius: var(--radius-sm)`。

### 7.3 标签与芯片

| 组件 | 规格 |
|---|---|
| `Tag`（类型标签） | 10px 等宽，`padding: 1px 6px`，`--radius-sm`；`tone="accent"` → `--color-accent-900` 底 + `--color-accent-300` 字；`tone="neutral"` → `--color-surface` 底 + `--color-neutral-400` 字 |
| `DecisionBadge`（D0–D3） | 10px 等宽，`padding: 2px 6px`，`--radius-sm`，`--color-accent-400` 实底 + `--color-bg` 字。**全站唯一允许的实底** |
| `StatusText` | 10.5px 等宽，**不加底色**，只着色：`--color-pass` / `--color-fail` / `--color-neutral-600` |
| `RefChip`（可移除引用） | 11px 等宽，`padding: 3px 9px`，`--radius-md`，`--s-raised` 底 + `--color-neutral-800` 描边；前置 12px 图标；尾部 `✕`（`--color-neutral-700`） |

```ts
export interface DecisionBadgeProps { level: 'D0' | 'D1' | 'D2' | 'D3' }
export interface StatusTextProps { value: string; tone: 'pass' | 'fail' | 'muted' }
export interface RefChipProps { icon: IconName; label: string; onRemove?: () => void }
```

**禁止用彩色圆点表示文件类型。** 类型靠 `Tag` 的文字表达。

### 7.4 卡片与列表项

三态固定，没有第四态：

| 态 | 底 | 描边 | 其他 |
|---|---|---|---|
| 默认 | `--s-inset` | `inset 0 0 0 1px var(--color-neutral-900)` | — |
| 选中 | `--s-raised` | `inset 0 0 0 1px var(--color-accent-700)` | — |
| 失效 | 透明 | `inset 0 0 0 1px var(--color-neutral-900)` | `opacity: .6`，状态文字用 `--color-fail` |

```ts
export interface CardProps { state?: 'default' | 'selected' | 'invalid'; children: ReactNode }
export interface CardHeaderProps { id: string; tag?: ReactNode; status?: ReactNode }
export interface ListItemProps extends CardProps { isCurrent?: boolean; onSelect?: () => void }
```

**卡片头顺序写死：** 等宽 ID → 类型标签 → 弹性空隙（`flex:1`）→ 右侧状态。
`CardHeader` 用固定 slot 实现这个顺序，**不给调用方重排的自由**。

- 正文 12–12.5
- 底部 meta 行 10.5 等宽 `--color-neutral-600`
- **左侧 2px accent 竖条（`isCurrent`）仅用于「当前工作 / 当前单元」**，不要滥用。
  这条竖条在代码里只有 `ListItem` 一个出口。

### 7.5 进度与选择控件

| 控件 | 规格 |
|---|---|
| `SegmentedProgress` | 用于「计划 N 个单元」。每段一 flex 单位，`height:3`，`gap:3`，`--radius-xs`；已完成段 `--color-accent-400`；**进行中的一段用渐变** `linear-gradient(90deg, var(--color-accent-600) <pct>, var(--color-neutral-800) <pct>)`；未开始段 `--color-neutral-800` |
| `BarProgress` | `height:4`，`--radius-xs`；轨道 `--color-neutral-800`，填充 `--color-accent-600` |
| `Switch` | 26×15，`999px`；圆点 11，`inset:2px`；开＝`--color-accent-700` 底 + `--color-accent-200` 圆点；关＝`--color-neutral-800` 底 + `--color-neutral-500` 圆点 |
| `Checkbox` | 13×13 方形，`--radius-sm`；选中＝`--color-accent-400` 实心 + `inset 0 0 0 1.5px var(--color-accent-400)`；未选＝`inset 0 0 0 1.5px var(--color-neutral-700)` |
| `Radio` | 13×13 圆形，其余同 `Checkbox` |

```ts
export interface SegmentedProgressProps {
  segments: Array<{ state: 'done' | 'active' | 'todo'; weight?: number; ratio?: number }>
}
export interface SwitchProps { checked: boolean; label: string; onChange: (next: boolean) => void }
```

**不要使用原生 `checkbox` / `radio` / `select` 外观。**
可访问性靠 `role` + `aria-checked` + 键盘处理补，不靠原生控件的默认皮肤。
（原生 `<input>` 仍可作为不可见的可访问性宿主，但视觉必须完全自绘。）

### 7.6 下拉与浮层 `Dropdown.tsx`

```ts
export interface DropdownItem { value: string; label: string; meta?: string }
export interface DropdownProps {
  label: string                       // 小标签，如「项目」「范围」
  value: string                       // 当前值，等宽显示
  items: DropdownItem[]
  onSelect: (value: string) => void
  footerAction?: { label: string; onClick: () => void }   // 「管理项目…」
}
```

| 部位 | 规格 |
|---|---|
| 触发器 | `padding: 5px 11px`，`--radius-md`，`--s-inset` 底 + `inset 0 0 0 1px var(--color-accent-700)`；结构＝小标签(10px `--color-neutral-600`) + 当前值(11.5 等宽 `--color-accent-300`) + `⌄`(10px `--color-neutral-600`) |
| 面板 | `--s-rail` 底 + `--shadow-lg`，`--radius-md`，`padding: 7`，条目间 `gap: 1px` |
| 条目 | `padding: 7px 9px`，`--radius-md`；右侧计数 10px 等宽 `--color-neutral-700` |
| 选中条目 | `--color-accent-900` 底 + `--color-accent-300` 字 |
| 尾部动作 | 前置 `1px var(--color-neutral-900)` 分隔线（`margin: 4px 0`），文字 11.5 `--color-neutral-500` |

**硬约束：**

- **同类选择器在不同页面必须放同一位置**：页头标题行下方第一行。
  记忆页的「范围」、Skill 页的「项目」、对话页的项目选择器都遵守这一条。
- **超过 8 项时面板设 `max-height` 并内部滚动**（`min-height:0 + overflow-y:auto`）。
- **浮层父级不得 `overflow:hidden`**。`Dropdown` 通过 portal 挂到 `#overlay-root`，
  但父级仍然不许写 `overflow:hidden`——输入区那一层尤其（§7.9）。

### 7.7 抽屉与对话框

| | `Drawer` | `Dialog` |
|---|---|---|
| 用途 | **查看**：契约、证据、审查意见 | **决定**：创建项目、新建工作、D2/D3 授权 |
| 位置 | 右侧滑入 | 居中 |
| 宽度 | 560 / 720 / 980（按内容三选一） | 560–620 |
| 起始边界 | `inset: 42px 0 0` | `inset: 42px 0 0` |
| 遮罩 | `--s-scrim` | `--s-scrim` |
| 点遮罩关闭 | ✓ | ✓，**但 D3 授权对话框不可点遮罩关闭** |

```ts
export interface DrawerProps {
  open: boolean
  kicker: string                       // 头部小标签
  title: string
  width: 560 | 720 | 980
  onClose: () => void
  children: ReactNode
}
export interface DialogProps {
  open: boolean
  kicker: string
  title: string
  width?: 560 | 620
  dismissible?: boolean                // D3 授权传 false
  onClose: () => void
  actions: ReactNode                   // 底部动作区
  children: ReactNode
}
```

- **两者头部结构一致**：kicker 标签 + 标题 + 弹性空隙 + 关闭 `✕`。
  头部 `padding: 9px 11px`，`--s-raised` 底 + `--color-neutral-800` 描边，标题 11.5。
- **底部动作区右对齐：次要在左、主要在右。**
- **`inset: 42px` 是硬要求**——不遮挡窗口栏，窗口栏上的折叠开关在抽屉打开时仍可点。
- 抽屉栈状态放 Zustand（`architecture.md` §5），不放组件局部 state。

### 7.8 代码与 Markdown

**代码高亮只用五色**（六个 token 类别），`CodeBlock` 的高亮映射表写死在
`src/constants/syntax.ts`，不许各处自定义：

| 类别 | 颜色 |
|---|---|
| 关键字 | `--color-accent-300` |
| 命令 / 函数 | `--color-accent-400` |
| 字符串 | `--color-pass` |
| 标识符 | `--color-neutral-300` |
| 参数 | `--color-neutral-500` |
| 注释 | `--color-neutral-700` |

`DiffView`：新增行 `--color-pass`、删除行 `--color-fail`。**不给 diff 加行底色**——
只给文字着色，底仍是 `--s-inset`。

代码块容器：`padding: 12px 14px`，`--radius-md`，`--s-inset` 底 + `--color-neutral-900` 描边，
`--font-mono` 11.5 / 行高 1.8。

`Markdown`：

```ts
export interface MarkdownProps { source: string; mode: 'rendered' | 'source'; onToggleMode: () => void }
```

- **默认渲染显示**，并提供「查看源码」切换（这是必须有的，不是可选）。
- 渲染规格：标题 15/500、正文 13/1.7、行内 `code` 为 `--s-raised` 底芯片、
  引用为左侧 2px 竖条。

### 7.9 输入区（固定五段）★

`src/features/conversation/`：

| 段 | 文件 | 规格 |
|---|---|---|
| ① 队列状态条 | `QueueStrip.tsx` | `padding: 6px 11px`，`--color-surface` 底，下边 1px `--color-neutral-900`；5px accent 圆点 + 11px 文字。**无队列时整条隐藏** |
| ② 多行输入 | `ComposerInput.tsx` | `padding: 10px 12px`，12.5px；**默认一行**，随内容增高 |
| ③ 功能按钮行 | `ComposerActions.tsx` | `padding: 0 8px 8px`，`gap: 6`；左＝附加 / 过滤 / 权限（13px 图标）；右＝发送（22×22 圆形，`inset 0 0 0 1px var(--color-accent-700)`，12px `ph-arrow-up`，`--color-accent-400`） |
| ④ 引用区 | `ComposerRefs.tsx` | `padding: 6px 11px`，上边 1px `--color-neutral-900`，`--s-rail` 底，10.5px `--color-neutral-600`。**无引用时隐藏**。文案固定「仅本轮有效」 |
| ⑤ 工作区信息行 | `WorkspaceInfoBar.tsx` | **在卡片外**。10.5px 等宽 `--color-neutral-600`，`gap: 7`；内容＝项目 · worktree · 分支 · 改动 |

外壳 `Composer.tsx`：

```ts
export interface ComposerProps {
  queued: QueuedMessage[]          // 空数组 → ① 整条不渲染
  refs: ComposerRef[]              // 空数组 → ④ 不渲染
  disabled?: boolean
  onSend: (text: string) => void
}
```

**硬约束（违反即回滚）：**

1. **五段顺序不可调换。** `Composer.tsx` 里五个子组件的 JSX 顺序写死，不接受 `slots` 之类的可配置项。
2. 卡片本身 `position: relative`，`--radius-lg`，`--s-raised` 底 + `--shadow-sm`。
3. **不得 `overflow: hidden`** —— 否则会裁掉向上展开的浮层（过滤器面板、文件选择浮层）。
   首尾行各自补圆角来达到"看起来被裁过"的效果，而不是真的裁。
4. ①②③④ 之间用 1px `--color-neutral-900` 分隔，不用 `gap`。

---

## 8. 布局骨架

### 8.1 尺寸常量

所有布局数值集中在 `src/constants/layout.ts`，**CSS 里只读 `var(--layout-*)`**：

```ts
// src/constants/layout.ts
export const LAYOUT = {
  titlebarHeight: 42,
  railWidth: 252,
  railMin: 180,
  railMax: 420,
  railCollapsed: 48,
  asideWidth: 300,
  asideMin: 220,
  asideMax: 460,
  conversationWidth: 800,
} as const
```

`src/app/AppShell.tsx` 把它们注入根节点的自定义属性：

```tsx
<div className={styles.shell} style={{ '--layout-rail-w': `${railWidth}px` } as CSSProperties}>
```

这是 **inline `style` 的唯一合法用途**：注入 `--*` 自定义属性。
inline style 里出现颜色或非自定义属性，由 §13 的 ESLint 规则拦下。

| 区域 | 值 | 可拖范围 | 折叠态 |
|---|---|---|---|
| 窗口栏 | 高 42 | — | — |
| 左栏 | 252 | 180–420 | 48（保留图标条） |
| 主区 · 对话列 | 800 居中 | — | — |
| 右栏 | 300 | 220–460 | 完全收起 |

### 8.2 六条布局规则

| # | 规则 | 实现要点 |
|---|---|---|
| ① | **所有折叠开关集中在窗口栏**，页面内不再放第二处；窗口栏左段宽度＝左栏宽度，随拖动同步 | 三个开关（左栏 / 计划 / 右栏）只在 `src/app/TitleBar.tsx` 出现一次；窗口栏左段用同一个 `--layout-rail-w` |
| ② | 左右栏可拖拽改宽，范围写死；折叠后左栏保留 48px 图标条，右栏完全收起 | `ResizeHandle` 的 `min`/`max` 从 `LAYOUT` 取，**不接受 props 传任意值** |
| ③ | **悬浮面板（计划）必须是纯 overlay**：显示与否不得改变对话布局，对话列顶部留白恒定 | 计划面板 `position: absolute` 挂在主区容器上，主区**不用** flex 给它分配空间；写一条组件测试断言"开关计划面板前后，对话列首条消息的位置不变" |
| ④ | **右栏只在对话页启用**，其他页面隐藏且窗口栏图标置灰 | 右栏可见性由路由派生，不由用户 toggle 单独决定；非对话页的窗口栏图标 `disabled` |
| ⑤ | 二级页面用「面包屑 + 返回上一处」，**不放「回到对话」按钮** | 返回目标来自导航栈，不是硬编码的对话路由 |
| ⑥ | **长列表容器一律 `min-height: 0` + `overflow-y: auto`**，禁止用 `overflow: hidden` 裁内容 | flex 子项默认 `min-height: auto` 会撑破容器——这是最常见的滚动 bug 来源 |

### 8.3 窗口栏

- `--s-raised` 底，下边 1px `--color-neutral-900`，`z-index: var(--z-titlebar)`。
- Tauri 下 `decorations: false`，窗口栏自绘并承担拖拽区（`data-tauri-drag-region`）。
- macOS 红绿灯区在左侧留白——**红绿灯是原生渲染的，前端不画**。
  设计稿里画了三个彩色圆点只是示意（见 §16 缺口 G7）。
- Web 模式下窗口控制按钮整体隐藏（`platform.window` 为 no-op，§12）。

---

## 9. 13 类事件渲染器注册表 ★

这是对话页的核心机制，也是**最容易被下一轮 AI 改坏的地方**。

### 9.1 为什么用注册表

`Record<EventType, EventRenderer>` 让「13 类事件必须全部有渲染器」变成**编译期约束**：
少一类 → `tsc` 红；多一类 → `tsc` 红。
过滤器 UI 也直接由这张表生成——**不许另建一份过滤项列表**，两份必然漂移。

### 9.2 类型签名

```ts
// src/features/conversation/events/registry.ts
import type { ComponentType } from 'react'
import type { DuetEvent, EventType } from '@/models/event'

/** 事件在时间线上的默认可见性 */
export type EventVisibility = 'shown' | 'collapsed' | 'hidden'

/** 应用事件必须能点开到的结构化产物 */
export type DrilldownTarget = 'plan' | 'contract' | 'evidence' | 'memory' | 'decision' | 'checkpoint'

export interface EventRendererProps<T extends EventType> {
  event: Extract<DuetEvent, { type: T }>
  collapsed: boolean
  onToggleCollapse: () => void
}

interface RendererBase<T extends EventType> {
  /** 与 api/openapi.yaml 的 type 枚举一一对应 */
  type: T
  /** 过滤器里的中文名 */
  label: string
  /** 过滤器里的英文标识，等宽显示，不翻译 */
  wire: string
  defaultVisibility: EventVisibility
  Component: ComponentType<EventRendererProps<T>>
}

/** ACP 事件：只做展示与折叠，没有下钻 */
interface AcpRenderer<T extends EventType> extends RendererBase<T> {
  source: 'acp'
  /** 是否阻塞当前轮 —— 只有 request_permission 为 true */
  blocksTurn: boolean
}

/** 应用事件：永远可点开到结构化产物，缺 openTarget 直接编译不过 */
interface AppRenderer<T extends EventType> extends RendererBase<T> {
  source: 'app'
  openTarget: DrilldownTarget
}

export type EventRenderer<T extends EventType = EventType> = AcpRenderer<T> | AppRenderer<T>

/** 少一类或多一类都会让 tsc 红 */
export const EVENT_RENDERERS: { [T in EventType]: EventRenderer<T> } = {
  message_chunk: { type: 'message_chunk', source: 'acp', /* … */ },
  // …13 条
}
```

判别联合把「**`app` 事件永远可点开到对应的结构化产物**」这条设计约束
变成了类型错误，而不是靠人记住。这是本文里最重要的一处设计。

### 9.3 13 类清单

一类一个组件文件，放 `src/features/conversation/events/`：

| # | `type` | source | 组件文件 | 展示形态 | 默认可见性 |
|---|---|---|---|---|---|
| 1 | `message_chunk` | `acp` | `MessageChunkEvent.tsx` | 徽标 + 角色名 + 正文，左侧 2px **角色色**竖条 | `shown` |
| 2 | `thought_chunk` | `acp` | `ThoughtChunkEvent.tsx` | 斜体 `--color-neutral-600` 单行 + `--color-neutral-800` 竖条，可折叠 | `shown` |
| 3 | `tool_call` | `acp` | `ToolCallEvent.tsx` | inset 表格：类型 · 目标 · 右侧 `+/−` 或运行中圆环 | `shown` |
| 4 | `request_permission` | `acp` | `RequestPermissionEvent.tsx` | `--color-accent-700` 描边卡 + 「允许一次」/「拒绝」，**阻塞当前轮** | `shown` |
| 5 | `turn_end` | `acp` | `TurnEndEvent.tsx` | 居中细线 + 等宽小字 | `collapsed` |
| 6 | `plan_version` | `app` | `PlanVersionEvent.tsx` | accent-700 竖条单行 + 「查看差异」→ `plan` | `shown` |
| 7 | `unit_contract` | `app` | `UnitContractEvent.tsx` | 同上 + 「查看契约」打开抽屉 → `contract` | `shown` |
| 8 | `state_change` | `app` | `StateChangeEvent.tsx` | 居中分隔线上的等宽小字 | `hidden` |
| 9 | `injection` | `app` | `InjectionEvent.tsx` | 灰色单行 + ID 芯片，点击跳记忆页 → `memory` | `shown` |
| 10 | `memory_candidate` | `app` | `MemoryCandidateEvent.tsx` | inset 卡 + 「审核」，**绝不自动写入** → `memory` | `shown` |
| 11 | `decision` | `app` | `DecisionEvent.tsx` | accent-700 描边卡：等级角标 + 选项 + 推荐标记 → `decision` | `shown` |
| 12 | `evidence` | `app` | `EvidenceEvent.tsx` | 单行 `✓` 计数 + 「打开证据」抽屉 → `evidence` | `shown` |
| 13 | `checkpoint` | `app` | `CheckpointEvent.tsx` | 居中 `ph-flag` + ck 编号 + commit hash → `checkpoint` | `shown` |

### 9.4 通用约束

- **任何事件行都必须能被过滤器开关。** 过滤器面板由 `EVENT_RENDERERS` 生成
  （中文名 + `wire` 等宽标识 + 计数），见原型「时间线显示」面板。
- **`app` 事件永远可点开到结构化产物；`acp` 事件只做展示与折叠。**
- **不展示模型私有思维链原文，只展示摘要。** `thought_chunk` 渲染器
  不接受也不渲染完整 CoT——payload 里若出现全文，前端截断并只显示摘要字段。
- `seq` 单调递增，事件乱序到达时按 `seq` 归位；断线用 `Last-Event-ID` 续传
  （`architecture.md` §4）。
- SSE 事件写入 Zustand 的 event slice，**不塞进 TanStack Query 缓存**。

### 9.5 新增第 14 类事件的完整步骤（四处，缺一不可）

| # | 改哪 | 改什么 |
|---|---|---|
| 1 | `api/openapi.yaml` | `type` 枚举加值 + payload schema。**必须最先改**（铁律 2 契约先行），然后 `make gen` |
| 2 | `docs/architecture.md` §4 | 事件表加一行（source / type / 展示形态） |
| 3 | `design/Duet Spec.dc.html` 第 07 节 | 加一行展示形态条目。**找不到条目就不许实现**（铁律 3） |
| 4 | `src/features/conversation/events/` | 新组件文件 + `registry.ts` 登记 + `src/constants/event.ts` 加枚举值 |

> `architecture.md` §4 末尾写的是"三处"，它漏算了自己。**以本表的四处为准。**

同时必须做的：

- 加一个渲染测试（`testing-strategy.md` §6 要求「13 类事件各至少一个渲染测试」，
  第 14 类进来就是 14 个）
- 在 `frontend/tests/INDEX.md` 登记
- 更新过滤器的穷举测试（断言过滤器项数 === `Object.keys(EVENT_RENDERERS).length`）

---

## 10. 交互状态

### 10.1 五件套

| 状态 | 具体样式 |
|---|---|
| **hover** | 出 `--color-surface` 底，**或**描边升到 `--color-accent-700`，文字升到 `--color-accent-300`。二选一，不叠加 |
| **active / 选中** | `--color-accent-900` 底 + `--color-accent-300` 字 |
| **focus-visible** | 2px accent 外轮廓 + 2px offset。**继承设计系统，不要覆盖**——不许写 `outline: none` |
| **disabled** | `opacity: .45` 且**移除 cursor**（`cursor: default`），不是 `not-allowed` |
| **加载中** | 1.5px `--color-accent-400` 圆环（`Spinner`）。**不用旋转图标**，不用三个跳动的点 |

每个可交互组件的 `.module.css` 必须显式写出这五个状态；缺哪个补哪个，
不许靠浏览器默认。

### 10.2 Tooltip

**所有纯图标控件必须有中文 tooltip。**

```ts
// src/ui/tooltip-props.ts
export function tooltipProps(label: string): { title: string; 'data-tt': string } {
  return { title: label, 'data-tt': label }
}
```

- `title` 与 `data-tt` **两个都要**：`title` 给可访问性与原生兜底，`data-tt` 给统一样式。
- `label` 必须来自 `t()`，不是硬编码字符串（[`i18n.md`](i18n.md) §5）。
- 样式由**全局** `[data-tt]:hover::after` 提供，写在 `src/design/duet.css`：
  `--color-surface` 底、1px `--color-neutral-800` 描边、11px、`--radius-tip`、
  `top: calc(100% + 7px)`、`z-index: var(--z-tooltip)`、`pointer-events: none`。
  **不要为单个组件重写 tooltip 样式，也不要引入 tooltip 库。**
- 带快捷键时写成「折叠侧栏 ⌘B」「搜索 ⌘K」——中文名 + 空格 + 快捷键。
- **带文字的按钮不加 tooltip**，除非需要补充说明（例如解释一个后果）。

### 10.3 滚动条

全局统一，写在 `duet.css`，**不许各处覆盖**：

```css
*::-webkit-scrollbar { width: 10px; height: 10px }
*::-webkit-scrollbar-track { background: transparent }
*::-webkit-scrollbar-thumb {
  background: var(--color-neutral-800);
  border-radius: 99px;
  border: 3px solid transparent;   /* 3px 透明留边 */
  background-clip: content-box;
}
*::-webkit-scrollbar-thumb:hover { background: var(--color-neutral-700); background-clip: content-box }
* { scrollbar-width: thin; scrollbar-color: var(--color-neutral-800) transparent }
```

**不允许出现系统默认浅色滚动条。** 出现了就是漏了 `duet.css` 或被某处覆盖了。

### 10.4 拖拽

| 场景 | 规格 |
|---|---|
| 分栏手柄（`ResizeHandle`） | 5px 宽，`--color-neutral-900` 底，hover `--color-accent-700`，`cursor: col-resize`，`z-index: var(--z-handle)` |
| 可移动浮层（`DragHandle`） | `⠿` 手柄区域，`cursor: grab`（拖动中 `grabbing`）；**拖动范围限制在主区内，上边界 8px** |

拖动过程用 `pointerdown/pointermove/pointerup` + `setPointerCapture`，
**不用 `mousedown` + `document` 全局监听**（后者在 WebView 里丢事件）。
拖动中禁用文本选择（`user-select: none`），松手立刻恢复。

栏宽持久化到 localStorage（Zustand persist），**不写后端**。

---

## 11. 文案与术语

术语表是 [`../AGENTS.md`](../AGENTS.md) §8，**不要在本文重抄**。
词条怎么存、怎么取、什么翻译什么不翻译，见 [`i18n.md`](i18n.md)——本文只管**视觉与措辞**。

| 规则 | 前端落地 |
|---|---|
| 界面语言简体中文 | 所有面向用户的字符串走 `t()`，词条真源 `src/i18n/locales/zh-CN.json`（[`i18n.md`](i18n.md) §4） |
| **状态词一律英文原值、不翻译、等宽显示** | `clarifying` `planning` `ready` `executing` `reviewing_unit` `waiting_user` `paused` `completed` `failed` —— 只经过 `StatusText` 一个出口，用 `--font-mono` |
| 标识符保留英文并等宽 | `Work` `Plan` `Subplan` `Unit` `UnitContract` `Attempt` `Evidence` `Checkpoint` `worktree` `set_mode` `request_permission` |
| **按钮用动词短语** | ✓ `创建 worktree 并开始` `请求 push 授权` `打开证据抽屉` ／ ✗ `确定` `提交` `OK` |
| **破坏性动作写清后果** | ✓ `丢弃 2:14 工作` ／ ✗ `取消` |
| 语气工程化、精准、可核对 | ✓ 「单元 unit-012 契约 v3 已冻结」／ ✗ 「我已经准备好开始写这块了 🎉」 |

`zh-CN.json` 里出现「确定」「提交」「保存」「OK」「Yes」这类空动词按钮文案，
评审直接驳回——**空动词是文案缺陷，不是风格偏好**。

---

## 12. 平台适配层的前端约束

接口定义与降级表在 [`architecture.md`](architecture.md) §5，**不在本文重复**。前端的硬约束：

### 12.1 唯一的例外目录

> **除 `src/platform/` 外，任何地方禁止 import `@tauri-apps/*`。**

由 §13.3 的 ESLint `no-restricted-imports` 强制。违反即 lint 红，不是"注意一下"。

组件侧只认这一个入口：

```ts
import { platform } from '@/platform'
await platform.pickDirectory({ title: '选择本机 Git 仓库' })
```

`src/platform/index.ts` 在运行时按 `window.__DUET__` 是否存在选择实现：

```
src/platform/
├── index.ts            运行时选择 tauri / web，导出单例 platform
├── types.ts            PlatformAdapter 接口（唯一真源）
├── tauri/              ★ 唯一允许 import @tauri-apps/* 的地方
└── web/                浏览器降级实现
```

### 12.2 Web 降级必须真的可用

**它是 AI 自测的主要通道**（`dev-web` 是默认开发形态）。空实现、
`throw new Error('not supported')`、静默 no-op 一律不合格。

| 能力 | Web 降级的验收标准 |
|---|---|
| 选择文件夹 / 文件 | 弹出手输路径的 `Dialog`，提交后调后端校验存在性并回显结果 |
| 在 Finder 中显示 | 复制路径到剪贴板 + 明确的成功反馈 |
| 在编辑器打开 | 复制路径 + 明确反馈 |
| 窗口控制 | 窗口栏的窗口控制按钮整体隐藏（不是置灰） |
| 自动更新 | 显示「有新版本 1.4.2 → 1.5.0」但更新按钮 `disabled`，附下载链接 |

每条降级都要有一个组件测试断言"用户能看到什么、点了之后发生什么"。

> 「在 Finder 中显示」在 Web 下需要一个成功反馈，而设计规范**没有 toast / 内联反馈条目**——
> 这是已登记的缺口 G3，先补设计条目再实现。

---

## 13. 设计合规 lint

设计系统自带一份合规配置 `design/_ds/<nocturne>/_adherence.oxlintrc.json`，三条规则：
**禁止裸 hex 字面量、禁止裸 px 字面量、字体只能 Inter**。
本仓库不跑 oxlint，把它转译成 ESLint + Stylelint。

> **规则改了但没有强制手段 = 规则没改**（[`coding-standards.md`](coding-standards.md) §6）。
> 本节的每条规则都必须真的进配置文件，不是写在文档里就算数。

### 13.1 Stylelint（`frontend/.stylelintrc.json`）

```json
{
  "extends": ["stylelint-config-standard"],
  "rules": {
    "color-no-hex": [true, { "message": "禁止 hex 字面量，用 var(--color-*)（Duet Spec §01）" }],
    "color-named": ["never", { "message": "禁止颜色关键字，用 var(--color-*)" }],
    "function-disallowed-list": [
      ["rgb", "rgba", "hsl", "hsla", "oklch", "lch", "color-mix"],
      { "message": "颜色只能来自令牌，禁止在组件里构造颜色" }
    ],
    "declaration-property-unit-disallowed-list": {
      "margin": ["px"], "margin-top": ["px"], "margin-right": ["px"],
      "margin-bottom": ["px"], "margin-left": ["px"],
      "padding": ["px"], "padding-top": ["px"], "padding-right": ["px"],
      "padding-bottom": ["px"], "padding-left": ["px"],
      "gap": ["px"], "row-gap": ["px"], "column-gap": ["px"],
      "border-radius": ["px"],
      "font-size": ["px"],
      "width": ["px"], "height": ["px"],
      "top": ["px"], "right": ["px"], "bottom": ["px"], "left": ["px"], "inset": ["px"]
    },
    "declaration-property-value-allowed-list": {
      "font-family": ["/^var\\(--font-(heading|body|mono)\\)$/"],
      "z-index": ["/^var\\(--z-[a-z]+\\)$/"]
    },
    "declaration-no-important": true,
    "selector-class-pattern": "^[a-z][a-zA-Z0-9]+$"
  },
  "overrides": [
    {
      "files": ["src/design/tokens.css", "src/design/duet.css"],
      "rules": {
        "color-no-hex": null,
        "function-disallowed-list": null,
        "declaration-property-unit-disallowed-list": null,
        "declaration-property-value-allowed-list": null
      }
    }
  ]
}
```

**关于 px 的现实说明：** 设计里确实存在 1px 发丝线、1.5px 描边、2px 竖条、
13px 复选框这类不可令牌化的物理尺寸。规则的边界是：
`border-width` / `box-shadow` / `stroke-width` **不禁 px**；
一切与**间距、圆角、字号、布局尺寸**相关的属性**禁 px**，必须走令牌或 `--layout-*`。
需要新数值 → 先在 `duet.css` 立令牌，不许就地写 px。

### 13.2 ESLint —— 设计合规部分（`frontend/eslint.config.js`）

```js
{
  rules: {
    'no-restricted-syntax': [
      'error',
      {
        selector: "Literal[value=/#[0-9a-fA-F]{3}(?:[0-9a-fA-F]{3}(?:[0-9a-fA-F]{2})?)?\\b/]",
        message: '禁止 hex 颜色字面量，用 CSS Modules + var(--color-*)（Duet Spec §01）',
      },
      {
        selector: "TemplateElement[value.raw=/#[0-9a-fA-F]{3,8}\\b/]",
        message: '禁止 hex 颜色字面量（模板字符串里也不行）',
      },
      {
        selector: "JSXAttribute[name.name='style'] Property[key.name]",
        message: '内联 style 只允许字符串字面量键，且必须是 --* 自定义属性',
      },
      {
        selector: "JSXAttribute[name.name='style'] Property[key.value!=/^--/]",
        message: '内联 style 只允许注入 --* 自定义属性；其余样式写进同名 *.module.css',
      },
      {
        selector: "Literal[value=/[\\u{1F000}-\\u{1FAFF}\\u{2190}-\\u{21FF}\\u{2600}-\\u{27BF}\\u{FE0F}]/u]",
        message: '禁止 emoji / 装饰性 Unicode 符号；图标从 Phosphor regular 取（Duet Spec §04）',
      },
      {
        selector: "Literal[value=/[◧⧉⚑◑◔▤◫▾▴◆◇■□●○★☆]/]",
        message: '禁止用 Unicode 几何符号代替图标；例外只有 ⌄ ✕ ⠿ ✓ ✗（Duet Spec §04）',
      },
      {
        selector: "JSXOpeningElement[name.name='svg']",
        message: '禁止自绘 SVG 图标；图标走 <Icon>。数据图表例外，只能放 src/ui/chart/',
      },
      {
        selector: "JSXOpeningElement[name.name='select']",
        message: '禁止原生 select 外观，用 <Dropdown>（Duet Spec §05）',
      },
      {
        selector: "JSXOpeningElement[name.name='input'] > JSXAttribute[name.name='type'][value.value=/^(checkbox|radio)$/]",
        message: '禁止原生 checkbox/radio 外观，用 <Checkbox> / <Radio>（Duet Spec §05）',
      },
      {
        selector: "Property[key.name='zIndex']",
        message: 'z-index 只能是 var(--z-*)，写在 CSS Modules 里（Duet Spec §03）',
      },
    ],
  },
}
```

`src/ui/chart/` 与 `src/ui/Icon.tsx` 需要 `overrides` 放行对应的 `svg` / `i` 规则。

### 13.3 ESLint —— 分层与依赖部分

```js
{
  rules: {
    'no-restricted-imports': ['error', {
      patterns: [
        {
          group: ['@tauri-apps/*'],
          message: '只有 src/platform/tauri/ 可以 import @tauri-apps/*（architecture.md §5）',
        },
        {
          group: ['@/features/*/*'],
          message: 'feature 之间不许互相深引用，需要共享就抽到 src/ui 或 src/models',
        },
      ],
    }],
  },
  overrides: [
    { files: ['src/platform/tauri/**'], rules: { 'no-restricted-imports': 'off' } },
  ],
}
```

命名、导入顺序、`enum` 禁用等规则见 [`coding-standards.md`](coding-standards.md) §4，本文不重复。

### 13.4 类型层强制（比 lint 更靠谱的部分）

有些规则用类型表达比用 lint 表达更彻底，`pnpm typecheck` 就是它们的检查命令：

| 设计规则 | 类型层怎么强制 |
|---|---|
| 纯图标按钮必须有中文 tooltip | `IconButtonProps.tooltip: string`（必填，非可选） |
| 图标只能从已登记集合里取 | `IconName = (typeof ICON)[keyof typeof ICON]` |
| 图标只有四档尺寸 | `size?: 16 \| 15 \| 13 \| 12` |
| 抽屉宽度只有三档 | `width: 560 \| 720 \| 980` |
| 13 类事件全部有渲染器 | `{ [T in EventType]: EventRenderer<T> }` |
| `app` 事件必须可下钻 | 判别联合，`source:'app'` 分支要求 `openTarget` |
| 决策等级只有四级 | `level: 'D0' \| 'D1' \| 'D2' \| 'D3'` |
| 状态词不翻译 | `StatusTextProps.value: WorkState`（来自 openapi 生成的联合类型） |

### 13.5 测试层强制

| 规则 | 测试 |
|---|---|
| 过滤器项数 === 事件类型数 | 断言 `filterItems.length === Object.keys(EVENT_RENDERERS).length` |
| 计划面板是纯 overlay | 断言开关面板前后对话列首条消息的位置不变 |
| 13 类事件各自的渲染形态 | 每类一个渲染测试（`testing-strategy.md` §6） |
| Web 降级真的可用 | 每条降级一个组件测试 |

**禁止在组件测试里断言 CSS 类名或 DOM 结构**（`testing-strategy.md` §6）——
视觉合规交给 Stylelint / ESLint，行为交给测试，两者不越界。

### 13.6 命令

```bash
pnpm -C frontend lint        # ESLint + Stylelint
pnpm -C frontend typecheck   # tsc --noEmit
make lint-frontend           # 上面两条
make check                   # 提交前必跑
```

---

## 14. 新增组件的流程

**顺序不可调换：**

```
① 在 design/Duet Spec.dc.html 里找条目
        │
        ├── 找到  ──▶ ③ 照条目实现
        │
        └── 找不到 ──▶ ② 先在设计规范新增条目（回 Claude Design 项目改，再同步下来）
                             │
                             └──▶ ③ 实现
```

- **禁止临时发明样式。** 这是铁律 3，违反即回滚。
- 找不到条目时**停下来上报**（AGENTS.md §3 停止条件），不要"先按感觉写、以后补规范"。
- `design/` 目录是**只读**的，前端任务不许直接改那两个 HTML；
  发现缺口先按 §16 的格式登记到本文，再由设计侧回填。
- 新条目至少要写清：**用在哪 · 三态（默认/hover/选中）· 具体数值 · 禁止事项**。
  写不出这四项说明还没想清楚。

新组件落地时同步要做的：

- [ ] 文件放对位置（`ui/` 原语 vs `features/` 组合体）
- [ ] `Props` 类型名 = `<组件名>Props`，定义在组件文件内
- [ ] 同名 `.module.css`，五个交互状态都写全（§10.1）
- [ ] 纯图标控件有 `tooltipProps()`
- [ ] 组件测试 + 在 `frontend/tests/INDEX.md` 登记
- [ ] `frontend/src/` 下新建关键目录 → 补 `AGENTS.md` + `CLAUDE.md`（AGENTS.md §4.1）

---

## 15. 反例清单

**设计侧（设计规范第 10 节，出现即不合规）：**

| ✗ |
|---|
| 写死 hex 或新造颜色（含语义色——语义色只有 `--color-pass` / `--color-fail`） |
| emoji、Unicode 几何符号当图标，或自绘 SVG 图标 |
| 实心填充的主按钮、彩色渐变按钮 |
| 悬浮层影响正文布局，或被父级 `overflow:hidden` 裁切 |
| 同类选择器在不同页面位置不一致 |
| 纯图标按钮没有中文 tooltip |
| 设计 ACP 不支持的设置项（如按角色设模型——模型不在协议里） |
| 用弹窗打断执行中的单元来展示非阻塞信息 |
| 结论不带证据入口（diff / 测试输出 / 命令记录） |
| 自动把聊天原文或一次成功经验写成长期记忆 |

**前端工程侧（本文补充）：**

| ✗ | ✓ |
|---|---|
| `import { open } from '@tauri-apps/plugin-dialog'` 出现在 `src/features/` | `platform.pickDirectory()` |
| 组件里 `fetch('/v1/works')` | `src/api/` 生成客户端 + TanStack Query |
| SSE 事件塞进 TanStack Query 缓存 | 写入 Zustand event slice |
| 另建一份过滤器项列表 | 由 `EVENT_RENDERERS` 生成 |
| 新增事件类型只改前端注册表 | 四处同改（§9.5） |
| `z-index: 9999` | `z-index: var(--z-drawer)` |
| `style={{ color: '#9184d9' }}` | `className={styles.accent}` |
| `outline: none` 去掉 focus 环 | 继承设计系统的 focus-visible |
| `overflow: hidden` 裁长列表 | `min-height: 0 + overflow-y: auto` |
| 输入区卡片 `overflow: hidden` | `position: relative` + 首尾行补圆角 |
| 五段输入区做成可配置 `slots` | 顺序写死在 `Composer.tsx` |
| 计划面板参与 flex 布局 | `position: absolute` 纯 overlay |
| 页面内再放一个折叠开关 | 折叠开关只在窗口栏 |
| 引入 tooltip / dropdown / modal 第三方库 | 自绘，走本文规格 |
| 引入第二种西文字体 | 只有 Inter + `--font-mono` |
| 断言 CSS 类名的组件测试 | 断言可见文本与交互后状态 |
| `enum EventType {}` | `const EVENT_TYPE = {} as const`（coding-standards §4.1） |

---

## 16. 已知设计缺口登记

**这些界面元素在原型里存在或被架构要求，但设计规范第 05 节找不到对应条目。**
按 §14，实现它们之前必须先补设计条目。**在缺口被填上之前，不许凭感觉实现。**

| # | 缺口 | 证据 | 影响 |
|---|---|---|---|
| G1 | **数据图表**（报表页折线面积图 + 环形图）没有组件条目 | 原型报表页两处内联 `<svg>`；用了 `--color-accent-500`，而规范强调阶只列 900/700/600/400/300 | 报表页无法开工；且与「禁止自绘 SVG」规则字面冲突，需要写清「图标 ≠ 数据图表」 |
| G2 | **搜索 / 命令面板（⌘K）** 没有条目 | 原型窗口栏有 `title="搜索 ⌘K"` 的图标按钮，但没有面板设计 | 点了没地方去 |
| G3 | **toast / 内联反馈** 没有条目，也没有 z-index 层 | `architecture.md` §5 要求 Web 降级「复制路径到剪贴板 + toast」 | Web 降级无法给出反馈；`--z-*` 缺一层 |
| G4 | **表格** 没有组件条目 | 原型有三张真表：项目管理、GitHub 按仓库绑定、角色与 Runtime 绑定。规范只在第 02 节提了一句「表格单元 12.5」 | 表头 / 行分隔 / 列对齐 / 行内下拉全无规格 |
| G5 | **文本输入框 / 文本域** 没有条目 | 原型里没有一个真的 `<input>`，全是 div 模拟 | placeholder 色、focus 态、选中高亮、校验错误态全缺 |
| G6 | **空状态 / 骨架屏 / 错误态 / SSE 断线提示** 没有条目 | 原型全是有数据的理想态 | 首次进入、加载中、请求失败、断线重连四种界面无依据 |
| G7 | **macOS 红绿灯** 在布局示意图里用了写死 hex（`#ff5f57` `#febc2e` `#28c840`） | `Duet Spec.dc.html` 第 06 节 | 需明确「红绿灯由原生渲染，前端不画」，否则会被照抄成硬编码颜色 |
| G8 | **2px / 3px / 6px 圆角**无令牌 | 进度条 `border-radius:2px`、tooltip `6px` | 本文已临时补 `--radius-xs` / `--radius-tip`，需回填设计规范 |
| G9 | **`--color-accent-500` / `--color-accent-200`** 未列入强调阶用法表 | accent-200 用于开关圆点、accent-500 用于图表 | 阶梯表不完整，容易被当成"可以随便挑一阶" |
| G10 | **动效 / 过渡**全文没有任何规定 | 设计稿零 `transition` / `animation`；但抽屉写着「右侧滑入」、加载写着「圆环」 | 时长、缓动、是否尊重 `prefers-reduced-motion` 全无依据 |
| G11 | **敏感值掩码**（`ghp_FaJhrnU0aCTGrF9H••••••••••••`）无条目 | 原型 GitHub 账号页 | 掩码位数、是否可展开、复制行为无规格 |
| G12 | **设置页 6 个 tab 的选中态**与第 08 节冲突 | 原型 tab 选中＝`--color-surface` 底 + `--color-text` 字；规范说选中＝`accent-900` 底 + `accent-300` 字 | 需要设计侧裁定：是新增「分段 tab」条目，还是改原型 |
| G13 | **面包屑**只在布局规则⑤被提到一句，没有视觉条目 | 原型顶部有 `crumbRoot / frameTitle / crumb2` 三级 | 分隔符、截断、可点范围无规格 |

发现新缺口 → **追加到本表**（不要删除已有行），同时按 §14 停下来上报。
缺口被设计侧填上后，把对应行移除并在提交说明里指出补上的设计条目。

---

## 17. AI 拟定的补充条目

> 这些是设计规范里**找不到条目**、但界面上确实需要的东西。
> 全部**从既有令牌与设计规则推导**，不是发明——每条都注明推导依据。
>
> 裁定见 [`adr/0006-open-question-rulings.md`](adr/0006-open-question-rulings.md)。
> **设计侧可推翻**：推翻时回 Claude Design 项目改 `Duet Spec.dc.html`，
> 同步下来后删掉这里对应条目。

### 17.1 文本输入框 / 文本域

沿用设计系统的 `.input`（surface 底、divider 描边、accent caret、focus 描边转 accent），
只调到 Duet 的密集档：

| 项 | 值 | 依据 |
|---|---|---|
| 高 | 24–26px | §03 与按钮对齐 |
| 字号 | 11.5–12px | §02「按钮、芯片、说明文字」档 |
| padding | `0 var(--space-2)` | §03 |
| placeholder | `--color-neutral-600` | §01「弱化」 |
| 校验错误 | 描边转 `--color-fail`，下方 11px fail 色说明 | §01 语义色只有两个 |
| 只读 | `opacity: .6` | §05 失效态 |

**多行输入（对话区）默认一行，随内容增高**，上限 40vh 后内部滚动——
设计稿的输入区五段结构里明写「多行输入（默认一行）」。

### 17.2 表格

沿用 `.table`（行级渐变分隔线、`th` 10px uppercase），调密集档：

| 项 | 值 | 依据 |
|---|---|---|
| 单元格字号 | 12.5px | §02 明写「表格单元 12.5」 |
| 单元格 padding | `var(--space-2)` | §03 |
| 行 hover | 既有的 4% text 叠加 | 设计系统已定义 |
| 等宽列（路径、ID、hash） | `var(--font-mono)` | §02「能复制粘贴的一律等宽」 |

### 17.3 空状态 / 加载 / 错误 / 断线

| 状态 | 规格 | 依据 |
|---|---|---|
| 空状态 | 居中单行，`--color-neutral-600`，12.5px。**不画插图** | §09 语气「工程化、精准、可核对」 |
| 加载中 | 1.5px `--color-accent-400` 圆环，**不用旋转图标** | §08 已规定 |
| **不做骨架屏** | 用上面的圆环 | 骨架屏是另一种视觉语言，与本系统不搭 |
| 错误态 | inset 卡 + `--color-fail` 文字 + 次操作档的「重试」按钮 | §01 危险动作用文字色不用红底 |
| SSE 断线 | 窗口栏右侧 6px `--color-fail` 圆点 + 中文 tooltip「事件流已断开，正在重连」 | §08 纯图标控件必须有 tooltip |

### 17.4 toast

| 项 | 值 |
|---|---|
| z-index | `--z-toast: 40`（在 `--z-drawer: 26` 与 `--z-tooltip: 60` 之间） |
| 位置 | 右下角，距边 `var(--space-6)` |
| 样式 | `--s-raised` 底 + `--shadow-lg` + `--radius-md` |
| 时长 | 3s 自动消失，可手动关（✕） |
| 数量 | 同时最多 3 条，超出替换最旧的 |

**toast 只承载非阻塞信息。** 需要用户决定的东西一律走对话框——
§10 反例明确禁止「用弹窗打断执行中的单元来展示非阻塞信息」，反过来也成立。

### 17.5 动效

| 场景 | 时长 | 缓动 |
|---|---|---|
| hover / 焦点 | 120ms | `ease-out` |
| 抽屉滑入滑出 | 180ms | `cubic-bezier(.2, 0, 0, 1)` |
| 浮层淡入 | 120ms | `ease-out` |
| 加载圆环 | 800ms 循环 | `linear` |

```css
@media (prefers-reduced-motion: reduce) {
  /* 全部置 0，包括加载圆环——改为静态描边 + 文字 */
  *, *::before, *::after {
    animation-duration: 0.01ms !important;
    transition-duration: 0.01ms !important;
  }
}
```

推导依据：这个系统的基调是克制（「强调色只做线与光」「加载中不用旋转图标」），
动效也该短促、不炫技。

### 17.6 敏感值掩码

```
ghp_FaJhrnU0aCTGrF9H••••••••••••
└──── 前 20 字符明文 ────┘└─ 固定 12 个 • ─┘
```

| 规则 | 理由 |
|---|---|
| 圆点数**固定 12 个**，不反映真实长度 | 反映长度会泄漏信息 |
| **不提供展开** | 需要核对时用「验证」按钮（设计稿已有），由后端去验 |
| 提供「复制」按钮 | 复制的是完整令牌 |
| 等宽字体 | §02「能复制粘贴的一律等宽」 |

### 17.7 数据图表（不受「禁止自绘 SVG」约束）

§04 的禁令原文是「禁止 emoji、自绘 SVG **图标**」——上下文是**图标来源**。
数据图表用内联 SVG 是唯一合理方式，不在禁令内。

| 项 | 值 |
|---|---|
| 主序列 | `--color-accent-600` 单色 |
| 多序列 | **同色不同透明度**（100% / 60% / 35%），**不引入新颜色** |
| 网格线 | `--color-neutral-800`，1px |
| 轴标签 | 10px `--color-neutral-600`，等宽 |
| 面积填充 | 主序列色 12% 透明度 |

**不引入第三种语义色** —— §01 明确「语义色只有两个」。

### 17.8 面包屑

| 段 | 颜色 | 交互 |
|---|---|---|
| 中间段 | `--color-neutral-500` | 可点击，hover 转 `--color-accent-300` |
| 分隔符 `/` | `--color-neutral-700` | — |
| 末段（当前） | `--color-neutral-300` | 不可点 |

字号 11.5px。**不放「回到对话」按钮** —— §06 规则⑤明确要求二级页面
用「面包屑 + 返回上一处」。

### 17.9 补充的令牌

```css
--radius-xs:   2px;   /* 进度条填充 */
--radius-2xs:  3px;   /* 角标 */
--radius-pill: 999px; /* 筛选胶囊、开关 */
--z-toast:     40;
```

强调阶补两条用法：

| 令牌 | 用途 |
|---|---|
| `--color-accent-500` | 进度条填充的 hover 态 |
| `--color-accent-200` | 极少用：仅用于 accent-900 底上需要更高对比的文字 |

### 17.10 设置页 tab 的选中态**不违反** §08

§08 的「选中 = accent-900 底 + accent-300 字」管的是**选择控件**
（列表项、下拉项、筛选胶囊）—— 它们表达「我选了这个值」。

**tab 是导航**，表达「我在这一页」。导航的当前项用 `--color-surface` 底 +
`--color-text` 字是对的：否则页面上会同时有两处 accent 竞争
（tab 的当前项 + 页内真正的选中项），注意力被分散。

**这两类控件的区分要写进设计规范**，当前先记在这里。
