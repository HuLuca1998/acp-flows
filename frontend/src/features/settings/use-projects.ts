import { useCallback, useEffect, useState } from 'react'

import { addProject, listProjects, removeProject } from '@/api/system'
import type { Project } from '@/models/project'
import { capabilities, pickDirectory } from '@/platform'

export type ProjectsState = {
  projects: Project[]
  loading: boolean
  /** 读列表或添加失败的原因码；null 表示没失败。 */
  errorCode: string | null
  canPickDirectory: boolean
  /** path 为空则弹系统对话框让用户选；非空表示用户手填了路径。 */
  add: (path?: string) => Promise<void>
  remove: (id: string) => Promise<void>
}

/**
 * 项目列表的读写。
 *
 * ★ 每次增删之后**重新拉一遍列表**，而不是在本地数组上改。
 * 本地改看起来更快，但服务端做了规整与查重（`/a/b/` 与 `/a/b` 是同一个），
 * 本地改会让界面显示的东西和真实存的东西对不上——而对不上的是路径，
 * 用户下一步就要靠它开工作。
 */
export function useProjects(): ProjectsState {
  const [projects, setProjects] = useState<Project[]>([])
  const [loading, setLoading] = useState(true)
  const [errorCode, setErrorCode] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    setLoading(true)
    try {
      setProjects(await listProjects())
      setErrorCode(null)
    } catch (err) {
      setErrorCode(errorCodeOf(err))
      // 上一轮的结果一并丢掉：留着会让用户看到一份说不清是什么时候的旧列表
      setProjects([])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void refresh()
  }, [refresh])

  const add = useCallback(
    async (path?: string) => {
      setErrorCode(null)
      try {
        // 没给路径就弹系统对话框。用户取消时 pickDirectory 返回 null，
        // 那不是错误——什么都不做就好。
        const target = path ?? (await pickDirectory())
        if (target === null || target === undefined || target === '') {
          return
        }
        await addProject(target)
        await refresh()
      } catch (err) {
        setErrorCode(errorCodeOf(err))
      }
    },
    [refresh],
  )

  const remove = useCallback(
    async (id: string) => {
      setErrorCode(null)
      try {
        await removeProject(id)
        await refresh()
      } catch (err) {
        setErrorCode(errorCodeOf(err))
      }
    },
    [refresh],
  )

  return {
    projects,
    loading,
    errorCode,
    canPickDirectory: capabilities().canPickDirectory,
    add,
    remove,
  }
}

/** 从错误里取机器可读的原因码；取不到时给兜底码，绝不返回 null。 */
function errorCodeOf(err: unknown): string {
  if (err !== null && typeof err === 'object' && 'type' in err) {
    const type = (err as { type?: unknown }).type
    if (typeof type === 'string' && type !== '') {
      return type
    }
  }
  return 'project_operation_failed'
}
