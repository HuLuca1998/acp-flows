import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { ChatPage } from './index'

// M2 U2.4.1 · 对话页（验收点 V5 + V6 真正连起来）
//
// ★ 这是用户第一次能「提一个需求、看着 AI 干活」的地方。
// 所以断言集中在**他会怎么用错**：没选项目就提需求、提了之后界面没反应、
// 工作建失败了却没人告诉他。

const listProjects = vi.fn()
const listWorks = vi.fn()
const startWork = vi.fn()

vi.mock('@/api/system', () => ({
  listProjects: (...a: unknown[]): unknown => listProjects(...a),
  listWorks: (...a: unknown[]): unknown => listWorks(...a),
  startWork: (...a: unknown[]): unknown => startWork(...a),
}))

class FakeEventSource {
  static instances: FakeEventSource[] = []
  closed = false
  constructor(public url: string) {
    FakeEventSource.instances.push(this)
  }
  addEventListener() {}
  close() {
    this.closed = true
  }
}

beforeEach(() => {
  FakeEventSource.instances = []
  vi.stubGlobal('EventSource', FakeEventSource)

  listProjects.mockReset().mockResolvedValue([
    { id: 'proj-01', name: 'my-app', path: '/Users/me/work/my-app', is_git_repo: true },
  ])
  listWorks.mockReset().mockResolvedValue([])
  startWork.mockReset().mockResolvedValue({
    id: 'work-01', state: 'clarifying',
    project: '/Users/me/work/my-app', worktree: '/tmp/wt/work-01', prompt: '帮我加个功能',
  })
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('对话页', () => {
  it('一个项目都没有时，让用户先去加项目而不是干瞪眼', async () => {
    listProjects.mockResolvedValue([])
    render(<ChatPage />)

    await waitFor(() => {
      expect(screen.getByText(/先添加一个项目/)).toBeInTheDocument()
    })
    // 没项目时不该给一个点了没用的输入框
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
  })

  it('有项目时给出输入框，提交后建工作', async () => {
    const user = userEvent.setup()
    render(<ChatPage />)

    const input = await screen.findByRole('textbox')
    await user.type(input, '帮我加个功能')
    await user.click(screen.getByRole('button', { name: /开始/ }))

    await waitFor(() => {
      expect(startWork).toHaveBeenCalledWith('/Users/me/work/my-app', '帮我加个功能')
    })
  })

  // ★ 空需求不发请求。发出去的话后端会拒，而用户看到的是一句
  // 莫名其妙的错误——他明明什么都没输入。
  it('空需求不发请求', async () => {
    const user = userEvent.setup()
    render(<ChatPage />)

    await screen.findByRole('textbox')
    await user.click(screen.getByRole('button', { name: /开始/ }))

    expect(startWork).not.toHaveBeenCalled()
  })

  // ★ 建工作失败要**说出来**。
  //
  // 静默失败的话，用户点了「开始」之后界面毫无变化——
  // 他不知道是没点上、还是在转圈、还是失败了。
  it('建工作失败时显示错误', async () => {
    const user = userEvent.setup()
    startWork.mockRejectedValue({ type: 'work_project_not_a_repo' })
    render(<ChatPage />)

    const input = await screen.findByRole('textbox')
    await user.type(input, '做点事')
    await user.click(screen.getByRole('button', { name: /开始/ }))

    await waitFor(() => {
      expect(screen.getByText(/不是 git 仓库/)).toBeInTheDocument()
    })
  })

  // 建好之后才连事件流——没有工作就没有要看的事件。
  it('建好工作之后才订阅事件流', async () => {
    const user = userEvent.setup()
    render(<ChatPage />)

    await screen.findByRole('textbox')
    expect(FakeEventSource.instances, '还没有工作就连上了').toHaveLength(0)

    await user.type(screen.getByRole('textbox'), '做点事')
    await user.click(screen.getByRole('button', { name: /开始/ }))

    await waitFor(() => {
      expect(FakeEventSource.instances).toHaveLength(1)
    })
  })

  // 已有工作时直接接着看，不用重新提需求。
  it('已有工作时自动选中最近的那个', async () => {
    listWorks.mockResolvedValue([
      { id: 'work-09', state: 'executing', project: '/Users/me/work/my-app' },
    ])
    render(<ChatPage />)

    await waitFor(() => {
      expect(FakeEventSource.instances).toHaveLength(1)
    })
  })
})
