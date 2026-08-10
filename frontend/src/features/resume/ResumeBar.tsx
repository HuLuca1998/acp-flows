import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { listResumable, resumeWork } from '@/api/library'
import type { ResumableWork } from '@/models/resume'

import styles from './ResumeBar.module.css'

export type ResumeBarProps = {
  /** 点一条 → 回到那个工作。 */
  onResume: (workID: string) => void
}

/**
 * 「接着做」入口（`U8.3.1`）。
 *
 * ★★ 这一整块是**探索发现的功能级缺口**补的：检查点、恢复逻辑、
 * `GET /v1/system/resume` 后端早就做完了，而界面上没有任何入口——
 * 用户打开应用永远看不到「有 2 个工作可以接着做」，那整套代码等于没用，
 * 而他不会知道自己少了什么。
 *
 * ★★ **不自动恢复**：接着做是用户的决定。自动跳进去的话，他打开应用
 * 想开个新工作，却发现自己落在昨天那个半截的现场里。
 */
export function ResumeBar({ onResume }: ResumeBarProps) {
  const { t } = useTranslation()
  const [items, setItems] = useState<ResumableWork[]>([])
  /** 工作区脏、正在等用户确认的那一条。 */
  const [dirty, setDirty] = useState('')

  useEffect(() => {
    let alive = true
    void listResumable()
      .then((list) => {
        if (alive) {
          setItems(list)
        }
      })
      .catch(() => {
        // ★ 查不了就不显示这一块。**不显示一句错误**：用户多半根本
        // 没有可恢复的工作，为一次后台查询失败在首屏摆一条红字，
        // 会让他以为应用坏了。
        if (alive) {
          setItems([])
        }
      })
    return () => {
      alive = false
    }
  }, [])

  const pick = useCallback(
    async (workID: string, force: boolean) => {
      try {
        // ★★ **先真的恢复**（切 worktree、脏检查），成功了再跳。
        // 直接跳的话，界面看起来能用而那一整套恢复逻辑从没跑过。
        await resumeWork(workID, force)
        setDirty('')
        onResume(workID)
      } catch (err) {
        // ★★ 脏是一个**用户能处理的状态**：问他一句，不静默继续。
        // 他手工改过那个 worktree，而状态推回可跑之后 AI 会接着往上写。
        if (err instanceof Error && err.message === 'worktree_dirty') {
          setDirty(workID)
          return
        }
        // 别的失败**不跳**：跳过去他会对着一个没恢复的工作接着说话
        setDirty('')
      }
    },
    [onResume],
  )

  // ★ 一个都没有时**整块不显示**——那是绝大多数人每次打开应用的状态。
  if (items.length === 0) {
    return null
  }

  return (
    <section className={styles.bar} aria-label={t('resume.title')}>
      <span className={styles.title}>{t('resume.title', { count: items.length })}</span>

      <div className={styles.list}>
        {items.map((it) => (
          <button
            key={it.work_id}
            type="button"
            className={styles.item}
            data-work={it.work_id}
            onClick={() => void pick(it.work_id, false)}
          >
            {/*
              ★★ 显示**停在哪个单元**。只有工作 id 的话，用户看到的是
              「work-03 · work-05」两行——那两个词对他没有任何意义。
              他记得的是「我在做那个取消功能」。
            */}
            <span className={styles.unit}>{it.unit_id ?? it.work_id}</span>
            {it.paused_at !== undefined && it.paused_at !== '' && (
              <span className={styles.when}>{formatWhen(it.paused_at)}</span>
            )}
          </button>
        ))}
      </div>

      {/*
        ★★ 工作区脏时**问一句再继续**。他手工改过那个 worktree，
        而恢复之后 AI 会接着往上写——先告诉他，他才有机会去看看
        自己改了什么。这里不擅自决定，只把选择摆出来。
      */}
      {dirty !== '' && (
        <div className={styles.dirty} role="alert">
          <span>{t('resume.dirty')}</span>
          <button type="button" className={styles.go} onClick={() => void pick(dirty, true)}>
            {t('resume.dirtyGo')}
          </button>
          <button type="button" className={styles.cancel} onClick={() => setDirty('')}>
            {t('resume.dirtyCancel')}
          </button>
        </div>
      )}
    </section>
  )
}

/**
 * 暂停时间显示成「几分钟前」。
 *
 * ★ 用户开着三四个工作时靠它认出「哪个是刚才那个」。显示绝对时间戳的话，
 * 他得自己做减法——而他要的答案是「哪个最近」。
 */
function formatWhen(iso: string): string {
  const then = new Date(iso).getTime()
  if (Number.isNaN(then)) {
    return ''
  }
  const mins = Math.max(0, Math.round((Date.now() - then) / 60000))
  if (mins < 1) {
    return 'just now'
  }
  if (mins < 60) {
    return `${mins}m`
  }
  const hours = Math.round(mins / 60)
  return hours < 24 ? `${hours}h` : `${Math.round(hours / 24)}d`
}
