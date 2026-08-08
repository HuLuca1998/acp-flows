import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'

import { PermissionCard } from './PermissionCard'
import type { PermissionRequest } from './model'

// M3 U3.1.3 · 权限卡片（验收点 V8）
//
// ★ 断言集中在「用户看得懂 AI 想干什么」与「他点的那个真的生效」。
// 这是整个产品里后果最严重的一处交互：点错一次，AI 就改了不该改的文件。

const ask: PermissionRequest = {
  id: 'ask-1',
  toolCallId: 'tool-1',
  runtime: 'codex',
  kind: 'edit',
  path: 'crates/engine/src/events.rs',
  outOfBounds: true,
  options: [
    { optionId: 'opt-allow', name: '允许一次', kind: 'allow_once' },
    { optionId: 'opt-deny', name: '拒绝', kind: 'reject_once' },
  ],
}

// ★★ R1：文案与设计稿一致。
//
// 设计稿里这张卡片说的是「Codex 请求写入 <路径> · 写入边界外」——
// 「请求写入」而不是「请求权限」：用户要一眼看出**它想干什么**，
// 而不是被问一个抽象问题。
it('R1 · 说清楚谁要干什么、动哪个文件', () => {
  render(<PermissionCard ask={ask} onDecide={vi.fn()} />)

  expect(screen.getByText(/请求写入/)).toBeInTheDocument()
  expect(screen.getByText('crates/engine/src/events.rs')).toBeInTheDocument()
  expect(screen.getByText(/写入边界外/)).toBeInTheDocument()
  // Agent 名要出现——用户可能同时开着 claude 与 codex
  expect(screen.getByText(/codex/i)).toBeInTheDocument()
})

// ★★ 按钮照 **Agent 给的 options** 渲染，不自己造一套。
//
// 自己造「允许 / 拒绝」两个的话，Agent 提供的第三种选项（比如
// 「这个目录以后都允许」）就消失了，而用户根本不知道自己少了一个选择。
it('按钮用 Agent 给的选项，一个不漏', () => {
  const three: PermissionRequest = {
    ...ask,
    options: [
      { optionId: 'a1', name: '允许一次', kind: 'allow_once' },
      { optionId: 'a2', name: '这个目录以后都允许', kind: 'allow_always' },
      { optionId: 'r1', name: '拒绝', kind: 'reject_once' },
    ],
  }
  render(<PermissionCard ask={three} onDecide={vi.fn()} />)

  expect(screen.getByRole('button', { name: '允许一次' })).toBeInTheDocument()
  expect(
    screen.getByRole('button', { name: '这个目录以后都允许' }),
    'Agent 给的第三种选项消失了——用户不知道自己少了一个选择',
  ).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '拒绝' })).toBeInTheDocument()
})

// ★★ R4：点「拒绝」发出去的是**拒绝那个选项的 id**。
//
// 按类别重新匹配或者搞反的话，用户点「拒绝」而 Agent 收到「允许」——
// 这是整个权限体系里后果最严重的一种错。
it('R4 · 点拒绝就发拒绝那个 id，绝不发允许类', async () => {
  const user = userEvent.setup()
  const onDecide = vi.fn()
  render(<PermissionCard ask={ask} onDecide={onDecide} />)

  await user.click(screen.getByRole('button', { name: '拒绝' }))

  expect(onDecide).toHaveBeenCalledWith('opt-deny')
  expect(onDecide).not.toHaveBeenCalledWith('opt-allow')
})

it('点允许就发允许那个 id', async () => {
  const user = userEvent.setup()
  const onDecide = vi.fn()
  render(<PermissionCard ask={ask} onDecide={onDecide} />)

  await user.click(screen.getByRole('button', { name: '允许一次' }))

  expect(onDecide).toHaveBeenCalledWith('opt-allow')
})

// ★ 提交中要**禁用全部按钮**。
//
// 不禁用的话，用户手快点两下就发出两条应答——第二条会被 Agent 当成
// 一个不认识的请求，而界面上什么提示都没有。
it('提交中禁用全部按钮', () => {
  render(<PermissionCard ask={ask} onDecide={vi.fn()} pending />)

  for (const name of ['允许一次', '拒绝']) {
    expect(screen.getByRole('button', { name }), `${name} 在提交中仍可点`).toBeDisabled()
  }
})

// 没有越界信息时不显示那一行——**不编造**。
//
// 「写入边界外」是一句很重的话，没有依据就说的话，用户会对所有提示脱敏。
it('不越界时不显示「写入边界外」', () => {
  render(<PermissionCard ask={{ ...ask, outOfBounds: false }} onDecide={vi.fn()} />)

  expect(screen.queryByText(/写入边界外/)).not.toBeInTheDocument()
})

// 不同的工具类别说不同的话：读文件说「请求读取」，执行命令说「请求执行」。
//
// 全都说「请求权限」的话，用户没法一眼分出「它要看一下」和「它要跑一条命令」。
it('按工具类别说人话', () => {
  const cases: Array<[PermissionRequest['kind'], RegExp]> = [
    ['read', /请求读取/],
    ['edit', /请求写入/],
    ['delete', /请求删除/],
    ['execute', /请求执行/],
  ]

  for (const [kind, want] of cases) {
    const { unmount } = render(<PermissionCard ask={{ ...ask, kind }} onDecide={vi.fn()} />)
    expect(screen.getByText(want), `${kind} 没有对应的说法`).toBeInTheDocument()
    unmount()
  }
})

// 认不出的类别用兜底文案，**不把原始码显示给用户**。
it('认不出的类别有兜底文案', () => {
  render(
    <PermissionCard
      ask={{ ...ask, kind: 'teleport' as PermissionRequest['kind'] }}
      onDecide={vi.fn()}
    />,
  )

  expect(screen.queryByText(/teleport/)).not.toBeInTheDocument()
  expect(screen.getByText(/请求执行操作/)).toBeInTheDocument()
})

// 没有路径时不显示一个空的等宽块——Agent 不一定给得出路径（比如执行命令）。
it('没有路径时不留空块', () => {
  // 造一个**没有 path 这个键**的载荷，而不是把它赋成 undefined：
  // tsconfig 开着 exactOptionalPropertyTypes，两者是两回事，
  // 而 Agent 真给过来的就是前者。
  const noPath: PermissionRequest = {
    id: ask.id,
    toolCallId: ask.toolCallId,
    runtime: ask.runtime,
    kind: ask.kind,
    options: ask.options,
  }
  render(<PermissionCard ask={noPath} onDecide={vi.fn()} />)

  expect(screen.getByText(/请求写入/)).toBeInTheDocument()
  expect(document.querySelector('[data-path]')).toBeNull()
})

// ★ 一个选项都没给时，卡片要说清楚**用户没法处理**，而不是显示一张空卡片。
//
// 空卡片会让他一直等，而这一轮永远不会继续。
it('没有选项时说清楚，而不是显示空卡片', () => {
  render(<PermissionCard ask={{ ...ask, options: [] }} onDecide={vi.fn()} />)

  expect(screen.getByText(/没有可选项|无法处理/)).toBeInTheDocument()
})
