import { useTranslation } from 'react-i18next'

import styles from './Context.module.css'
import { WorkspacePanel } from './WorkspacePanel'


export type ContextPanelProps = {
  width?: number
  /** 当前打开的工作 id；为空表示还没有工作。 */
  workID?: string
}

/**
 * 右栏。设计稿 §四 的标题就是**「工作区」**。
 *
 * ★★ 2026-08-09 之前这里是「时间线过滤器 + 引用」两块占位——
 * 而设计稿里过滤器属于**计划面板那一侧**，完全不是一回事。
 * 现在换成设计稿要的东西：AI 到底动了什么。
 *
 * **只在对话主区出现**，其余页面是全宽内容页。
 */
export function ContextPanel({ width, workID = '' }: ContextPanelProps) {
  const { t } = useTranslation()

  return (
    <aside
      className={styles.panel}
      style={width === undefined ? undefined : { width }}
      aria-label={t('workspace.title')}
    >
      <header className={styles.head}>
        <span className={styles.title}>{t('workspace.title')}</span>
      </header>
      <WorkspacePanel workID={workID} />
    </aside>
  )
}
