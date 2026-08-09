import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { freezeRequirement, getRequirement } from '@/api/system'
import type { Requirement } from '@/models/requirement'

import styles from './RequirementBar.module.css'

export type RequirementBarProps = {
  workID: string
}

/**
 * 需求快照条：当前是第几版、冻结了没有、还有什么没确认。
 *
 * ★★ **冻结由用户点，不由 AI 判断。** AI 说「我觉得问清楚了」和用户说
 * 「就这样」是两件事——而冻结之后这一版就进了计划与契约，改不动了。
 *
 * ★ 设计稿把「requirement v2 已冻结」画成消息头上的一枚标签（那个在
 * `Timeline` 里），但**没有画冻结这个动作**——用户总得有个地方点。
 * 这一条是实现补充，登记在 `design/PARITY.md`。
 */
export function RequirementBar({ workID }: RequirementBarProps) {
  const { t } = useTranslation()
  const [req, setReq] = useState<Requirement | null>(null)
  const [pending, setPending] = useState(false)
  const [errorCode, setErrorCode] = useState<string | null>(null)

  const reload = useCallback(async () => {
    if (workID === '') {
      setReq(null)
      return
    }
    try {
      setReq(await getRequirement(workID))
    } catch {
      // ★ 读不到就**不显示这一条**，不报错：还没提需求是新工作的常态，
      // 而弹一句「读取需求失败」会让用户以为出了什么事。
      setReq(null)
    }
  }, [workID])

  useEffect(() => {
    void reload()
  }, [reload])

  const freeze = useCallback(async () => {
    setPending(true)
    setErrorCode(null)
    try {
      setReq(await freezeRequirement(workID))
    } catch (err) {
      // ★ 冻不上要说清**为什么**：还有待确认的事实是最常见的原因，
      // 而那时用户该做的是先回答那几个问题，不是重试。
      setErrorCode(codeOf(err))
    } finally {
      setPending(false)
    }
  }, [workID])

  if (req === null) {
    return null
  }

  return (
    <div className={styles.bar} data-frozen={req.frozen}>
      {/* 等宽标识照设计稿，不翻译 */}
      <span className={styles.version}>{`requirement v${req.version}`}</span>
      <span className={styles.count}>{t('requirement.itemCount', { count: req.items.length })}</span>

      {req.frozen ? (
        <span className={styles.frozen}>{t('requirement.frozen')}</span>
      ) : (
        <button
          type="button"
          className={styles.freeze}
          disabled={pending}
          onClick={() => void freeze()}
        >
          {t(pending ? 'requirement.freezing' : 'requirement.freeze')}
        </button>
      )}

      {/* ★ 还剩几条没确认要**摆在明处**：带着没问清的问题往下走，
          AI 会自己替用户做决定，而那些决定会一路固化进计划与契约。 */}
      {req.open_facts.length > 0 && (
        <span className={styles.open}>
          {t('requirement.openFacts', { count: req.open_facts.length })}
        </span>
      )}
      {errorCode !== null && <span className={styles.error}>{t(problemKey(errorCode))}</span>}
    </div>
  )
}

/** 错误码 → 词条 key 的**显式映射**（不许模板拼接，check-i18n 会拦）。 */
const ERROR_KEY: Record<string, string> = {
  requirement_open_facts_remain: 'requirement.error.openFactsRemain',
  requirement_already_frozen: 'requirement.error.alreadyFrozen',
  requirement_not_found: 'requirement.error.notFound',
  requirement_store_unavailable: 'requirement.error.storeUnavailable',
}

function problemKey(code: string): string {
  return ERROR_KEY[code] ?? 'requirement.error.freezeFailed'
}

function codeOf(err: unknown): string {
  if (err instanceof Error && err.message !== '') {
    return err.message
  }
  return 'requirement_operation_failed'
}
