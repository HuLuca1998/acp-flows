import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { ResumeBar } from './ResumeBar'

// M8 U8.3.1 · 启动时列出「能接着做的」
//
// ★★ 这一整块是**探索发现的功能级缺口**补的：后端早就做完了，
// 而界面上没有任何入口——用户永远看不到「有 2 个工作可以接着做」，
// 那整套检查点代码等于没用，而他不会知道自己少了什么。

const listResumable = vi.fn()
const resumeWork = vi.fn()

vi.mock('@/api/library', () => ({
  listResumable: (): unknown => listResumable(),
  resumeWork: (...a: unknown[]): unknown => resumeWork(...a),
}))

beforeEach(() => {
  resumeWork.mockReset().mockResolvedValue(undefined)
  listResumable.mockReset().mockResolvedValue([
    { work_id: 'work-03', checkpoint_id: 'work-03', unit_id: 'unit-013', paused_at: '2026-08-10T02:00:00Z' },
  ])
})

describe('接着做', () => {
  // R1 ★★ 显示**停在哪个单元**。
  //
  // 只有工作 id 的话，用户看到的是「work-03 · work-05」两行——
  // 那两个词对他没有任何意义。他记得的是「我在做那个取消功能」。
  it('显示停在哪个单元，不只是工作 id', async () => {
    render(<ResumeBar onResume={vi.fn()} />)

    expect(
      await screen.findByText('unit-013'),
      '只显示了 work-0N——用户认不出哪个是他刚才在做的',
    ).toBeInTheDocument()
  })

  // R2 ★ 一个都没有时**整块不显示**。
  //
  // 那是绝大多数人每次打开应用的状态，摆一个空标题是纯噪音。
  it('一个都没有时整块不显示', async () => {
    listResumable.mockResolvedValue([])
    const { container } = render(<ResumeBar onResume={vi.fn()} />)

    await waitFor(() => {
      expect(listResumable).toHaveBeenCalled()
    })
    expect(container).toBeEmptyDOMElement()
  })

  // R4 ★★ 点一条**先真的恢复**（切 worktree、脏检查），成功了再跳。
  //
  // 直接跳的话，界面看起来能用而那一整套恢复逻辑从没跑过。
  it('点一条先恢复再跳，不是直接跳', async () => {
    const onResume = vi.fn()
    const user = userEvent.setup()
    render(<ResumeBar onResume={onResume} />)

    await user.click(await screen.findByText('unit-013'))

    await waitFor(() => {
      expect(resumeWork, '直接跳了——那一整套恢复逻辑从没跑过').toHaveBeenCalledWith(
        'work-03',
        false,
      )
    })
    expect(onResume).toHaveBeenCalledWith('work-03')
  })

  // R3 ★★ 工作区脏时**先告知**，不静默恢复。
  //
  // 他手工改过那个 worktree，而状态推回可跑之后 AI 会接着往上写。
  it('工作区脏时问一句，不静默继续', async () => {
    const onResume = vi.fn()
    resumeWork.mockRejectedValueOnce(new Error('worktree_dirty'))
    const user = userEvent.setup()
    render(<ResumeBar onResume={onResume} />)

    await user.click(await screen.findByText('unit-013'))

    expect(await screen.findByText(/没提交的改动/)).toBeInTheDocument()
    expect(onResume, '脏着就跳进去了——AI 会接着往他的改动上写').not.toHaveBeenCalled()
  })

  // ★ 他确认之后带 force 再来一次。
  it('确认之后带 force 继续', async () => {
    const onResume = vi.fn()
    resumeWork.mockRejectedValueOnce(new Error('worktree_dirty')).mockResolvedValueOnce(undefined)
    const user = userEvent.setup()
    render(<ResumeBar onResume={onResume} />)

    await user.click(await screen.findByText('unit-013'))
    await user.click(await screen.findByText('我知道了，继续'))

    await waitFor(() => {
      expect(resumeWork).toHaveBeenLastCalledWith('work-03', true)
    })
    expect(onResume).toHaveBeenCalledWith('work-03')
  })

  // ★ 恢复失败（不是脏）时**不跳**：跳过去他会对着一个没恢复的工作接着说话。
  it('恢复失败时不跳过去', async () => {
    const onResume = vi.fn()
    resumeWork.mockRejectedValue(new Error('resume_failed'))
    const user = userEvent.setup()
    render(<ResumeBar onResume={onResume} />)

    await user.click(await screen.findByText('unit-013'))

    await waitFor(() => {
      expect(resumeWork).toHaveBeenCalled()
    })
    expect(onResume, '没恢复成却跳过去了——他会对着一个没恢复的工作接着说话').not.toHaveBeenCalled()
  })

  // ★★ **不自动恢复**：接着做是用户的决定。
  //
  // 自动跳进去的话，他打开应用想开个新工作，
  // 却发现自己落在昨天那个半截的现场里。
  it('不自动恢复，等用户点', async () => {
    const onResume = vi.fn()
    render(<ResumeBar onResume={onResume} />)
    await screen.findByText('unit-013')

    expect(onResume, '没等用户点就自己跳进去了').not.toHaveBeenCalled()
  })

  // ★ 查不了就不显示这一块，**不摆一条红字**。
  //
  // 用户多半根本没有可恢复的工作，为一次后台查询失败在首屏摆错误提示，
  // 会让他以为应用坏了。
  it('查不了时安静地不显示', async () => {
    listResumable.mockRejectedValue(new Error('boom'))
    const { container } = render(<ResumeBar onResume={vi.fn()} />)

    await waitFor(() => {
      expect(listResumable).toHaveBeenCalled()
    })
    expect(container).toBeEmptyDOMElement()
  })
})
