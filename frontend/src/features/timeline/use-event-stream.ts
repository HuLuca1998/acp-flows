import { useEffect, useState } from 'react'

import type { TimelineEvent } from './event-registry'

export type EventStreamState = {
  events: TimelineEvent[]
  /** 连接断了；EventSource 会自己重连，这个只用来给界面一点提示。 */
  disconnected: boolean
}

/**
 * 订阅某个工作的事件流。
 *
 * ★ 用浏览器原生的 `EventSource`：它**自带断线重连，并自动带回
 * `Last-Event-ID`**——服务端据此只补没收到的那些（见 `api/events.go`）。
 * 自己用 fetch + ReadableStream 实现的话，这套续传要重写一遍。
 *
 * ★ 事件流是**全局的**（一条连接推所有工作），所以这里按 `work_id` 过滤。
 * 不过滤的话，用户开着 A 工作却看到 B 工作的 AI 在说话，
 * 那会让他以为自己点错了。
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

    const source = new EventSource('/v1/events')

    source.addEventListener('message', (raw: MessageEvent) => {
      let parsed: TimelineEvent
      try {
        parsed = JSON.parse(String(raw.data)) as TimelineEvent
      } catch {
        // 一条坏消息不该让整条流断掉：跳过它，后面的照常
        return
      }

      if (parsed.work_id !== workID) {
        return
      }

      setEvents((prev) => {
        // ★ 按 seq 去重。断线重连时服务端可能重发边界上的那条——
        // 我们宁可重复也不愿缺失（api/events.go 的「先订阅再补历史」），
        // 而重复要在这里收掉，否则用户会看到某句话被说了两遍。
        if (prev.some((e) => e.seq === parsed.seq)) {
          return prev
        }
        return [...prev, parsed]
      })
    })

    source.addEventListener('error', () => {
      // EventSource 会自己重连，这里只是给界面一点提示。
      // 主动 close 再自己重连的话，反而丢掉了浏览器内建的退避策略。
      setDisconnected(true)
    })

    return () => {
      // ★ 必须关。不关的话用户每切一次工作就多一条连接，
      // 而后端每条事件都要往所有订阅者投递一遍——开一天会肉眼可见地变慢。
      source.close()
    }
  }, [workID])

  return { events, disconnected }
}
