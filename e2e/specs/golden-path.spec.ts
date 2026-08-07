import { expect, test } from '@playwright/test'

/**
 * 黄金路径。**这条必须一直绿。**
 *
 * 完整链路（M2 S2.13 U2.13.1）：
 *   创建项目 → 新建工作(切 worktree) → 计划冻结 → 单元契约冻结
 *   → 执行 → 权限裁决 → 证据采集 → 独立审查 → 验收 → 检查点落盘
 *
 * 当前是 M0 的冒烟：只证明前后端这条链是通的。
 * 每完成一个 M2 单元就往这里加一段，不要另起 spec。
 */
test.describe('黄金路径', () => {
  test('应用能加载，窗口栏与主区渲染出来', async ({ page }) => {
    await page.goto('/')

    // 用 role/text 定位，**不用 CSS 选择器** —— 设计会改，绑死结构的用例只会变成噪音
    // 窗口栏的三个折叠开关（设计规范 §06 规则①：全部集中在窗口栏）
    await expect(page.getByRole('button', { name: /折叠侧栏|Toggle sidebar/ })).toBeVisible()
    await expect(page.getByRole('button', { name: /计划面板|Plan panel/ })).toBeVisible()
    // 主区渲染出内容（当前是对话页的骨架占位）
    await expect(page.locator('main')).not.toBeEmpty()
  })

  test('后端可达且鉴权生效', async ({ request }) => {
    // 无 token 必须 401 —— 这是「本机其他程序不能静默驱动 Agent」的防线
    const denied = await request.get('/v1/system/version')
    expect(denied.status()).toBe(401)
  })
})
