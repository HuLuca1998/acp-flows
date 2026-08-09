import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { UnitAcceptance } from './UnitAcceptance'

// M8 U8.2.1 · 标准与证据对照
//
// ★★ 没证据**不等于通过**。把它当成通过的话，一个什么都没做的单元
// 也能「全部通过」——而用户是照着这张表决定「要不要点通过」的。

const getAcceptance = vi.fn()
const collectEvidence = vi.fn()
const acceptUnit = vi.fn()

vi.mock('@/api/system', () => ({
  getAcceptance: (...a: unknown[]): unknown => getAcceptance(...a),
  collectEvidence: (...a: unknown[]): unknown => collectEvidence(...a),
  acceptUnit: (...a: unknown[]): unknown => acceptUnit(...a),
}))

const DATA = {
  unit_id: 'unit-013',
  criteria: [
    { id: 'ac-1', text: '取消必须幂等', evidence_ids: ['ev-441'] },
    { id: 'ac-2', text: '现场证据可读', evidence_ids: [] },
  ],
  evidence: [
    {
      id: 'ev-441',
      unit_id: 'unit-013',
      kind: 'diff',
      source: 'app',
      summary: '3 个文件 +64 −12',
      trustworthy: true,
      criteria: ['ac-1'],
    },
  ],
}

beforeEach(() => {
  getAcceptance.mockReset().mockResolvedValue(DATA)
  collectEvidence.mockReset()
  acceptUnit.mockReset()
})

describe('验收对照', () => {
  // ★★ 有证据标 `✓ ev-441`，没证据标 `○ 无证据`——**不是通过**。
  it('没证据的标准标出来，且与有证据的分得开', async () => {
    render(<UnitAcceptance workID="work-01" unitID="unit-013" />)

    expect(await screen.findByText(/✓ ev-441/)).toBeInTheDocument()
    expect(
      screen.getByText('○ 无证据'),
      '没证据的标准没标出来——用户会以为它通过了，而那个单元什么都没做',
    ).toBeInTheDocument()

    const uncovered = document.querySelector('[data-covered="false"]')
    expect(uncovered).not.toBeNull()
  })

  // ★ 两个数**各自算**：证据多于标准时不该显示成「超额通过」。
  it('证据数与已覆盖数各自算', async () => {
    render(<UnitAcceptance workID="work-01" unitID="unit-013" />)

    // 1 条证据、已覆盖 1 条、共 2 条
    expect(await screen.findByText(/1 条 \/ 已覆盖 1 条标准（共 2 条）/)).toBeInTheDocument()
  })

  // ★★ AI 转述的证据**标出来**。
  //
  // 不标的话，一条转述会和一份真 diff 长得一样——
  // 而用户判断「该不该信」全靠这一点。
  it('AI 转述的证据标出来', async () => {
    getAcceptance.mockResolvedValue({
      ...DATA,
      evidence: [
        { ...DATA.evidence[0], id: 'ev-442', source: 'agent', trustworthy: false },
      ],
    })
    render(<UnitAcceptance workID="work-01" unitID="unit-013" />)

    expect(await screen.findByText(/AI 转述，未经核实/)).toBeInTheDocument()
  })

  // 应用采集的**不标**——大多数证据都是它，标了反而没人看。
  it('应用采集的不标「AI 转述」', async () => {
    render(<UnitAcceptance workID="work-01" unitID="unit-013" />)
    await screen.findByText(/✓ ev-441/)

    expect(screen.queryByText(/AI 转述/)).not.toBeInTheDocument()
  })

  it('点「采集」调后端，采完刷新对照表', async () => {
    const user = userEvent.setup()
    collectEvidence.mockResolvedValue({
      ...DATA,
      criteria: [
        { id: 'ac-1', text: '取消必须幂等', evidence_ids: ['ev-441'] },
        { id: 'ac-2', text: '现场证据可读', evidence_ids: ['ev-442'] },
      ],
    })
    render(<UnitAcceptance workID="work-01" unitID="unit-013" />)
    await screen.findByText(/✓ ev-441/)

    await user.click(screen.getByRole('button', { name: /采集 diff 证据/ }))

    await waitFor(() => {
      expect(screen.queryByText('○ 无证据')).not.toBeInTheDocument()
    })
    expect(collectEvidence).toHaveBeenCalledWith('work-01', 'unit-013')
  })

  // 还没有契约时**整块不显示**，不报错——那是常态。
  it('还没有契约时什么都不显示', async () => {
    getAcceptance.mockRejectedValue(new Error('contract_not_found'))
    const { container } = render(<UnitAcceptance workID="work-01" unitID="unit-013" />)

    await waitFor(() => {
      expect(getAcceptance).toHaveBeenCalled()
    })
    expect(container).toBeEmptyDOMElement()
  })
})

// ★★ M8 U8.2.2 · 「通过」由**用户点**。
describe('验收通过', () => {
  it('一条证据都没有时按钮点不了', async () => {
    getAcceptance.mockResolvedValue({
      ...DATA,
      criteria: [{ id: 'ac-1', text: '取消必须幂等', evidence_ids: [] }],
      evidence: [],
    })
    render(<UnitAcceptance workID="work-01" unitID="unit-013" />)

    const btn = await screen.findByRole('button', { name: /验收通过/ })
    expect(
      btn,
      '零证据却能点通过——那时「验收」这个动作没有内容，用户以为自己核对过什么',
    ).toBeDisabled()
  })

  it('点通过之后显示 commit，且按钮消失', async () => {
    const user = userEvent.setup()
    acceptUnit.mockResolvedValue('abc1234')
    render(<UnitAcceptance workID="work-01" unitID="unit-013" />)

    await user.click(await screen.findByRole('button', { name: /验收通过/ }))

    await waitFor(() => {
      expect(screen.getByText(/abc1234/)).toBeInTheDocument()
    })
    // ★ 点第二次会撞上「没有改动」而报错，而那个错看起来像出了问题
    expect(screen.queryByRole('button', { name: /验收通过/ })).not.toBeInTheDocument()
  })

  // ★★ 拒绝的理由要**说清**：「没有证据」与「没有改动」是两回事，
  // 而用户下一步该做什么完全不同。
  it('没有改动时说清是没东西可提交', async () => {
    const user = userEvent.setup()
    acceptUnit.mockRejectedValue(new Error('nothing_to_commit'))
    render(<UnitAcceptance workID="work-01" unitID="unit-013" />)

    await user.click(await screen.findByRole('button', { name: /验收通过/ }))

    await waitFor(() => {
      expect(screen.getByText(/没东西可提交/)).toBeInTheDocument()
    })
  })
})
