import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import type { UpdateStatus } from '@/models/update'

import { UpdateSection } from './UpdateSection'

// M0 U0.3.2 · 更新区
//
// ★ 这里的失败模式是**静默**的：把「检查失败」显示成「已是最新版本」，
// 用户永远不会知道自己在用旧版。所以错误路径的断言比正常路径更重要。

const noop = () => {}

function renderSection(over: Partial<Parameters<typeof UpdateSection>[0]> = {}) {
  return render(
    <UpdateSection
      status={null}
      phase="idle"
      errorCode={null}
      blocked={[]}
      progress={0}
      onCheck={noop}
      onUpdate={noop}
      {...over}
    />,
  )
}

describe('更新区', () => {
  it('有新版本时显示版本号、说明与「一键更新并重启」', () => {
    const status: UpdateStatus = {
      state: 'available',
      current_version: '1.4.2',
      latest_version: '1.5.0',
      notes: '修复取消超时后 Runtime 仍在改文件',
    }
    renderSection({ status })

    expect(screen.getByText(/发现新版本/)).toBeInTheDocument()
    expect(screen.getByText('1.5.0')).toBeInTheDocument()
    expect(screen.getByText('修复取消超时后 Runtime 仍在改文件')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /一键更新并重启/ })).toBeInTheDocument()
  })

  // ★★ 最重要的一条：检查失败**绝不能**显示「已是最新版本」。
  it('检查失败时明确报错，且不出现「已是最新版本」', () => {
    renderSection({ errorCode: 'update_check_failed' })

    expect(screen.getByTestId('update-error')).toBeInTheDocument()
    expect(screen.queryByText(/已是最新版本/)).not.toBeInTheDocument()
    // 也不能冒出一个点不动的更新按钮
    expect(screen.queryByRole('button', { name: /一键更新并重启/ })).not.toBeInTheDocument()
  })

  it('已是最新时不显示更新按钮', () => {
    const status: UpdateStatus = { state: 'idle', current_version: '1.4.2' }
    renderSection({ status })

    expect(screen.getByTestId('update-idle')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /一键更新并重启/ })).not.toBeInTheDocument()
  })

  // Web 形态给一条真能走通的路，而不是一个死按钮。
  it('Web 形态显示前往下载，而不是一键更新', () => {
    const status: UpdateStatus = { state: 'unsupported', current_version: '1.4.2' }
    renderSection({ status })

    expect(screen.getByTestId('update-unsupported')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /前往下载/ })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /一键更新并重启/ })).not.toBeInTheDocument()
  })

  // 即使检查失败，用户至少要知道自己在用哪个版本。
  it('当前版本在任何状态下都显示', () => {
    renderSection({ errorCode: 'update_check_failed' })
    expect(screen.getByText(/当前版本/)).toBeInTheDocument()
  })

  it('点检查会触发一次检查', () => {
    const onCheck = vi.fn()
    const { getByRole } = renderSection({ onCheck })
    getByRole('button', { name: /检查更新/ }).click()
    expect(onCheck).toHaveBeenCalledTimes(1)
  })

  // ★ 有工作在跑时：停下、列出来、**不给继续安装的入口**。
  it('blocked 时列出卡住的工作，且不显示更新按钮', () => {
    renderSection({
      phase: 'blocked',
      status: { state: 'available', current_version: '1.4.2', latest_version: '1.5.0' },
      blocked: [{ work_id: 'work-08', reason: 'work_in_progress' }],
    })

    expect(screen.getByTestId('update-blocked')).toBeInTheDocument()
    expect(screen.getByText('work-08')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /一键更新并重启/ })).not.toBeInTheDocument()
  })

  it('下载中显示进度，且检查按钮不可点', () => {
    renderSection({ phase: 'downloading', progress: 42 })
    expect(screen.getByTestId('update-progress')).toHaveTextContent('42%')
    expect(screen.getByRole('button', { name: /检查更新/ })).toBeDisabled()
  })
})
