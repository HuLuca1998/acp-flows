import { render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { WorkspacePanel } from './WorkspacePanel'

// M4 U4.2.2 · 右栏「工作区」
//
// ★★ 这一栏回答的是**「AI 到底动了什么」**。
// 2026-08-09 之前这里是「时间线过滤器 + 引用」两块占位——
// 而设计稿里过滤器属于计划面板那一侧，完全不是一回事。

const getWorkWorktree = vi.fn()

vi.mock('@/api/system', () => ({
  getWorkWorktree: (...a: unknown[]): unknown => getWorkWorktree(...a),
}))

const realState = {
  branch: 'duet/work-08',
  base_commit: '7c1de90',
  ahead: 3,
  changes: [
    { path: 'crates/engine/src/cancel.rs', added: 64, removed: 12 },
    { path: 'crates/engine/tests/cancel_idempotent.rs', added: 118, removed: 0 },
  ],
  commits: [
    { sha: 'a1c9f30', subject: 'unit-011: 抽出 Runtime 能力探针', when: '2 分钟前' },
    { sha: 'b2d0e41', subject: 'unit-010: 两段式取消', when: '10 分钟前' },
  ],
}

beforeEach(() => {
  getWorkWorktree.mockReset().mockResolvedValue(realState)
})

afterEach(() => {
  vi.clearAllMocks()
})

describe('右栏工作区', () => {
  // ★★ 未提交改动**逐个文件带增删行数**（设计稿的 `+64 −12`）。
  //
  // 只说「改了 2 个文件」的话，用户判断不出这次改动有多大。
  it('逐个文件显示增删行数并给出合计', async () => {
    render(<WorkspacePanel workID="work-08" />)

    const item = await waitFor(() => {
      const el = document.querySelector('[data-file="crates/engine/src/cancel.rs"]')
      expect(el).not.toBeNull()
      return el as HTMLElement
    })
    expect(within(item).getByText('+64')).toBeInTheDocument()
    expect(within(item).getByText('−12')).toBeInTheDocument()

    // 合计：64+118 / 12+0
    expect(screen.getByText(/\+182/)).toBeInTheDocument()
    expect(screen.getByText(/−12/, { selector: 'p' })).toBeInTheDocument()
  })

  it('显示分支与领先几个 commit', async () => {
    render(<WorkspacePanel workID="work-08" />)

    expect(await screen.findByText('duet/work-08')).toBeInTheDocument()
    const ahead = screen.getByText(/领先 3 个 commit/)
    expect(ahead.textContent).toContain('7c1de90')
  })

  // ★★ 没有基线时**不显示「领先 0」**。
  //
  // 那会让用户以为 AI 什么都没干，而实际是我们不知道起点。
  it('没有基线时不报领先数', async () => {
    getWorkWorktree.mockResolvedValue({ ...realState, base_commit: '', ahead: 0, commits: [] })
    render(<WorkspacePanel workID="work-08" />)

    await screen.findByText('duet/work-08')
    expect(
      screen.queryByText(/领先/),
      '不知道基线却报了「领先 N 个」——用户会以为 AI 什么都没干',
    ).not.toBeInTheDocument()
  })

  // ★ 本次工作的 commit：带 sha、相对时间、「仅本地」。
  it('列出本次工作的 commit', async () => {
    render(<WorkspacePanel workID="work-08" />)

    expect(await screen.findByText('a1c9f30')).toBeInTheDocument()
    expect(screen.getByText(/unit-011/)).toBeInTheDocument()
    expect(screen.getByText('2 分钟前')).toBeInTheDocument()
    // 「仅本地」提醒用户这些还没推上去
    expect(screen.getAllByText(/仅本地/).length).toBe(2)
  })

  // ★★ 设计稿原文：push 属于 D3，要用户逐次授权。
  it('明说 push 需要逐次授权', async () => {
    render(<WorkspacePanel workID="work-08" />)
    expect(await screen.findByText(/push 与发 PR 属于 D3/)).toBeInTheDocument()
  })

  // ★ 还没切 worktree 时明说，而不是一片空白或报错。
  it('还没有工作区时明说', async () => {
    getWorkWorktree.mockResolvedValue({ branch: '', ahead: 0, changes: [], commits: [] })
    render(<WorkspacePanel workID="work-08" />)

    expect(await screen.findByText(/还没有工作区/)).toBeInTheDocument()
  })

  // ★★ 读不到要说出来，不装作「什么都没改」。
  //
  // 装作没改的话，用户会以为 AI 一事无成。
  it('读不到状态时说清楚', async () => {
    getWorkWorktree.mockRejectedValue(new Error('worktree 目录没了'))
    render(<WorkspacePanel workID="work-08" />)

    expect(await screen.findByText(/worktree 目录没了/)).toBeInTheDocument()
    expect(screen.queryByText(/工作区是干净的/)).not.toBeInTheDocument()
  })

  it('没有工作时不去查', () => {
    render(<WorkspacePanel workID="" />)
    expect(screen.getByText(/还没有打开任何工作/)).toBeInTheDocument()
    expect(getWorkWorktree).not.toHaveBeenCalled()
  })

  it('干净的工作区明说', async () => {
    getWorkWorktree.mockResolvedValue({ ...realState, changes: [] })
    render(<WorkspacePanel workID="work-08" />)

    expect(await screen.findByText(/工作区是干净的/)).toBeInTheDocument()
  })
})
