import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { DecisionDock } from './DecisionDock'

// M9 U9.2.1 · 决策卡片
//
// ★★ **决定权在用户手里**。AI 自己选一个往下走的话，用户是在几十个
// 文件之后才发现「它怎么这么做了」。

const listPendingDecisions = vi.fn()
const answerDecision = vi.fn()

vi.mock('@/api/system', () => ({
  listPendingDecisions: (...a: unknown[]): unknown => listPendingDecisions(...a),
  answerDecision: (...a: unknown[]): unknown => answerDecision(...a),
}))

const DECISION = {
  id: 'dec-01',
  unit_id: 'unit-013',
  level: 'D2',
  question: '取消之后要不要回滚已写入的文件？',
  options: [
    {
      id: 'a',
      text: '不回滚，仅停止',
      impact: '已写入的文件留着，下次从这里接着干',
      recommended: true,
    },
    { id: 'b', text: '回退到检查点', impact: '回到 ck-07，这一轮的三个文件改动会没有', recommended: false },
  ],
}

beforeEach(() => {
  listPendingDecisions.mockReset().mockResolvedValue([DECISION])
  answerDecision.mockReset()
})

describe('决策卡片', () => {
  // ★★ **每个选项都显示影响**：没有它用户在盲选——
  // 他看到两个名字，而不知道选哪个会发生什么。
  it('每个选项都显示影响', async () => {
    render(<DecisionDock workID="work-01" />)

    expect(await screen.findByText(/取消之后要不要回滚/)).toBeInTheDocument()
    expect(screen.getByText(/已写入的文件留着/)).toBeInTheDocument()
    expect(
      screen.getByText(/这一轮的三个文件改动会没有/),
      '选项没有影响说明——用户会在盲选',
    ).toBeInTheDocument()
  })

  // ★★ 推荐的**只标出来，不预选**。
  //
  // 预选中的话用户会顺手点确定，而那正好绕过了「让他自己决定」这件事。
  it('推荐的只标出来，不预选中', async () => {
    render(<DecisionDock workID="work-01" />)
    await screen.findByText(/取消之后要不要回滚/)

    expect(screen.getByText('推荐')).toBeInTheDocument()
    // ★ 判据：**没有任何一个按钮是选中态**
    expect(document.querySelector('[aria-pressed="true"]')).toBeNull()
    expect(document.querySelector('input:checked')).toBeNull()
  })

  // 等级显示**原值**（D2），不翻译——术语表硬要求。
  it('等级显示英文原值', async () => {
    render(<DecisionDock workID="work-01" />)
    expect(await screen.findByText('D2')).toBeInTheDocument()
  })

  it('点一个选项就把它送回去，并刷新待决列表', async () => {
    const user = userEvent.setup()
    answerDecision.mockResolvedValue(undefined)
    listPendingDecisions.mockResolvedValueOnce([DECISION]).mockResolvedValueOnce([])
    render(<DecisionDock workID="work-01" />)

    await user.click(await screen.findByText('回退到检查点'))

    await waitFor(() => {
      expect(answerDecision).toHaveBeenCalledWith('work-01', 'dec-01', 'b')
    })
    // 答完之后卡片消失
    await waitFor(() => {
      expect(screen.queryByText(/取消之后要不要回滚/)).not.toBeInTheDocument()
    })
  })

  // ★ 「稍后决定」只是一句说明，**不是第四个按钮**——
  // 它不该是一个会改变现场的动作。
  it('「稍后决定」不是一个按钮', async () => {
    render(<DecisionDock workID="work-01" />)
    await screen.findByText(/取消之后要不要回滚/)

    expect(screen.getByText(/它会一直在这儿等你/)).toBeInTheDocument()
    // 只有两个选项按钮，没有第三个
    expect(screen.getAllByRole('button')).toHaveLength(2)
  })

  // 没有待决策时**整块不显示**——那是常态。
  it('没有待决策时什么都不显示', async () => {
    listPendingDecisions.mockResolvedValue([])
    const { container } = render(<DecisionDock workID="work-01" />)

    await waitFor(() => {
      expect(listPendingDecisions).toHaveBeenCalled()
    })
    expect(container).toBeEmptyDOMElement()
  })
})
