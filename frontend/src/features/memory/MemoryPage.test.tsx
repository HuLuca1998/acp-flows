import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { MemoryPage } from './index'

// M2 U2.3.1 · 记忆页
//
// ★★ 这一族里最值钱的是 INV-MEM-2「绝不自动写入」在界面上的落点：
// 候选**必须由人点一下**才会生效。

const listMemories = vi.fn()
const reviewMemory = vi.fn()

vi.mock('@/api/system', () => ({
  listMemories: (...a: unknown[]): unknown => listMemories(...a),
  reviewMemory: (...a: unknown[]): unknown => reviewMemory(...a),
}))

const active = {
  id: 'mem-203',
  kind: 'constraint',
  scope: 'acp-engine',
  status: 'active',
  source_refs: ['ev-412', 'unit-009'],
  created_by: 'memory_curator',
  confirmed_by: 'luca',
  injectable: true,
}

const candidate = {
  id: 'cand-07',
  kind: 'experience',
  scope: 'acp-engine',
  status: 'candidate',
  source_refs: ['ev-500'],
  created_by: 'memory_curator',
  injectable: false,
}

const retired = {
  id: 'mem-100',
  kind: 'fact',
  scope: 'acp-engine',
  status: 'invalid',
  source_refs: ['ev-1'],
  injectable: false,
}

beforeEach(() => {
  listMemories.mockReset().mockResolvedValue([active, candidate, retired])
  reviewMemory.mockReset().mockResolvedValue({ ...candidate, status: 'active', injectable: true })
})

afterEach(() => {
  vi.clearAllMocks()
})

describe('记忆页', () => {
  // ★★ 「全部」**不含候选**（Q25 裁定）。
  //
  // 混进去的话，用户会以为那些 AI 刚提的东西已经生效了——
  // 而它们还在等他拍板。
  it('「全部」不含候选', async () => {
    render(<MemoryPage />)

    const allTab = await waitFor(() => screen.getByRole('tab', { name: /全部/ }))
    // active + invalid = 2，不含那条候选
    expect(allTab.textContent, '「全部」把候选也算进去了——候选是待办不是库存').toContain('2')

    expect(screen.getByRole('tab', { name: /候选/ }).textContent).toContain('1')
  })

  // ★ 「已失效」这一档同时装 invalid 与 obsolete——
  // 两者对用户长得一样，对系统不一样。
  it('候选筛选只显示候选', async () => {
    render(<MemoryPage />)

    await waitFor(() => screen.getByRole('tab', { name: /候选/ }))
    await userEvent.click(screen.getByRole('tab', { name: /候选/ }))

    expect(screen.getByText('cand-07')).toBeInTheDocument()
    expect(screen.queryByText('mem-203')).not.toBeInTheDocument()
  })

  // ★★ 候选**必须由人点一下**才生效（INV-MEM-2）。
  //
  // 没有这两个按钮的话，AI 提的东西要么永远不生效，要么就得有一条
  // 自动路径——而后者正是这条不变量禁止的。
  it('候选带审核按钮，且点了之后重新拉取', async () => {
    render(<MemoryPage />)

    await waitFor(() => screen.getByRole('tab', { name: /候选/ }))
    await userEvent.click(screen.getByRole('tab', { name: /候选/ }))

    const confirm = screen.getByRole('button', { name: /收下/ })
    expect(screen.getByRole('button', { name: /不要/ })).toBeInTheDocument()

    listMemories.mockResolvedValue([active, { ...candidate, status: 'active', injectable: true }])
    await userEvent.click(confirm)

    await waitFor(() => {
      expect(reviewMemory).toHaveBeenCalled()
    })
    // ★★ 判据：调用**带上了 actor**——不带的话后端会拒绝，
    // 而那正是我们要的：没有任何路径能让候选自己变成生效
    const call = reviewMemory.mock.calls[0] as unknown[]
    expect(call[1]).toBe('confirm')
    expect(call[2], '审核没带 actor——那就等于允许匿名确认了').toBeTruthy()
  })

  // ★ 已生效的没有审核按钮：它已经被人拍过板了。
  it('已生效的记忆没有审核按钮', async () => {
    listMemories.mockResolvedValue([active])
    render(<MemoryPage />)

    await waitFor(() => screen.getByText('mem-203'))
    expect(screen.queryByRole('button', { name: /收下/ })).not.toBeInTheDocument()
  })

  // ★ 「会不会被注入」要直接标出来——用户看这一页就是想知道
  // 「AI 下一轮会带着哪些规矩干活」。
  it('标出哪些会被注入', async () => {
    render(<MemoryPage />)

    await waitFor(() => screen.getByText('mem-203'))
    expect(screen.getByText(/会被注入/)).toBeInTheDocument()

    // 候选那条不该有这个标记
    await userEvent.click(screen.getByRole('tab', { name: /候选/ }))
    expect(screen.queryByText(/会被注入/)).not.toBeInTheDocument()
  })

  // ★★ 查不动要说出来，不装作「一条都没有」——
  // 装作没有的话，用户以为 Duet 把记忆忘光了。
  it('查询失败时说清楚，不显示空态', async () => {
    listMemories.mockRejectedValue(new Error('数据库打不开'))
    render(<MemoryPage />)

    await waitFor(() => {
      expect(screen.getByText(/数据库打不开/)).toBeInTheDocument()
    })
    expect(screen.queryByRole('tab')).not.toBeInTheDocument()
  })

  // 审核失败要说出来，且**不假装成功**。
  it('审核失败时说清楚', async () => {
    reviewMemory.mockRejectedValue(new Error('这条已经生效了'))
    render(<MemoryPage />)

    await waitFor(() => screen.getByRole('tab', { name: /候选/ }))
    await userEvent.click(screen.getByRole('tab', { name: /候选/ }))
    await userEvent.click(screen.getByRole('button', { name: /收下/ }))

    await waitFor(() => {
      expect(screen.getByText(/这条已经生效了/)).toBeInTheDocument()
    })
  })

  it('显示依据，没有依据时明说', async () => {
    listMemories.mockResolvedValue([{ ...active, source_refs: [] }])
    render(<MemoryPage />)

    await waitFor(() => screen.getByText('mem-203'))
    expect(screen.getByText(/（无）/)).toBeInTheDocument()
  })
})
