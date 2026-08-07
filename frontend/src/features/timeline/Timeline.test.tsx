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

// ── U2.4.1 · 真机走查补的：工具调用要看得出在干什么 ──────────

function toolEv(payload: Record<string, unknown>): TimelineEvent {
  seq += 1
  return {
    id: `evt_${seq}`,
    seq,
    source: 'acp',
    type: 'tool_call',
    ts: '2026-08-08T00:00:00Z',
    payload,
  }
}

describe('工具调用', () => {
  // ★★ 一次工具调用是**一张卡片**，不是四张。
  //
  // ACP 会为同一次调用连发 tool_call + 若干 tool_call_update（状态变化），
  // 它们共用一个 toolCallId。不归并的话，用户看到四条一模一样的「工具调用」，
  // 以为 AI 动了四个文件——真机上撞到的第一个问题。
  it('同一个 toolCallId 归并成一张卡片', () => {
    render(
      <Timeline
        events={[
          toolEv({ acp_kind: 'tool_call', toolCallId: 't1', title: 'Read README.md', kind: 'read' }),
          toolEv({ acp_kind: 'tool_call_update', toolCallId: 't1', status: 'in_progress' }),
          toolEv({ acp_kind: 'tool_call_update', toolCallId: 't1', status: 'completed' }),
        ]}
      />,
    )

    // ★ 断言的是**卡片数量**，不是标题出现了几次。
    // 只有第一条事件带 title，所以数标题的话，不归并时它照样只出现一次——
    // 那样这条测试什么都证明不了（造负例时发现的）。
    const cards = document.querySelectorAll('[data-event-type="tool_call"]')
    expect(cards, '同一次调用被拆成了多张卡片——用户会以为 AI 动了好几个文件').toHaveLength(1)
    expect(screen.getByText('Read README.md')).toBeInTheDocument()
  })

  // 不同的调用不能并到一起——并了的话用户以为 AI 只动了一个文件。
  it('不同的 toolCallId 各占一张卡片', () => {
    render(
      <Timeline
        events={[
          toolEv({ acp_kind: 'tool_call', toolCallId: 't1', title: 'Read a.md' }),
          toolEv({ acp_kind: 'tool_call', toolCallId: 't2', title: 'Read b.md' }),
        ]}
      />,
    )

    expect(document.querySelectorAll('[data-event-type="tool_call"]')).toHaveLength(2)
    expect(screen.getByText('Read a.md')).toBeInTheDocument()
    expect(screen.getByText('Read b.md')).toBeInTheDocument()
  })

  // ★ 卡片上要写清楚**在干什么**。
  //
  // 只显示「工具调用」四个字的话，信息量是零——设计稿里每条事件行都是
  // 「图标 + 类型 + 等宽标识 + 一句人话」，光有类型标签比设计稿差。
  it('显示 Agent 给的标题', () => {
    render(<Timeline events={[toolEv({ toolCallId: 't1', title: 'Edit src/main.go' })]} />)

    expect(screen.getByText('Edit src/main.go')).toBeInTheDocument()
  })

  // 没有 title 时退到文件路径——总比只显示「工具调用」强。
  it('没有标题时退到文件路径', () => {
    render(
      <Timeline
        events={[toolEv({ toolCallId: 't1', rawInput: { file_path: '/repo/README.md' } })]}
      />,
    )

    expect(screen.getByText(/README\.md/)).toBeInTheDocument()
  })

  // ★ 最终状态要盖住中间态：一次调用完成之后，卡片上不该还写着「进行中」。
  it('状态取最后一次更新', () => {
    render(
      <Timeline
        events={[
          toolEv({ toolCallId: 't1', title: 'Read a.md', status: 'in_progress' }),
          toolEv({ toolCallId: 't1', status: 'completed' }),
        ]}
      />,
    )

    const card = screen.getByText('Read a.md').closest('[data-event-type="tool_call"]')
    expect(card?.getAttribute('data-status'), '状态停在中间态——用户以为还在跑').toBe('completed')
  })

  // ★★ 归并时，**后来的低质量摘要不许顶掉先前的标题**。
  //
  // 真机撞到的：tool_call 带 title「Read README.md」，随后的
  // tool_call_update 只带 kind，结果卡片上显示的是「tool_call_update」——
  // 用户看不出 AI 读的是哪个文件，等于白归并了。
  it('状态更新不会把标题顶掉', () => {
    render(
      <Timeline
        events={[
          toolEv({ toolCallId: 't1', title: 'Read README.md', kind: 'read' }),
          toolEv({ toolCallId: 't1', kind: 'tool_call_update', status: 'completed' }),
        ]}
      />,
    )

    expect(
      screen.getByText('Read README.md'),
      '标题被后来的状态更新顶掉了——用户看不出 AI 在读哪个文件',
    ).toBeInTheDocument()
  })

  // ★★ 同一档的后来者要**覆盖**先前的。
  //
  // 真机上 Claude 先给泛称「Read File」，随后的 tool_call_update 才补上
  // 具体的「Read README.md」——两者都在 title 上。只让「更好的档」覆盖的话，
  // 卡片会停在「Read File」，用户仍然看不出读的是哪个文件。
  it('后来的同档标题会覆盖先前的', () => {
    render(
      <Timeline
        events={[
          toolEv({ toolCallId: 't1', title: 'Read File', kind: 'read' }),
          toolEv({ toolCallId: 't1', title: 'Read README.md' }),
        ]}
      />,
    )

    expect(
      screen.getByText('Read README.md'),
      'ACP 后来补的具体标题没生效——卡片停在泛称，看不出读的是哪个文件',
    ).toBeInTheDocument()
    expect(screen.queryByText('Read File')).not.toBeInTheDocument()
  })

  // 反过来：真带了更好的标题时要更新。
  it('后来带了标题时会补上', () => {
    render(
      <Timeline
        events={[
          toolEv({ toolCallId: 't1', kind: 'read' }),
          toolEv({ toolCallId: 't1', title: 'Read README.md' }),
        ]}
      />,
    )

    expect(screen.getByText('Read README.md')).toBeInTheDocument()
  })

  // 载荷里什么都没有时不能白屏，也不能显示一个空卡片。
  it('载荷是空的也不崩', () => {
    render(<Timeline events={[toolEv({})]} />)

    expect(screen.getByText(/工具调用/)).toBeInTheDocument()
  })
})
