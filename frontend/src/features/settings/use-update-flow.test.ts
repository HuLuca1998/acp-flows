import { act, renderHook, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import * as systemApi from '@/api/system'
import * as platform from '@/platform'

import { useUpdateFlow } from './use-update-flow'

// M1 U1.1.3 · 一键更新流程
//
// ★ 这里锁住的两条直接关系到「会不会丢用户的工作」：
//   ① 先 prepare 再下载，顺序不可颠倒
//   ② prepare 返回 blocked 时**必须停**，绝不继续安装

const w = globalThis as { __TAURI_INTERNALS__?: unknown }

beforeEach(() => {
  // 装成桌面形态，否则流程在第一步就退出了
  w.__TAURI_INTERNALS__ = {}
})

afterEach(() => {
  delete w.__TAURI_INTERNALS__
  vi.restoreAllMocks()
})

describe('一键更新流程', () => {
  it('先 prepare 再下载，顺序不可颠倒', async () => {
    const order: string[] = []
    vi.spyOn(systemApi, 'prepareUpdate').mockImplementation(() => {
      order.push('prepare')
      return Promise.resolve({ status: 'ready' as const, prepared: [], blocked: [] })
    })
    vi.spyOn(platform, 'downloadAndInstall').mockImplementation(() => {
      order.push('download')
      return Promise.resolve()
    })

    const { result } = renderHook(() => useUpdateFlow())
    await act(async () => {
      await result.current.applyUpdate()
    })

    expect(order).toEqual(['prepare', 'download'])
  })

  // ★★ 最重要的一条：blocked 时**不下载**，并把卡住的工作列出来。
  // 装下去会丢掉用户几十分钟的活，而他事后无从追回。
  it('prepare 返回 blocked 时停下，且绝不发起下载', async () => {
    vi.spyOn(systemApi, 'prepareUpdate').mockResolvedValue({
      status: 'blocked',
      prepared: [],
      blocked: [{ work_id: 'work-08', reason: 'work_in_progress' }],
    })
    const download = vi.spyOn(platform, 'downloadAndInstall').mockResolvedValue()

    const { result } = renderHook(() => useUpdateFlow())
    await act(async () => {
      await result.current.applyUpdate()
    })

    expect(download).not.toHaveBeenCalled()
    expect(result.current.phase).toBe('blocked')
    expect(result.current.blocked).toHaveLength(1)
    expect(result.current.blocked[0]?.work_id).toBe('work-08')
  })

  it('prepare 本身失败时也不下载', async () => {
    vi.spyOn(systemApi, 'prepareUpdate').mockRejectedValue(new Error('update_prepare_failed'))
    const download = vi.spyOn(platform, 'downloadAndInstall').mockResolvedValue()

    const { result } = renderHook(() => useUpdateFlow())
    await act(async () => {
      await result.current.applyUpdate()
    })

    expect(download).not.toHaveBeenCalled()
    expect(result.current.phase).toBe('failed')
    expect(result.current.errorCode).toBe('update_prepare_failed')
  })

  it('浏览器形态下直接失败，不去碰 prepare', async () => {
    delete w.__TAURI_INTERNALS__
    const prepare = vi.spyOn(systemApi, 'prepareUpdate').mockResolvedValue({
      status: 'ready',
      prepared: [],
      blocked: [],
    })

    const { result } = renderHook(() => useUpdateFlow())
    await act(async () => {
      await result.current.applyUpdate()
    })

    expect(prepare).not.toHaveBeenCalled()
    expect(result.current.errorCode).toBe('update_not_supported')
  })

  // 检查失败绝不降级成「已是最新版本」。
  it('检查失败时报错，且不留下过期的状态', async () => {
    vi.spyOn(systemApi, 'checkUpdate').mockRejectedValue(new Error('update_check_failed'))

    const { result } = renderHook(() => useUpdateFlow())
    await act(async () => {
      await result.current.check()
    })

    expect(result.current.status).toBeNull()
    expect(result.current.errorCode).toBe('update_check_failed')
    expect(result.current.phase).toBe('failed')
  })

  it('下载进度会反馈给界面', async () => {
    vi.spyOn(systemApi, 'prepareUpdate').mockResolvedValue({
      status: 'ready',
      prepared: [],
      blocked: [],
    })
    vi.spyOn(platform, 'downloadAndInstall').mockImplementation((onProgress) => {
      onProgress?.(50, 100)
      return Promise.resolve()
    })

    const { result } = renderHook(() => useUpdateFlow())
    await act(async () => {
      await result.current.applyUpdate()
    })

    await waitFor(() => {
      expect(result.current.progress).toBe(50)
    })
  })
})
