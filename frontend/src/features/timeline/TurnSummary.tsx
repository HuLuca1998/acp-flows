import { useTranslation } from 'react-i18next'

import styles from './TurnSummary.module.css'

export type TurnSummaryPayload = {
  outcome?: string
  plan_version?: number
  contract_version?: number
  unit_id?: string
  memory_ids?: string[]
  skill_refs?: string[]
}

/** 收场方式 → 词条 key 的**显式映射**（不许模板拼接，check-i18n 会拦）。 */
const OUTCOME_KEY: Record<string, string> = {
  done: 'summary.outcome.done',
  cancelled: 'summary.outcome.cancelled',
  failed: 'summary.outcome.failed',
  queue_full: 'summary.outcome.queueFull',
}

/**
 * 本轮小结（M7 完成标志第 6 条）。
 *
 * ★★ 每一行都来自**应用记的事实**，不是 AI 的自述。它说「我改了计划」时
 * 可能什么都没改——而用户看这几行正是为了不用往回滚就知道这轮发生了什么。
 *
 * ★★ **没发生的那一行不显示**：塞一个「计划：无变更」进去的话，
 * 四行里有三行是废话，用户会开始整块跳过——那正好淹掉真正变了的那一行。
 */
export function TurnSummary({ payload }: { payload: TurnSummaryPayload }) {
  const { t } = useTranslation()
  const memories = payload.memory_ids ?? []
  const skills = payload.skill_refs ?? []

  return (
    <div className={styles.summary} data-outcome={payload.outcome}>
      {payload.plan_version !== undefined && (
        <span className={styles.row} data-row="plan">
          {t('summary.plan', { version: payload.plan_version })}
        </span>
      )}

      {payload.contract_version !== undefined && (
        <span className={styles.row} data-row="contract">
          {t('summary.contract', {
            unit: payload.unit_id ?? '',
            version: payload.contract_version,
          })}
        </span>
      )}

      {memories.length + skills.length > 0 && (
        <span className={styles.row} data-row="inject">
          {t('summary.injected', { count: memories.length + skills.length })}
          {/* ★ 把 id 列出来：只说「注入了 3 条」的话，用户没法判断
              是不是他刚收下的那条真的被带上了。 */}
          <span className={styles.refs}>{[...memories, ...skills].join(' · ')}</span>
        </span>
      )}

      {/* ★ 收场方式**总是显示**：同样是「停了」，他自己点的停与
          AI 跑挂了是完全不同的两件事。 */}
      <span className={styles.row} data-row="outcome">
        {t(OUTCOME_KEY[payload.outcome ?? 'done'] ?? 'summary.outcome.done')}
      </span>
    </div>
  )
}
