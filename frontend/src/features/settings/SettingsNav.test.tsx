import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { SettingsPage } from './index'

// M0 U0.3.3 · 设置页的两栏二级导航
//
// ★ 这个单元是**补登记**的：U0.3.1 当时标了 ✓，但它的验收标准里没有一条
// 管布局结构，所以「做成了单栏堆叠」这个偏离没有任何检查会红。
// 这份测试就是把结构本身变成断言。

const listRuntimes = vi.fn()
const checkUpdate = vi.fn()
const prepareUpdate = vi.fn()

vi.mock('@/api/system', () => ({
  listRuntimes: (...args: unknown[]): unknown => listRuntimes(...args),
  checkUpdate: (...args: unknown[]): unknown => checkUpdate(...args),
  prepareUpdate: (...args: unknown[]): unknown => prepareUpdate(...args),
}))

beforeEach(() => {
  listRuntimes.mockReset().mockResolvedValue([
    { name: 'claude', status: 'ready', installed: true, authenticated: true, active_version: '0.63.0' },
  ])
  checkUpdate.mockReset().mockResolvedValue({ state: 'up_to_date', current_version: '1.4.2' })
  prepareUpdate.mockReset()
})

// 设计稿 design/ACP Duet 1a.dc.html 的设置页左栏，逐字。
const NAV = [
  ['ACP Runtime', '检测 · 安装 · 版本'],
  ['环境检测', 'node · git · cargo'],
  ['应用更新', '当前版本与一键更新'],
  ['项目管理', '添加 · 移除 · 默认分支'],
  ['GitHub 账号', '令牌 · 按仓库绑定'],
  ['通用', '语言 · 启动 · 数据目录'],
] as const

describe('设置页二级导航', () => {
  it('左栏恰好六项，名字与副标题逐字照设计稿且顺序一致', () => {
    render(<SettingsPage />)

    const nav = screen.getByRole('tablist', { name: /设置分区/ })
    const items = within(nav).getAllByRole('tab')
    expect(items).toHaveLength(NAV.length)

    NAV.forEach(([name, sub], i) => {
      expect(items[i]).toHaveTextContent(name)
      expect(items[i]).toHaveTextContent(sub)
    })
  })

  // ★ 副标题里**不许出现编造的数字**。
  // 设计稿写的是「1.4.2 → 1.5.0」「3 个项目」——那是示意数据。
  // 照抄的话，一个还没接项目功能的应用会告诉用户他有 3 个项目
  // （App.test.tsx 已有同款规则：编造数据比空白更糟）。
  it('副标题里没有编造的数字', () => {
    render(<SettingsPage />)

    const nav = screen.getByRole('tablist', { name: /设置分区/ })
    for (const item of within(nav).getAllByRole('tab')) {
      const sub = item.querySelector('[data-sub]')?.textContent ?? ''
      expect(sub).not.toMatch(/\d/)
    }
  })

  it('同一时刻只渲染选中的那一个分区', async () => {
    const user = userEvent.setup()
    render(<SettingsPage />)

    // 默认落在第一项
    expect(screen.getByRole('heading', { name: 'ACP Runtime 检测' })).toBeInTheDocument()

    await user.click(screen.getByRole('tab', { name: /通用/ }))

    expect(screen.queryByRole('heading', { name: 'ACP Runtime 检测' })).not.toBeInTheDocument()
    expect(screen.getByText('简体中文')).toBeInTheDocument()
  })

  // ★ 每切一次分区就重新探测的话，探测要拉起子进程，
  // 用户会感到设置页「点一下卡一下」。
  it('来回切分区不会重新发起检测请求', async () => {
    const user = userEvent.setup()
    render(<SettingsPage />)

    await screen.findByText('claude')
    expect(listRuntimes).toHaveBeenCalledTimes(1)

    for (const name of ['通用', '项目管理', 'ACP Runtime', 'GitHub 账号', 'ACP Runtime']) {
      await user.click(screen.getByRole('tab', { name: new RegExp(name) }))
    }

    expect(listRuntimes).toHaveBeenCalledTimes(1)
  })

  it('选中项在无障碍树上是唯一的', async () => {
    const user = userEvent.setup()
    render(<SettingsPage />)

    await user.click(screen.getByRole('tab', { name: /GitHub 账号/ }))

    const selected = screen.getAllByRole('tab').filter((t) => t.getAttribute('aria-selected') === 'true')
    expect(selected).toHaveLength(1)
    expect(selected[0]).toHaveTextContent('GitHub 账号')
  })
})
