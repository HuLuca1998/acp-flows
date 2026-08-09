import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { RequirementBar } from './RequirementBar'

// M5 U5.2.1 · 需求快照条
//
// ★★ 冻结**由用户点，不由 AI 判断**。AI 说「我觉得问清楚了」和用户说
// 「就这样」是两件事——而冻结之后这一版就进了计划与契约，改不动了。

const getRequirement = vi.fn()
const freezeRequirement = vi.fn()

vi.mock('@/api/system', () => ({
  getRequirement: (...a: unknown[]): unknown => getRequirement(...a),
  freezeRequirement: (...a: unknown[]): unknown => freezeRequirement(...a),
}))

beforeEach(() => {
  getRequirement.mockReset().mockResolvedValue({
    version: 1,
    items: ['用户能取消正在运行的 turn', '取消后现场证据要保留'],
    open_facts: [],
    frozen: false,
  })
  freezeRequirement.mockReset()
})

describe('需求快照条', () => {
  it('显示版本号与条数', async () => {
    render(<RequirementBar workID="work-01" />)

    expect(await screen.findByText('requirement v1')).toBeInTheDocument()
    expect(screen.getByText(/2 条需求/)).toBeInTheDocument()
  })

  it('用户点冻结之后显示「已冻结」', async () => {
    const user = userEvent.setup()
    freezeRequirement.mockResolvedValue({
      version: 1,
      items: ['用户能取消正在运行的 turn'],
      open_facts: [],
      frozen: true,
    })
    render(<RequirementBar workID="work-01" />)

    await user.click(await screen.findByRole('button', { name: /冻结这一版/ }))

    await waitFor(() => {
      expect(screen.getByText('已冻结')).toBeInTheDocument()
    })
    expect(freezeRequirement).toHaveBeenCalledWith('work-01')
    // 冻完就没有可点的按钮了——冻结是不可逆的
    expect(screen.queryByRole('button', { name: /冻结这一版/ })).not.toBeInTheDocument()
  })

  // ★★ 还有待确认的事实时**说清楚**，而不是一句「操作失败」。
  //
  // 那时用户该做的是先回答那几个问题，不是重试。
  it('还有待确认的事实时说清为什么冻不上', async () => {
    const user = userEvent.setup()
    getRequirement.mockResolvedValue({
      version: 1,
      items: ['用户能取消正在运行的 turn'],
      open_facts: ['取消时是否需要回滚已写入的文件'],
      frozen: false,
    })
    freezeRequirement.mockRejectedValue(new Error('requirement_open_facts_remain'))
    render(<RequirementBar workID="work-01" />)

    // 还剩几条要摆在明处
    expect(await screen.findByText(/还有 1 条待确认/)).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /冻结这一版/ }))
    await waitFor(() => {
      expect(screen.getByText(/先把它们问清楚再冻结/)).toBeInTheDocument()
    })
  })

  // ★★ 还没有需求时**整条不显示**，且不报错。
  //
  // 那是新工作的常态——弹一句「读取需求失败」会让用户以为出了什么事。
  it('还没有需求时安静地什么都不显示', async () => {
    getRequirement.mockRejectedValue(new Error('requirement_not_found'))
    const { container } = render(<RequirementBar workID="work-01" />)

    await waitFor(() => {
      expect(getRequirement).toHaveBeenCalled()
    })
    expect(container).toBeEmptyDOMElement()
    expect(screen.queryByText(/失败/)).not.toBeInTheDocument()
  })

  // 没有工作时不去查——查一个空 id 只会拿到 404。
  it('没有工作时不发请求', () => {
    render(<RequirementBar workID="" />)
    expect(getRequirement).not.toHaveBeenCalled()
  })
})
