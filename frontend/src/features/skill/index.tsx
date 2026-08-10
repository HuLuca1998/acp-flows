import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { getSkillBody, listSkills } from '@/api/library'
import { listProjects } from '@/api/system'
import type { Skill } from '@/models/skill'
import { Dropdown } from '@/ui/Dropdown'
import { ListItem } from '@/ui/ListItem'
import { Markdown } from '@/ui/Markdown'
import { StatusText } from '@/ui/StatusText'
import { Tag } from '@/ui/Tag'

import styles from './SkillPage.module.css'

/** 项目选择器需要的最小形状（来自 GET /v1/projects）。 */
type ProjectOption = { id: string; name: string; path: string }

/**
 * 状态 → 词条 key。★ 显式映射，理由同角色页的 PERMISSION_KEY。
 */
const STATUS_KEY: Record<string, string> = {
  draft: 'skill.status.draft',
  active: 'skill.status.active',
  deprecated: 'skill.status.deprecated',
}

/** 状态 → StatusText 的 tone。active 是「能被注入」的那一档。 */
const STATUS_TONE: Record<string, 'pass' | 'fail' | 'muted'> = {
  draft: 'muted',
  active: 'pass',
  deprecated: 'fail',
}

/** 一个 Skill 的正文。 */
type SkillBody = { dir: string; frontmatter: string; text: string }

/**
 * Skill 页。设计稿标题：**「L4 Skill 库 · 版本化复用与回滚」**。
 *
 * ★★ 设计稿是**两栏**：左列表 + 右侧 SKILL.md 详情（U10.7.2）——
 * 2026-08-10 用户裁定「与设计图纸完全偏离」之前这里只有左边那栏。
 * 详情的正文**从磁盘现读**（用户随时会用编辑器改它）。
 *
 * ★ 页头有项目选择器（§7.6）：「全局」扫 `~/.acpflows/skills`，
 * 选项目则扫那个项目的约定目录——选了就真的按项目拉取，不是装样子。
 */
