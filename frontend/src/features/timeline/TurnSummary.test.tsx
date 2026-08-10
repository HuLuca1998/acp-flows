import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { TurnSummary } from './TurnSummary'

// M7 U7.4.1 · 本轮小结
//
// ★★ 每一行都来自**应用记的事实**。用户看这几行正是为了不用往回滚
// 就知道这轮发生了什么。

describe('本轮小结', () => {
  // R2 ★★ 没发生的那一行**不显示**。
  //
  // 塞一个「计划：无变更」进去的话，四行里有三行是废话，
  // 用户会开始整块跳过——那正好淹掉真正变了的那一行。
  it('没发生的那一行不显示，不留空行', () => {
    const { container } = render(<TurnSummary payload={{ outcome: 'done' }} />)

    expect(container.querySelector('[data-row="plan"]'), '什么都没改却给计划留了一行').toBeNull()
    expect(container.querySelector('[data-row="contract"]')).toBeNull()
    expect(container.querySelector('[data-row="inject"]')).toBeNull()
    // ★ 收场方式总是有：这一行是用户最先看的
    expect(container.querySelector('[data-row="outcome"]')).not.toBeNull()
  })

  it('计划变了就显示新版本', () => {
    render(<TurnSummary payload={{ outcome: 'done', plan_version: 2 }} />)
    expect(screen.getByText(/v2/)).toBeInTheDocument()
  })

  // ★ 把 id 列出来：只说「注入了 3 条」的话，用户没法判断
  // 是不是他刚收下的那条真的被带上了。
  it('注入那一行把 id 列出来，不只报个数', () => {
    render(
      <TurnSummary
        payload={{ outcome: 'done', memory_ids: ['mem-01'], skill_refs: ['global:go-unit-testing'] }}
      />,
    )

    expect(screen.getByText(/mem-01/)).toBeInTheDocument()
    expect(
      screen.getByText(/go-unit-testing/),
      '只报了个数——用户没法判断他刚收下的那条有没有被带上',
    ).toBeInTheDocument()
  })

  // ★★ 同样是「停了」，他自己点的停与 AI 跑挂了是完全不同的两件事。
  it('自己点的停与跑挂了说法不同', () => {
    const { rerender, container } = render(<TurnSummary payload={{ outcome: 'cancelled' }} />)
    expect(screen.getByText('你停下了它')).toBeInTheDocument()

    rerender(<TurnSummary payload={{ outcome: 'failed' }} />)
    expect(screen.getByText('跑挂了')).toBeInTheDocument()
    expect(container.querySelector('[data-outcome="failed"]')).not.toBeNull()
  })

  // ★ 排队没排上不算失败：用户只是手快点了几下，那几句话一句都没丢。
  it('没排上队要说清楚话没丢', () => {
    render(<TurnSummary payload={{ outcome: 'queue_full' }} />)
    expect(screen.getByText(/没丢/)).toBeInTheDocument()
  })
})
