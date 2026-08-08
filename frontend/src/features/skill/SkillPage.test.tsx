import { render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { SkillPage } from './index'

// M2 U2.4.1 · Skill 页

const listSkills = vi.fn()

vi.mock('@/api/system', () => ({
  listSkills: (...a: unknown[]): unknown => listSkills(...a),
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
  listSkills.mockReset().mockResolvedValue(realSkills)
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
