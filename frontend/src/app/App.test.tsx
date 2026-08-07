import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'

import { App } from './App'
import { hasContextPanel, NAV_PAGES, navPageById, normalizePageId } from './pages'

// M0 U0.1.1 / U0.1.2 · 窗口布局与左栏导航
//
// 验收标准见 docs/plan/milestones/M0-foundation.md S0.1。
// 结构严格照 design/ACP Duet 1a.dc.html —— 布局跟着设计稿做，不自由发挥。

describe('导航注册表', () => {
  // ★ 设计稿里左栏导航**只有 5 项**。
  // 这条曾经写错过：把「对话」和「计划」也做成了导航项，
  // 但对话是主区本身、计划是悬浮面板——那是对信息架构的误读。
  it('左栏导航恰好 5 项，不含对话与计划', () => {
    expect(NAV_PAGES).toHaveLength(5)
    const ids = NAV_PAGES.map((p) => p.id)
    expect(ids).toEqual(['report', 'memory', 'skill', 'roles', 'settings'])
    expect(ids).not.toContain('chat')
    expect(ids).not.toContain('plan')
  })

  it('未知页面标识不是导航页，主区回落到对话', () => {
    expect(navPageById('no-such-page')).toBeNull()
    expect(navPageById(null)).toBeNull()
  })

  // 右栏只在对话出现——其余是全宽内容页。
  it('右栏只在对话主区出现', () => {
    expect(hasContextPanel('chat')).toBe(true)
    for (const p of NAV_PAGES) {
      expect(hasContextPanel(p.id)).toBe(false)
    }
  })

  it('所有导航标题都是 i18n key，不是中文字面量', () => {
    for (const page of NAV_PAGES) {
      expect(page.titleKey).toMatch(/^nav\./)
    }
  })
})

describe('应用骨架', () => {
  it('默认进对话主区，且右栏在场', () => {
    render(<App />)
    expect(screen.getByText(/这里将来是对话时间线/)).toBeInTheDocument()
    expect(screen.getByRole('complementary', { name: '上下文面板' })).toBeInTheDocument()
  })

  it('五个导航页都能打开，没有一个白屏', async () => {
    const user = userEvent.setup()
    render(<App />)

    const nav = screen.getByRole('navigation', { name: '主导航' })
    for (const page of NAV_PAGES) {
      await user.click(within(nav).getByRole('button', { name: labelOf(page.id) }))
      expect(document.querySelector('main')?.textContent ?? '').not.toBe('')
    }
  })

  it('切到非对话页时右栏收起', async () => {
    const user = userEvent.setup()
    render(<App />)

    const nav = screen.getByRole('navigation', { name: '主导航' })
    await user.click(within(nav).getByRole('button', { name: '报表' }))
    expect(screen.queryByRole('complementary', { name: '上下文面板' })).not.toBeInTheDocument()
  })

  it('当前页在左栏高亮，且高亮项唯一', async () => {
    const user = userEvent.setup()
    render(<App />)

    const nav = screen.getByRole('navigation', { name: '主导航' })
    await user.click(within(nav).getByRole('button', { name: '记忆' }))

    const current = within(nav)
      .getAllByRole('button')
      .filter((el) => el.getAttribute('aria-current') === 'page')
    expect(current).toHaveLength(1)
    expect(current[0]).toHaveTextContent('记忆')
  })

  it('窗口栏收纳三个折叠开关：左栏 / 计划 / 上下文', () => {
    render(<App />)
    expect(screen.getByRole('button', { name: /折叠侧栏/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /计划面板/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /上下文面板/ })).toBeInTheDocument()
  })

  it('折叠开关能收起左栏', async () => {
    const user = userEvent.setup()
    render(<App />)

    expect(screen.getByRole('navigation', { name: '主导航' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: /折叠侧栏/ }))
    expect(screen.queryByRole('navigation', { name: '主导航' })).not.toBeInTheDocument()
  })

  it('计划面板由窗口栏唤出，不是导航页', async () => {
    const user = userEvent.setup()
    render(<App />)

    expect(screen.queryByRole('dialog', { name: '计划' })).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: /计划面板/ }))
    expect(screen.getByRole('dialog', { name: '计划' })).toBeInTheDocument()
  })
})

// ★ 骨架占位**绝不编造数据**。
// 一个显示「一次通过率 87%」但其实是编的界面，比空白更糟——
// 用户会在假信息上做判断。
describe('骨架占位', () => {
  it('对话页与左栏的占位里不含任何数字', () => {
    render(<App />)
    const main = document.querySelector('main')?.textContent ?? ''
    expect(main).not.toMatch(/\d/)
  })

  it('每个骨架都说明了这里将来是什么', () => {
    render(<App />)
    for (const el of screen.getAllByTestId('skeleton')) {
      expect(el.textContent ?? '').toMatch(/将来/)
    }
  })
})

function labelOf(id: string): string {
  const map: Record<string, string> = {
    report: '报表',
    memory: '记忆',
    skill: 'Skill',
    roles: '角色与 Runtime',
    settings: '设置',
  }
  return map[id] ?? id
}

// 持久化的页面标识可能被手工改坏或是旧版本遗留。
describe('页面标识规整', () => {
  it('非法值一律回落到对话，不白屏', () => {
    expect(normalizePageId('no-such-page')).toBe('chat')
    expect(normalizePageId(null)).toBe('chat')
    expect(normalizePageId(42)).toBe('chat')
    // 旧版本里「计划」曾是导航页，现在不是了
    expect(normalizePageId('plan')).toBe('chat')
  })

  it('合法值原样返回', () => {
    expect(normalizePageId('chat')).toBe('chat')
    for (const p of NAV_PAGES) {
      expect(normalizePageId(p.id)).toBe(p.id)
    }
  })
})
