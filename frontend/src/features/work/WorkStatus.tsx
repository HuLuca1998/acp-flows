import { useCallback, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { cancelWork } from '@/api/system'

import styles from './WorkStatus.module.css'

export type WorkStatusProps = {
  workID: string
  /** 工作状态。**原值**，不翻译（AGENTS.md §8 术语表）。 */
  state: string
  /** 停下之后通知上层刷新。 */
  onCancelled?: () => void
}

/**
 * 工作状态 + 「停下」。
 *
 * ★ 状态词**显示英文原值**，不翻译。术语表明写这一条：用户在文档、日志、
 * 界面上看到的必须是同一个词，否则他没法把三者对上。
 *
 * ★ 不能停的时候**不给按钮，并说清楚为什么**。只把按钮变灰的话，
 * 用户会以为是应用卡了——他需要知道「不是不让你停，是现在停了会留下
 * 一个说不清的半成品」。
 */
export function WorkStatus({ workID, state, onCancelled }: WorkStatusProps) {
  const { t } = useTranslation()
  const [pending, setPending] = useState(false)
  const [errorCode, setErrorCode] = useState<string | null>(null)

  const stop = useCallback(async () => {
    setPending(true)
    setErrorCode(null)
    try {
      await cancelWork(workID)
      onCancelled?.()
    } catch (err) {
      // ★ 如实告知。静默失败的话，用户以为停住了而 AI 照跑——
      // 账单继续涨，而他不会再看这个界面。
      setErrorCode(codeOf(err))
    } finally {
      setPending(false)
    }
  }, [workID, onCancelled])

  return (
    <div className={styles.bar} data-work-state={state}>
      <span className={styles.dot} aria-hidden="true" />
      {/* 等宽英文原值，照设计稿的 `executing · 3/7` */}
      <span className={styles.state}>{state}</span>

      {CANCELLABLE.has(state) && (
        <button type="button" className={styles.stop} disabled={pending} onClick={() => void stop()}>
          {t(pending ? 'work.stopping' : 'work.stop')}
        </button>
      )}
      {state === 'reviewing_unit' && <span className={styles.hint}>{t('work.stopBlockedReview')}</span>}
      {errorCode !== null && <span className={styles.error}>{t(problemKey(errorCode))}</span>}
    </div>
  )
}

/**
 * 能停的状态。
 *
 * ★ 与后端 `domain/model.CanCancel` 是**同一份判断的两处表达**。
 * 前端这份只决定「给不给按钮」——真正说了算的是后端，它会回 409。
 * 不一致时用户看到的是「点了按钮被拒」，而不是「按钮点不动」——
 * 前者至少说得出原因。
 */
const CANCELLABLE = new Set(['initializing', 'clarifying', 'planning', 'ready', 'executing'])

/**
 * 错误码 → 词条 key 的**显式映射**（不许模板拼接，check-i18n 会拦）。
 *
 * ★ 「现在不能停」与「停失败了」要分得开：都说「失败」的话，
 * 用户会一直重试一个注定被拒的操作。
 */
const ERROR_KEY: Record<string, string> = {
  work_cancel_not_allowed: 'work.error.cancelNotAllowed',
  work_cancel_unavailable: 'work.error.cancelUnavailable',
  work_not_found: 'work.error.notFound',
}

function problemKey(code: string): string {
  return ERROR_KEY[code] ?? 'work.error.cancelFailed'
}

function codeOf(err: unknown): string {
  if (err instanceof Error && err.message !== '') {
    return err.message
  }
  return 'work_operation_failed'
}
