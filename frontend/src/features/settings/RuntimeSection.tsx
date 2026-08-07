import { useTranslation } from 'react-i18next'

import type { Runtime, RuntimeStatus } from '@/models/runtime'
import { Skeleton } from '@/ui/Skeleton'

import styles from './RuntimeSection.module.css'
import sectionStyles from './Settings.module.css'

/**
 * 状态 → 词条 key 的显式映射。
 *
 * ★ **不许写成 `t(`settings.runtime.state.${status}`)`。** 动态拼接之后：
 * 静态分析查不出词条缺失，删掉一条也没有任何检查会红（docs/rules/i18n.md §4）。
 * 写成 Record 还有个额外好处——加第五种状态时 TypeScript 直接报错。
 */
const STATE_KEY: Record<RuntimeStatus, string> = {
  ready: 'settings.runtime.state.ready',
  not_installed: 'settings.runtime.state.not_installed',
  not_authenticated: 'settings.runtime.state.not_authenticated',
  probe_failed: 'settings.runtime.state.probe_failed',
}

export type RuntimeSectionProps = {
  runtimes: Runtime[]
  loading: boolean
  /** 整体检测失败的原因码；null 表示没失败。 */
  errorCode: string | null
  onRetry: () => void
}

/**
 * ACP Runtime 检测区（验收点 V3）。
 *
 * ★ 这一块最容易犯的错是**把不确定说成确定**：
 * 检测失败时显示「未安装」，用户就去装已经装好的东西，装完还是不行。
 * 所以 `probe_failed` 有自己的文案，且**不给修复命令**——
 * 那时我们并不知道要修什么。
 *
 * 每一行的判断都基于 `status`，**没有一处看 `name`**：
 * 修复命令由后端随响应给出（openapi 的 `Runtime.remedy`），
 * 加第三个 Runtime 不需要改这个文件。
 */
export function RuntimeSection({ runtimes, loading, errorCode, onRetry }: RuntimeSectionProps) {
  const { t } = useTranslation()

  return (
    <section className={sectionStyles.section}>
      <h3 className={sectionStyles.heading}>{t('settings.runtime.title')}</h3>
      {renderBody()}
    </section>
  )

  function renderBody() {
    if (loading) {
      return <Skeleton hintKey="settings.runtime.checking" rows={2} />
    }

    // 整体检测不可用：显示成错误 + 重试。**绝不退化成空列表**——
    // 那会让「检测不了」看起来像「一个都没装」。
    if (errorCode !== null) {
      return (
        <div className={styles.error}>
          <p className={styles.errorText}>{t('settings.runtime.unavailable')}</p>
          <button type="button" className={styles.retry} onClick={onRetry}>
            {t('settings.runtime.retry')}
          </button>
        </div>
      )
    }

    if (runtimes.length === 0) {
      return <p className={styles.empty}>{t('settings.runtime.none')}</p>
    }

    return (
      <div className={styles.list}>
        {runtimes.map((runtime) => (
          <RuntimeCard key={runtime.name} runtime={runtime} />
        ))}
      </div>
    )
  }
}

function RuntimeCard({ runtime }: { runtime: Runtime }) {
  const { t } = useTranslation()
  const status = runtime.status ?? 'probe_failed'
  const isReady = status === 'ready'
  const command = runtime.remedy?.command

  return (
    <div className={styles.card}>
      <div className={styles.head}>
        <span className={`${styles.dot} ${isReady ? styles.dotReady : ''}`} />
        <span className={styles.name}>{runtime.name}</span>
        {runtime.active_version !== undefined && (
          <span className={styles.version}>{runtime.active_version}</span>
        )}
        <div className={styles.spacer} />
        <span className={`${styles.state} ${isReady ? styles.stateReady : ''}`}>
          {t(STATE_KEY[status])}
        </span>
      </div>

      {runtime.path !== undefined && <p className={styles.path}>{runtime.path}</p>}

      {/* 命令由后端给。没有命令就不显示这一块——probe_failed 正是这种情况：
          我们不知道要修什么，给一条瞎猜的命令比不给更糟。 */}
      {command !== undefined && command !== '' && (
        <div className={styles.remedy}>
          <p className={styles.remedyHint}>{t('settings.runtime.remedyHint')}</p>
          <p className={styles.command}>{command}</p>
        </div>
      )}
    </div>
  )
}
