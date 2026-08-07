import { useCallback, useState } from 'react'

import { checkUpdate, prepareUpdate } from '@/api/system'
import type { UpdatePrepareResult, UpdateStatus } from '@/models/update'
import { capabilities, downloadAndInstall } from '@/platform'

/** 一键更新的阶段。界面据此决定显示什么。 */
export type UpdatePhase =
  | 'idle'
  | 'checking'
  | 'preparing'
  | 'downloading'
  | 'blocked'
  | 'failed'

export type UpdateFlow = {
  phase: UpdatePhase
  status: UpdateStatus | null
  /** 检查或更新失败的原因码（机器可读），null 表示没失败。 */
  errorCode: string | null
  /** blocked 时挡住更新的工作，供界面列给用户看。 */
  blocked: UpdatePrepareResult['blocked']
  /** 下载进度 0–100；未在下载时为 0。 */
  progress: number
  check: () => Promise<void>
  applyUpdate: () => Promise<void>
}

/**
 * 一键更新的完整流程。
 *
 * ★ **顺序不可颠倒：先 prepare，再下载。**
 * prepare 判断现在更新会不会打断用户；返回 blocked 时**必须停下**，
 * 并把卡住的工作列给他看——装下去会丢掉他几十分钟的活。
 *
 * 全程**不自动安装**：每一步都由用户点出来（M1 全局停止条件第 2 条）。
 */
export function useUpdateFlow(): UpdateFlow {
  const [phase, setPhase] = useState<UpdatePhase>('idle')
  const [status, setStatus] = useState<UpdateStatus | null>(null)
  const [errorCode, setErrorCode] = useState<string | null>(null)
  const [blocked, setBlocked] = useState<UpdatePrepareResult['blocked']>([])
  const [progress, setProgress] = useState(0)

  const check = useCallback(async () => {
    setPhase('checking')
    setErrorCode(null)
    try {
      setStatus(await checkUpdate())
      setPhase('idle')
    } catch (err) {
      // ★ 绝不降级成「已是最新版本」：网络断了、后端挂了都会走到这里。
      setStatus(null)
      setErrorCode(err instanceof Error ? err.message : 'update_check_failed')
      setPhase('failed')
    }
  }, [])

  const applyUpdate = useCallback(async () => {
    if (!capabilities().canSelfUpdate) {
      setErrorCode('update_not_supported')
      setPhase('failed')
      return
    }

    setErrorCode(null)
    setBlocked([])
    setPhase('preparing')

    let prepared: UpdatePrepareResult
    try {
      prepared = await prepareUpdate()
    } catch (err) {
      setErrorCode(err instanceof Error ? err.message : 'update_prepare_failed')
      setPhase('failed')
      return
    }

    // ★ blocked 时**绝不继续安装**。这不是错误，是一个要展示给用户的业务结论。
    if (prepared.status === 'blocked') {
      setBlocked(prepared.blocked)
      setPhase('blocked')
      return
    }

    setPhase('downloading')
    setProgress(0)
    try {
      await downloadAndInstall((downloaded, total) => {
        setProgress(total > 0 ? Math.round((downloaded / total) * 100) : 0)
      })
      // 走到这里说明 relaunch 没生效——正常情况下进程已经被替换了。
    } catch (err) {
      setErrorCode(err instanceof Error ? err.message : 'update_install_failed')
      setPhase('failed')
    }
  }, [])

  return { phase, status, errorCode, blocked, progress, check, applyUpdate }
}
