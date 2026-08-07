import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import type { Runtime } from '@/models/runtime'

import { RuntimeSection } from './RuntimeSection'

// M1 U1.3.1 · ACP Runtime 检测（验收点 V3）
//
// ★ 这一块的失败模式是**误导**：把「检测不了」显示成「一个都没装」，
// 用户就会去装已经装好的东西，装完发现还是不行。
// 所以「分不清就别下结论」的断言比正常路径更重要。

const noop = () => {}

function renderSection(over: Partial<Parameters<typeof RuntimeSection>[0]> = {}) {
  return render(
    <RuntimeSection runtimes={[]} loading={false} errorCode={null} onRetry={noop} {...over} />,
  )
}

const ready: Runtime = {
  name: 'claude',
  status: 'ready',
  installed: true,
  authenticated: true,
  active_version: '0.63.0',
  path: '/Users/x/.npm-global/bin/claude-agent-acp',
}

describe('ACP Runtime 检测', () => {
  it('就绪的 Runtime 显示名字、版本与「已安装 · 已登录」', () => {
    renderSection({ runtimes: [ready] })

    expect(screen.getByText('claude')).toBeInTheDocument()
    expect(screen.getByText('0.63.0')).toBeInTheDocument()
    expect(screen.getByText(/已安装 · 已登录/)).toBeInTheDocument()
  })

  // R2：提示必须含**具体命令**，不是「请检查配置」。
  it('没装时把该敲的命令原样显示出来，能选中复制', () => {
    const missing: Runtime = {
      name: 'codex',
      status: 'not_installed',
      installed: false,
      remedy: { command: 'npm i -g @agentclientprotocol/codex-acp' },
    }
    renderSection({ runtimes: [missing] })

    expect(screen.getByText(/未安装/)).toBeInTheDocument()
    // 整条命令原样出现，用户能直接选中复制去终端敲
    expect(screen.getByText('npm i -g @agentclientprotocol/codex-acp')).toBeInTheDocument()
  })

  it('没登录时给的是登录命令，不是安装命令', () => {
    const notAuthed: Runtime = {
      name: 'codex',
      status: 'not_authenticated',
      installed: true,
      authenticated: false,
      active_version: '1.1.7',
      remedy: { command: 'codex login' },
    }
    renderSection({ runtimes: [notAuthed] })

    expect(screen.getByText(/未登录/)).toBeInTheDocument()
    expect(screen.getByText('codex login')).toBeInTheDocument()
    // 已经装上了，就别再劝人安装
    expect(screen.queryByText(/npm i -g/)).not.toBeInTheDocument()
  })

  // ★ 最重要的一条：检测失败不许伪装成任何确定结论。
  it('探测失败时说「检测不出来」，不说「未安装」，也不给命令', () => {
    const broken: Runtime = {
      name: 'claude',
      status: 'probe_failed',
      installed: false,
    }
    renderSection({ runtimes: [broken] })

    expect(screen.getByText(/检测不出来/)).toBeInTheDocument()
    // 不知道装没装，就不能说「未安装」——用户会照着去装已经装好的东西
    expect(screen.queryByText(/未安装/)).not.toBeInTheDocument()
    expect(screen.queryByText(/npm i -g/)).not.toBeInTheDocument()
  })

  // R4：整个检测挂了，这一块显示错误并给重试，不是显示成空。
  it('整体检测失败时显示错误与重试，而不是「一个都没装」', () => {
    renderSection({ errorCode: 'runtime_detection_unavailable' })

    expect(screen.getByRole('button', { name: /重新检测/ })).toBeInTheDocument()
    expect(screen.queryByText(/未安装/)).not.toBeInTheDocument()
  })

  it('加载中显示骨架，不显示空状态', () => {
    renderSection({ loading: true })

    expect(screen.queryByText(/未安装/)).not.toBeInTheDocument()
    expect(screen.queryByText(/检测不出来/)).not.toBeInTheDocument()
  })
})
