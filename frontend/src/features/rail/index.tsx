import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { listRuntimes } from '@/api/system'
import { NAV_PAGES, type PageId } from '@/app/pages'
import type { Runtime } from '@/models/runtime'
import { Skeleton } from '@/ui/Skeleton'

import { ProjectTree } from './ProjectTree'
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
  /** 折叠后**保留 48px 图标条**，不是整条消失（设计规范 §06 规则②）。 */
  collapsed?: boolean
  /** 当前宽度（px）。折叠时忽略，走固定的图标条宽度。 */
  width?: number
  /** 在某个项目下开一个新对话。 */
  onNewWork: (projectPath: string) => void
  /** 打开一条已有的工作。 */
  onOpenWork: (workID: string) => void
}

/**
 * Runtime 状态 → 词条 key 的**显式映射**。
 *
 * ★ 不许写成 `rail.runtimeState.${status}`：动态拼接之后静态分析查不出
 * 词条缺失，删掉一条也没有任何检查会红（docs/rules/i18n.md §4）。
 *
 * ★ 认不出的状态用兜底文案，**不把原始码显示给用户**——
 * `not_authenticated` 这种字符串对他没有意义。
 */
const RUNTIME_STATE_KEY: Record<string, string> = {
  ready: 'rail.runtimeState.ready',
  not_installed: 'rail.runtimeState.not_installed',
  not_authenticated: 'rail.runtimeState.not_authenticated',
  probe_failed: 'rail.runtimeState.probe_failed',
}

function runtimeStateKey(status: string | undefined): string {
  return (status === undefined ? undefined : RUNTIME_STATE_KEY[status]) ?? 'rail.runtimeState.unknown'
}

export function Rail({
  currentPage,
  onNavigate,
  collapsed = false,
  width,
  onNewWork,
  onOpenWork,
}: RailProps) {
  const { t } = useTranslation()
  const [runtimes, setRuntimes] = useState<Runtime[] | null>(null)
  const [probeFailed, setProbeFailed] = useState(false)

  useEffect(() => {
    void (async () => {
      try {
        setRuntimes(await listRuntimes())
      } catch {
        // ★ 查不到**不能让左栏整个白掉**。后端没起来时用户最需要的正是这条
        // 左栏——他要点进设置页看看怎么回事，而抛异常会让整棵组件树塌掉。
        setProbeFailed(true)
        setRuntimes([])
      }
    })()
  }, [])

  return (
    <aside
      className={collapsed ? styles.railCollapsed : styles.rail}
      style={collapsed || width === undefined ? undefined : { width }}
    >
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
            {!collapsed && <span>{t(p.titleKey)}</span>}
          </button>
        ))}
      </nav>

      {!collapsed && (
        <section className={styles.section}>
          <header className={styles.sectionHead}>
            <span className={styles.sectionTitle}>{t('rail.projects')}</span>
            {/* ★ 「创建项目」不再是死按钮：它跳到设置页的项目管理。
                之前 disabled 的那个，用户点了毫无反应，
                而他不知道是坏了还是没做。 */}
            <button
              type="button"
              className={styles.sectionAction}
              onClick={() => onNavigate('settings')}
            >
              {t('rail.createProject')}
            </button>
          </header>
          <ProjectTree onNewWork={onNewWork} onOpenWork={onOpenWork} />
        </section>
      )}

      {/* ★ 「最近」是设计稿里的一块（跨项目的最近打开过）。
          现在还没有「打开过」这个记录，所以仍是骨架——但**不能删掉它**：
          删了的话下一个人不知道这里欠着东西。归 U5.1.1。 */}
      {!collapsed && (
        <section className={styles.section}>
          <span className={styles.sectionTitle}>{t('rail.recent')}</span>
          <Skeleton hintKey="rail.recentHint" rows={2} />
        </section>
      )}

      {/* 折叠成 48px 图标条时不显示——硬塞会溢出 */}
      {!collapsed && (
        <footer className={styles.runtimeBar}>
          <span className={styles.runtimeLabel}>{t('rail.runtime')}</span>
          {renderRuntimes()}
        </footer>
      )}
    </aside>
  )

  function renderRuntimes() {
    // 还没查回来：留空而不是先说「没有」——闪一下「没有」再变成两个，
    // 比慢半拍更让人怀疑自己看错了
    if (runtimes === null) {
      return <span className={styles.runtimeHint}>{t('rail.runtimeProbing')}</span>
    }
    if (runtimes.length === 0) {
      return (
        <span className={styles.runtimeHint}>
          {t(probeFailed ? 'rail.runtimeProbeFailed' : 'rail.runtimeNone')}
        </span>
      )
    }

    return runtimes.map((r) => (
      // ★ data-status 既给样式用，也给测试用：ready 与 not_authenticated
      // 对用户是两件事——后者意味着他得去登录，界面不说他就一直等
      <span key={r.name} className={styles.runtimeRow} data-runtime={r.name} data-status={r.status}>
        <span className={styles.runtimeDot} aria-hidden="true" />
        <span className={styles.runtimeName}>
          {r.name}
          {r.active_version !== undefined && r.active_version !== '' && ` ${r.active_version}`}
        </span>
        <span className={styles.runtimeState}>{t(runtimeStateKey(r.status))}</span>
      </span>
    ))
  }
}
