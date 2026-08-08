import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { listProjects, listWorks } from '@/api/system'
import type { Project } from '@/models/project'
import type { Work } from '@/models/work'

import styles from './ProjectTree.module.css'

export type ProjectTreeProps = {
  /** 在某个项目下开一个新对话。参数是项目的本地绝对路径。 */
  onNewWork: (projectPath: string) => void
  /** 打开一条已有的工作。 */
  onOpenWork: (workID: string) => void
}

/**
 * 左栏项目树：项目 → 它的工作 → 「新建对话」。
 *
 * ★ 这是用户流程的**第一步**（创建项目 → 创建对话 → 观测对话）。
 * 2026-08-08 之前这里是骨架占位，而 `/v1/projects` 明明有数据——
 * 用户打开应用第一句话就是「为什么菜单没有显示项目列表和对话记录」。
 *
 * 形态照设计稿：可折叠的项目名 + 下挂工作（标题 + 状态）+ 「新建对话」。
 */
export function ProjectTree({ onNewWork, onOpenWork }: ProjectTreeProps) {
  const { t } = useTranslation()
  const [projects, setProjects] = useState<Project[] | null>(null)
  const [works, setWorks] = useState<Work[]>([])
  const [failed, setFailed] = useState(false)
  const [collapsed, setCollapsed] = useState<ReadonlySet<string>>(new Set())

  useEffect(() => {
    void (async () => {
      try {
        const [ps, ws] = await Promise.all([listProjects(), listWorks()])
        setProjects(ps)
        setWorks(ws)
      } catch {
        // ★ 查不到要**说出来**，不装作「还没有项目」——
        // 装作没有的话，用户以为自己的项目丢了，而实际是后端没起来。
        setFailed(true)
        setProjects([])
      }
    })()
  }, [])

  const toggle = useCallback((id: string) => {
    setCollapsed((prev) => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }, [])

  if (failed) {
    return <p className={styles.hint}>{t('rail.projectsFailed')}</p>
  }
  if (projects === null) {
    return null // 还没查回来：留白，别闪一下「还没有项目」
  }
  if (projects.length === 0) {
    return <p className={styles.hint}>{t('rail.noProjects')}</p>
  }

  return (
    <div className={styles.tree}>
      {projects.map((p) => {
        const open = !collapsed.has(p.id)
        // ★ 按**路径**归位：工作记的是项目路径，不是项目 id。
        // 归错的话，用户在 A 项目下看到 B 项目的工作，会以为自己点错了。
        const mine = works.filter((w) => w.project === p.path)

        return (
          <section key={p.id} className={styles.project} data-project={p.id}>
            <button
              type="button"
              className={styles.projectName}
              aria-expanded={open}
              onClick={() => toggle(p.id)}
            >
              <i className={`ph ${open ? 'ph-caret-down' : 'ph-caret-right'}`} aria-hidden="true" />
              <span className={styles.projectLabel}>{p.name}</span>
            </button>

            {open && (
              <>
                {mine.map((w) => (
                  <button
                    key={w.id}
                    type="button"
                    className={styles.work}
                    data-state={w.state}
                    onClick={() => onOpenWork(w.id ?? '')}
                  >
                    <span className={styles.workTitle}>{w.prompt ?? w.id}</span>
                    <span className={styles.workState}>{w.state}</span>
                  </button>
                ))}
                <button
                  type="button"
                  className={styles.newWork}
                  onClick={() => onNewWork(p.path ?? '')}
                >
                  <i className="ph ph-plus" aria-hidden="true" />
                  {t('rail.newWork')}
                </button>
              </>
            )}
          </section>
        )
      })}
    </div>
  )
}
