import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import { Button } from './Button'

/**
 * 设计规范 §05「按钮」的行为与可访问性契约。
 *
 * ★ 只断言用户能观察到的东西与可访问性契约，**不断言 CSS 类名与 DOM 结构**——
 * 设计会改，绑死结构的测试只会变成噪音。见 docs/testing-strategy.md §6。
 *
 * 「主按钮永不填充实心」这类纯视觉规则由 stylelint + 人工走查保证，不在这里测。
 */
describe('Button', () => {
  it('渲染成可访问的 button，名称取自内容', () => {
    render(<Button>创建 worktree 并开始</Button>)
    expect(screen.getByRole('button', { name: '创建 worktree 并开始' })).toBeVisible()
  })

  it('点击时调用 onClick', async () => {
    const onClick = vi.fn()
    render(<Button onClick={onClick}>发送</Button>)

    await userEvent.click(screen.getByRole('button', { name: '发送' }))

    expect(onClick).toHaveBeenCalledOnce()
  })

  it('disabled 时不可点击也不触发回调', async () => {
    const onClick = vi.fn()
    render(
      <Button disabled onClick={onClick}>
        发送
      </Button>,
    )

    const btn = screen.getByRole('button', { name: '发送' })
    expect(btn).toBeDisabled()

    await userEvent.click(btn)
    expect(onClick).not.toHaveBeenCalled()
  })

  // ★ 设计规范 §08：所有纯图标控件必须有中文 tooltip。
  // 这不是样式问题，是可访问性契约——没有它，屏幕阅读器与键盘用户拿不到任何信息。
  it('纯图标按钮必须同时有 title 与 data-tt', () => {
    render(<Button icon="ph-sidebar" label="折叠侧栏" />)

    const btn = screen.getByRole('button', { name: '折叠侧栏' })
    expect(btn).toHaveAttribute('title', '折叠侧栏')
    expect(btn).toHaveAttribute('data-tt', '折叠侧栏')
  })

  it('带快捷键时 tooltip 里要带上快捷键', () => {
    render(<Button icon="ph-sidebar" label="折叠侧栏" shortcut="⌘B" />)

    const btn = screen.getByRole('button', { name: /折叠侧栏/ })
    expect(btn).toHaveAttribute('data-tt', '折叠侧栏 ⌘B')
  })

  // 带文字的按钮不加 tooltip（设计规范 §08），否则鼠标一停就冒出重复的浮层
  it('带文字的按钮默认不加 tooltip', () => {
    render(<Button>发送</Button>)

    const btn = screen.getByRole('button', { name: '发送' })
    expect(btn).not.toHaveAttribute('data-tt')
  })

  it('type 默认是 button，不会误触发表单提交', () => {
    render(<Button>发送</Button>)
    expect(screen.getByRole('button', { name: '发送' })).toHaveAttribute('type', 'button')
  })
})
