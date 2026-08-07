import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { Timeline } from './Timeline'
import type { TimelineEvent } from './event-registry'

// M2 U2.3.2 · 时间线渲染（验收点 V6）

let seq = 0
function ev(type: string, text?: string): TimelineEvent {
  seq += 1
  return {
    id: `evt_${seq}`,
    seq,
    source: 'acp',
    type: type as TimelineEvent['type'],
    ts: '2026-08-08T00:00:00Z',
    ...(text === undefined ? {} : { payload: { text } }),
  }
}

describe('时间线', () => {
  // ★ R4：文字流式追加**不闪烁**。
  //
  // 流式文本一个字一个字地来。每片一个气泡的话，界面会在打字过程中疯狂重排——
  // 用户看到的是一堆跳动的小方块，而不是一句正在被写出来的话。
  it('同一条消息的多个片段合并进同一个气泡', () => {
    render(
      <Timeline
        events={[
          ev('message_chunk', '我先'),
          ev('message_chunk', '看一下'),
          ev('message_chunk', '这个文件'),
        ]}
      />,
    )

    const items = screen.getAllByText(/我先看一下这个文件/)
    expect(items).toHaveLength(1)
    // 三片合成一个，而不是三个
    expect(document.querySelectorAll('[data-event-type="message_chunk"]')).toHaveLength(1)
  })

  // ★ 但**不是什么都合并**。工具调用两次就是两次——
  // 合并的话用户会以为 AI 只动了一个文件。
  it('工具调用不合并，两次就是两条', () => {
    render(<Timeline events={[ev('tool_call'), ev('tool_call')]} />)

    expect(document.querySelectorAll('[data-event-type="tool_call"]')).toHaveLength(2)
  })

  it('不同类型之间不会串成一条', () => {
    render(
      <Timeline
        events={[ev('message_chunk', 'A'), ev('thought_chunk', 'B'), ev('message_chunk', 'C')]}
      />,
    )

    // A、B、C 三段各自成条：中间隔了一类，A 与 C 不该接上
    expect(document.querySelectorAll('[data-event-type]')).toHaveLength(3)
  })

  // ★ R3：未知类型**不白屏**。
  //
  // 后端加一类事件而前端还没跟上时，用户看到的应该是「有一条我暂时看不懂的记录」，
  // 而不是整页没了——白屏会让他以为整个应用坏了。
  it('没见过的事件类型照样渲染，其余内容不受影响', () => {
    render(
      <Timeline
        events={[
          ev('message_chunk', '正常内容'),
          ev('something_from_the_future'),
          ev('message_chunk', '后面的内容'),
        ]}
      />,
    )

    expect(screen.getByText('正常内容')).toBeInTheDocument()
    expect(screen.getByText('后面的内容')).toBeInTheDocument()
    // 未知那条也在，用兜底文案
    expect(screen.getByText(/还不认识它/)).toBeInTheDocument()
  })

  // 载荷形状意外时也不能炸——后端加字段、改结构是常事。
  it('载荷里没有 text 时不崩，只是那条没内容', () => {
    render(<Timeline events={[ev('message_chunk'), ev('state_change')]} />)

    expect(document.querySelectorAll('[data-event-type]')).toHaveLength(2)
  })

  it('被过滤掉的类型不渲染', () => {
    render(
      <Timeline
        events={[ev('message_chunk', '看得见'), ev('thought_chunk', '被关掉了')]}
        hidden={new Set(['thought_chunk'])}
      />,
    )

    expect(screen.getByText('看得见')).toBeInTheDocument()
    expect(screen.queryByText('被关掉了')).not.toBeInTheDocument()
  })

  it('一条都没有时给出空状态而不是一片空白', () => {
    render(<Timeline events={[]} />)

    expect(screen.getByText(/还没有/)).toBeInTheDocument()
  })

  // 三种形态各自有自己的样式钩子，渲染器决定用哪个。
  it('形态由注册表决定：气泡 / 卡片 / 单行', () => {
    render(
      <Timeline events={[ev('message_chunk', 'x'), ev('tool_call'), ev('state_change')]} />,
    )

    expect(document.querySelector('[data-shape="bubble"]')).not.toBeNull()
    expect(document.querySelector('[data-shape="card"]')).not.toBeNull()
    expect(document.querySelector('[data-shape="line"]')).not.toBeNull()
  })
})
