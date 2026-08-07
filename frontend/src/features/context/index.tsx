import { useTranslation } from 'react-i18next'

import { Skeleton } from '@/ui/Skeleton'

import styles from './Context.module.css'

/**
 * 右栏（上下文面板）。设计稿里它承载：
 * 时间线事件过滤 · 权限档 · 引用文件 · 工作区状态。
 *
 * **只在对话主区出现**，其余页面是全宽内容页。
 */
export function ContextPanel({ width }: { width?: number }) {
  const { t } = useTranslation()

  return (
    <aside
      className={styles.panel}
      style={width === undefined ? undefined : { width }}
      aria-label={t('context.title')}
    >
      <header className={styles.head}>
        <span className={styles.title}>{t('context.timelineFilter')}</span>
      </header>
      <Skeleton hintKey="context.filterHint" rows={3} />

      <header className={styles.head}>
        <span className={styles.title}>{t('context.references')}</span>
      </header>
      <Skeleton hintKey="context.referencesHint" rows={2} />
    </aside>
  )
}
