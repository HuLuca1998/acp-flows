import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { CreateProjectDialog } from './CreateProjectDialog'

// M3 U3.2.1 · 创建项目对话框
//
// ★★ 这一族守的是**先说再做**。用户交出来的是他自己的代码仓库——
// 在他点「创建项目」之前，Duet 将要动的每一样东西都要列出来，**包括为什么**。

const previewProject = vi.fn()
const addProject = vi.fn()
const pickDirectory = vi.fn()
const capabilities = vi.fn()

vi.mock('@/api/system', () => ({
  previewProject: (...a: unknown[]): unknown => previewProject(...a),
  addProject: (...a: unknown[]): unknown => addProject(...a),
}))

vi.mock('@/platform', () => ({
  pickDirectory: (...a: unknown[]): unknown => pickDirectory(...a),
  capabilities: (): unknown => capabilities(),
}))

// 真实形状的预演（照 /v1/projects/preview 实际返回的那份）
const realPreview = {
  path: '/tmp/demo',
  name: 'demo',
  is_git_repo: true,
  actions: [
    {
      kind: 'create_dir',
      path: '/tmp/demo/.acpflows',
      reason: 'Duet 在这个项目里唯一会碰的目录',
      already_there: false,
    },
    {
      kind: 'append_lines',
      path: '/tmp/demo/.gitignore',
      reason: '执行过程记录不该进版本库',
      already_there: false,
      lines: ['.acpflows/runs/'],
    },
  ],
  skills: [
    {
      name: 'broken-one',
      dir: 'broken-one',
      source: '.claude/skills',
      scope: 'project',
      status: 'draft',
      validation_ok: false,
      validation_reason: '校验未通过：frontmatter 缺 description',
    },
  ],
  remote: { url: 'git@github.com:acme/demo.git', host: 'github.com', slug: 'acme/demo', is_github: true },
  gh: { status: 'ready', version: '2.62.0', account: 'someone' },
}

const noop = () => undefined

beforeEach(() => {
  previewProject.mockReset().mockResolvedValue(realPreview)
  addProject.mockReset().mockResolvedValue({ id: 'proj-01', name: 'demo' })
  pickDirectory.mockReset().mockResolvedValue('/tmp/demo')
  // 默认按**壳形态**（能选文件夹）测
  capabilities.mockReset().mockReturnValue({ canPickDirectory: true, canSelfUpdate: true })
})

afterEach(() => {
  vi.clearAllMocks()
})

async function openAndPick() {
  render(<CreateProjectDialog open onClose={noop} onCreated={noop} />)
  await userEvent.click(screen.getByRole('button', { name: /选择文件夹/ }))
  await waitFor(() => {
    expect(previewProject).toHaveBeenCalledWith('/tmp/demo')
  })
}

