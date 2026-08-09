import { render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { PlanPanel } from './PlanPanel'

// M6 U6.3.1 · 子计划 DAG 与单元列表
//
// ★★ **每个单元都标着由哪个角色做**（裁定三）。不标的话用户看不出
// 「这条谁在干」——而那正是他判断「该不该信这个产出」的依据：
// 实现方审查自己的产出是 INV-ATT-8 明令禁止的。

const getPlan = vi.fn()

vi.mock('@/api/system', () => ({
  getPlan: (...a: unknown[]): unknown => getPlan(...a),
}))

const PLAN = {
  version: 5,
  title: '取消运行中的 Agent turn',
  subplan_count: 1,
  unit_count: 2,
  subplans: [
    {
      id: 'subplan-01',
      title: 'ACP Runtime 抽象层',
      status: 'in_progress',
      done: 1,
      total: 2,
      units: [
        {
          id: 'unit-012',
          title: '取消协议',
          role_id: 'implementer',
          role_display_name: '实现工程师',
          depends_on: [],
          contract_frozen: true,
          accepted: true,
        },
        {
          id: 'unit-013',
          title: '取消后证据读取',
          role_id: 'unit_reviewer',
          role_display_name: '单元审查员',
          depends_on: ['unit-012'],
          contract_frozen: false,
          accepted: false,
        },
      ],
    },
  ],
}

beforeEach(() => {
  getPlan.mockReset().mockResolvedValue(PLAN)
})

describe('计划面板', () => {
  it('显示版本号与「N 子计划 · M 单元」', async () => {
    render(<PlanPanel workID="work-01" />)

    expect(await screen.findByText('plan v5')).toBeInTheDocument()
    expect(screen.getByText(/1 子计划 · 2 单元/)).toBeInTheDocument()
  })

  // ★ 设计稿的 `accepted · 3/3`。状态词**原值不翻译**（术语表）。
  it('子计划带状态与进度', async () => {
    render(<PlanPanel workID="work-01" />)

    expect(await screen.findByText('subplan-01')).toBeInTheDocument()
    expect(screen.getByText('ACP Runtime 抽象层')).toBeInTheDocument()
    expect(screen.getByText('in_progress')).toBeInTheDocument()
    expect(screen.getByText('1/2')).toBeInTheDocument()
  })

  // ★★ R2 · 每个单元带**角色**、依赖、契约冻结态。
  it('单元带角色、依赖与契约状态', async () => {
    render(<PlanPanel workID="work-01" />)

    expect(await screen.findByText('unit-012')).toBeInTheDocument()
    expect(
      screen.getByText('实现工程师'),
      '单元没标由谁做——用户看不出这条谁在干，而那正是他判断该不该信这个产出的依据',
    ).toBeInTheDocument()
    expect(screen.getByText('单元审查员')).toBeInTheDocument()
    expect(screen.getByText(/依赖 unit-012/)).toBeInTheDocument()
    expect(screen.getByText('契约未冻结')).toBeInTheDocument()
  })

  // ★ 契约**冻结了的**不显示那句提示——「契约未冻结」是个待办，
  // 挂在已经冻结的单元上会让用户以为它也没准备好。
  it('契约冻结的单元不显示「契约未冻结」', async () => {
    render(<PlanPanel workID="work-01" />)
    await screen.findByText('unit-012')

    expect(screen.getAllByText('契约未冻结')).toHaveLength(1)
  })

  // ★★ 认不出的角色**显示后端给的原始 id**，不编一个名字。
  //
  // 编出来的名字与角色页那张表对不上，用户会以为有两个不同的角色。
  it('认不出的角色显示原始 id', async () => {
    getPlan.mockResolvedValue({
      ...PLAN,
      subplans: [
        {
          ...PLAN.subplans[0],
          units: [
            {
              id: 'unit-099',
              title: '将来的活',
              role_id: 'a_future_role',
              depends_on: [],
              contract_frozen: false,
              accepted: false,
            },
          ],
        },
      ],
    })
    render(<PlanPanel workID="work-01" />)

    expect(await screen.findByText('a_future_role')).toBeInTheDocument()
    // 内容照常显示，不吞
    expect(screen.getByText('将来的活')).toBeInTheDocument()
  })

  // ★★ 还没规划时**整块不显示**，且不报错——新工作的常态。
  it('还没规划时安静地什么都不显示', async () => {
    getPlan.mockRejectedValue(new Error('plan_not_found'))
    const { container } = render(<PlanPanel workID="work-01" />)

    await waitFor(() => {
      expect(getPlan).toHaveBeenCalled()
    })
    expect(container).toBeEmptyDOMElement()
    expect(screen.queryByText(/失败/)).not.toBeInTheDocument()
  })

  // 没有工作时不发请求——查一个空 id 只会拿到 404。
  it('没有工作时不发请求', () => {
    render(<PlanPanel workID="" />)
    expect(getPlan).not.toHaveBeenCalled()
  })
})
