import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import type { Project } from '@/models/project'

import { ProjectSection } from './ProjectSection'

// M2 U2.1.1 · 项目管理（验收点 V4）
//
// ★ 这一块的失败模式是**让用户以为 Duet 动了他的东西**。
// 所以「移除」的措辞、以及非 git 目录给的是提示而不是拒绝，
// 比「列表能不能渲染出来」重要得多。

const noop = () => {}

function renderSection(over: Partial<Parameters<typeof ProjectSection>[0]> = {}) {
  return render(
    <ProjectSection
      projects={[]}
      loading={false}
      errorCode={null}
      canPickDirectory
      onAdd={noop}
      onRemove={noop}
      {...over}
    />,
  )
}

const gitProject: Project = {
  id: 'proj-01',
  name: 'my-app',
  path: '/Users/me/work/my-app',
  is_git_repo: true,
  default_branch: 'main',
}

describe('项目管理', () => {
  it('列出项目的名字、路径与默认分支', () => {
    renderSection({ projects: [gitProject] })

    expect(screen.getByText('my-app')).toBeInTheDocument()
    expect(screen.getByText('/Users/me/work/my-app')).toBeInTheDocument()
    expect(screen.getByText('main')).toBeInTheDocument()
  })

  // R3：非 git 目录**不拒绝**，但要给出能直接敲的命令。
  it('非 git 目录照样列出来，并显示 git init', () => {
    const plain: Project = {
      id: 'proj-02',
      name: 'notes',
      path: '/Users/me/notes',
      is_git_repo: false,
      remedy: { command: 'git init' },
    }
    renderSection({ projects: [plain] })

    expect(screen.getByText('notes')).toBeInTheDocument()
    expect(screen.getByText('git init')).toBeInTheDocument()
  })

  // ★ 措辞是这个单元的一部分：用户交出的是自己的代码目录，
  // 按钮写「删除」会让他以为文件没了。
  it('移除按钮写的是「移除」，不是「删除」', async () => {
    const onRemove = vi.fn()
    const user = userEvent.setup()
    renderSection({ projects: [gitProject], onRemove })

    const row = screen.getByText('my-app').closest('[data-row]')
    if (row === null) throw new Error('找不到项目行')
    const button = within(row as HTMLElement).getByRole('button')

    expect(button).toHaveTextContent('移除')
    expect(button).not.toHaveTextContent('删除')

    await user.click(button)
    expect(onRemove).toHaveBeenCalledWith('proj-01')
  })

  it('一个项目都没有时给出可操作的空状态，而不是一片空白', () => {
    renderSection({ projects: [] })

    expect(screen.getByText(/还没有添加/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /添加本地仓库/ })).toBeInTheDocument()
  })

  // ★ 浏览器里 showDirectoryPicker 只给句柄不给路径，拿不到路径就无从登记。
  // 装作能选、再拿一个假路径去请求，用户会在后端撞上「路径不存在」——
  // 而他明明刚从对话框里选过，那种错误没人能自己解决。
  it('拿不到目录选择能力时，给的是手动填路径而不是一个点了没反应的按钮', () => {
    renderSection({ canPickDirectory: false })

    expect(screen.getByRole('textbox')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /添加本地仓库/ })).not.toBeInTheDocument()
  })

  it('加载中不显示空状态', () => {
    renderSection({ loading: true })

    expect(screen.queryByText(/还没有添加/)).not.toBeInTheDocument()
  })

  it('出错时显示错误而不是伪装成「一个项目都没有」', () => {
    renderSection({ errorCode: 'project_service_unavailable' })

    expect(screen.queryByText(/还没有添加/)).not.toBeInTheDocument()
    expect(screen.getByText(/读不到/)).toBeInTheDocument()
  })
})