export function SkillPage() {
  const { t } = useTranslation()
  const [skills, setSkills] = useState<Skill[] | null>(null)
  const [error, setError] = useState('')
  const [selected, setSelected] = useState('')
  const [body, setBody] = useState<SkillBody | null>(null)
  const [bodyError, setBodyError] = useState('')
  const [mdMode, setMdMode] = useState<'rendered' | 'source'>('rendered')
  const [projects, setProjects] = useState<ProjectOption[]>([])
  /** '' = 全局；否则是项目路径（记忆的 scope 用的也是路径）。 */
  const [projectSel, setProjectSel] = useState('')

  useEffect(() => {
    // 项目列表拉不到不拦这一页：选择器里只剩「全局」，Skill 库照常显示。
    void (async () => {
      try {
        setProjects(await listProjects())
      } catch {
        setProjects([])
      }
    })()
  }, [])

  useEffect(() => {
    setSkills(null)
    setSelected('')
    setBody(null)
    setBodyError('')
    void (async () => {
      try {
        setSkills(projectSel ? await listSkills({ project: projectSel }) : await listSkills())
        setError('')
      } catch (e) {
        // ★ 扫不动要**说出来**，不装作「一个都没有」——
        // 装作没有的话，用户以为自己的 skill 丢了，而实际是目录读不了。
        setError(e instanceof Error ? e.message : t('skill.failed'))
      }
    })()
  }, [projectSel, t])

  useEffect(() => {
    if (!selected) {
      return
    }
    setBody(null)
    setBodyError('')
    setMdMode('rendered')
    void (async () => {
      try {
        setBody(await getSkillBody(selected))
      } catch (e) {
        // ★ 正文读不到时**明说**（后端的错误里带路径），不显示成空白——
        // 空白看起来像「这个 Skill 没写内容」，而真相是文件读不了。
        setBodyError(e instanceof Error ? e.message : t('skill.detailFailed'))
      }
    })()
  }, [selected, t])

  // 来源目录：设计稿在标题旁显示库的位置。用户要能照着去找。
  const source = useMemo(() => skills?.[0]?.source ?? '', [skills])
  const current = useMemo(() => skills?.find((s) => s.dir === selected), [skills, selected])

  if (error) {
    return <p className={styles.hint}>{error}</p>
  }
  if (skills === null) {
    return <p className={styles.hint}>{t('skill.loading')}</p>
  }

  const currentScopeLabel = projectSel
    ? (projects.find((p) => p.path === projectSel)?.name ?? projectSel)
    : t('skill.globalScope')

  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <p className={styles.eyebrow}>{t('skill.eyebrow')}</p>
        <h1 className={styles.title}>{t('skill.title')}</h1>
      </header>

      {/* 选择器行：§7.6 硬约束——页头标题行下方第一行，与记忆页同位。 */}
      <div className={styles.toolbar}>
        <Dropdown
          label={t('skill.projectLabel')}
          value={currentScopeLabel}
          items={[
            { value: '', label: t('skill.globalScope') },
            ...projects.map((p) => ({ value: p.path, label: p.name })),
          ]}
          onSelect={setProjectSel}
        />
        <p className={styles.subtitle}>
          {source && <code className={styles.source}>{source}</code>}
          <span className={styles.count}>{t('skill.count', { count: skills.length })}</span>
        </p>
      </div>

      {skills.length === 0 ? (
        <p className={styles.hint}>{t('skill.empty')}</p>
      ) : (
        <div className={styles.columns}>
          <ul className={styles.list}>
            {skills.map((s) => (
              <li key={s.dir} data-skill={s.dir} data-status={s.status}>
                <ListItem
                  state={s.dir === selected ? 'selected' : 'default'}
                  label={s.name}
                  onSelect={() => setSelected(s.dir)}
                >
                  <div className={styles.row}>
                    <span className={styles.name}>{s.name}</span>
                    {s.version && <Tag tone="accent">v{s.version}</Tag>}
                    {/*
                      ★★ 命中计数：这个 Skill 被注入过几次。**0 也要显示**——
                      空白会让用户以为这个数字坏了，而「从没被用过」正是他
                      判断「该不该留着它」最需要的一条信息。
                    */}
                    <span className={styles.hits} data-hits={s.hit_count ?? 0}>
                      {t('skill.hits', { count: s.hit_count ?? 0 })}
                    </span>
                    <span className={styles.status} data-ok={s.validation_ok}>
                      <StatusText
                        value={t(STATUS_KEY[s.status] ?? 'skill.status.draft')}
                        tone={s.validation_ok ? (STATUS_TONE[s.status] ?? 'muted') : 'fail'}
                      />
                    </span>
                  </div>

                  {s.description && <p className={styles.desc}>{s.description}</p>}

                  {/*
                    ★★ 校验没过时**必须说清为什么**（INV-SKL-2）。
                    只显示一个 draft 标签的话，用户唯一能做的事是删了重建——
                    而重建出来还是 draft。
                  */}
                  {!s.validation_ok && s.validation_reason && (
                    <p className={styles.reason}>{s.validation_reason}</p>
                  )}

                  {s.compatibility && (
                    <p className={styles.meta}>
                      {t('skill.compatibility')}
                      <code>{s.compatibility}</code>
                    </p>
                  )}
                </ListItem>
              </li>
            ))}
          </ul>

          {/* 右侧详情栏：SKILL.md 的 frontmatter 与正文（设计稿这一页的主体）。 */}
          <section className={styles.detail} aria-label={t('skill.detail')}>
            {!selected ? (
              <p className={styles.hint}>{t('skill.detailEmpty')}</p>
            ) : (
              <>
                <header className={styles.detailHead}>
                  <span className={styles.detailName}>{current?.name ?? selected}</span>
                  {current?.version && <Tag tone="accent">v{current.version}</Tag>}
                  {current && (
                    <StatusText
                      value={t(STATUS_KEY[current.status] ?? 'skill.status.draft')}
                      tone={current.validation_ok ? (STATUS_TONE[current.status] ?? 'muted') : 'fail'}
                    />
                  )}
                </header>
                {source && (
                  <code className={styles.detailPath}>
                    {source}/{selected}/SKILL.md
                  </code>
                )}

                {/* 校验没过的那条：详情里也要说清为什么（R3）。 */}
                {current && !current.validation_ok && current.validation_reason && (
                  <p className={styles.reason}>{current.validation_reason}</p>
                )}

                {bodyError && <p className={styles.hint}>{bodyError}</p>}
                {!body && !bodyError && <p className={styles.hint}>{t('skill.detailLoading')}</p>}
                {body && (
                  <>
                    {/* frontmatter 原样显示——解析失败时用户要看得到自己写了什么。 */}
                    {body.frontmatter && (
                      <pre className={styles.frontmatter}>{body.frontmatter}</pre>
                    )}
                    <Markdown
                      source={body.text}
                      mode={mdMode}
                      onToggleMode={() => setMdMode((m) => (m === 'rendered' ? 'source' : 'rendered'))}
                    />
                  </>
                )}
              </>
            )}
          </section>
        </div>
      )}
    </div>
  )
}
