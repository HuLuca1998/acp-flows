import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { NewWorkDialog } from './NewWorkDialog'

// M4 U4.2.1 · 新建工作对话框
//
// ★★ 这一步要回答用户两个问题：**我的工作区现在什么样**、
// **Duet 会在哪儿干活**。第二个答案是硬的：他的工作区一个字节都不动。

const prepareWork = vi.fn()

vi.mock('@/api/system', () => ({
  prepareWork: (...a: unknown[]): unknown => prepareWork(...a),
}))

const realPrep = {
  current_branch: 'main',
  branches: ['develop', 'main'],
  head_commit: '7c1de98',
  tracked_dirty: 0,
  untracked: 0,
}

const noop = () => undefined

beforeEach(() => {
  prepareWork.mockReset().mockResolvedValue(realPrep)
})

afterEach(() => {
  vi.clearAllMocks()
})

function open(onConfirm = noop) {
  render(
    <NewWorkDialog open projectPath="/tmp/demo" onClose={noop} onConfirm={onConfirm} />,
  )
}

describe('新建工作对话框', () => {
  // ★★ 已跟踪与未跟踪**分开说**。
  //
  // 合成一句「有 7 处改动」的话，用户没法判断要不要先提交——
  // 「新建了几个临时文件」和「改了正在跟踪的代码」是两件事。
  it('未提交改动分已跟踪与未跟踪两个数', async () => {
    prepareWork.mockResolvedValue({ ...realPrep, tracked_dirty: 2, untracked: 5 })
    open()

    const status = await screen.findByText(/已跟踪文件改了/)
    expect(status.textContent).toContain('2')
    expect(status.textContent, '两个数被合并了——用户没法判断要不要先提交').toContain('5')
  })

  it('干净时明说无未提交改动', async () => {
    open()
    expect(await screen.findByText(/无未提交改动/)).toBeInTheDocument()
  })

  // ★ 三个基线选项：分支们 + 「指定 commit…」，并标出当前分支。
  it('列出分支并标出当前分支', async () => {
    open()

    expect(await screen.findByRole('radio', { name: 'develop' })).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: 'main' })).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: /指定 commit/ })).toBeInTheDocument()
    expect(screen.getByText(/（当前）/)).toBeInTheDocument()
    // 默认选中当前分支
    expect(screen.getByRole('radio', { name: 'main' })).toBeChecked()
  })

  // ★★ 选了哪条分支就传哪条。
  //
  // 传不下去的话，用户选了 `develop` 而工作还是从当前分支开的——
  // 而当前分支上可能正躺着他没提交完的东西。
  it('确认时把选中的基线传出去', async () => {
    const onConfirm = vi.fn()
    open(onConfirm)

    await userEvent.click(await screen.findByRole('radio', { name: 'develop' }))
    await userEvent.click(screen.getByRole('button', { name: /创建 worktree 并开始/ }))

    expect(onConfirm).toHaveBeenCalledWith('develop')
  })

  // 「指定 commit…」要能输入，且**没输入之前不给确认**。
  it('指定 commit 时没填就不给确认', async () => {
    const onConfirm = vi.fn()
    open(onConfirm)

    await userEvent.click(await screen.findByRole('radio', { name: /指定 commit/ }))
    expect(screen.getByRole('button', { name: /创建 worktree 并开始/ })).toBeDisabled()

    await userEvent.type(screen.getByRole('textbox', { name: /指定 commit/ }), '7c1de98')
    await userEvent.click(screen.getByRole('button', { name: /创建 worktree 并开始/ }))
    expect(onConfirm).toHaveBeenCalledWith('7c1de98')
  })

  // ★★ 三种探测失败要**说清是哪一种**：要做的事完全不同。
  it('rebase 中途时告诉用户先把它收尾', async () => {
    prepareWork.mockRejectedValue(new Error('work_repo_mid_operation'))
    open()

    expect(await screen.findByText(/rebase \/ merge 没做完/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /创建 worktree 并开始/ })).toBeDisabled()
  })

  it('空仓库时告诉用户先提交一次', async () => {
    prepareWork.mockRejectedValue(new Error('work_repo_no_commits'))
    open()

    expect(await screen.findByText(/先提交一次/)).toBeInTheDocument()
  })

  it('非 git 仓库时说清楚', async () => {
    prepareWork.mockRejectedValue(new Error('work_project_not_a_repo'))
    open()

    expect(await screen.findByText(/不是 git 仓库/)).toBeInTheDocument()
  })

  // ★ 认不出的错误**原样显示**，不换成一句笼统的「出错了」。
  //
  // 原文至少能让他去搜、去问；「出错了」什么都不是。
  it('认不出的错误原样显示', async () => {
    prepareWork.mockRejectedValue(new Error('某个没见过的后端错误'))
    open()

    expect(await screen.findByText(/某个没见过的后端错误/)).toBeInTheDocument()
  })

  // ★★ 设计稿原文：用户最怕的是「它会不会在我正在改的分支上乱来」。
  it('明说不动用户的工作区', async () => {
    open()
    expect(await screen.findByText(/你的工作区不受影响/)).toBeInTheDocument()
    expect(screen.getByText(/initializing_failed/)).toBeInTheDocument()
    expect(screen.getByText(/不会退回原目录执行/)).toBeInTheDocument()
  })

  // ★ 「将创建」只说形态，**不编一个具体的分支号**。
  //
  // 编出来的 `duet/work-09` 与实际拿到的号对不上，比不说更糟。
  it('将创建只说形态并给出基线 commit', async () => {
    open()

    expect(await screen.findByText(/duet\/ 开头的分支/)).toBeInTheDocument()
    expect(screen.getByText(/acpflows\/worktrees/)).toBeInTheDocument()
    expect(screen.getByText(/7c1de98/)).toBeInTheDocument()
  })

  it('关掉时不渲染，也不去探测', () => {
    render(
      <NewWorkDialog open={false} projectPath="/tmp/demo" onClose={noop} onConfirm={noop} />,
    )
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(prepareWork).not.toHaveBeenCalled()
  })
})
