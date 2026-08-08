import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { App } from './App'
import { hasContextPanel, NAV_PAGES, navPageById, normalizePageId } from './pages'

const listProjects = vi.fn()
const listWorks = vi.fn()
const startWork = vi.fn()
const listRuntimes = vi.fn()

vi.mock('@/api/system', () => ({
  listProjects: (...a: unknown[]): unknown => listProjects(...a),
  listWorks: (...a: unknown[]): unknown => listWorks(...a),
  startWork: (...a: unknown[]): unknown => startWork(...a),
  listRuntimes: (...a: unknown[]): unknown => listRuntimes(...a),
}))

beforeEach(() => {
  listProjects.mockReset().mockResolvedValue([])
  listWorks.mockReset().mockResolvedValue([])
  startWork.mockReset().mockResolvedValue({ id: 'work-01', state: 'clarifying' })
  listRuntimes.mockReset().mockResolvedValue([])
})

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
    // 对话页已经做实（U2.4.1），不再是骨架占位。
    // 没有项目时它引导用户先去加一个——这就是「默认进了对话主区」的证据。
    expect(screen.getByText(/先添加一个项目/)).toBeInTheDocument()
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

  // ★ 折叠后**保留图标条**，不是整条消失（设计规范 §06 规则②）。
  // 整条消失会让用户失去「我在哪」的锚点，也让重新展开变成一次寻找。
  it('折叠后左栏保留图标条：文字收起但导航仍在', async () => {
    const user = userEvent.setup()
    render(<App />)

    const nav = () => screen.getByRole('navigation', { name: '主导航' })
    expect(within(nav()).getByText('设置')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /折叠侧栏/ }))

    // 导航还在——只是文字收起了
    expect(nav()).toBeInTheDocument()
    expect(within(nav()).queryByText('设置')).not.toBeInTheDocument()
    // 项目树与最近也一并收起，只留图标条
    expect(screen.queryByText('项目')).not.toBeInTheDocument()
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

  // ★ 用 queryAll 而不是 getAll：这是一条**全称命题**（「每个骨架都…」），
  // 一个骨架都没有时它应当为真——而 `getAllByTestId` 在空集合上会抛错。
  //
  // 2026-08-09 撞上过：角色/Skill/记忆三页做完之后默认页上再没有骨架，
  // 这条测试红了，而代码是对的。骨架越做越少是**进展**，不是回归。
  it('每个骨架都说明了这里将来是什么', () => {
    render(<App />)
    for (const el of screen.queryAllByTestId('skeleton')) {
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

// ── 真机走查补的：窗口栏别再说谎 ────────────────────────────

describe('窗口栏面包屑', () => {
  // ★★ 有项目时显示**项目名**，不是「还没有项目」。
  //
  // 真机撞到的：后端明明有 proj-01，顶栏却一直写着
  // 「还没有项目 —— 项目管理即将上线」。两句话都不对：项目有了，
  // 项目管理也早就在设置页上线了。
  it('有项目时显示项目名', async () => {
    listProjects.mockResolvedValue([
      { id: 'proj-01', name: 'my-app', path: '/Users/me/work/my-app', is_git_repo: true },
    ])
    render(<App />)

    expect(await screen.findByText('my-app')).toBeInTheDocument()
    expect(screen.queryByText(/还没有项目/)).not.toBeInTheDocument()
  })

  // 没项目时引导他去加，**不说「即将上线」**——那功能已经上线了。
  it('没项目时引导去设置页添加', async () => {
    listProjects.mockResolvedValue([])
    render(<App />)

    // ★ 断言的是**这句文案本身**，不是「页面上没有『即将上线』」——
    // 左栏那几个功能确实还没做，它们说「即将上线」是对的。
    const hint = await screen.findByText(/还没有项目/)
    expect(
      hint.textContent,
      '项目管理早就在设置页上线了，顶栏还在说「即将上线」',
    ).not.toMatch(/即将上线/)
  })

  // ★ 没有第三条「查询失败时窗口栏照常在」。
  //
  // 造负例时发现它证明不了任何事：`project` 的初始值就是 null，
  // 失败路径与「还没查回来」路径在页面上完全一样——把 catch 删掉，
  // 它照样绿。留着就是一条恒真断言（testing-strategy.md §3.2）。
  //
  // 「不让窗口白掉」这个保证靠的是 App.tsx 里那个 catch 本身，
  // 理由写在它旁边。
})
