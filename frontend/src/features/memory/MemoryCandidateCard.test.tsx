import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { MemoryCandidateCard } from './MemoryCandidateCard'

// M10 U10.5.1 · 时间线上的记忆候选卡片
//
// ★★ **记忆由用户收下，不由 AI 自己写进去。** 这张卡片是那道关口——
// 后端保证候选一律是 candidate，这里保证他**看得到正文**再决定。

const getMemoryBody = vi.fn()
const reviewMemory = vi.fn()

vi.mock('@/api/library', () => ({
  getMemoryBody: (...a: unknown[]): unknown => getMemoryBody(...a),
  reviewMemory: (...a: unknown[]): unknown => reviewMemory(...a),
}))

const BODY_TEXT =
  'gorm AutoMigrate 会把 events 表的 role 列改成 NOT NULL，' +
  '而旧数据里那列是空的——启动直接失败。'

beforeEach(() => {
  getMemoryBody.mockReset().mockResolvedValue({ title: '迁移必须手写 SQL', text: BODY_TEXT })
  reviewMemory.mockReset().mockResolvedValue({})
})

describe('记忆候选卡片', () => {
  // R1 ★★ 卡片显示**正文全文**，不只是标题。
  //
  // 只给标题的话他在盲选——而收下之后这条会影响后面每一轮。
  it('显示正文全文，不只是标题', async () => {
    render(<MemoryCandidateCard memoryID="mem-01" title="迁移必须手写 SQL" />)

    expect(
      await screen.findByText(new RegExp('启动直接失败')),
      '只显示了标题——用户在盲选，而收下之后这条会影响后面每一轮',
    ).toBeInTheDocument()
  })

  // R2 ★★ 「收下」「不要」**都不预选中**。
  //
  // 预选「收下」的话他会顺手点确定，而那正好绕过了「让他自己决定」。
  it('两个按钮都不预选中', async () => {
    render(<MemoryCandidateCard memoryID="mem-01" title="迁移必须手写 SQL" />)
    await screen.findByText(new RegExp('启动直接失败'))

    expect(screen.getByText('收下')).toBeInTheDocument()
    expect(screen.getByText('不要')).toBeInTheDocument()
    expect(document.querySelector('[aria-pressed="true"]')).toBeNull()
    expect(document.querySelector('input:checked')).toBeNull()
  })

  // R3 ★ 收下之后卡片变成历史，不能再答。
  it('收下之后按钮消失，留下一条历史', async () => {
    const user = userEvent.setup()
    render(<MemoryCandidateCard memoryID="mem-01" title="迁移必须手写 SQL" />)
    await screen.findByText(new RegExp('启动直接失败'))

    await user.click(screen.getByText('收下'))

    await waitFor(() => {
      expect(reviewMemory).toHaveBeenCalledWith('mem-01', 'confirm', 'user')
    })
    expect(screen.queryByText('收下'), '还能再点一次——一条记忆会被收两遍').toBeNull()
    // ★ 决定过的**留在时间线上**：擦掉的话用户回头想不起来自己收了没有
    expect(screen.getByText('已收下')).toBeInTheDocument()
  })

  it('「不要」走的是 reject', async () => {
    const user = userEvent.setup()
    render(<MemoryCandidateCard memoryID="mem-01" title="迁移必须手写 SQL" />)
    await screen.findByText(new RegExp('启动直接失败'))

    await user.click(screen.getByText('不要'))

    await waitFor(() => {
      expect(reviewMemory).toHaveBeenCalledWith('mem-01', 'reject', 'user')
    })
    expect(screen.getByText('已丢弃')).toBeInTheDocument()
  })

  // ★★ 正文读不到时**明说**，不显示成空白。
  //
  // 空白看起来像「这条记忆没内容」，他会照着这个印象直接收下——
  // 而真相是那个 md 文件丢了。
  it('正文读不到时明说，不显示成空白', async () => {
    getMemoryBody.mockRejectedValue(new Error('memory_body_missing'))
    render(<MemoryCandidateCard memoryID="mem-01" title="迁移必须手写 SQL" />)

    expect(
      await screen.findByText(new RegExp('正文文件不在了')),
      '正文读不到却显示成空白——他会以为「这条没内容」直接收下',
    ).toBeInTheDocument()
  })
})
