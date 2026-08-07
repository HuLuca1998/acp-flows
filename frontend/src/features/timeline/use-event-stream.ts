import { useEffect, useState } from 'react'

import { readEventStream } from '@/api/events'

import type { TimelineEvent } from './event-registry'

export type EventStreamState = {
  events: TimelineEvent[]
  /** 连接断了；会自动重连，这个只用来给界面一点提示。 */
  disconnected: boolean
}

/** 重连退避的起点与上限。 */
const RETRY_MIN_MS = 500
const RETRY_MAX_MS = 10_000

/**
 * 订阅某个工作的事件流。
 *
 * 连接本身在 `@/api/events`——**那是全前端唯一允许裸 fetch 的地方**，
 * 理由写在那个文件里（一句话：SSE 走不了 openapi-fetch 的一次性响应，
 * 而 `EventSource` 带不了 `Authorization` 头，真机上会一路 401）。
 * 这里只管状态、去重与重连退避。
 *
 * ★ 事件流是**全局的**（一条连接推所有工作），所以这里按 `work_id` 过滤。
 * 不过滤的话，用户开着 A 工作却看到 B 工作的 AI 在说话。
 */
export function useEventStream(workID: string | null): EventStreamState {
  const [events, setEvents] = useState<TimelineEvent[]>([])
  const [disconnected, setDisconnected] = useState(false)

  useEffect(() => {
    // 没有当前工作就不连——白连一条会在后端留一个永远收不到东西的订阅者
    if (workID === null || workID === '') {
      return
    }

    setEvents([])
    setDisconnected(false)

    const controller = new AbortController()
    // ★ 从 0 开始，**首次连接就带 `Last-Event-ID: 0`**——
    // 它的意思是「我手上一条都没有，从头给我」，服务端据此补历史。
    // 不带的话服务端按「全新连接」处理、什么都不补，表现是用户关掉应用
    // 再打开，AI 之前干过的活全不见了。
    let lastSeq = 0
    let retry = RETRY_MIN_MS

    const push = (parsed: TimelineEvent) => {
      if (parsed.work_id !== workID) {
        return
      }
      lastSeq = parsed.seq ?? lastSeq
      setEvents((prev) => {
        // ★ 按 seq 去重。重连时服务端可能重发边界上的那条——
        // 我们宁可重复也不愿缺失（api/events.go 的「先订阅再补历史」），
        // 而重复要在这里收掉，否则用户会看到某句话被说了两遍。
        if (prev.some((e) => e.seq === parsed.seq)) {
          return prev
        }
        return [...prev, parsed]
      })
    }

    const connect = async () => {
      while (!controller.signal.aborted) {
        try {
          await readEventStream(controller.signal, lastSeq, push)
          retry = RETRY_MIN_MS
        } catch {
          // 连不上或读断了。不区分原因：能做的都是等一会儿再来一次。
        }
        if (controller.signal.aborted) {
          return
        }
        setDisconnected(true)
        await sleep(retry, controller.signal)
        // 指数退避：后端没起来时，固定间隔重连会一直刷日志
        retry = Math.min(retry * 2, RETRY_MAX_MS)
      }
    }

    void connect()

    return () => {
      // ★ 必须中止。不中止的话用户每切一次工作就多一条挂着的请求，
      // 而后端每条事件都要往所有订阅者投递一遍——开一天会肉眼可见地变慢。
      controller.abort()
    }
  }, [workID])

  return { events, disconnected }
}

/** 可被中止的等待。 */
function sleep(ms: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve) => {
    const timer = setTimeout(resolve, ms)
    signal.addEventListener('abort', () => {
      clearTimeout(timer)
      resolve()
    })
  })
}
