import { useTranslation } from 'react-i18next'

import type { UpdateStatus } from '@/models/update'
import { Button } from '@/ui/Button'

import styles from './UpdateSection.module.css'

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
}

/**
 * 应用更新区。设计规范的更新卡片。
 *
 * ★ **三条硬约束**：
 *   1. 检查失败**绝不显示「已是最新」** —— 那会让用户永远不知道自己在用旧版
 *   2. 已是最新时**不显示更新按钮** —— 一个点不动的按钮比没有按钮更糟
 *   3. 进设置页时检查一次，**不轮询**（docs/adr/0007 修订 3）
 */
export type UpdateSectionProps = {
  status: UpdateStatus | null
  /** 检查中。null status + loading 才是「正在查」，null status + 非 loading 是「还没查」。 */
  loading: boolean
  /** 检查失败的原因码（机器可读），null 表示没失败。 */
  errorCode: string | null
  onCheck: () => void
  onUpdate: () => void
}

export function UpdateSection({
  status,
  loading,
  errorCode,
  onCheck,
  onUpdate,
}: UpdateSectionProps) {
  const { t } = useTranslation()

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

      {status?.state === 'available' && (
        <div className={styles.available} data-testid="update-available">
          <p className={styles.found}>
            {t('settings.update.found')}
            <code className={styles.version}>{status.latest_version}</code>
          </p>
          {status.notes !== undefined && status.notes !== '' && (
            <p className={styles.notes}>{status.notes}</p>
          )}
          <div className={styles.actions}>
            <Button variant="primary" icon="ph-arrows-clockwise" onClick={onUpdate}>
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
        <Button variant="secondary" onClick={onCheck} disabled={loading}>
          {loading ? t('settings.update.checking') : t('settings.update.check')}
        </Button>
      </div>
    </section>
  )
}
