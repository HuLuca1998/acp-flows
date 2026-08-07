import { expect, test } from '@playwright/test'

/**
 * 布局不变量。**这类问题单测与类型检查一个都查不出来。**
 *
 * 起因是一个真实的 bug：鼠标移到窗口栏最右侧的折叠按钮上时，
 * 它的 tooltip（`[data-tt]:hover::after`，默认居中对齐）溢出视口右缘，
 * 把整个页面撑出了横向滚动条。
 *
 * 这类「悬浮层溢出视口」在 AI 写前端时特别容易漏：
 * 组件测试渲染在 jsdom 里没有视口概念，tsc 更不可能发现。
 * 只有真的把鼠标移上去才看得见——所以必须自动化。
 */
test.describe('布局不变量', () => {
  /** 视口不该出现横向滚动条：出现了就说明有东西溢出了。 */
  async function horizontalOverflow(page: import('@playwright/test').Page) {
    return page.evaluate(() => {
      const el = document.documentElement
      return el.scrollWidth - el.clientWidth
    })
  }

  test('初始渲染不出横向滚动条', async ({ page }) => {
    await page.goto('/')
    expect(await horizontalOverflow(page)).toBe(0)
  })


  // ★ 设置页是**两栏**（左侧 196px 定宽导航），比其它页更容易撑宽：
  // 右栏里有一条完整的可执行文件路径，而 flex 子项默认 min-width: auto。
  //
  // ★★ 这里**不能只看 documentElement 的溢出**。主区自己有 overflow，
  // 内部撑宽会被它吸收成一条内部滚动条，页面级看起来毫无异常——
  // 第一版就是这么写的，把导航栏宽度改成 2000px 都测不红。
  // 所以要逐个元素问「你自己溢出了吗」。
  test('设置页在窄窗口下不出横向滚动条，切换分区也不出', async ({ page }) => {
    /**
     * 找出**会把父容器撑破**的元素。
     *
     * ★ 必须跳过自己声明了 overflow 的元素。设了 `overflow: hidden` +
     * `text-overflow: ellipsis` 的元素，`scrollWidth` 本来就是完整内容宽度——
     * 那正是省略号生效的表现，不是 bug。不跳过的话，每加一个带省略号的
     * 长路径都会让这条测试变红，而它其实工作得好好的。
     *
     * 判据是「内容溢出**且**没有做任何 overflow 处理」——只有这种才会
     * 一路传播上去，最终表现为整页的横向滚动条。
     */
    async function widestOverflow(p: import('@playwright/test').Page) {
      return p.evaluate(() => {
        let worst = { tag: '', by: 0 }
        for (const el of document.querySelectorAll<HTMLElement>('body *')) {
          const ox = getComputedStyle(el).overflowX
          if (ox !== 'visible') continue // hidden / auto / scroll 都由元素自己兜住了
          const by = el.scrollWidth - el.clientWidth
          // clientWidth 为 0 的不参与布局（隐藏、内联），跳过
          if (el.clientWidth > 0 && by > worst.by) {
            worst = { tag: `${el.tagName}.${el.className}`.slice(0, 60), by }
          }
        }
        return worst
      })
    }

    await page.setViewportSize({ width: 1024, height: 768 })
    await page.goto('/')

    await page.getByRole('button', { name: '设置' }).click()
    const nav = page.getByRole('tablist', { name: '设置分区' })
    await expect(nav).toBeVisible()

    let worst = await widestOverflow(page)
    expect(worst.by, `刚进设置页就有元素横向溢出：${worst.tag}`).toBe(0)

    // 逐个分区切过去——每个分区的内容宽度不一样，只测第一个等于没测
    const tabs = nav.getByRole('tab')
    const count = await tabs.count()
    expect(count, '设置页应有六个分区').toBe(6)
    for (let i = 0; i < count; i++) {
      const name = (await tabs.nth(i).textContent()) ?? `第 ${i} 项`
      await tabs.nth(i).click()
      worst = await widestOverflow(page)
      expect(worst.by, `切到「${name}」后 ${worst.tag} 横向溢出`).toBe(0)
    }
  })

  // ★ 逐个 hover 每个纯图标控件。
  // 设计规范要求所有纯图标控件都有中文 tooltip（§08），
  // 而 tooltip 正是最容易溢出的那一类元素。
  test('悬停任何图标按钮都不撑出横向滚动条', async ({ page }) => {
    await page.goto('/')

    const tipped = page.locator('[data-tt]')
    const count = await tipped.count()
    expect(count).toBeGreaterThan(0)

    for (let i = 0; i < count; i++) {
      const el = tipped.nth(i)
      if (!(await el.isVisible())) continue

      await el.hover()
      const overflow = await horizontalOverflow(page)
      const name = await el.getAttribute('data-tt')
      expect(overflow, `悬停「${name ?? '?'}」时页面被撑出 ${overflow}px 横向滚动条`).toBe(0)
    }
  })

  // 折叠/展开是最容易把宽度算错的操作。
  test('折叠与展开侧栏都不撑出横向滚动条', async ({ page }) => {
    await page.goto('/')

    const toggle = page.getByRole('button', { name: /折叠侧栏|Toggle sidebar/ })
    for (let i = 0; i < 2; i++) {
      await toggle.click()
      expect(await horizontalOverflow(page)).toBe(0)
    }
  })

  // 规范 §06 规则②：折叠后左栏**保留 48px 图标条**，不是整条消失。
  test('左栏折叠后仍保留图标条', async ({ page }) => {
    await page.goto('/')

    const rail = page.getByRole('complementary').first()
    const before = (await rail.boundingBox())?.width ?? 0
    expect(before).toBeGreaterThan(200)

    await page.getByRole('button', { name: /折叠侧栏|Toggle sidebar/ }).click()

    const after = (await rail.boundingBox())?.width ?? 0
    expect(after, '折叠后应保留图标条而不是整条消失').toBeGreaterThan(0)
    expect(after).toBeLessThan(before)
  })

  // 规范 §06 规则②：左栏可拖 180–420，右栏可拖 220–460。
  // ★ 拖到边界要**钳制**而不是继续跟随——让栏宽超出范围会把主区挤没，
  // 而用户很难把它拖回来。
  test('左栏可拖动改宽，且被钳制在 180–420', async ({ page }) => {
    await page.goto('/')

    const rail = page.getByRole('complementary').first()
    const handle = page.getByRole('separator').first()
    const box = await handle.boundingBox()
    if (box === null) throw new Error('找不到拖拽手柄')

    // 往右狠拖，远超上限
    await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2)
    await page.mouse.down()
    await page.mouse.move(box.x + 900, box.y + box.height / 2, { steps: 8 })
    await page.mouse.up()

    const widened = (await rail.boundingBox())?.width ?? 0
    expect(widened, '拖过上限时应被钳制在 420').toBeLessThanOrEqual(421)
    expect(widened).toBeGreaterThan(252)

    // 往左狠拖，远超下限
    const box2 = await handle.boundingBox()
    if (box2 === null) throw new Error('找不到拖拽手柄')
    await page.mouse.move(box2.x + box2.width / 2, box2.y + box2.height / 2)
    await page.mouse.down()
    await page.mouse.move(box2.x - 900, box2.y + box2.height / 2, { steps: 8 })
    await page.mouse.up()

    const narrowed = (await rail.boundingBox())?.width ?? 0
    expect(narrowed, '拖过下限时应被钳制在 180').toBeGreaterThanOrEqual(179)

    // 拖动全程不得撑出横向滚动条
    expect(await horizontalOverflow(page)).toBe(0)
  })
})
