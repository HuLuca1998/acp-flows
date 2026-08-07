import { useTranslation } from 'react-i18next'

import type { UpdatePrepareResult, UpdateStatus } from '@/models/update'
import { Button } from '@/ui/Button'

import styles from './UpdateSection.module.css'
import type { UpdatePhase } from './use-update-flow'

/**
 * 后端错误码 → i18n key 的**显式映射**。
 *
 * ★ 不要写成 t(`error.${code}`)：动态拼 key 会让静态分析查不出
 * 「词条缺了」和「词条没人用」，两个方向都失守（docs/rules/i18n.md §4）。
 * 认不出的错误码回落到一句通用文案 —— 绝不把机器码直接甩给用户看。
 */
const ERROR_MESSAGES: Record<string, string> = {
  update_check_failed: 'error.update_check_failed',
  update_not_configured: 'error.update_not_configured',
  update_not_supported: 'error.update_not_supported',
  update_prepare_failed: 'error.update_prepare_failed',
  update_install_failed: 'error.update_install_failed',
}

/**
 * 应用更新区。设计规范的更新卡片。
 *
 * ★ **四条硬约束**：
 *   1. 检查失败**绝不显示「已是最新」** —— 那会让用户永远不知道自己在用旧版
 *   2. 已是最新时**不显示更新按钮** —— 一个点不动的按钮比没有按钮更糟
 *   3. 进设置页时检查一次，**不轮询**（docs/adr/0007 修订 3）
 *   4. `blocked` 时**列出卡住的工作**并停下，绝不继续安装
 */
export type UpdateSectionProps = {
  status: UpdateStatus | null
  phase: UpdatePhase
  errorCode: string | null
  blocked: UpdatePrepareResult['blocked']
  progress: number
  onCheck: () => void
  onUpdate: () => void
}

export function UpdateSection({
  status,
  phase,
  errorCode,
  blocked,
  progress,
  onCheck,
  onUpdate,
}: UpdateSectionProps) {
  const { t } = useTranslation()
  const busy = phase === 'checking' || phase === 'preparing' || phase === 'downloading'

  return (
    <section className={styles.card} aria-labelledby="update-heading">
      <h3 id="update-heading" className={styles.heading}>
        {t('settings.update.title')}
      </h3>

      {/* 当前版本永远显示——即使检查失败，用户至少知道自己在用哪个版本 */}
      <p className={styles.current}>
        {t('settings.update.current')}
        <code className={styles.version}>{status?.current_version ?? '—'}</code>
      </p>

      {errorCode !== null && (
        <p className={styles.error} role="alert" data-testid="update-error">
          {/* 认不出的错误码回落到通用文案——绝不把机器码甩给用户看 */}
          {ERROR_MESSAGES[errorCode] === undefined
            ? t('settings.update.checkFailed')
            : t(ERROR_MESSAGES[errorCode])}
        </p>
      )}

      {/* ★ 有工作在跑：停下并把它们列出来，让用户知道要先处理什么 */}
      {phase === 'blocked' && (
        <div className={styles.blocked} role="alert" data-testid="update-blocked">
          <p className={styles.error}>{t('settings.update.blocked')}</p>
          <ul className={styles.blockedList}>
            {blocked.map((b) => (
              <li key={b.work_id}>
                <code className={styles.version}>{b.work_id}</code>
                <span className={styles.notes}>{t('settings.update.blockedReason')}</span>
              </li>
            ))}
          </ul>
        </div>
      )}

      {phase === 'downloading' && (
        <p className={styles.notes} data-testid="update-progress">
          {t('settings.update.downloading')}
          <code className={styles.version}>{progress}%</code>
        </p>
      )}

      {status?.state === 'available' && phase !== 'blocked' && (
        <div className={styles.available} data-testid="update-available">
          <p className={styles.found}>
            {t('settings.update.found')}
            <code className={styles.version}>{status.latest_version}</code>
          </p>
          {status.notes !== undefined && status.notes !== '' && (
            <p className={styles.notes}>{status.notes}</p>
          )}
          <div className={styles.actions}>
            <Button
              variant="primary"
              icon="ph-arrows-clockwise"
              onClick={onUpdate}
              disabled={busy}
            >
              {t('settings.update.applyAndRestart')}
            </Button>
            <Button variant="ghost">{t('settings.update.fullChangelog')}</Button>
            <Button variant="ghost">{t('common.action.later')}</Button>
          </div>
        </div>
      )}

      {/* Web 形态：没有 updater，给一条真的能走通的路，而不是一个死按钮 */}
      {status?.state === 'unsupported' && (
        <div className={styles.available} data-testid="update-unsupported">
          <p className={styles.notes}>{t('settings.update.webHint')}</p>
          <Button variant="secondary">{t('settings.update.openDownloads')}</Button>
        </div>
      )}

      {status?.state === 'idle' && errorCode === null && (
        <p className={styles.upToDate} data-testid="update-idle">
          {t('settings.update.upToDate')}
        </p>
      )}

      <div className={styles.actions}>
        <Button variant="secondary" onClick={onCheck} disabled={busy}>
          {phase === 'checking' ? t('settings.update.checking') : t('settings.update.check')}
        </Button>
      </div>
    </section>
  )
}
