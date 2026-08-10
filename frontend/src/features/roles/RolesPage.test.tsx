import { render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { RolesPage } from './index'

// M2 U2.4.1 · 角色页
//
// ★ 这一页在 M2 之前一直是骨架占位。而八个预置角色是**内置的**——
// 用户打开它看到空白，只会以为应用坏了。

const listRoles = vi.fn()

vi.mock('@/api/system', () => ({
  listRoles: (...a: unknown[]): unknown => listRoles(...a),
}))

// 真实形状的响应（照 /v1/roles 实际返回的那份）
const realRoles = [
  {
    id: 'requirement_analyst',
    display_name: '需求分析师',
    operations: ['clarify', 'snapshot'],
    personality: '追问式，逐条确认，不放过「大概/应该」',
    boundary: '不写代码、不定技术方案；无法判定等级时上调 D2',
    session_mode: 'read_only',
    mode_name: 'plan',
    permission_policy: 'ask_each',
    runtime_name: 'claude',
    is_preset: true,
  },
  {
    id: 'implementer',
    display_name: '实现工程师',
    operations: ['implement'],
    personality: '沉默执行、报告详尽，触发停止条件立即停',
    boundary: '不改目标、外部行为、测试标准与写入边界；D1 以上只报告',
    session_mode: 'guarded_write',
    mode_name: 'agent',
    permission_policy: 'ask_each',
    runtime_name: 'codex',
    is_preset: true,
  },
]

beforeEach(() => {
  listRoles.mockReset().mockResolvedValue(realRoles)
})

afterEach(() => {
  vi.clearAllMocks()
})

describe('角色页', () => {
  it('列出角色、承担的操作与绑定的 Runtime', async () => {
    render(<RolesPage />)

    await waitFor(() => {
      expect(screen.getByText('需求分析师')).toBeInTheDocument()
    })
    expect(screen.getByText('实现工程师')).toBeInTheDocument()
    // 承担的操作照设计稿是 `clarify · snapshot` 这个形态
    expect(screen.getByText('clarify · snapshot')).toBeInTheDocument()
    expect(screen.getByText('codex')).toBeInTheDocument()
  })

  // ★★ 只读与能写要**一眼分得出**：用户点开这一页就是想知道谁能动他的文件。
  //
  // 判据是**语义档**（data-mode），不是档名——按档名判断的话，
  // 加一端就要加一串条件，而漏一个的表现是「界面说它只读，实际它能写」。
  it('只读与能写在界面上分得出来', async () => {
    render(<RolesPage />)

    const analyst = await waitFor(() => {
      const el = screen.getByText('需求分析师').closest('[data-role]')
      expect(el).not.toBeNull()
      return el as HTMLElement
    })
    expect(within(analyst).getByText(/plan/)).toHaveAttribute('data-mode', 'read_only')

    const impl = screen.getByText('实现工程师').closest('[data-role]') as HTMLElement
    expect(
      within(impl).getByText(/agent/),
      '能写的角色和只读的角色在界面上长得一样——用户不知道谁能动他的文件',
    ).toHaveAttribute('data-mode', 'guarded_write')
  })

  // ★ 显示的是后端翻译好的档名，不是前端自己翻的。
  //
  // 前端翻译就得认识 `plan` / `read-only` 这些品牌相关的取值，
  // 而那正是分层要挡住的东西。
  it('档名来自后端，不在前端翻译', async () => {
    listRoles.mockResolvedValue([{ ...realRoles[0], mode_name: '某个后端说了算的档名' }])
    render(<RolesPage />)

    await waitFor(() => {
      expect(screen.getByText(/某个后端说了算的档名/)).toBeInTheDocument()
    })
  })

  // ★★ 查不到要**说出来**，不装作「一个角色都没有」。
  //
  // 装作没有的话，用户会去找「怎么添加角色」——而预置角色本来就该在那儿，
  // 问题在别处（后端没起来 / 没装配）。
  it('查询失败时说清楚，不显示空表', async () => {
    listRoles.mockRejectedValue(new Error('后端没起来'))
    render(<RolesPage />)

    await waitFor(() => {
      expect(screen.getByText(/读不到角色表/)).toBeInTheDocument()
    })
    expect(screen.queryByRole('table')).not.toBeInTheDocument()
  })

  // ★ 绑定坏掉的角色照样列出来，带着原因。
  it('绑定坏掉的角色列出来并说明原因', async () => {
    listRoles.mockResolvedValue([
      { ...realRoles[0], runtime_name: '', mode_name: '', problem: '绑定表读不出来' },
    ])
    render(<RolesPage />)

    await waitFor(() => {
      expect(screen.getByText('需求分析师')).toBeInTheDocument()
    })
    expect(screen.getByText(/绑定表读不出来/)).toBeInTheDocument()
  })

  // ★★ 设计稿底部那两块说明是**原文**，不是可有可无的装饰。
  //
  // 它们回答了用户打开这一页最可能问的两个问题：
  // 这些开关到底在控制什么、为什么没有「选模型」。
  it('照设计稿给出「对应 ACP 里的什么」与「为什么不设模型」', async () => {
    render(<RolesPage />)

    await waitFor(() => {
      expect(screen.getByText(/这些设置对应 ACP 里的什么/)).toBeInTheDocument()
    })
    expect(screen.getByText(/ACP 不提供、因此这里不设/)).toBeInTheDocument()
    expect(screen.getByText(/模型与推理强度不在协议里/)).toBeInTheDocument()
  })

  // ★ 说明里写的是**现在真的在用的方法**。
  //
  // 设计稿原文写的是 `session/set_mode`，而它官方已废弃、我们走的是
  // `set_config_option`。照抄的话，用户按这句话去查日志会找不到那条帧。
  it('说明里写的是 set_config_option 而不是已废弃的 set_mode', async () => {
    render(<RolesPage />)

    const note = await waitFor(() => screen.getByText(/session\/set_config_option/))
    expect(note).toBeInTheDocument()
  })

  // ★ 编辑提示词要到 M11。做成 disabled 而不是拿掉：
  // 拿掉的话用户不知道以后能改，做成能点的又会点了没反应。
  it('提示词按钮是禁用的且说明了原因', async () => {
    render(<RolesPage />)

    const buttons = await waitFor(() => screen.getAllByRole('button', { name: /提示词/ }))
    expect(buttons.length).toBe(realRoles.length)
    for (const b of buttons) {
      expect(b).toBeDisabled()
    }
  })
})
