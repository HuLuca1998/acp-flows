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

/**
 * 带角色的事件。
 *
 * ★ 复用 `ev()` 而不是另写一份：另写的话，将来契约加一个必填字段时
 * 会有两处要改，而漏掉一处只有 tsc 会红。
 */
function roleEv(
  type: string,
  role: string,
  roleName: string,
  text?: string,
  runtime?: string,
): TimelineEvent {
  return {
    ...ev(type, text),
    role,
    role_display_name: roleName,
    ...(runtime === undefined ? {} : { runtime }),
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

// M5 U5.3.1 · 消息按角色分栏
//
// ★★ 用户正是靠角色标签判断「现在是谁在说话、他能不能动我的文件」。

describe('角色标签', () => {
  // ★ 标签形态照设计稿：`Claude · 需求分析师`。
  it('显示角色与它用的 Runtime', () => {
    render(
      <Timeline
        events={[
          roleEv('message_chunk', 'requirement_analyst', '需求分析师', '我先问几个问题', 'claude'),
        ]}
      />,
    )

    expect(screen.getByText('需求分析师')).toBeInTheDocument()
    expect(screen.getByText('claude')).toBeInTheDocument()
  })

  // ★★ **角色不同就不合并**：那是两个人在说话。
  //
  // 并进去的话，需求分析师和实现工程师的话会挤在同一个气泡里，
  // 而标签只剩一个——用户分不清哪句是谁说的。
  it('不同角色的连续消息不合并', () => {
    render(
      <Timeline
        events={[
          roleEv('message_chunk', 'requirement_analyst', '需求分析师', '问题问完了。'),
          roleEv('message_chunk', 'implementer', '实现工程师', '我开始写。'),
        ]}
      />,
    )

    expect(screen.getByText('需求分析师')).toBeInTheDocument()
    expect(
      screen.getByText('实现工程师'),
      '两个角色的话被并进同一个气泡了——用户分不清哪句是谁说的',
    ).toBeInTheDocument()
  })

  // 同一个角色的连续片段照常合并（那是流式文本，不合并会疯狂重排）。
  it('同角色的连续片段仍然合并', () => {
    render(
      <Timeline
        events={[
          roleEv('message_chunk', 'implementer', '实现工程师', '前半句'),
          roleEv('message_chunk', 'implementer', '实现工程师', '后半句'),
        ]}
      />,
    )

    expect(screen.getByText('前半句后半句')).toBeInTheDocument()
    expect(screen.getAllByText('实现工程师').length).toBe(1)
  })

  // ★★ 没有角色的事件**不显示这一块**。
  //
  // 填个「系统」上去，会让用户以为有个叫「系统」的角色在干活。
  it('应用自己发的事件不显示角色', () => {
    render(
      <Timeline events={[{ ...ev('state_change'), payload: { to: 'executing' } }]} />,
    )

    expect(screen.queryByText(/系统/)).not.toBeInTheDocument()
    // 消息本身照常显示
    expect(document.querySelector('[data-event-type="state_change"]')).not.toBeNull()
  })

  // ★ 认不出的角色**照常显示消息**，只是标签退化成后端给的原文。
  it('认不出的角色不吞掉消息', () => {
    render(
      <Timeline
        events={[
          roleEv('message_chunk', 'some_future_role', '某个新角色', '重要的话'),
        ]}
      />,
    )

    expect(screen.getByText('重要的话')).toBeInTheDocument()
    expect(screen.getByText('某个新角色')).toBeInTheDocument()
  })
})

// M5 U5.3.1 R2 · 用户消息右对齐气泡
//
// ★★ 挤在同一侧的话，一屏滚下来「哪句是我说的、哪句是它说的」
// 要逐条读文字才分得清。

describe('用户自己说的话', () => {
  it('和 AI 的话分列两侧', () => {
    render(
      <Timeline
        events={[
          ev('user_message', '先别写代码，先把范围说清楚。'),
          roleEv('message_chunk', 'requirement_analyst', '需求分析师', '好，我先问几个问题。'),
        ]}
      />,
    )

    const mine = document.querySelector('[data-event-type="user_message"]')
    const theirs = document.querySelector('[data-event-type="message_chunk"]')
    expect(mine?.getAttribute('data-align')).toBe('end')
    expect(
      theirs?.getAttribute('data-align'),
      'AI 的话也排到了右边——两侧分不开，用户要逐条读文字才知道哪句是自己说的',
    ).toBe('start')
  })

  // ★ 内容照常显示——对齐方式变了而话没了是更糟的结果。
  it('原话一个字不少', () => {
    render(<Timeline events={[ev('user_message', '取消后现场证据要保留')]} />)
    expect(screen.getByText('取消后现场证据要保留')).toBeInTheDocument()
  })

  // ★★ 用户消息**没有角色标签**：照设计稿，那一侧不画头像也不写名字。
  //
  // 给它安一个「用户 · 你」之类的标签，会让他以为自己也是被编排的一个角色。
  it('不带角色标签', () => {
    render(<Timeline events={[ev('user_message', '我说的话')]} />)

    const mine = document.querySelector('[data-event-type="user_message"]')
    expect(mine?.querySelector('[data-role]')).toBeNull()
  })
})

// M5 U5.3.1 R3 · `requirement v1` `已冻结` 标签
//
// ★★ 说这句话时需求是第几版——它决定用户下一步能做什么
// （没冻结就不能进计划）。

describe('需求版本标签', () => {
  function reqEv(version: number, frozen: boolean): TimelineEvent {
    return {
      ...roleEv('message_chunk', 'requirement_analyst', '需求分析师', '需求快照已更新'),
      requirement_version: version,
      requirement_frozen: frozen,
    }
  }

  it('显示版本号与冻结态', () => {
    render(<Timeline events={[reqEv(2, true)]} />)

    expect(screen.getByText('requirement v2')).toBeInTheDocument()
    expect(screen.getByText('已冻结')).toBeInTheDocument()
  })

  // 没冻结时只显示版本号——写个「未冻结」上去会让人以为那是个警告。
  it('没冻结时不显示「已冻结」', () => {
    render(<Timeline events={[reqEv(1, false)]} />)

    expect(screen.getByText('requirement v1')).toBeInTheDocument()
    expect(screen.queryByText('已冻结')).not.toBeInTheDocument()
  })

  // ★★ 还没有需求快照时**整块不显示**——「requirement v0」比不显示更糟。
  it('还没有需求快照时不显示这一块', () => {
    render(
      <Timeline
        events={[roleEv('message_chunk', 'requirement_analyst', '需求分析师', '我先问几个问题')]}
      />,
    )

    expect(screen.getByText('我先问几个问题')).toBeInTheDocument()
    expect(screen.queryByText(/requirement v/)).not.toBeInTheDocument()
  })

  // ★ 版本号来自**事件载荷**，不是前端另查一次。
  //
  // 另查拿到的是「现在」的版本，而用户看的是一条历史消息——
  // 他会以为当时就已经是 v3 了。
  it('每条消息各自带着当时的版本', () => {
    render(<Timeline events={[reqEv(1, true), reqEv(2, false)]} />)

    expect(screen.getByText('requirement v1')).toBeInTheDocument()
    expect(screen.getByText('requirement v2')).toBeInTheDocument()
  })
})

// M5 U5.3.2 · 消息带头像
//
// ★★ 一屏扫过去**不读文字**就分得清哪几条是同一个人说的——
// 只有标签的话，用户要逐条读文字才认得出来。

describe('头像', () => {
  it('有角色的消息带头像，缩写由 Runtime 名推出来', () => {
    render(
      <Timeline
        events={[
          roleEv('message_chunk', 'requirement_analyst', '需求分析师', '我先问几个问题', 'claude'),
        ]}
      />,
    )

    const avatar = document.querySelector('[data-runtime="claude"]')
    expect(avatar, '没有头像——用户要逐条读文字才分得清说话的人').not.toBeNull()
    expect(avatar?.textContent).toBe('CL')
  })

  // ★★ 缩写**由名字推导**，不硬编码对照表：加一个 Runtime 时不该还要
  // 回来补一行——漏补的话那个 Runtime 的头像是空白，用户看到一个没有身份的方块。
  it('没见过的 Runtime 也有两个字母', () => {
    render(
      <Timeline
        events={[roleEv('message_chunk', 'implementer', '实现工程师', '我开始写', 'codex')]}
      />,
    )
    expect(document.querySelector('[data-runtime="codex"]')?.textContent).toBe('CX')

    render(
      <Timeline
        events={[roleEv('message_chunk', 'implementer', '实现工程师', '我开始写', 'gemini')]}
      />,
    )
    const unknown = document.querySelector('[data-runtime="gemini"]')
    expect(unknown?.textContent).toHaveLength(2)
  })

  // ★ 没有角色的事件**不画头像**——画一个的话，用户会以为有个角色在说话。
  it('应用自己发的事件没有头像', () => {
    render(<Timeline events={[{ ...ev('state_change'), payload: { to: 'executing' } }]} />)

    expect(document.querySelector('[data-runtime]')).toBeNull()
  })

  // ★ 用户自己说的话也不画——他不是被编排的一个角色。
  it('用户消息没有头像', () => {
    render(<Timeline events={[ev('user_message', '先别写代码')]} />)

    const mine = document.querySelector('[data-event-type="user_message"]')
    expect(mine?.querySelector('[data-runtime]')).toBeNull()
  })
})
