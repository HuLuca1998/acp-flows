import { render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { Rail } from './index'

// 真机走查补的：左栏底部的 Runtime 栏原本写死「尚未检测」，
// 而后端明明返回了两个 ready 的 Runtime——**界面说的和事实不符**。
//
// 设计稿里这一栏是「圆点 + 名称版本 + 右对齐状态词」，数据 /v1/runtimes 全都有。

const listRuntimes = vi.fn()

vi.mock('@/api/system', () => ({
  listRuntimes: (...a: unknown[]): unknown => listRuntimes(...a),
}))

beforeEach(() => {
  listRuntimes.mockReset().mockResolvedValue([
    { name: 'claude', status: 'ready', active_version: '0.63.0' },
    { name: 'codex', status: 'not_authenticated', active_version: '1.1.7' },
  ])
})

afterEach(() => {
  vi.clearAllMocks()
})

const noop = () => undefined

describe('左栏 · Runtime 状态', () => {
  // ★★ 显示**真实的**检测结果，不是写死的占位。
  //
  // 写死「尚未检测」而后端返回两个 ready，用户会以为应用没在工作——
  // 界面说谎比界面简陋糟得多。
  it('列出检测到的 Runtime 与版本', async () => {
    render(<Rail currentPage="chat" onNavigate={noop} onNewWork={noop} onOpenWork={noop} />)

    await waitFor(() => {
      expect(screen.getByText(/claude/i)).toBeInTheDocument()
    })
    expect(screen.getByText(/0\.63\.0/)).toBeInTheDocument()
    expect(screen.getByText(/codex/i)).toBeInTheDocument()
    expect(screen.getByText(/1\.1\.7/)).toBeInTheDocument()
  })

  // ★ 状态要**分得出好坏**：ready 与 not_authenticated 对用户是两件事——
  // 后者意味着他得去登录，而界面不说他就一直等。
  it('区分就绪与未登录', async () => {
    render(<Rail currentPage="chat" onNavigate={noop} onNewWork={noop} onOpenWork={noop} />)

    const claude = await waitFor(() => {
      const el = screen.getByText(/claude/i).closest('[data-runtime]')
      expect(el).not.toBeNull()
      return el
    })
    expect(claude?.getAttribute('data-status')).toBe('ready')

    const codex = screen.getByText(/codex/i).closest('[data-runtime]')
    expect(
      codex?.getAttribute('data-status'),
      '未登录和就绪显示成一样——用户不知道自己该去登录',
    ).toBe('not_authenticated')
  })

  // 一个都没检测到时说清楚，而不是留一片空白。
  it('一个都没有时给出提示', async () => {
    listRuntimes.mockResolvedValue([])
    render(<Rail currentPage="chat" onNavigate={noop} onNewWork={noop} onOpenWork={noop} />)

    await waitFor(() => {
      expect(screen.getByText(/没有检测到/)).toBeInTheDocument()
    })
  })

  // ★ 查询失败**不能让左栏整个白掉**。
  //
  // 后端没起来时用户最需要的就是这条左栏（他要点进设置页看看怎么回事），
  // 而抛异常会让整棵组件树塌掉。
  it('查询失败时左栏照常显示', async () => {
    listRuntimes.mockRejectedValue(new Error('后端没起来'))
    render(<Rail currentPage="chat" onNavigate={noop} onNewWork={noop} onOpenWork={noop} />)

    await waitFor(() => {
      expect(screen.getByText(/检测失败|没有检测到/)).toBeInTheDocument()
    })
    // 导航还在，用户点得进设置页
    expect(screen.getByRole('navigation')).toBeInTheDocument()
  })

  // 折叠成图标条时不显示 Runtime 栏——48px 放不下，硬塞会溢出。
  it('折叠时不显示 Runtime 栏', async () => {
    render(<Rail currentPage="chat" onNavigate={noop} onNewWork={noop} onOpenWork={noop} collapsed />)

    await waitFor(() => {
      expect(listRuntimes).toHaveBeenCalled()
    })
    expect(screen.queryByText(/0\.63\.0/)).not.toBeInTheDocument()
  })
})
