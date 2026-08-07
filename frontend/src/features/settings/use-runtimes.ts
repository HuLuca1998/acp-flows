import { useCallback, useEffect, useState } from 'react'

import { listRuntimes } from '@/api/system'
import type { Runtime } from '@/models/runtime'

export type RuntimesState = {
  runtimes: Runtime[]
  loading: boolean
  /** 整体检测失败的原因码；null 表示没失败。 */
  errorCode: string | null
  refresh: () => Promise<void>
}

/**
 * 拉取 ACP Runtime 检测结果。
 *
 * ★ **进页面查一次，不轮询**——与更新检查同一个道理（docs/adr/0007 修订 3）：
 * 用户不看设置页的时候查了也没人看，而探测每次都要拉起子进程。
 * 用户装完东西回来点「重新检测」即可。
 *
 * ★ 失败时**必须留住 errorCode**，不能吞掉后返回空数组：
 * 空数组会被界面显示成「一个都没装」，用户就去装已经装好的东西。
 */
export function useRuntimes(): RuntimesState {
  const [runtimes, setRuntimes] = useState<Runtime[]>([])
  const [loading, setLoading] = useState(true)
  const [errorCode, setErrorCode] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    setLoading(true)
    setErrorCode(null)
    try {
      setRuntimes(await listRuntimes())
    } catch (err) {
      setErrorCode(errorCodeOf(err))
      // 上一轮的结果一并丢掉：留着会让用户看到一份说不清是什么时候的旧结论
      setRuntimes([])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void refresh()
  }, [refresh])

  return { runtimes, loading, errorCode, refresh }
}

/** 从错误里取机器可读的原因码；取不到时给一个兜底码，绝不返回 null。 */
function errorCodeOf(err: unknown): string {
  if (err !== null && typeof err === 'object' && 'type' in err) {
    const type = (err as { type?: unknown }).type
    if (typeof type === 'string' && type !== '') {
      return type
    }
  }
  return 'runtime_detection_failed'
}
