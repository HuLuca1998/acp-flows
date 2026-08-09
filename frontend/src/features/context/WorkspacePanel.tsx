import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { getWorkWorktree } from '@/api/system'
import type { WorktreeState } from '@/models/worktree'
import { totalOf } from '@/models/worktree'

import styles from './WorkspacePanel.module.css'

export type WorkspacePanelProps = {
  /** 当前打开的工作 id；为空表示还没有工作。 */
  workID: string
}

/**
 * 右栏「工作区」。设计稿 §四。
 *
 * ★★ 这一栏回答的是**「AI 到底动了什么」**——
 * 分支、领先几个 commit、未提交的每个文件改了多少行、本次工作的提交。
 *
 * ★ 2026-08-09 之前这里是「时间线过滤器 + 引用」两块占位，
 * 而设计稿里过滤器属于**计划面板那一侧**——完全不是一回事。
 */
export function WorkspacePanel({ workID }: WorkspacePanelProps) {
  const { t } = useTranslation()
  const [state, setState] = useState<WorktreeState | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!workID) {
      setState(null)
      setError('')
      return
    }
    void (async () => {
      try {
        setState(await getWorkWorktree(workID))
        setError('')
      } catch (e) {
        // ★ 读不到要说出来，不装作「什么都没改」——
        // 装作没改的话，用户会以为 AI 一事无成。
        setState(null)
        setError(e instanceof Error ? e.message : t('workspace.failed'))
      }
    })()
  }, [workID, t])

  if (!workID) {
    return <p className={styles.hint}>{t('workspace.noWork')}</p>
  }
  if (error) {
    return <p className={styles.hint}>{error}</p>
  }
  if (state === null) {
    return <p className={styles.hint}>{t('workspace.loading')}</p>
  }
  if (!state.branch) {
    // ★ 还没切 worktree（initializing / initializing_failed）——
    // 这不是错误，明说「还没有工作区」而不是一片空白。
    return <p className={styles.hint}>{t('workspace.notReady')}</p>
  }

  const total = totalOf(state.changes)

  return (
    <div className={styles.panel}>
      <header className={styles.head}>
        <span className={styles.branch}>{state.branch}</span>
        {/*
          ★ 没有基线时**不显示「领先 0」**：那会让用户以为 AI 什么都没干，
          而实际是我们不知道起点。
        */}
        {state.base_commit && (
          <span className={styles.base}>
            {t('workspace.ahead', { base: state.base_commit, n: state.ahead })}
          </span>
        )}
      </header>

      <section className={styles.block}>
        <h3 className={styles.blockTitle}>
          {t('workspace.uncommitted', { count: state.changes.length })}
        </h3>
        {state.changes.length === 0 ? (
          <p className={styles.hint}>{t('workspace.noChanges')}</p>
        ) : (
          <>
            <ul className={styles.list}>
              {state.changes.map((c) => (
                <li key={c.path} className={styles.change} data-file={c.path}>
                  <code className={styles.path}>{c.path}</code>
                  {/*
                    ★★ 逐个文件带增删行数（设计稿的 `+64 −12`）。
                    只说「改了 3 个文件」的话，用户判断不出这次改动有多大。
                  */}
                  <span className={styles.added}>+{c.added}</span>
                  <span className={styles.removed}>−{c.removed}</span>
                </li>
              ))}
            </ul>
            <p className={styles.total}>
              {t('workspace.total', { added: total.added, removed: total.removed })}
            </p>
          </>
        )}
      </section>

      <section className={styles.block}>
        <h3 className={styles.blockTitle}>
          {t('workspace.commits', { count: state.commits.length })}
        </h3>
        {state.commits.length === 0 ? (
          <p className={styles.hint}>{t('workspace.noCommits')}</p>
        ) : (
          <ul className={styles.list}>
            {state.commits.map((c) => (
              <li key={c.sha} className={styles.commit} data-commit={c.sha}>
                <div className={styles.commitHead}>
                  <code className={styles.sha}>{c.sha}</code>
                  <span className={styles.when}>{c.when}</span>
                  {/* ★ 「仅本地」是设计稿原文：提醒用户这些还没推上去 */}
                  <span className={styles.localOnly}>{t('workspace.localOnly')}</span>
                </div>
                <div className={styles.subject}>{c.subject}</div>
              </li>
            ))}
          </ul>
        )}
      </section>

      {/* ★★ 设计稿原文。push 属于 D3，要用户逐次授权——这句话必须在 */}
      <p className={styles.note}>{t('workspace.pushNote')}</p>
    </div>
  )
}
