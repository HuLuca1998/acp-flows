import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { SkillPage } from './index'

// M2 U2.4.1 · Skill 页

const listSkills = vi.fn()
const getSkillBody = vi.fn()

vi.mock('@/api/library', () => ({
  listSkills: (...a: unknown[]): unknown => listSkills(...a),
  getSkillBody: (...a: unknown[]): unknown => getSkillBody(...a),
}))

const listProjects = vi.fn()

vi.mock('@/api/system', () => ({
  listProjects: (...a: unknown[]): unknown => listProjects(...a),
}))

const realSkills = [
  {
    name: 'rust-test-first',
    dir: 'rust-test-first',
    version: '2.1',
    description: '先写测试再写实现，测试要能证明契约',
    compatibility: 'cargo >= 1.80',
    scope: 'global',
    source: '/Users/luca/.acpflows/skills',
    status: 'draft',
    validation_ok: true,
  },
  {
    name: 'git-worktree-guard',
    dir: 'broken-one',
    version: '0.4',
    scope: 'global',
    source: '/Users/luca/.acpflows/skills',
    status: 'draft',
    validation_ok: false,
    validation_reason: '校验未通过：frontmatter 缺 description',
  },
]

beforeEach(() => {
  listProjects.mockReset().mockResolvedValue([
    { id: 'proj-01', name: 'acp-flows', path: '/Users/luca/work/acp-flows' },
  ])
  listSkills.mockReset().mockResolvedValue(realSkills)
  getSkillBody.mockReset().mockResolvedValue({
    dir: 'rust-test-first',
    frontmatter: 'name: rust-test-first\nversion: "2.1"',
    text: '\n先写失败测试，再实现。\n\n## 步骤\n\n1. 按验收标准逐条写失败测试\n',
  })
})

afterEach(() => {
  vi.clearAllMocks()
})

describe('Skill 页', () => {
  it('列出 Skill 与版本号', async () => {
    render(<SkillPage />)

    await waitFor(() => {
      expect(screen.getByText('rust-test-first')).toBeInTheDocument()
    })
    expect(screen.getByText('v2.1')).toBeInTheDocument()
    expect(screen.getByText(/先写测试再写实现/)).toBeInTheDocument()
  })

  // ★★ 校验没过时**必须说清为什么**（INV-SKL-2）。
  //
  // 只显示一个 draft 标签的话，用户唯一能做的事是删了重建——
  // 而重建出来还是 draft。
  it('校验没过时说清楚缺什么', async () => {
    render(<SkillPage />)

    await waitFor(() => {
      expect(screen.getByText(/frontmatter 缺 description/)).toBeInTheDocument()
    })
  })

  // ★ 校验没过的那条要**看得出不一样**——它是需要用户动手的那一条。
  it('校验没过的条目在界面上有区分', async () => {
    render(<SkillPage />)

    const broken = await waitFor(() => {
      const el = document.querySelector('[data-skill="broken-one"]')
      expect(el).not.toBeNull()
      return el as HTMLElement
    })
    const badge = broken.querySelector('[data-ok]')
    expect(badge, '校验态没有可区分的标记').not.toBeNull()
    expect(badge).toHaveAttribute('data-ok', 'false')

    const good = document.querySelector('[data-skill="rust-test-first"]')
    expect(good?.querySelector('[data-ok]')).toHaveAttribute('data-ok', 'true')
  })

  // ★ 显示**真实路径**，不是写死的 `~/.acpflows/skills`。
  //
  // 写死的话，开发态与自定义数据目录下界面会告诉用户一个不存在的路径——
  // 他照着去找，发现那儿什么都没有，然后以为是应用坏了。真机走查抓到的。
  it('显示库的真实路径', async () => {
    render(<SkillPage />)

    await waitFor(() => {
      expect(screen.getByText('/Users/luca/.acpflows/skills')).toBeInTheDocument()
    })
  })

  // ★★ 扫不动要说出来，不装作「一个都没有」。
  it('扫描失败时说清楚，不显示空态', async () => {
    listSkills.mockRejectedValue(new Error('目录读不了'))
    render(<SkillPage />)

    await waitFor(() => {
      expect(screen.getByText(/目录读不了/)).toBeInTheDocument()
    })
    expect(screen.queryByText(/还没有 Skill/)).not.toBeInTheDocument()
  })

  // 一个都没有时给出可操作的提示，而不是一片空白。
  it('空库时说清楚怎么加', async () => {
    listSkills.mockResolvedValue([])
    render(<SkillPage />)

    await waitFor(() => {
      expect(screen.getByText(/还没有 Skill/)).toBeInTheDocument()
    })
    expect(screen.getByText(/SKILL\.md/)).toBeInTheDocument()
  })
})

// ── U10.4.1 · 命中计数 ──────────────────────────────────────

