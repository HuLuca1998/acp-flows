import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { PermissionDock } from './PermissionDock'
import type { PermissionRequest } from './model'

// M3 U3.1.3 · 待裁决的权限请求（R2 / R3）
//
// ★ 这一层管的是「哪些还没应答」。R2「点了之后卡片消失」与
// R3「不点它不会自己消失」都在这里——卡片自己不知道这件事。

const ask = (id: string): PermissionRequest => ({
  id,
  toolCallId: `tool-${id}`,
  runtime: 'codex',
  kind: 'edit',
  path: `src/${id}.rs`,
  options: [
    { optionId: 'opt-allow', name: '允许一次', kind: 'allow_once' },
    { optionId: 'opt-deny', name: '拒绝', kind: 'reject_once' },
  ],
})

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true })
})

afterEach(() => {
  vi.useRealTimers()
})

describe('待裁决队列', () => {
  // ★★ R3：**不点它就不会自己消失。**
  //
  // 自动消失的话，用户去倒杯水回来，卡片没了、AI 也停着——他不知道
  // 刚才发生过什么，更不知道该做什么。这比多等一会儿糟得多。
  it('R3 · 5 秒后卡片仍在', async () => {
    render(<PermissionDock asks={[ask('a')]} onDecide={vi.fn()} />)

    expect(await screen.findByText(/请求写入/)).toBeInTheDocument()

    act(() => {
      vi.advanceTimersByTime(5_000)
    })

    expect(
      screen.getByText(/请求写入/),
      '5 秒后卡片自己没了——用户去倒杯水回来，不知道刚才发生过什么',
    ).toBeInTheDocument()
  })

  // ★★ R2：点了之后卡片消失。
  //
  // 不消失的话，用户不确定自己点上没有，很可能再点一次——
  // 而第二条应答会被 Agent 当成一个不认识的请求。
  it('R2 · 应答之后这张卡片不再渲染', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    const onDecide = vi.fn().mockResolvedValue(undefined)
    render(<PermissionDock asks={[ask('a')]} onDecide={onDecide} />)

    await user.click(await screen.findByRole('button', { name: '拒绝' }))

    await waitFor(() => {
      expect(screen.queryByText(/请求写入/), '应答之后卡片还在').not.toBeInTheDocument()
    })
    expect(onDecide).toHaveBeenCalledWith('a', 'opt-deny')
  })

  // ★ 应答失败时卡片**留下**，并说清楚失败了。
  //
  // 静默移除的话，用户以为自己处理完了，而 AI 那边还在等——
  // 他会对着一个不动的界面等下去。
  it('应答失败时卡片留下并给出提示', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    const onDecide = vi.fn().mockRejectedValue(new Error('后端没起来'))
    render(<PermissionDock asks={[ask('a')]} onDecide={onDecide} />)

    await user.click(await screen.findByRole('button', { name: '拒绝' }))

    await waitFor(() => {
      expect(screen.getByText(/没能提交|再试/)).toBeInTheDocument()
    })
    expect(
      screen.getByText(/请求写入/),
      '应答失败了卡片却没了——用户以为处理完了，而 AI 那边还在等',
    ).toBeInTheDocument()
    // 还能再点一次
    expect(screen.getByRole('button', { name: '拒绝' })).toBeEnabled()
  })

  // 多条请求各自独立：处理掉一条，另一条还在。
  it('处理一条不影响另一条', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    const onDecide = vi.fn().mockResolvedValue(undefined)
    render(<PermissionDock asks={[ask('a'), ask('b')]} onDecide={onDecide} />)

    await waitFor(() => {
      expect(screen.getAllByRole('button', { name: '拒绝' })).toHaveLength(2)
    })
    await user.click(screen.getAllByRole('button', { name: '拒绝' })[0]!)

    await waitFor(() => {
      expect(screen.getAllByRole('button', { name: '拒绝' })).toHaveLength(1)
    })
    expect(screen.getByText('src/b.rs')).toBeInTheDocument()
  })

  // 一条待裁决都没有时不占地方——空容器会在时间线上留一条无意义的分隔。
  it('没有待裁决时什么都不渲染', () => {
    const { container } = render(<PermissionDock asks={[]} onDecide={vi.fn()} />)

    expect(container).toBeEmptyDOMElement()
  })

  // ★ 提交中禁用按钮，挡住连点。
  it('提交中不接受第二次点击', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    let release: (() => void) | undefined
    const onDecide = vi.fn(
      () =>
        new Promise<void>((resolve) => {
          release = resolve
        }),
    )
    render(<PermissionDock asks={[ask('a')]} onDecide={onDecide} />)

    const deny = await screen.findByRole('button', { name: '拒绝' })
    await user.click(deny)

    await waitFor(() => {
      expect(deny).toBeDisabled()
    })
    expect(onDecide).toHaveBeenCalledTimes(1)

    release?.()
  })

  // 上游把某条请求撤了（比如这一轮被取消）时，卡片跟着消失。
  it('上游撤回时卡片消失', async () => {
    const { rerender } = render(<PermissionDock asks={[ask('a')]} onDecide={vi.fn()} />)
    expect(await screen.findByText(/请求写入/)).toBeInTheDocument()

    rerender(<PermissionDock asks={[]} onDecide={vi.fn()} />)

    expect(screen.queryByText(/请求写入/)).not.toBeInTheDocument()
  })
})
