import { act, renderHook, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { TimelineEvent } from '../timeline/event-registry'

import { usePermissions } from './use-permissions'

// M3 U3.1.4 · 从事件流里挑出待裁决的请求
//
// ★ 这一层把「时间线上的一条事件」变成「一张要用户点的卡片」。
// 断言集中在：挑对了没、点完消没消、载荷缺胳膊少腿时崩不崩。

const answerPermission = vi.fn()

vi.mock('@/api/system', () => ({
  answerPermission: (...a: unknown[]): unknown => answerPermission(...a),
}))

let seq = 0

function askEvent(overrides: Record<string, unknown> = {}): TimelineEvent {
  seq += 1
  return {
    id: `evt_${seq}`,
    seq,
    work_id: 'work-01',
    source: 'acp',
    type: 'request_permission',
    ts: '2026-08-08T00:00:00Z',
    payload: {
      ask_id: `ask-${seq}`,
      tool_call_id: 'tool-1',
      runtime: 'AI',
      kind: 'edit',
      path: 'src/main.go',
      options: [
        { option_id: 'opt-allow', name: '允许一次', kind: 'allow_once' },
        { option_id: 'opt-deny', name: '拒绝', kind: 'reject_once' },
      ],
      ...overrides,
    },
  }
}

function otherEvent(): TimelineEvent {
  seq += 1
  return {
    id: `evt_${seq}`,
    seq,
    work_id: 'work-01',
    source: 'acp',
    type: 'message_chunk',
    ts: '2026-08-08T00:00:00Z',
    payload: { text: '我在干活' },
  }
}

beforeEach(() => {
  seq = 0
  answerPermission.mockReset().mockResolvedValue(undefined)
})

afterEach(() => {
  vi.clearAllMocks()
})

describe('待裁决请求', () => {
  // ★ 只挑权限请求，别的事件一律不管。
  it('从事件流里挑出权限请求', () => {
    const { result } = renderHook(() =>
      usePermissions('work-01', [otherEvent(), askEvent(), otherEvent()]),
    )

    expect(result.current.asks).toHaveLength(1)
    expect(result.current.asks[0]?.path).toBe('src/main.go')
    expect(result.current.asks[0]?.options).toHaveLength(2)
  })

  // ★★ 载荷字段名要**从蛇形转成驼峰**——契约用 `option_id`，组件用 `optionId`。
  //
  // 转错的话按钮上的 id 是 undefined，用户点了什么都发不出去。
  it('把契约的蛇形字段转成组件要的形状', () => {
    const { result } = renderHook(() => usePermissions('work-01', [askEvent()]))

    const ask = result.current.asks[0]
    expect(ask?.id, 'ask_id 没转成 id').toBe('ask-1')
    expect(ask?.toolCallId, 'tool_call_id 没转成 toolCallId').toBe('tool-1')
    expect(
      ask?.options[0]?.optionId,
      'option_id 没转成 optionId——按钮上的 id 是 undefined，点了什么都发不出去',
    ).toBe('opt-allow')
  })

  // ★★ 提交走的是**用户点的那个 optionId**，一个字符不改。
  it('提交时原样送出 optionId', async () => {
    const { result } = renderHook(() => usePermissions('work-01', [askEvent()]))

    await act(async () => {
      await result.current.decide('ask-1', 'opt-deny')
    })

    expect(answerPermission).toHaveBeenCalledWith('work-01', 'ask-1', 'opt-deny')
  })

  // ★★ 载荷缺字段时**跳过这一条**，不让整个时间线白屏。
  //
  // 后端加字段、改结构时，用户看到的应该是「少了一张卡片」而不是整页没了。
  it('载荷不完整时跳过而不是崩', () => {
    const { result } = renderHook(() =>
      usePermissions('work-01', [
        askEvent({ ask_id: undefined }),
        askEvent({ options: undefined }),
        askEvent({ options: 'not-an-array' }),
        askEvent(),
      ]),
    )

    expect(result.current.asks, '坏载荷没被跳过').toHaveLength(1)
  })

  // ★ 一个选项都没有的请求**照样显示**——卡片会说清楚「没法处理」。
  //
  // 悄悄跳过的话，AI 挂在那儿而用户完全不知道，他会一直等。
  it('零选项的请求照样交给界面', () => {
    const { result } = renderHook(() => usePermissions('work-01', [askEvent({ options: [] })]))

    expect(
      result.current.asks,
      '零选项的请求被悄悄跳过了——AI 挂在那儿而用户完全不知道',
    ).toHaveLength(1)
  })

  // ★★ 已经应答过的不再出现。
  //
  // 事件流不会撤回历史事件，所以这条得自己记。不记的话，
  // 用户点完之后一刷新，同一张卡片又回来了。
  it('应答过的不再出现', async () => {
    const events = [askEvent()]
    const { result, rerender } = renderHook(({ e }) => usePermissions('work-01', e), {
      initialProps: { e: events },
    })

    await act(async () => {
      await result.current.decide('ask-1', 'opt-allow')
    })
    rerender({ e: events })

    await waitFor(() => {
      expect(
        result.current.asks,
        '应答过的又回来了——用户点完一刷新，同一张卡片再问一遍',
      ).toHaveLength(0)
    })
  })

  // ★ 提交失败时**不能**把它记成已应答。
  //
  // 记了的话卡片消失，而 AI 那边还在等——用户对着不动的界面等下去。
  it('提交失败时不算已应答', async () => {
    answerPermission.mockRejectedValue(new Error('后端没起来'))
    const events = [askEvent()]
    const { result } = renderHook(() => usePermissions('work-01', events))

    // ★ 在 act 内部吞掉异常，让 React 把这一轮 state 更新提交完。
    // 用 `await expect(act(...)).rejects` 的话 act 是抛出结束的，
    // React 没 flush，断言看的是旧状态——那样即使实现把失败也记成已应答，
    // 这条测试照样绿（造负例时发现的）。
    let thrown: unknown
    await act(async () => {
      thrown = await result.current.decide('ask-1', 'opt-allow').then(
        () => undefined,
        (e: unknown) => e,
      )
    })

    expect(thrown, 'decide 提交失败却没抛出').toBeInstanceOf(Error)
    expect(
      result.current.asks,
      '提交失败却当成应答了——卡片没了而 AI 还在等',
    ).toHaveLength(1)
  })

  // 没有当前工作时什么都不返回。
  it('没有工作时是空的', () => {
    const { result } = renderHook(() => usePermissions(null, [askEvent()]))

    expect(result.current.asks).toHaveLength(0)
  })

  // 切换工作时清空「已应答」的记录——那是上一个工作的事。
  it('切换工作时重新开始', async () => {
    const events = [askEvent()]
    const { result, rerender } = renderHook(({ id }) => usePermissions(id, events), {
      initialProps: { id: 'work-01' },
    })

    await act(async () => {
      await result.current.decide('ask-1', 'opt-allow')
    })
    expect(result.current.asks).toHaveLength(0)

    rerender({ id: 'work-02' })
    expect(result.current.asks).toHaveLength(1)
  })
})
