import { act, renderHook, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { useEventStream } from './use-event-stream'

// M2 U2.4.1 · SSE 订阅（验收点 V6 的 R4）
//
// ★ 用一个**假 EventSource** 替换全局实现：jsdom 里没有真的 EventSource，
// 而这一层要验的是「收到消息之后我们怎么处理」——不是浏览器能不能连上。

type Listener = (e: MessageEvent) => void

class FakeEventSource {
  static instances: FakeEventSource[] = []

  url: string
  closed = false
  private listeners = new Map<string, Listener[]>()

  constructor(url: string) {
    this.url = url
    FakeEventSource.instances.push(this)
  }

  addEventListener(type: string, fn: Listener) {
    const list = this.listeners.get(type) ?? []
    list.push(fn)
    this.listeners.set(type, list)
  }

  close() {
    this.closed = true
  }

  /** 模拟服务端推来一条消息。 */
  emit(data: unknown, lastEventId = '') {
    const e = new MessageEvent('message', { data: JSON.stringify(data), lastEventId })
    for (const fn of this.listeners.get('message') ?? []) {
      fn(e)
    }
  }

  /** 模拟连接断开。 */
  fail() {
    for (const fn of this.listeners.get('error') ?? []) {
      fn(new MessageEvent('error'))
    }
  }

  static latest(): FakeEventSource {
    const last = FakeEventSource.instances.at(-1)
    if (last === undefined) throw new Error('没有创建过 EventSource')
    return last
  }
}

beforeEach(() => {
  FakeEventSource.instances = []
  vi.stubGlobal('EventSource', FakeEventSource)
})

afterEach(() => {
  vi.unstubAllGlobals()
})

const event = (seq: number, workID: string, text: string) => ({
  id: `evt_${seq}`,
  seq,
  work_id: workID,
  source: 'acp',
  type: 'message_chunk',
  ts: '2026-08-08T00:00:00Z',
  payload: { text },
})

describe('事件流订阅', () => {
  it('收到的事件按顺序累积', async () => {
    const { result } = renderHook(() => useEventStream('work-01'))

    act(() => {
      FakeEventSource.latest().emit(event(1, 'work-01', '第一句'))
      FakeEventSource.latest().emit(event(2, 'work-01', '第二句'))
    })

    await waitFor(() => {
      expect(result.current.events).toHaveLength(2)
    })
    expect(result.current.events.map((e) => e.payload?.text)).toEqual(['第一句', '第二句'])
  })

  // ★ **只收当前工作的事件。**
  //
  // 事件流是全局的（一条连接推所有工作），不过滤的话，用户开着 A 工作
  // 却看到 B 工作的 AI 在说话——那会让他以为自己点错了。
  it('过滤掉其它工作的事件', async () => {
    const { result } = renderHook(() => useEventStream('work-01'))

    act(() => {
      FakeEventSource.latest().emit(event(1, 'work-01', '我的'))
      FakeEventSource.latest().emit(event(2, 'work-02', '别人的'))
      FakeEventSource.latest().emit(event(3, 'work-01', '还是我的'))
    })

    await waitFor(() => {
      expect(result.current.events).toHaveLength(2)
    })
    expect(result.current.events.map((e) => e.payload?.text)).toEqual(['我的', '还是我的'])
  })

  // ★ 按 seq 去重。
  //
  // 断线重连时服务端可能重发边界上的那条（我们宁可重复也不愿缺失，
  // 见 api/events.go 的「先订阅再补历史」）。不去重的话，
  // 用户会看到某句话被说了两遍。
  it('同一个 seq 只保留一条', async () => {
    const { result } = renderHook(() => useEventStream('work-01'))

    act(() => {
      FakeEventSource.latest().emit(event(1, 'work-01', 'A'))
      FakeEventSource.latest().emit(event(1, 'work-01', 'A'))
      FakeEventSource.latest().emit(event(2, 'work-01', 'B'))
    })

    await waitFor(() => {
      expect(result.current.events).toHaveLength(2)
    })
  })

  // 载荷不是合法 JSON 时跳过这一条，**不让整条流断掉**。
  it('坏消息不影响后续事件', async () => {
    const { result } = renderHook(() => useEventStream('work-01'))

    act(() => {
      const es = FakeEventSource.latest()
      const bad = new MessageEvent('message', { data: '{ 不是 JSON' })
      // 直接触发 listener，绕过 emit 的 JSON.stringify
      es.addEventListener('message', () => {})
      for (const fn of (es as unknown as { listeners: Map<string, Listener[]> }).listeners.get('message') ?? []) {
        fn(bad)
      }
      es.emit(event(1, 'work-01', '后面的还在'))
    })

    await waitFor(() => {
      expect(result.current.events).toHaveLength(1)
    })
    expect(result.current.events[0]?.payload?.text).toBe('后面的还在')
  })

  // workID 为空时不连——没有工作就没有要看的事件，
  // 白连一条会在后端留一个永远收不到东西的订阅者。
  it('没有当前工作时不建立连接', () => {
    renderHook(() => useEventStream(null))

    expect(FakeEventSource.instances).toHaveLength(0)
  })

  // ★ 组件卸载时**必须关掉连接**。
  //
  // 不关的话，用户每切一次工作就多一条 SSE 连接，而后端每条事件都要
  // 往所有订阅者投递一遍——开一天之后应用会肉眼可见地变慢。
  it('卸载时关掉连接', () => {
    const { unmount } = renderHook(() => useEventStream('work-01'))
    const es = FakeEventSource.latest()

    unmount()

    expect(es.closed, '卸载后没关连接——每切一次工作就泄漏一条').toBe(true)
  })

  // 切换工作时：关掉旧的，开一条新的。
  it('切换工作时换一条连接', () => {
    const { rerender } = renderHook(({ id }) => useEventStream(id), {
      initialProps: { id: 'work-01' },
    })
    const first = FakeEventSource.latest()

    rerender({ id: 'work-02' })

    expect(first.closed, '切换工作后旧连接没关').toBe(true)
    expect(FakeEventSource.instances).toHaveLength(2)
  })
})
