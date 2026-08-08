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

/**
 * 假的**网络**，不是假的 EventSource。
 *
 * ★ 这里只关心「有没有去连事件流」，所以流本身永远不吐数据。
 * 但替身必须换在 fetch 这一层——真实现用的是 fetch + ReadableStream
 * （`EventSource` 带不了 Authorization 头，真机上一路 401）。
 * 假在更高层的话，测的就不是真跑的那条路。
 */
class FakeEventStream {
  static instances: FakeEventStream[] = []
  aborted = false

  constructor(
    readonly url: string,
    init?: RequestInit,
  ) {
    FakeEventStream.instances.push(this)
    init?.signal?.addEventListener('abort', () => {
      this.aborted = true
    })
  }

  static response(url: string, init?: RequestInit): Promise<Response> {
    new FakeEventStream(url, init)
    return Promise.resolve({
      ok: true,
      status: 200,
      // 永不结束、永不吐数据的流：本页只关心有没有去连
      body: new ReadableStream<Uint8Array>({ start: () => undefined }),
    } as Response)
  }
}

beforeEach(() => {
  FakeEventStream.instances = []
  vi.stubGlobal('fetch', (url: string, init?: RequestInit) =>
    FakeEventStream.response(url, init),
  )

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
    render(<ChatPage intent={null} intentSeq={0} />)

    await waitFor(() => {
      expect(screen.getByText(/先添加一个项目/)).toBeInTheDocument()
    })
    // 没项目时不该给一个点了没用的输入框
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
  })

  it('有项目时给出输入框，提交后建工作', async () => {
    const user = userEvent.setup()
    render(<ChatPage intent={null} intentSeq={0} />)

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
    render(<ChatPage intent={null} intentSeq={0} />)

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
    render(<ChatPage intent={null} intentSeq={0} />)

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
    render(<ChatPage intent={null} intentSeq={0} />)

    await screen.findByRole('textbox')
    expect(FakeEventStream.instances, '还没有工作就连上了').toHaveLength(0)

    await user.type(screen.getByRole('textbox'), '做点事')
    await user.click(screen.getByRole('button', { name: /开始/ }))

    await waitFor(() => {
      expect(FakeEventStream.instances).toHaveLength(1)
    })
  })

  // 已有工作时直接接着看，不用重新提需求。
  it('已有工作时自动选中最近的那个', async () => {
    listWorks.mockResolvedValue([
      { id: 'work-09', state: 'executing', project: '/Users/me/work/my-app' },
    ])
    render(<ChatPage intent={null} intentSeq={0} />)

    await waitFor(() => {
      expect(FakeEventStream.instances).toHaveLength(1)
    })
  })
})

// ── 左栏点「新建对话」/「打开工作」时的响应（M2 完成标志第 1 条）──

describe('响应左栏的动作', () => {
  // ★★ 点某个项目下的「新建对话」→ **在那个项目下**开，不是在第一个项目下。
  //
  // 用固定的 projects[0] 的话，用户在 B 项目下点「新建对话」，
  // 工作却建到了 A 项目里——而他要到 AI 开始读错文件时才发现。
  it('在点的那个项目下建工作', async () => {
    const user = userEvent.setup()
    listProjects.mockResolvedValue([
      { id: 'proj-01', name: 'a', path: '/work/a', is_git_repo: true },
      { id: 'proj-02', name: 'b', path: '/work/b', is_git_repo: true },
    ])
    render(<ChatPage intent={{ kind: 'new', projectPath: '/work/b' }} intentSeq={1} />)

    await user.type(await screen.findByRole('textbox'), '做点事')
    await user.click(screen.getByRole('button', { name: /开始/ }))

    await waitFor(() => {
      expect(startWork).toHaveBeenCalledWith('/work/b', '做点事')
    })
  })

  // ★ 点「打开工作」→ 直接看那条工作的时间线，不用重新提需求。
  it('打开已有工作时直接连它的事件流', async () => {
    listWorks.mockResolvedValue([
      { id: 'work-07', state: 'executing', project: '/work/a' },
      { id: 'work-09', state: 'paused', project: '/work/a' },
    ])
    render(<ChatPage intent={{ kind: 'open', workID: 'work-09' }} intentSeq={1} />)

    await waitFor(() => {
      expect(FakeEventStream.instances).toHaveLength(1)
    })
  })

  // ★★ 同一个项目连点两次「新建对话」要有两次反应。
  //
  // 只看 intent 内容的话，第二次点 intent 没变，界面毫无动静——
  // 而用户明明点了两下。序号就是为这个存在的。
  it('连点两次「新建对话」都有反应', async () => {
    const { rerender } = render(
      <ChatPage intent={{ kind: 'new', projectPath: '/work/a' }} intentSeq={1} />,
    )
    await screen.findByRole('textbox')

    rerender(<ChatPage intent={{ kind: 'new', projectPath: '/work/a' }} intentSeq={2} />)

    // 第二次点之后输入框仍在、且是空的（新的一轮）
    expect((await screen.findByRole('textbox')).getAttribute('value') ?? '').toBe('')
  })
})
