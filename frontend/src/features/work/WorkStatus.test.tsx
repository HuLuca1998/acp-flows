import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { WorkStatus } from './WorkStatus'

// M3 U3.2.2 · 取消按钮与状态（验收点 V9）
//
// ★ 设计稿里没有「取消态」的独立条目（本单元 stop_conditions 的那一条）。
// 判断：它是**已有语汇的组合**，不是新发明——
//   · 状态词照设计稿的 `executing · 3/7`：等宽英文原值 + 状态圆点
//   · 「停下」按钮照权限卡片的「拒绝」：灰边框次要按钮
// 铁律 3 防的是临时发明样式，组合已有条目不算。

const cancelWork = vi.fn()

vi.mock('@/api/system', () => ({
  cancelWork: (...a: unknown[]): unknown => cancelWork(...a),
}))

beforeEach(() => {
  cancelWork.mockReset().mockResolvedValue(undefined)
})

describe('工作状态与停下', () => {
  // ★★ R3：状态词是**等宽英文原值**，不翻译。
  //
  // 术语表明写状态词不翻译（AGENTS.md §8）：用户在文档、日志、界面上
  // 看到的必须是同一个词，否则他没法把三者对上。
  it('R3 · 状态词显示英文原值', () => {
    render(<WorkStatus workID="work-01" state="executing" />)

    expect(screen.getByText('executing')).toBeInTheDocument()
  })

  it('R3 · 停下之后显示终态原值', () => {
    render(<WorkStatus workID="work-01" state="paused" />)

    expect(screen.getByText('paused')).toBeInTheDocument()
  })

  // 能停的状态才给按钮。不能停的时候给一个点了没用的按钮，
  // 用户会以为是应用卡了。
  it('在跑的时候才有「停下」', () => {
    const { rerender } = render(<WorkStatus workID="work-01" state="executing" />)
    expect(screen.getByRole('button', { name: /停下/ })).toBeInTheDocument()

    rerender(<WorkStatus workID="work-01" state="paused" />)
    expect(screen.queryByRole('button', { name: /停下/ })).not.toBeInTheDocument()
  })

  // ★★ 审查中**不给按钮，并说清楚为什么**。
  //
  // 只把按钮变灰的话，用户会以为是应用卡了。他需要知道
  // 「不是不让你停，是现在停了会留下一个说不清的半成品」。
  it('审查中说清楚为什么不能停', () => {
    render(<WorkStatus workID="work-01" state="reviewing_unit" />)

    expect(screen.queryByRole('button', { name: /停下/ })).not.toBeInTheDocument()
    expect(
      screen.getByText(/审查/),
      '只是没有按钮，没说为什么——用户会以为是应用卡了',
    ).toBeInTheDocument()
  })

  // ★★ R2：点了之后**立刻**看到反馈，不是无响应。
  it('R2 · 提交中显示进行中态并禁用', async () => {
    const user = userEvent.setup()
    let release: (() => void) | undefined
    cancelWork.mockImplementation(
      () =>
        new Promise<void>((resolve) => {
          release = resolve
        }),
    )
    render(<WorkStatus workID="work-01" state="executing" />)

    await user.click(screen.getByRole('button', { name: /停下/ }))

    const btn = await screen.findByRole('button', { name: /正在停/ })
    expect(btn, '点了之后按钮没变——用户不知道点上没有，会再点一次').toBeDisabled()

    release?.()
  })

  it('点「停下」调对端点', async () => {
    const user = userEvent.setup()
    render(<WorkStatus workID="work-07" state="executing" />)

    await user.click(screen.getByRole('button', { name: /停下/ }))

    await waitFor(() => {
      expect(cancelWork).toHaveBeenCalledWith('work-07')
    })
  })

  // ★★ R4：停不下来时**如实告知**，不假装成功。
  //
  // 静默失败的话，用户以为停住了而 AI 照跑——账单继续涨，
  // 而他不会再看这个界面。
  it('R4 · 失败时说出来，不假装成功', async () => {
    const user = userEvent.setup()
    cancelWork.mockRejectedValue(new Error('work_operation_failed'))
    render(<WorkStatus workID="work-01" state="executing" />)

    await user.click(screen.getByRole('button', { name: /停下/ }))

    await waitFor(() => {
      expect(
        screen.getByText(/没能停下|失败/),
        '停不下来却没有任何提示——用户以为停住了而 AI 照跑',
      ).toBeInTheDocument()
    })
    // 还能再试一次
    expect(screen.getByRole('button', { name: /停下/ })).toBeEnabled()
  })

  // ★ 「现在不能停」与「停失败了」是两回事，文案要分得开。
  //
  // 都说「失败」的话，用户会一直重试一个注定被拒的操作。
  it('被拒绝时给的是解释，不是「失败」', async () => {
    const user = userEvent.setup()
    cancelWork.mockRejectedValue(new Error('work_cancel_not_allowed'))
    render(<WorkStatus workID="work-01" state="executing" />)

    await user.click(screen.getByRole('button', { name: /停下/ }))

    await waitFor(() => {
      expect(screen.getByText(/现在不能停|等它/)).toBeInTheDocument()
    })
  })

  // 认不出的状态照样显示原值，不显示空白。
  //
  // 显示空白的话，用户不知道是「没状态」还是「界面坏了」。
  it('认不出的状态也显示原值', () => {
    render(<WorkStatus workID="work-01" state="量子叠加态" />)

    expect(screen.getByText('量子叠加态')).toBeInTheDocument()
  })
})
