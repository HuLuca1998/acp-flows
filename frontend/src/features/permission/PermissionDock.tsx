import { useCallback, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { PermissionCard } from './PermissionCard'
import styles from './PermissionDock.module.css'
import type { PermissionRequest } from './model'

export type PermissionDockProps = {
  /** 还没被上游撤回的全部请求。 */
  asks: PermissionRequest[]
  /** 提交用户的选择。抛异常表示没提交上去。 */
  onDecide: (askID: string, optionID: string) => void | Promise<void>
}

/**
 * 待裁决的权限请求。
 *
 * ★ 这一层管的是「哪些还没应答」。两条要紧的：
 *
 * - **点了才消失**（R2）：不消失的话用户不确定自己点上没有，很可能再点一次，
 *   而第二条应答会被 Agent 当成一个不认识的请求。
 * - **不点它不会自己消失**（R3）：没有自动关闭、没有超时。用户去倒杯水回来，
 *   卡片没了、AI 也停着——他不知道刚才发生过什么，更不知道该做什么。
 */
export function PermissionDock({ asks, onDecide }: PermissionDockProps) {
  const { t } = useTranslation()
  // 已经提交成功的：本地先移除，不等上游的事件回来——那要一个来回，
  // 期间用户对着一张点过的卡片，会以为没点上。
  const [settled, setSettled] = useState<ReadonlySet<string>>(new Set())
  const [pending, setPending] = useState<ReadonlySet<string>>(new Set())
  const [failed, setFailed] = useState<ReadonlySet<string>>(new Set())

  const decide = useCallback(
    async (askID: string, optionID: string) => {
      setPending((prev) => new Set(prev).add(askID))
      setFailed((prev) => without(prev, askID))
      try {
        await onDecide(askID, optionID)
        setSettled((prev) => new Set(prev).add(askID))
      } catch {
        // ★ 提交失败时卡片**留下**。静默移除的话，用户以为自己处理完了，
        // 而 AI 那边还在等——他会对着一个不动的界面等下去。
        setFailed((prev) => new Set(prev).add(askID))
      } finally {
        setPending((prev) => without(prev, askID))
      }
    },
    [onDecide],
  )

  const open = asks.filter((a) => !settled.has(a.id))
  if (open.length === 0) {
    // 一条都没有时什么都不渲染——空容器会在时间线上留一条无意义的分隔
    return null
  }

  return (
    <div className={styles.dock}>
      {open.map((a) => (
        <div key={a.id} className={styles.slot}>
          <PermissionCard
            ask={a}
            pending={pending.has(a.id)}
            onDecide={(optionID) => void decide(a.id, optionID)}
          />
          {failed.has(a.id) && <p className={styles.error}>{t('permission.submitFailed')}</p>}
        </div>
      ))}
    </div>
  )
}

function without(set: ReadonlySet<string>, id: string): Set<string> {
  const next = new Set(set)
  next.delete(id)
  return next
}
