import { act, renderHook, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { useEventStream } from './use-event-stream'

// M2 U2.4.1 · SSE 订阅（验收点 V6 的 R4）
//
// ★ 假的是**网络**（fetch），不是我们自己的读取逻辑。
// 第一版把整个 EventSource 换成假的，结果真机上 /v1/events 直接 401——
// 被替换掉的正是出问题的那部分。这次替身只负责「把字节喂进来」。

/** 一条可以逐段喂数据的假响应流。 */
class FakeStream {
  static instances: FakeStream[] = []

  aborted = false
  private controller!: ReadableStreamDefaultController<Uint8Array>
  readonly body: ReadableStream<Uint8Array>

  constructor(
    readonly url: string,
    readonly init: RequestInit | undefined,
  ) {
    FakeStream.instances.push(this)
    this.body = new ReadableStream<Uint8Array>({
      start: (c) => {
        this.controller = c
      },
    })
    init?.signal?.addEventListener('abort', () => {
      this.aborted = true
      try {
        this.controller.close()
      } catch {
        // 已经关了
      }
    })
  }

  /** 服务端推来一段原始 SSE 文本。 */
  push(raw: string) {
    this.controller.enqueue(new TextEncoder().encode(raw))
  }

  /** 推一条完整的事件消息。 */
  send(e: unknown, id?: number) {
    const head = id === undefined ? '' : `id: ${id}\n`
    this.push(`${head}data: ${JSON.stringify(e)}\n\n`)
  }

  /** 服务端把流关了（= 断线）。 */
  end() {
    this.controller.close()
  }

  header(name: string): string | undefined {
    return (this.init?.headers as Record<string, string> | undefined)?.[name]
  }

  static latest(): FakeStream {
    const last = FakeStream.instances.at(-1)
    if (last === undefined) throw new Error('没有发起过请求')
    return last
  }
}

beforeEach(() => {
  FakeStream.instances = []
  vi.stubGlobal('fetch', (url: string, init?: RequestInit) => {
    const s = new FakeStream(url, init)
    return Promise.resolve({ ok: true, status: 200, body: s.body } as Response)
  })
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

/** 等第一条请求发出去（连接是异步建立的）。 */
async function connected() {
  await waitFor(() => {
    expect(FakeStream.instances.length).toBeGreaterThan(0)
  })
  return FakeStream.latest()
}

describe('事件流订阅', () => {
  // ★★ 请求必须带 **Authorization 头**。
  //
  // 这条是真机撞出来的：第一版用 EventSource，而 EventSource 带不了自定义头，
  // 于是 /v1/events 一路 401，界面上时间线永远是空的——
  // 单测因为整个 EventSource 都是假的，全绿。
  it('带上认证头', async () => {
    renderHook(() => useEventStream('work-01'))
    const s = await connected()

    expect(s.header('Authorization'), '没带认证头——后端会 401，时间线永远是空的').toMatch(
      /^Bearer /,
    )
  })

  // ★ token 不许出现在 URL 里：URL 会进浏览器历史与访问日志，
  // 而这个 token 等于「驱动 Agent 改用户代码」的权限。
  it('不把 token 放进 URL', async () => {
    renderHook(() => useEventStream('work-01'))
    const s = await connected()

    expect(s.url, 'token 跑到 URL 里去了——它会进浏览器历史和访问日志').not.toMatch(/token|Bearer/i)
  })

  // ★★ 打开页面就该看到**之前的记录**。
  //
  // 首次连接带 `Last-Event-ID: 0`（「我一条都没有，从头给我」），服务端据此补历史。
  // 不带的话服务端什么都不补，表现是用户关掉应用再打开，
  // AI 之前干过的活全不见了——而「看着 AI 干活」的另一半正是「回来还能看到干过什么」。
  it('首次连接就把历史要回来', async () => {
    renderHook(() => useEventStream('work-01'))
    const s = await connected()

    expect(
      s.header('Last-Event-ID'),
      '首次连接没带游标——服务端不补历史，用户重开应用后时间线是空的',
    ).toBe('0')
  })

  it('收到的事件按顺序累积', async () => {
    const { result } = renderHook(() => useEventStream('work-01'))
    const s = await connected()

    act(() => {
      s.send(event(1, 'work-01', '第一句'), 1)
      s.send(event(2, 'work-01', '第二句'), 2)
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
    const s = await connected()

    act(() => {
      s.send(event(1, 'work-01', '我的'), 1)
      s.send(event(2, 'work-02', '别人的'), 2)
      s.send(event(3, 'work-01', '还是我的'), 3)
    })

    await waitFor(() => {
      expect(result.current.events).toHaveLength(2)
    })
    expect(result.current.events.map((e) => e.payload?.text)).toEqual(['我的', '还是我的'])
  })

  // ★ 按 seq 去重。
  //
  // 重连时服务端可能重发边界上的那条（我们宁可重复也不愿缺失，
  // 见 api/events.go 的「先订阅再补历史」）。不去重的话，
  // 用户会看到某句话被说了两遍。
  it('同一个 seq 只保留一条', async () => {
    const { result } = renderHook(() => useEventStream('work-01'))
    const s = await connected()

    act(() => {
      s.send(event(1, 'work-01', 'A'), 1)
      s.send(event(1, 'work-01', 'A'), 1)
      s.send(event(2, 'work-01', 'B'), 2)
    })

    await waitFor(() => {
      expect(result.current.events).toHaveLength(2)
    })
  })

  // 载荷不是合法 JSON 时跳过这一条，**不让整条流断掉**。
  it('坏消息不影响后续事件', async () => {
    const { result } = renderHook(() => useEventStream('work-01'))
    const s = await connected()

    act(() => {
      s.push('data: { 不是 JSON\n\n')
      s.send(event(1, 'work-01', '后面的还在'), 1)
    })

    await waitFor(() => {
      expect(result.current.events).toHaveLength(1)
    })
    expect(result.current.events[0]?.payload?.text).toBe('后面的还在')
  })

  // 心跳是注释行（以冒号开头），不该被当成事件。
  it('心跳不算事件', async () => {
    const { result } = renderHook(() => useEventStream('work-01'))
    const s = await connected()

    act(() => {
      s.push(': ping\n\n')
      s.send(event(1, 'work-01', '真事件'), 1)
    })

    await waitFor(() => {
      expect(result.current.events).toHaveLength(1)
    })
  })

  // 一条消息被拆成多个网络分片时要能拼回来——
  // 流式文本几乎必然被拆，拼不回来的话用户看到的是残句。
  it('跨分片的消息能拼回来', async () => {
    const { result } = renderHook(() => useEventStream('work-01'))
    const s = await connected()
    const raw = `data: ${JSON.stringify(event(1, 'work-01', '被拆开的一句'))}\n\n`

    act(() => {
      s.push(raw.slice(0, 20))
      s.push(raw.slice(20))
    })

    await waitFor(() => {
      expect(result.current.events).toHaveLength(1)
    })
    expect(result.current.events[0]?.payload?.text).toBe('被拆开的一句')
  })

  // workID 为空时不连——没有工作就没有要看的事件，
  // 白连一条会在后端留一个永远收不到东西的订阅者。
  it('没有当前工作时不建立连接', () => {
    renderHook(() => useEventStream(null))

    expect(FakeStream.instances).toHaveLength(0)
  })

  // ★ 组件卸载时**必须中止请求**。
  //
  // 不中止的话，用户每切一次工作就多一条挂着的连接，而后端每条事件都要
  // 往所有订阅者投递一遍——开一天之后应用会肉眼可见地变慢。
  it('卸载时中止连接', async () => {
    const { unmount } = renderHook(() => useEventStream('work-01'))
    const s = await connected()

    unmount()

    await waitFor(() => {
      expect(s.aborted, '卸载后没中止——每切一次工作就泄漏一条').toBe(true)
    })
  })

  // 切换工作时：中止旧的，开一条新的。
  it('切换工作时换一条连接', async () => {
    const { rerender } = renderHook(({ id }) => useEventStream(id), {
      initialProps: { id: 'work-01' },
    })
    const first = await connected()

    rerender({ id: 'work-02' })

    await waitFor(() => {
      expect(first.aborted, '切换工作后旧连接没中止').toBe(true)
      expect(FakeStream.instances).toHaveLength(2)
    })
  })

  // ★★ 断线重连时带上 **Last-Event-ID**，服务端只补它之后的。
  //
  // 不带的话服务端从头补，用户会看到整条时间线重放一遍；
  // 带错了则中间有洞，而洞是看不出来的——他不知道自己漏了什么。
  it('重连时带上最后收到的 seq', async () => {
    renderHook(() => useEventStream('work-01'))
    const first = await connected()

    act(() => {
      first.send(event(7, 'work-01', '断线前最后一句'), 7)
    })
    // 服务端把流关了
    act(() => {
      first.end()
    })

    await waitFor(
      () => {
        expect(FakeStream.instances.length).toBeGreaterThan(1)
      },
      { timeout: 3000 },
    )
    expect(
      FakeStream.latest().header('Last-Event-ID'),
      '重连没带 Last-Event-ID——服务端会从头补，用户看到整条时间线重放一遍',
    ).toBe('7')
  })
})
