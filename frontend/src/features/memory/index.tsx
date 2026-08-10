import { useCallback, useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { getMemoryBody, listMemories, reviewMemory } from '@/api/library'
import { listProjects } from '@/api/system'
import type { Memory, MemoryFilterTab } from '@/models/memory'
import { matchesTab } from '@/models/memory'
import { Dropdown } from '@/ui/Dropdown'
import { ListItem } from '@/ui/ListItem'
import { Markdown } from '@/ui/Markdown'
import { StatusText } from '@/ui/StatusText'
import { Tag } from '@/ui/Tag'

import styles from './MemoryPage.module.css'

/** 范围选择器需要的最小形状（来自 GET /v1/projects）。 */
type ProjectOption = { id: string; name: string; path: string }

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

/** 状态 → StatusText 的 tone。active 是「会被注入」的那一档。 */
const STATUS_TONE: Record<string, 'pass' | 'fail' | 'muted'> = {
  active: 'pass',
  candidate: 'muted',
  discarded: 'muted',
  invalid: 'fail',
  obsolete: 'fail',
}

const TABS: readonly MemoryFilterTab[] = ['all', 'active', 'candidate', 'retired']

/** 一条记忆的正文。 */
type MemoryBody = { title: string; text: string }

/**
 * 记忆页。设计稿标题：**「L2 项目记忆 · L3 跨项目记忆」**。
 *
 * ★★ 设计稿是**两栏**：左条目卡 + 右详情（正文 + 系统数据库记录，U10.7.1）——
 * 2026-08-10 用户裁定「与设计图纸完全偏离」之前这里只有一列索引。
 * 正文在 md 文件里（INV-MEM-8），点开一条才去读。
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
  const [selected, setSelected] = useState('')
  const [body, setBody] = useState<MemoryBody | null>(null)
  const [bodyError, setBodyError] = useState('')
  const [mdMode, setMdMode] = useState<'rendered' | 'source'>('rendered')
  const [projects, setProjects] = useState<ProjectOption[]>([])
  /** '' = 全部；`*` = 跨项目；否则是项目路径（记忆的 scope 存的就是路径）。 */
  const [scopeSel, setScopeSel] = useState('')

  useEffect(() => {
    // 项目列表拉不到不拦这一页：选择器里只剩「全部 / 跨项目」。
    void (async () => {
      try {
        setProjects(await listProjects())
      } catch {
        setProjects([])
      }
    })()
  }, [])

  const load = useCallback(async () => {
    try {
      setMemories(scopeSel ? await listMemories({ scope: scopeSel }) : await listMemories())
      setError('')
    } catch (e) {
      // ★ 查不动要**说出来**，不装作「一条都没有」——
      // 装作没有的话，用户以为 Duet 把记忆忘光了。
      setError(e instanceof Error ? e.message : t('memory.failed'))
    }
  }, [scopeSel, t])

  useEffect(() => {
    void load()
  }, [load])

  useEffect(() => {
    if (!selected) {
      return
    }
    setBody(null)
    setBodyError('')
    setMdMode('rendered')
    void (async () => {
      try {
        setBody(await getMemoryBody(selected))
      } catch (e) {
        // ★ 正文读不到时**明说**（后端的错误里带路径），不显示成空白——
        // 空白看起来像「这条记忆没内容」，而真相是 md 文件读不了。
        setBodyError(e instanceof Error ? e.message : t('memory.detailFailed'))
      }
    })()
  }, [selected, t])

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
  const current = useMemo(
    () => (memories ?? []).find((m) => m.id === selected),
    [memories, selected],
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

      {/* 选择器行：§7.6 硬约束——页头标题行下方第一行，与 Skill 页同位。 */}
      <div className={styles.toolbar}>
        <Dropdown
          label={t('memory.scopeLabel')}
          value={
            scopeSel === ''
              ? t('memory.scope.all')
              : scopeSel === '*'
                ? t('memory.scope.cross')
                : (projects.find((p) => p.path === scopeSel)?.name ?? scopeSel)
          }
          items={[
            { value: '', label: t('memory.scope.all') },
            { value: '*', label: t('memory.scope.cross') },
            ...projects.map((p) => ({ value: p.path, label: p.name })),
          ]}
          onSelect={(v) => {
            setScopeSel(v)
            setSelected('')
            setBody(null)
            setBodyError('')
          }}
        />
      </div>

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

      <div className={styles.columns}>
        {shown.length === 0 ? (
          <p className={styles.hint}>{t('memory.empty')}</p>
        ) : (
          <ul className={styles.list}>
            {shown.map((m) => (
              <li key={m.id} className={styles.item} data-memory={m.id} data-status={m.status}>
                <ListItem
                  state={m.id === selected ? 'selected' : 'default'}
                  label={m.id}
                  onSelect={() => setSelected(m.id)}
                >
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
                    {m.injectable && (
                      <span className={styles.injectable}>{t('memory.injectable')}</span>
                    )}
                  </div>

                  {/*
                    ★★ 正文**不在列表里**（INV-MEM-8）：它只存在于 md 文件里。
                    列表是索引与状态，点开一条右边才去读正文（U10.7.1）。
                  */}
                  <p className={styles.refs}>
                    {t('memory.basis')}
                    {(m.source_refs ?? []).join(' · ') || t('memory.noBasis')}
                  </p>

                  {m.confirmed_by && (
                    <p className={styles.meta}>
                      {t('memory.confirmedBy', { actor: m.confirmed_by })}
                    </p>
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
                </ListItem>
              </li>
            ))}
          </ul>
        )}

        {/* 右侧详情栏：正文 + 系统数据库记录（设计稿这一页的主体）。 */}
        <section className={styles.detail} aria-label={t('memory.detail')}>
          {!current ? (
            <p className={styles.hint}>{t('memory.detailEmpty')}</p>
          ) : (
            <>
              <header className={styles.detailHead}>
                <span className={styles.detailId}>{current.id}</span>
                <Tag tone="accent">{t(KIND_KEY[current.kind] ?? 'memory.kind.fact')}</Tag>
                <StatusText
                  value={current.status}
                  tone={STATUS_TONE[current.status] ?? 'muted'}
                />
                {current.injectable && <Tag tone="neutral">{t('memory.injectable')}</Tag>}
              </header>

              {bodyError && <p className={styles.hint}>{bodyError}</p>}
              {!body && !bodyError && <p className={styles.hint}>{t('memory.detailLoading')}</p>}
              {body && (
                <>
                  <h2 className={styles.detailTitle}>{body.title}</h2>
                  <Markdown
                    source={body.text}
                    mode={mdMode}
                    onToggleMode={() =>
                      setMdMode((m) => (m === 'rendered' ? 'source' : 'rendered'))
                    }
                  />
                </>
              )}

              {/* 「系统数据库记录 · SQLite」：值全部来自接口，不在前端拼。 */}
              <section className={styles.record}>
                <h3 className={styles.recordTitle}>{t('memory.detailRecord')}</h3>
                <dl className={styles.fields}>
                  <dt>{t('memory.field.scope')}</dt>
                  <dd>{current.scope}</dd>
                  <dt>{t('memory.field.kind')}</dt>
                  <dd>{current.kind}</dd>
                  <dt>{t('memory.field.status')}</dt>
                  <dd>{current.status}</dd>
                  <dt>{t('memory.field.refs')}</dt>
                  <dd>{(current.source_refs ?? []).join(' · ') || t('memory.noBasis')}</dd>
                  {current.created_by && (
                    <>
                      <dt>{t('memory.field.createdBy')}</dt>
                      <dd>{current.created_by}</dd>
                    </>
                  )}
                  {current.confirmed_by && (
                    <>
                      <dt>{t('memory.field.confirmedBy')}</dt>
                      <dd>{t('memory.confirmedBy', { actor: current.confirmed_by })}</dd>
                    </>
                  )}
                  {current.reason && (
                    <>
                      <dt>{t('memory.field.reason')}</dt>
                      <dd>{current.reason}</dd>
                    </>
                  )}
                  {current.supersedes && (
                    <>
                      <dt>{t('memory.field.supersedes')}</dt>
                      <dd>{current.supersedes}</dd>
                    </>
                  )}
                </dl>
              </section>

              {/* 候选在详情里也能当场拍板（R3），两个按钮都不预选中。 */}
              {current.status === 'candidate' && (
                <div className={styles.actions}>
                  <button
                    type="button"
                    className={styles.confirm}
                    disabled={busy === current.id}
                    onClick={() => void review(current.id, 'confirm')}
                  >
                    {t('memory.confirm')}
                  </button>
                  <button
                    type="button"
                    className={styles.reject}
                    disabled={busy === current.id}
                    onClick={() => void review(current.id, 'reject')}
                  >
                    {t('memory.reject')}
                  </button>
                </div>
              )}
            </>
          )}
        </section>
      </div>

      <p className={styles.footnote}>{t('memory.footnote')}</p>
    </div>
  )
}