// R4 ★★ 从没用过显示 **0**，不是空白。
//
// 空白会让用户以为这个数字坏了，而「从没被用过」正是他判断
// 「该不该留着这个 Skill」最需要的一条信息。
it('从没用过的 Skill 显示 0，不是空白', async () => {
  listSkills.mockResolvedValue([
    { name: 'go-unit-testing', dir: 'go-unit-testing', version: '1.2.0',
      scope: 'global', source: '/s', status: 'draft', validation_ok: true, hit_count: 0 },
  ])
  render(<SkillPage />)

  const item = await screen.findByText('go-unit-testing')
  const row = item.closest('[data-skill]')
  expect(row?.textContent, '计数是空白——用户会以为这个数字坏了').toContain('0')
})

// ★ 用过的显示真实次数。
it('用过的 Skill 显示真实次数', async () => {
  listSkills.mockResolvedValue([
    { name: 'go-unit-testing', dir: 'go-unit-testing', version: '1.2.0',
      scope: 'global', source: '/s', status: 'active', validation_ok: true, hit_count: 7 },
  ])
  render(<SkillPage />)

  expect(await screen.findByText(/7/)).toBeInTheDocument()
})

// ── U10.7.2 · 两栏详情 ──────────────────────────────────────
//
// 用户裁定（2026-08-10）：「skill 页面和设计图纸完全偏离」——
// 设计稿的主体是右侧详情栏（frontmatter + 正文渲染），这里还账。

describe('Skill 页 · 详情栏（U10.7.2）', () => {
  // R1：点一条 → 详情显示 frontmatter 块与正文。
  it('点一条 Skill 显示 frontmatter 与正文', async () => {
    const user = userEvent.setup()
    render(<SkillPage />)

    await user.click(await screen.findByRole('button', { name: /rust-test-first/ }))

    const detail = await screen.findByRole('region', { name: /详情/ })
    expect(await within(detail).findByText(/先写失败测试，再实现/)).toBeInTheDocument()
    expect(within(detail).getByText(/name: rust-test-first/)).toBeInTheDocument()
    expect(getSkillBody).toHaveBeenCalledWith('rust-test-first')
  })

  // 正文默认**渲染**显示，「查看源码」切到原文（§7.8：这是必须有的）。
  it('查看源码能在渲染与原文之间切换', async () => {
    const user = userEvent.setup()
    render(<SkillPage />)

    await user.click(await screen.findByRole('button', { name: /rust-test-first/ }))
    const detail = await screen.findByRole('region', { name: /详情/ })

    // 渲染态：`## 步骤` 变成了标题，井号不该原样出现
    expect(await within(detail).findByRole('heading', { name: '步骤' })).toBeInTheDocument()
    expect(within(detail).queryByText(/## 步骤/)).not.toBeInTheDocument()

    await user.click(within(detail).getByRole('button', { name: /查看源码/ }))
    expect(within(detail).getByText(/## 步骤/)).toBeInTheDocument()
  })

  // R3：校验没过的那条，详情里也要说清为什么——并且让他看到自己写了什么。
  it('校验没过的条目详情里说明原因', async () => {
    getSkillBody.mockResolvedValue({
      dir: 'broken-one',
      frontmatter: 'name: git-worktree-guard\nversion: "0.4"',
      text: '\n正文还在。\n',
    })
    const user = userEvent.setup()
    render(<SkillPage />)

    await user.click(await screen.findByRole('button', { name: /git-worktree-guard/ }))

    const detail = await screen.findByRole('region', { name: /详情/ })
    expect(await within(detail).findByText(/frontmatter 缺 description/)).toBeInTheDocument()
    expect(within(detail).getByText(/name: git-worktree-guard/)).toBeInTheDocument()
  })

  // R4：正文读不到时明说（错误里带路径），不显示成空白。
  it('正文读不到时明说，不显示成空白', async () => {
    getSkillBody.mockRejectedValue(new Error('读不到 /s/skills/rust-test-first/SKILL.md'))
    const user = userEvent.setup()
    render(<SkillPage />)

    await user.click(await screen.findByRole('button', { name: /rust-test-first/ }))

    const detail = await screen.findByRole('region', { name: /详情/ })
    expect(
      await within(detail).findByText(/读不到 \/s\/skills\/rust-test-first\/SKILL\.md/),
    ).toBeInTheDocument()
  })

  // 没选中时的空态：告诉用户点左边，不是一片空白。
  it('没选中时详情栏说明怎么用', async () => {
    render(<SkillPage />)

    await screen.findByText('rust-test-first')
    const detail = screen.getByRole('region', { name: /详情/ })
    expect(within(detail).getByText(/选一条/)).toBeInTheDocument()
  })

  // ★ 项目选择器（设计稿页头，§7.6：标题行下方第一行）——
  // 切到某个项目后**按项目拉取**，不是换个标签装装样子。
  it('项目选择器切到项目后按项目拉取', async () => {
    const user = userEvent.setup()
    render(<SkillPage />)

    await screen.findByText('rust-test-first')
    await user.click(screen.getByRole('button', { name: /项目/ }))
    await user.click(await screen.findByRole('option', { name: /acp-flows/ }))

    await waitFor(() => {
      expect(listSkills).toHaveBeenCalledWith({ project: '/Users/luca/work/acp-flows' })
    })
  })
})
