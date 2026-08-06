---
name: web-ui-test
description: 测试前端界面时使用。触发场景：写或改 React 组件测试、写 Playwright 端到端用例、需要在真实浏览器里模拟用户操作验证一个页面、验收设计稿还原度、排查「代码看着对但页面不对」的问题。覆盖三层——组件行为测试、端到端自动化、真实浏览器人工走查。
---

# 前端测试三层

> 完整策略见 [`docs/testing-strategy.md`](../../docs/testing-strategy.md) §6。
> 设计合规规则见 [`docs/frontend-guide.md`](../../docs/frontend-guide.md)。

选哪一层：

| 要验证的东西 | 用哪层 |
|---|---|
| 组件在某状态下渲染成什么、交互后变成什么 | ① 组件行为测试 |
| 跨页面、跨前后端的完整流程 | ② 端到端自动化 |
| 视觉还原度、布局、悬浮层、拖拽手感 | ③ 真实浏览器走查 |

---

## ① 组件行为测试（Vitest + Testing Library）

### 按用户看得见的东西查询

```tsx
// ✓ 用户能看见 / 能操作的
screen.getByRole('button', { name: '请求 push 授权' })
screen.getByText('waiting_user')
await user.click(screen.getByRole('button', { name: '允许一次' }))

// ✗ 绑死实现，设计稿一改就全红且毫无价值
container.querySelector('.event-row')
screen.getByTestId('event-row-3')
expect(el).toHaveClass('accent-700')
```

**禁止断言 CSS 类名和 DOM 结构。** 设计会改，这些测试只会变成噪音。
`data-testid` 只在实在没有可访问名称时用，且要写注释说明为什么。

### mock API 用 MSW，handler 从契约生成

```ts
import { handlers } from '@/../tests/msw/handlers'   // 由 api/openapi.yaml 生成
```

**不要手写 mock 响应对象。** 手写的 mock 与 spec 会漂移，
漂移之后组件测试全绿但联调必炸。spec 改了 → `make gen` → mock 自动跟着变。

### 事件流测试

13 类事件**每类至少一个渲染测试**（见 `docs/architecture.md` §4 的表）。
用 `tests/fixtures/` 里录制的事件序列 + Fake `EventSource`：

```ts
const es = new FakeEventSource()
render(<EventStream workId="work-08" />)
es.emit({ seq: 1, source: 'acp', type: 'tool_call', payload: {...} })
expect(await screen.findByText('crates/engine/src/cancel.rs')).toBeVisible()
```

要覆盖的行为：断线重连、`Last-Event-ID` 续传、**乱序事件按 `seq` 归位**、过滤器开关。

### 登记索引

新增 `*.test.tsx` → 在 [`frontend/tests/INDEX.md`](../../frontend/tests/INDEX.md) 加一行。
**加之前先搜**，别和已有的重复。

```bash
pnpm -C frontend test --run
make check-test-index
```

---

## ② 端到端自动化（Playwright）

跑**真实 `duetd`**（临时数据目录 + Fake ACP Runtime），不是 mock 服务。

### E2E 只测黄金路径

端到端慢且脆。**只覆盖跨越整个系统才能验证的行为**，
单个模块能测出来的一律下沉到 ① 或后端集成测试。

```
创建项目 → 新建工作(切 worktree) → 计划冻结 → 单元执行 → 权限裁决
→ 证据生成 → 单元验收 → 检查点落盘
```

### 写法

```ts
test('黄金路径：从建项目到检查点落盘', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('button', { name: '创建项目' }).click()
  // …用 role/name 定位，不用 CSS 选择器
  await expect(page.getByText('ck-01')).toBeVisible()
})
```

同样**禁止 CSS 选择器定位**，理由同 ①。

```bash
pnpm -C e2e test
pnpm -C e2e test --headed --debug     # 看着它跑
```

新增 spec → 登记 [`e2e/INDEX.md`](../../e2e/INDEX.md)。

---

## ③ 真实浏览器走查（模拟用户）

自动化测不出「看着对不对」。**验收设计还原度、排查布局问题时用这一层。**

### 起服务

```bash
make dev-web        # duetd + vite，http://localhost:5173
```

这是本仓库的**默认开发形态**——不需要 Rust 工具链，浏览器直接开。

### 用浏览器工具驱动

Claude 用 `agent-browser` skill 或 `mcp__Claude_Browser__*` 工具：

1. `preview_start` 打开页面
2. `read_page` 读可访问性树 —— **比截图更适合核对文本与结构**
3. `computer` 做点击 / 输入 / 滚动 / 拖拽
4. `resize_window` 换视口，验证左右栏可拖拽范围（左 180–420 / 右 220–460）
5. `read_console_messages` / `read_network_requests` 排查报错与请求

### 走查清单（对照设计规范）

每次改完 UI 至少过一遍：

- [ ] 悬浮层（计划面板、下拉、tooltip）**不影响正文布局**，且没被父级 `overflow:hidden` 裁掉
- [ ] 长列表容器是 `min-height:0 + overflow-y:auto`，内容没被裁
- [ ] 每个**纯图标按钮**都有中文 tooltip（`title` + `data-tt`）
- [ ] 没有系统默认浅色滚动条
- [ ] 同类选择器在不同页面**位置一致**（页头标题行下方第一行）
- [ ] 主按钮是描边不是实心；没有彩色渐变
- [ ] 状态词是英文等宽（`executing` 不是「执行中」）
- [ ] 焦点可见（2px accent 外轮廓），键盘能走完主流程
- [ ] 没有 emoji / Unicode 几何符号当图标

反例完整清单见 `design/Duet Spec.dc.html` 第 10 节。

### 发现问题怎么办

- 属于当前任务范围 → 直接修，**并补一个能复现它的自动化测试**（① 或 ②）
- 超出范围 → 用 `create-issue` skill 开 issue，写清现象与证据

**只走查不补测试等于没修** —— 下一轮 AI 会再犯一次。

---

## 禁止

- ✗ 断言 CSS 类名、DOM 结构、`data-testid` 兜底
- ✗ 手写 API mock 对象（用 MSW + 生成的 handler）
- ✗ E2E 里塞单元测试能覆盖的用例
- ✗ 只在浏览器里点一遍就说「验证过了」，不留下自动化证据
- ✗ 截图当唯一证据 —— 文本与结构用 `read_page` 核对更可靠
