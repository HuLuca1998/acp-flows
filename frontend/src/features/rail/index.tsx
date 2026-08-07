import { useTranslation } from 'react-i18next'

import { NAV_PAGES, type PageId } from '@/app/pages'
import { Skeleton } from '@/ui/Skeleton'

import styles from './Rail.module.css'

/**
 * 左栏。结构照设计稿：**5 项导航 → 项目树 → 最近 → Runtime 状态**。
 *
 * ★ 左栏回答的是「你在哪个项目的哪个工作里」，不是「你要去哪个功能页」。
 * 项目树是它的主体，导航只是顶部一小条。
 */
export type RailProps = {
  currentPage: PageId
  onNavigate: (id: PageId) => void
}

export function Rail({ currentPage, onNavigate }: RailProps) {
  const { t } = useTranslation()

  return (
    <aside className={styles.rail}>
      <nav className={styles.nav} aria-label={t('nav.label')}>
        {NAV_PAGES.map((p) => (
          <button
            key={p.id}
            type="button"
            className={p.id === currentPage ? styles.navItemActive : styles.navItem}
            aria-current={p.id === currentPage ? 'page' : undefined}
            onClick={() => onNavigate(p.id)}
          >
            <i className={`ph ${p.icon}`} aria-hidden="true" />
            <span>{t(p.titleKey)}</span>
          </button>
        ))}
      </nav>

      <section className={styles.section}>
        <header className={styles.sectionHead}>
          <span className={styles.sectionTitle}>{t('rail.projects')}</span>
          <button type="button" className={styles.sectionAction} disabled>
            {t('rail.createProject')}
          </button>
        </header>
        {/* 还没有项目管理功能——**不编造项目列表**，用骨架说明这里将来是什么 */}
        <Skeleton hintKey="rail.projectsHint" rows={2} />
      </section>

      <section className={styles.section}>
        <span className={styles.sectionTitle}>{t('rail.recent')}</span>
        <Skeleton hintKey="rail.recentHint" rows={2} />
      </section>

      <footer className={styles.runtimeBar}>
        <span className={styles.runtimeLabel}>{t('rail.runtime')}</span>
        <span className={styles.runtimeHint}>{t('rail.runtimeUnknown')}</span>
      </footer>
    </aside>
  )
}
