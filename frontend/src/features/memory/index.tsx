import { useCallback, useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { listMemories, reviewMemory } from '@/api/system'
import type { Memory, MemoryFilterTab } from '@/models/memory'
import { matchesTab } from '@/models/memory'

import styles from './MemoryPage.module.css'

/** 筛选档 → 词条 key。★ 显式映射，不动态拼 key。 */
const TAB_KEY: Record<MemoryFilterTab, string> = {
  all: 'memory.tab.all',
  active: 'memory.tab.active',
  candidate: 'memory.tab.candidate',
  retired: 'memory.tab.retired',
}

/** 类型 → 词条 key。 */
const KIND_KEY: Record<string, string> = {
  constraint: 'memory.kind.constraint',
  experience: 'memory.kind.experience',
  fact: 'memory.kind.fact',
}

const TABS: readonly MemoryFilterTab[] = ['all', 'active', 'candidate', 'retired']

/**
 * 记忆页。设计稿标题：**「L2 项目记忆 · L3 跨项目记忆」**。
 *
 * 照 `design/INVENTORY.md` §九：状态筛选四档 + 每条带类型与状态。
 *
 * ★★ **候选那一档是这一页的重点**：AI 提的东西在那儿等人拍板，
 * 而在用户点之前它**不会**进任何注入清单（INV-MEM-2）。
 */
export function MemoryPage() {
  const { t } = useTranslation()
  const [memories, setMemories] = useState<Memory[] | null>(null)
  const [error, setError] = useState('')
  const [tab, setTab] = useState<MemoryFilterTab>('all')
  const [busy, setBusy] = useState('')

  const load = useCallback(async () => {
    try {
      setMemories(await listMemories())
      setError('')
    } catch (e) {
      // ★ 查不动要**说出来**，不装作「一条都没有」——
      // 装作没有的话，用户以为 Duet 把记忆忘光了。
      setError(e instanceof Error ? e.message : t('memory.failed'))
    }
  }, [t])

  useEffect(() => {
    void load()
  }, [load])

  const counts = useMemo(() => {
    const all = memories ?? []
    return Object.fromEntries(
      TABS.map((k) => [k, all.filter((m) => matchesTab(m, k)).length]),
    ) as Record<MemoryFilterTab, number>
  }, [memories])

  const shown = useMemo(
    () => (memories ?? []).filter((m) => matchesTab(m, tab)),
    [memories, tab],
  )

  const review = useCallback(
    async (id: string, decision: 'confirm' | 'reject') => {
      setBusy(id)
      try {
        // ★★ actor 必填（INV-MEM-2）。这里是**用户点的那一下**——
        // 不带 actor 的话后端会拒绝，而那正是我们要的：
        // 没有任何路径能让候选自己变成生效。
        await reviewMemory(id, decision, 'user')
        await load()
      } catch (e) {
        setError(e instanceof Error ? e.message : t('memory.reviewFailed'))
      } finally {
        setBusy('')
      }
    },
    [load, t],
  )

  if (error && memories === null) {
    return <p className={styles.hint}>{error}</p>
  }
  if (memories === null) {
    return <p className={styles.hint}>{t('memory.loading')}</p>
  }

  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <p className={styles.eyebrow}>{t('memory.eyebrow')}</p>
        <h1 className={styles.title}>{t('memory.title')}</h1>
      </header>

      <div className={styles.tabs} role="tablist">
        {TABS.map((k) => (
          <button
            key={k}
            type="button"
            role="tab"
            aria-selected={tab === k}
            className={styles.tab}
            data-active={tab === k}
            onClick={() => setTab(k)}
          >
            {t(TAB_KEY[k])}
            <span className={styles.tabCount}>{counts[k]}</span>
          </button>
        ))}
      </div>

      {error && <p className={styles.error}>{error}</p>}

      {shown.length === 0 ? (
        <p className={styles.hint}>{t('memory.empty')}</p>
      ) : (
        <ul className={styles.list}>
          {shown.map((m) => (
            <li key={m.id} className={styles.item} data-memory={m.id} data-status={m.status}>
              <div className={styles.row}>
                <span className={styles.id}>{m.id}</span>
                <span className={styles.kind}>{t(KIND_KEY[m.kind] ?? 'memory.kind.fact')}</span>
                <span className={styles.status} data-status={m.status}>
                  {m.status}
                </span>
                {/*
                  ★ 「能不能被注入」要直接标出来。用户看这一页就是想知道
                  「AI 下一轮会带着哪些规矩干活」——只显示状态词的话，
                  他得自己记住哪几个状态算数。
                */}
                {m.injectable && <span className={styles.injectable}>{t('memory.injectable')}</span>}
              </div>

              {/*
                ★★ 正文**不在这里**（INV-MEM-8）：它只存在于 md 文件里。
                这一页显示的是索引与状态。正文渲染归 U10.1.1。
              */}
              <p className={styles.refs}>
                {t('memory.basis')}
                {(m.source_refs ?? []).join(' · ') || t('memory.noBasis')}
              </p>

              {m.confirmed_by && (
                <p className={styles.meta}>{t('memory.confirmedBy', { actor: m.confirmed_by })}</p>
              )}
              {m.reason && <p className={styles.meta}>{m.reason}</p>}

              {/*
                ★★ 候选**必须由人拍板**。这两个按钮就是 INV-MEM-2 里
                那个「用户确认动作」——没有它们，候选永远不会生效。
              */}
              {m.status === 'candidate' && (
                <div className={styles.actions}>
                  <button
                    type="button"
                    className={styles.confirm}
                    disabled={busy === m.id}
                    onClick={() => void review(m.id, 'confirm')}
                  >
                    {t('memory.confirm')}
                  </button>
                  <button
                    type="button"
                    className={styles.reject}
                    disabled={busy === m.id}
                    onClick={() => void review(m.id, 'reject')}
                  >
                    {t('memory.reject')}
                  </button>
                </div>
              )}
            </li>
          ))}
        </ul>
      )}

      <p className={styles.footnote}>{t('memory.footnote')}</p>
    </div>
  )
}