describe('创建项目对话框', () => {
  // ★★ 列出将做的每一步，**每一步都带为什么**。
  //
  // 不说为什么的话，用户看到的是一串路径——他凭什么点确认？
  it('列出将创建与将追加，每一步都说得出为什么', async () => {
    await openAndPick()

    expect(await screen.findByText(/\.acpflows$/)).toBeInTheDocument()
    expect(screen.getByText('Duet 在这个项目里唯一会碰的目录')).toBeInTheDocument()
    // ★ 「将追加」单独一块：它动的是**用户自己的文件**
    expect(screen.getByText(/将追加/)).toBeInTheDocument()
    expect(screen.getByText('.acpflows/runs/')).toBeInTheDocument()
  })

  // ★★ 打开对话框本身**不创建任何东西**。
  //
  // 判据是 addProject 一次都没被调用——先看后做是这一步的全部意义。
  it('只看预演时不创建任何东西', async () => {
    await openAndPick()
    expect(addProject).not.toHaveBeenCalled()
  })

  // ★★ 点了确认才创建，且**带 initialize=true**。
  //
  // 不带的话，用户看了一份「将创建 .acpflows/」的清单，点完确认
  // 磁盘上什么都没有——那比不显示清单更糟。
  it('确认后照预演的计划初始化', async () => {
    const onCreated = vi.fn()
    render(<CreateProjectDialog open onClose={noop} onCreated={onCreated} />)
    await userEvent.click(screen.getByRole('button', { name: /选择文件夹/ }))
    await waitFor(() => expect(previewProject).toHaveBeenCalled())

    await userEvent.click(screen.getByRole('button', { name: '创建项目' }))

    await waitFor(() => expect(addProject).toHaveBeenCalled())
    const call = addProject.mock.calls[0] as unknown[]
    expect(call[0]).toBe('/tmp/demo')
    expect(call[1], '没带 initialize——用户看了清单，点完确认磁盘上却什么都没有').toBe(true)
    expect(onCreated).toHaveBeenCalled()
  })

  // ★★ 没有预演就**不给创建**。
  //
  // 让他对着一个空对话框点「创建」等于回到了静默写。
  it('没看过预演时创建按钮是禁用的', () => {
    render(<CreateProjectDialog open onClose={noop} onCreated={noop} />)
    expect(screen.getByRole('button', { name: '创建项目' })).toBeDisabled()
  })

  it('预演失败时说清楚且仍不给创建', async () => {
    previewProject.mockRejectedValue(new Error('这个目录不存在'))
    render(<CreateProjectDialog open onClose={noop} onCreated={noop} />)
    await userEvent.click(screen.getByRole('button', { name: /选择文件夹/ }))

    await waitFor(() => {
      expect(screen.getByText(/这个目录不存在/)).toBeInTheDocument()
    })
    expect(screen.getByRole('button', { name: '创建项目' })).toBeDisabled()
  })

  // ★★ 换一个目录预演失败时，**上一个的清单要消失**。
  //
  // 留着的话：用户选了 A 看到清单，又选了 B 但预演失败，
  // 他点确认——而 `create` 用的是 preview.path，于是**对 A 执行了**，
  // 而他以为自己在建 B。
  it('换目录预演失败时不留着上一次的清单', async () => {
    capabilities.mockReturnValue({ canPickDirectory: false, canSelfUpdate: false })
    render(<CreateProjectDialog open onClose={noop} onCreated={noop} />)

    const input = screen.getByRole('textbox', { name: /仓库目录/ })
    await userEvent.type(input, '/tmp/demo')
    await userEvent.click(screen.getByRole('button', { name: /看看这个目录/ }))
    await waitFor(() => {
      expect(screen.getByText('Duet 在这个项目里唯一会碰的目录')).toBeInTheDocument()
    })

    previewProject.mockRejectedValue(new Error('第二个目录不存在'))
    await userEvent.clear(input)
    await userEvent.type(input, '/tmp/nope')
    await userEvent.click(screen.getByRole('button', { name: /看看这个目录/ }))

    await waitFor(() => {
      expect(screen.getByText(/第二个目录不存在/)).toBeInTheDocument()
    })
    expect(
      screen.queryByText('Duet 在这个项目里唯一会碰的目录'),
      '上一个目录的清单还留着——用户点确认会对上一个目录执行，而他以为在建这一个',
    ).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '创建项目' })).toBeDisabled()
  })

  // ★ 扫到的 skill 要列出来，校验没过的**说清缺什么**。
  it('列出已有 Skill 并标出校验没过的原因', async () => {
    await openAndPick()

    expect(await screen.findByText(/发现已有 Skill 目录/)).toBeInTheDocument()
    expect(screen.getByText('.claude/skills')).toBeInTheDocument()
    expect(screen.getByText(/frontmatter 缺 description/)).toBeInTheDocument()
  })

  it('显示 remote 与 gh 状态', async () => {
    await openAndPick()

    expect(await screen.findByText('git@github.com:acme/demo.git')).toBeInTheDocument()
    expect(screen.getByText('acme/demo')).toBeInTheDocument()
    expect(screen.getByText(/gh 已登录/)).toBeInTheDocument()
  })

  // ★ gh 没装时把**后端给的命令**原样显示，前端不拼。
  it('gh 没装时显示后端给的修复命令', async () => {
    previewProject.mockResolvedValue({
      ...realPreview,
      gh: { status: 'not_installed', remedy: 'brew install gh' },
    })
    await openAndPick()

    expect(await screen.findByText(/没装 gh/)).toBeInTheDocument()
    expect(screen.getByText('brew install gh')).toBeInTheDocument()
  })

  // ★★ 不是 git 仓库**如实说**，且不擅自 git init。
  it('非 git 仓库如实告知', async () => {
    previewProject.mockResolvedValue({ ...realPreview, is_git_repo: false, remote: undefined })
    await openAndPick()

    expect(await screen.findByText(/不是 git 仓库/)).toBeInTheDocument()
    expect(screen.getByText(/不会替你 git init/)).toBeInTheDocument()
  })

  // ★ 设计稿原文的提示：用户最怕的是「加进去它就自己开始干了」。
  it('明说创建项目不会开始任何工作', () => {
    render(<CreateProjectDialog open onClose={noop} onCreated={noop} />)
    expect(screen.getByText(/创建项目不会开始任何工作/)).toBeInTheDocument()
  })

  // ★★ 浏览器形态下拿不到真实路径，要降级成手动粘贴。
  //
  // 装作能选、然后拿一个假路径去请求的话，用户会在后端拿到「路径不存在」，
  // 而他明明刚从对话框里选过——那种错误没人能自己解决。
  it('浏览器形态下降级成手动粘贴路径', async () => {
    capabilities.mockReturnValue({ canPickDirectory: false, canSelfUpdate: false })
    render(<CreateProjectDialog open onClose={noop} onCreated={noop} />)

    expect(screen.queryByRole('button', { name: /选择文件夹/ })).not.toBeInTheDocument()
    const input = screen.getByRole('textbox', { name: /仓库目录/ })
    await userEvent.type(input, '/tmp/manual')
    await userEvent.click(screen.getByRole('button', { name: /看看这个目录/ }))

    await waitFor(() => {
      expect(previewProject).toHaveBeenCalledWith('/tmp/manual')
    })
  })

  // 已经在了的条目仍然列出来——用户要看的是「最终长什么样」。
  it('已经在了的条目也列出来并标注', async () => {
    previewProject.mockResolvedValue({
      ...realPreview,
      actions: [{ ...realPreview.actions[0], already_there: true }],
    })
    await openAndPick()

    const item = await waitFor(() => {
      const el = screen.getByText(/\.acpflows$/).closest('li')
      expect(el).not.toBeNull()
      return el as HTMLElement
    })
    expect(item).toHaveAttribute('data-already', 'true')
    expect(within(item).getByText(/已经在了/)).toBeInTheDocument()
  })

  it('关掉时不渲染', () => {
    render(<CreateProjectDialog open={false} onClose={noop} onCreated={noop} />)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })
})
