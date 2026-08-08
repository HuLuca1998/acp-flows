import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { ProjectTree } from './ProjectTree'

// U5.1.1 · 左栏项目树（M2 完成标志第 1 条）
//
// ★ 这是用户流程的**第一步**：创建项目 → 创建对话 → 观测对话。
// 2026-08-08 用户打开应用第一句话就是「为什么菜单没有显示项目列表和对话记录」——
// 那时这里是骨架占位，而 /v1/projects 明明有数据。
//
// 设计稿的形态：项目（可折叠）→ 下挂工作（标题 + 状态）→「新建对话」。

const listProjects = vi.fn()
const listWorks = vi.fn()

vi.mock('@/api/system', () => ({
  listProjects: (...a: unknown[]): unknown => listProjects(...a),
  listWorks: (...a: unknown[]): unknown => listWorks(...a),
}))

const proj = (id: string, name: string) => ({
  id,
  name,
  path: `/Users/me/work/${name}`,
  is_git_repo: true,
})

beforeEach(() => {
  listProjects.mockReset().mockResolvedValue([proj('proj-01', 'acp-engine')])
  listWorks.mockReset().mockResolvedValue([])
})

afterEach(() => {
  vi.clearAllMocks()
})

const noop = () => undefined

describe('左栏项目树', () => {
  // ★★ 有项目就**列出来**。
  //
  // 这条是用户当场发现的那个 bug：数据在后端躺着，界面显示骨架占位。
  it('列出真实的项目', async () => {
    listProjects.mockResolvedValue([proj('proj-01', 'acp-engine'), proj('proj-02', 'acp-sidecar')])
    render(<ProjectTree onNewWork={noop} onOpenWork={noop} />)

    expect(await screen.findByText('acp-engine')).toBeInTheDocument()
    expect(screen.getByText('acp-sidecar')).toBeInTheDocument()
  })

  // ★★ 项目下面挂着**它的工作**，带状态。
  //
  // 设计稿里每条工作是「标题 + `executing · 3/7`」。只列项目不列工作的话，
  // 用户看不到「我上次做到哪了」——而那正是他打开应用要找的东西。
  it('项目下面挂着它的工作与状态', async () => {
    listWorks.mockResolvedValue([
      { id: 'work-01', state: 'executing', project: '/Users/me/work/acp-engine', prompt: '补齐推送通道' },
    ])
    render(<ProjectTree onNewWork={noop} onOpenWork={noop} />)

    expect(await screen.findByText(/补齐推送通道/)).toBeInTheDocument()
    expect(screen.getByText(/executing/)).toBeInTheDocument()
  })

  // ★ 工作要挂在**自己那个项目**下面。
  //
  // 挂错的话，用户在 A 项目下看到 B 项目的工作，会以为自己点错了。
  it('工作按项目归位', async () => {
    listProjects.mockResolvedValue([proj('proj-01', 'acp-engine'), proj('proj-02', 'acp-sidecar')])
    listWorks.mockResolvedValue([
      { id: 'work-01', state: 'executing', project: '/Users/me/work/acp-engine', prompt: '引擎的活' },
      { id: 'work-02', state: 'paused', project: '/Users/me/work/acp-sidecar', prompt: '边车的活' },
    ])
    render(<ProjectTree onNewWork={noop} onOpenWork={noop} />)

    const engine = await screen.findByText('acp-engine')
    const group = engine.closest('[data-project]')
    expect(group?.textContent, '工作挂错了项目——用户会以为自己点错了').toContain('引擎的活')
    expect(group?.textContent).not.toContain('边车的活')
  })

  // ★★「新建对话」是**能点的**，点了要有反应。
  //
  // 设计稿里它在每个项目下面。之前那个「创建项目」按钮是 disabled 的死按钮——
  // 用户点了毫无反应，而他不知道是坏了还是没做。
  it('每个项目下面的「新建对话」能点', async () => {
    const user = userEvent.setup()
    const onNewWork = vi.fn()
    render(<ProjectTree onNewWork={onNewWork} onOpenWork={noop} />)

    const btn = await screen.findByRole('button', { name: /新建对话/ })
    expect(btn, '「新建对话」是死按钮——用户点了没反应，不知道是坏了还是没做').toBeEnabled()

    await user.click(btn)
    expect(onNewWork).toHaveBeenCalledWith('/Users/me/work/acp-engine')
  })

  // ★ 点一条工作要能打开它。
  it('点工作能打开', async () => {
    const user = userEvent.setup()
    const onOpenWork = vi.fn()
    listWorks.mockResolvedValue([
      { id: 'work-01', state: 'executing', project: '/Users/me/work/acp-engine', prompt: '干点什么' },
    ])
    render(<ProjectTree onNewWork={noop} onOpenWork={onOpenWork} />)

    await user.click(await screen.findByText(/干点什么/))
    expect(onOpenWork).toHaveBeenCalledWith('work-01')
  })

  // ★ 项目能折叠——设计稿里每个项目名前面有一个 ▾。
  //
  // 开着五六个项目时不折叠的话，左栏会长到看不见底。
  it('项目能折叠起来', async () => {
    const user = userEvent.setup()
    listWorks.mockResolvedValue([
      { id: 'work-01', state: 'executing', project: '/Users/me/work/acp-engine', prompt: '干点什么' },
    ])
    render(<ProjectTree onNewWork={noop} onOpenWork={noop} />)

    expect(await screen.findByText(/干点什么/)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: /acp-engine/ }))

    await waitFor(() => {
      expect(screen.queryByText(/干点什么/), '折叠之后工作还在').not.toBeInTheDocument()
    })
  })

  // 一个项目都没有时，引导他去加——而不是显示一片空白。
  it('没有项目时给出引导', async () => {
    listProjects.mockResolvedValue([])
    render(<ProjectTree onNewWork={noop} onOpenWork={noop} />)

    expect(await screen.findByText(/还没有项目|添加一个/)).toBeInTheDocument()
  })

  // ★★ 查询失败时**说出来**，不装作「没有项目」。
  //
  // 装作没有的话，用户以为自己的项目丢了——而实际是后端没起来。
  it('查询失败时说出来', async () => {
    listProjects.mockRejectedValue(new Error('后端没起来'))
    render(<ProjectTree onNewWork={noop} onOpenWork={noop} />)

    expect(
      await screen.findByText(/没能加载|加载失败/),
      '查不到却显示「还没有项目」——用户以为自己的项目丢了',
    ).toBeInTheDocument()
  })
})
