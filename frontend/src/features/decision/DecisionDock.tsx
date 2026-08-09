import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { answerDecision, listPendingDecisions } from '@/api/system'
import type { Decision } from '@/models/decision'

import styles from './DecisionDock.module.css'

export type DecisionDockProps = {
  workID: string
}

/**
 * 待决策卡片。
 *
 * ★★ **决定权在用户手里**：AI 停在这里等他。这一块排在时间线**上方**，
 * 与权限卡片同理——他要一眼看到「有件事在等我」，而不是往下滚才发现。
 *
 * ★★ 推荐的那个**只标出来，不预选**：预选中的话他会顺手点确定，
 * 而那正好绕过了「让他自己决定」这件事。
 */
export function DecisionDock({ workID }: DecisionDockProps) {
  const { t } = useTranslation()
  const [pending, setPending] = useState<Decision[]>([])
  const [busy, setBusy] = useState('')
  const [errorCode, setErrorCode] = useState<string | null>(null)

  const reload = useCallback(async () => {
    if (workID === '') {
      setPending([])
      return
    }
    try {
      setPending(await listPendingDecisions(workID))
    } catch {
      // ★ 读不到就不显示——没装配决策存储时不该在对话页弹一句错误
      setPending([])
    }
  }, [workID])

  useEffect(() => {
    void reload()
  }, [reload])

  const answer = useCallback(
    async (decisionID: string, optionID: string) => {
      setBusy(decisionID)
      setErrorCode(null)
      try {
        await answerDecision(workID, decisionID, optionID)
        await reload()
      } catch (err) {
        setErrorCode(err instanceof Error ? err.message : 'decision_failed')
      } finally {
        setBusy('')
      }
    },
    [workID, reload],
  )

  if (pending.length === 0) {
    return null
  }

  return (
    <div className={styles.dock}>
      {pending.map((d) => (
        <section key={d.id} className={styles.card} data-level={d.level} aria-label={d.question}>
          <header className={styles.head}>
            {/* 等级是**原值**（D2/D3），不翻译——术语表硬要求 */}
            <span className={styles.level}>{d.level}</span>
            <span className={styles.question}>{d.question}</span>
          </header>

          <div className={styles.options}>
            {d.options.map((o) => (
              <button
                key={o.id}
                type="button"
                className={styles.option}
                data-recommended={o.recommended}
                disabled={busy === d.id}
                onClick={() => void answer(d.id, o.id)}
              >
                <span className={styles.optionText}>{o.text}</span>
                {/*
                  ★★ **影响必须显示**：没有它用户在盲选——
                  他看到三个名字，而不知道选哪个会发生什么。
                */}
                <span className={styles.impact}>{o.impact}</span>
                {/* ★ 推荐**只是标记**，按钮不预选中 */}
                {o.recommended && (
                  <span className={styles.recommended}>{t('decision.recommended')}</span>
                )}
              </button>
            ))}
          </div>

          {/* ★ 「稍后决定」= 什么都不做。它不该是一个会改变现场的按钮——
              所以这里只是一句说明，而不是第四个选项。 */}
          <p className={styles.later}>{t('decision.later')}</p>
        </section>
      ))}

      {errorCode !== null && <p className={styles.error}>{t(problemKey(errorCode))}</p>}
    </div>
  )
}

/** 错误码 → 词条 key 的**显式映射**（不许模板拼接，check-i18n 会拦）。 */
const ERROR_KEY: Record<string, string> = {
  decision_answered: 'decision.error.answered',
  decision_not_found: 'decision.error.notFound',
  decision_store_unavailable: 'decision.error.storeUnavailable',
}

function problemKey(code: string): string {
  return ERROR_KEY[code] ?? 'decision.error.failed'
}
