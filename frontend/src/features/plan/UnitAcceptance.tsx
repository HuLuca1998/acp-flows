import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { acceptUnit, collectEvidence, getAcceptance } from '@/api/system'
import type { Acceptance } from '@/models/acceptance'

import styles from './UnitAcceptance.module.css'

export type UnitAcceptanceProps = {
  workID: string
  unitID: string
}

/**
 * 验收：每条标准旁边是**它有没有证据**。
 *
 * ★★ 没证据**不等于通过**（设计稿的 `○ 无证据`）。把它当成通过的话，
 * 一个什么都没做的单元也能「全部通过」——而用户是照着这张表决定
 * 「要不要点通过」的。
 *
 * ★★ AI 转述的证据**标出来**：分不出来源的话，一条转述会和一份真 diff
 * 长得一样，而用户判断「该不该信」全靠这一点。
 */
export function UnitAcceptance({ workID, unitID }: UnitAcceptanceProps) {
  const { t } = useTranslation()
  const [data, setData] = useState<Acceptance | null>(null)
  const [pending, setPending] = useState(false)
  // 验收通过之后拿到的 commit——用户要看得到「它落到哪儿了」
  const [commit, setCommit] = useState('')
  const [errorCode, setErrorCode] = useState<string | null>(null)

  const reload = useCallback(async () => {
    try {
      setData(await getAcceptance(workID, unitID))
    } catch {
      // 还没有契约时这一块整个不显示——那是常态，不是错
      setData(null)
    }
  }, [workID, unitID])

  useEffect(() => {
    void reload()
  }, [reload])

  const collect = useCallback(async () => {
    setPending(true)
    try {
      setData(await collectEvidence(workID, unitID))
    } catch {
      await reload()
    } finally {
      setPending(false)
    }
  }, [workID, unitID, reload])

  const accept = useCallback(async () => {
    setPending(true)
    setErrorCode(null)
    try {
      setCommit(await acceptUnit(workID, unitID))
    } catch (err) {
      // ★ 拒绝的理由要说清：「没有证据」与「没有改动」是两回事，
      // 而用户下一步该做什么完全不同
      setErrorCode(err instanceof Error ? err.message : 'accept_failed')
    } finally {
      setPending(false)
    }
  }, [workID, unitID])

  if (data === null) {
    return null
  }

  // ★ 两个数**各自算**：证据多于标准时不该显示成「超额通过」
  const covered = data.criteria.filter((c) => c.evidence_ids.length > 0).length

  return (
    <div className={styles.panel}>
      <div className={styles.head}>
        <span className={styles.counts}>
          {t('acceptance.counts', {
            evidence: data.evidence.length,
            covered,
            total: data.criteria.length,
          })}
        </span>
        <button
          type="button"
          className={styles.collect}
          disabled={pending}
          onClick={() => void collect()}
        >
          {t(pending ? 'acceptance.collecting' : 'acceptance.collect')}
        </button>
        {/*
          ★★ 「通过」**由用户点**。已经通过的显示 commit 而不是再给一个
          能点第二次的按钮——点第二次会撞上「没有改动」而报错，
          而那个错看起来像出了问题。
        */}
        {commit === '' ? (
          <button
            type="button"
            className={styles.accept}
            disabled={pending || covered === 0}
            onClick={() => void accept()}
          >
            {t('acceptance.accept')}
          </button>
        ) : (
          <span className={styles.accepted}>{t('acceptance.accepted', { commit })}</span>
        )}
      </div>

      <ul className={styles.criteria}>
        {data.criteria.map((c) => (
          <li key={c.id} className={styles.criterion} data-covered={c.evidence_ids.length > 0}>
            {/* ★★ 有证据标 `✓ ev-441`，没证据标 `○ 无证据`——**不是通过** */}
            <span className={styles.mark}>
              {c.evidence_ids.length > 0 ? `✓ ${c.evidence_ids.join(' ')}` : t('acceptance.noEvidence')}
            </span>
            <span className={styles.text}>{c.text}</span>
          </li>
        ))}
      </ul>

      {errorCode !== null && (
        <span className={styles.error}>{t(acceptProblemKey(errorCode))}</span>
      )}

      {data.evidence.map((e) => (
        <div key={e.id} className={styles.evidence} data-kind={e.kind}>
          <span className={styles.evID}>{e.id}</span>
          <span className={styles.kind}>{e.kind}</span>
          <span className={styles.summary}>{e.summary}</span>
          {/*
            ★★ AI 转述的**标出来**。不标的话，一条转述会和一份真 diff
            长得一样——而用户判断「该不该信」全靠这一点。
          */}
          {!e.trustworthy && <span className={styles.hearsay}>{t('acceptance.fromAgent')}</span>}
        </div>
      ))}
    </div>
  )
}

/** 错误码 → 词条 key 的**显式映射**（不许模板拼接，check-i18n 会拦）。 */
const ACCEPT_ERROR_KEY: Record<string, string> = {
  nothing_accepted: 'acceptance.error.nothingAccepted',
  nothing_to_commit: 'acceptance.error.nothingToCommit',
  commit_unavailable: 'acceptance.error.commitUnavailable',
}

function acceptProblemKey(code: string): string {
  return ACCEPT_ERROR_KEY[code] ?? 'acceptance.error.failed'
}
