import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { getMemoryBody, reviewMemory } from '@/api/library'

import styles from './MemoryCandidateCard.module.css'

export type MemoryCandidateCardProps = {
  memoryID: string
  /** 事件载荷里带的标题，正文还没读回来时先显示它。 */
  title?: string
  kind?: string
}

/**
 * 时间线上的记忆候选卡片。
 *
 * ★★ **记忆由用户收下，不由 AI 自己写进去。** 这张卡片就是那道关口——
 * 后端已经保证候选一律是 `candidate`，而这里保证他**看得到正文**再决定。
 */
export function MemoryCandidateCard({ memoryID, title, kind }: MemoryCandidateCardProps) {
  const { t } = useTranslation()
  const [text, setText] = useState('')
  const [loadError, setLoadError] = useState(false)
  const [done, setDone] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    let alive = true
    void getMemoryBody(memoryID)
      .then((b) => {
        if (alive) {
          setText(b.text)
        }
      })
      .catch(() => {
        // ★★ 正文读不到时**明说**，不显示成空白：空白看起来像
        // 「这条记忆没内容」，他会照着这个印象直接收下——
        // 而真相是那个 md 文件丢了。
        if (alive) {
          setLoadError(true)
        }
      })
    return () => {
      alive = false
    }
  }, [memoryID])

  const decide = useCallback(
    async (accept: boolean) => {
      setBusy(true)
      try {
        await reviewMemory(memoryID, accept ? 'confirm' : 'reject', 'user')
        setDone(accept ? 'accepted' : 'rejected')
      } finally {
        setBusy(false)
      }
    },
    [memoryID],
  )

  // ★ 决定过的**留在时间线上**，变成一条历史——擦掉的话，
  // 用户回头想不起来自己当时到底收了没有。
  if (done !== '') {
    return (
      <div className={styles.card} data-decided={done}>
        <span className={styles.head}>{title}</span>
        <span className={styles.verdict}>
          {t(done === 'accepted' ? 'memory.accepted' : 'memory.rejected')}
        </span>
      </div>
    )
  }

  return (
    <section className={styles.card} data-memory={memoryID} aria-label={title}>
      <header className={styles.head}>
        <span className={styles.label}>{t('memory.candidate')}</span>
        {kind !== undefined && kind !== '' && <span className={styles.kind}>{kind}</span>}
        <span className={styles.title}>{title}</span>
      </header>

      {/*
        ★★ **正文全文**，不只是标题：他要读到它才决定得了收不收。
        只给标题的话，他在盲选——而收下之后这条会影响后面每一轮。
      */}
      {loadError ? (
        <p className={styles.missing}>{t('memory.bodyMissing')}</p>
      ) : (
        <p className={styles.text}>{text}</p>
      )}

      {/* ★★ 两个按钮**都不预选中**：预选「收下」的话他会顺手点确定，
          而那正好绕过了「让他自己决定」这件事。 */}
      <div className={styles.actions}>
        <button
          type="button"
          className={styles.accept}
          disabled={busy}
          onClick={() => void decide(true)}
        >
          {t('memory.accept')}
        </button>
        <button
          type="button"
          className={styles.reject}
          disabled={busy}
          onClick={() => void decide(false)}
        >
          {t('memory.reject')}
        </button>
      </div>
    </section>
  )
}
