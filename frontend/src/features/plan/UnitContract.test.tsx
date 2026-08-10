import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { UnitContract } from './UnitContract'

// M7 完成标志 1 · 契约可查看全文
//
// ★★ **边界要摆在明处**：它是用户唯一的防线——没有它，「AI 要动文件」
// 只能靠他逐条判断。

const getContract = vi.fn()
const freezeContract = vi.fn()

vi.mock('@/api/system', () => ({
  getContract: (...a: unknown[]): unknown => getContract(...a),
  freezeContract: (...a: unknown[]): unknown => freezeContract(...a),
}))

const CONTRACT = {
  unit_id: 'unit-013',
  version: 1,
  frozen: false,
  criteria: [
    { id: 'ac-1', text: '连点两次取消只发一次协议取消请求' },
    { id: 'ac-2', text: '取消后 diff 与最后事件游标可读' },
  ],
  boundary: { allowed: ['internal/acp/'], forbidden: ['internal/api/gen/'] },
}

beforeEach(() => {
  getContract.mockReset().mockResolvedValue(CONTRACT)
  freezeContract.mockReset()
})

describe('单元契约', () => {
  // ★ 按需展开：十几个单元全展开的话，真正要看的那一个会淹在里面
  it('点开才拉取', async () => {
    const user = userEvent.setup()
    render(<UnitContract workID="work-01" unitID="unit-013" />)

    expect(getContract).not.toHaveBeenCalled()
    await user.click(screen.getByRole('button', { name: /查看契约/ }))

    await waitFor(() => {
      expect(getContract).toHaveBeenCalledWith('work-01', 'unit-013')
    })
  })

  // ★★ 验收标准与**写入边界**都看得到——边界是用户唯一的防线。
  it('显示验收标准与写入边界', async () => {
    const user = userEvent.setup()
    render(<UnitContract workID="work-01" unitID="unit-013" />)
    await user.click(screen.getByRole('button', { name: /查看契约/ }))

    expect(await screen.findByText('contract v1')).toBeInTheDocument()
    expect(screen.getByText('连点两次取消只发一次协议取消请求')).toBeInTheDocument()
    expect(screen.getByText('internal/acp/')).toBeInTheDocument()
    expect(screen.getByText('internal/api/gen/')).toBeInTheDocument()
  })

  // ★★ 一条允许项都没有时**明说**：空着的话用户以为「没限制」，
  // 而实际上是「一个字节都不许改」。
  it('没有允许项时明说', async () => {
    const user = userEvent.setup()
    getContract.mockResolvedValue({
      ...CONTRACT,
      boundary: { allowed: [], forbidden: [] },
    })
    render(<UnitContract workID="work-01" unitID="unit-013" />)
    await user.click(screen.getByRole('button', { name: /查看契约/ }))

    expect(await screen.findByText(/一个字节都不许改/)).toBeInTheDocument()
  })

  // ★ 冻结**由用户点**：冻结之后 AI 才能照着它开工。
  it('冻结之后显示「已冻结」且按钮消失', async () => {
    const user = userEvent.setup()
    freezeContract.mockResolvedValue({ ...CONTRACT, frozen: true })
    render(<UnitContract workID="work-01" unitID="unit-013" />)
    await user.click(screen.getByRole('button', { name: /查看契约/ }))
    await screen.findByText('contract v1')

    await user.click(screen.getByRole('button', { name: /冻结契约/ }))

    await waitFor(() => {
      expect(screen.getByText('已冻结')).toBeInTheDocument()
    })
    expect(screen.queryByRole('button', { name: /冻结契约/ })).not.toBeInTheDocument()
  })

  // ★★ 空契约冻不上时**说清为什么**——那时用户该做的是先补一条标准。
  it('空契约冻不上时说清为什么', async () => {
    const user = userEvent.setup()
    freezeContract.mockRejectedValue(new Error('contract_empty'))
    render(<UnitContract workID="work-01" unitID="unit-013" />)
    await user.click(screen.getByRole('button', { name: /查看契约/ }))
    await screen.findByText('contract v1')

    await user.click(screen.getByRole('button', { name: /冻结契约/ }))

    await waitFor(() => {
      expect(screen.getByText(/没有判据/)).toBeInTheDocument()
    })
  })

  // 还没有契约时说清楚，而不是一句「操作失败」。
  it('还没有契约时说清楚', async () => {
    const user = userEvent.setup()
    getContract.mockRejectedValue(new Error('contract_not_found'))
    render(<UnitContract workID="work-01" unitID="unit-013" />)
    await user.click(screen.getByRole('button', { name: /查看契约/ }))

    expect(await screen.findByText(/还没有契约/)).toBeInTheDocument()
  })
})
