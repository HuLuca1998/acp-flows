import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { listSkills } from '@/api/system'
import type { Skill } from '@/models/skill'

import styles from './SkillPage.module.css'

/**
 * 状态 → 词条 key。★ 显式映射，理由同角色页的 PERMISSION_KEY。
 */
const STATUS_KEY: Record<string, string> = {
  draft: 'skill.status.draft',
  active: 'skill.status.active',
  deprecated: 'skill.status.deprecated',
}

/**
 * Skill 页。设计稿标题：**「L4 Skill 库 · 版本化发布与回滚」**。
 *
 * 照 `design/INVENTORY.md` §十：每条带**版本号**、状态、
 * **校验没过时说明为什么**。
 *
 * ★ M2 只有全局库（`~/.acpflows/skills`）。项目级的在创建项目时初始化（M3），
 * 那时才有项目——所以这一页现在不做项目选择器，**不放一个选了没反应的下拉**。
 */
export function SkillPage() {
  const { t } = useTranslation()
  const [skills, setSkills] = useState<Skill[] | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    void (async () => {
      try {
        setSkills(await listSkills())
      } catch (e) {
        // ★ 扫不动要**说出来**，不装作「一个都没有」——
        // 装作没有的话，用户以为自己的 skill 丢了，而实际是目录读不了。
        setError(e instanceof Error ? e.message : t('skill.failed'))
      }
    })()
  }, [t])

  // 来源目录：设计稿在标题旁显示库的位置。用户要能照着去找。
  const source = useMemo(() => skills?.[0]?.source ?? '', [skills])

  if (error) {
    return <p className={styles.hint}>{error}</p>
  }
  if (skills === null) {
    return <p className={styles.hint}>{t('skill.loading')}</p>
  }

  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <h1 className={styles.title}>{t('skill.title')}</h1>
        <p className={styles.subtitle}>
          {t('skill.globalScope')}
          {source && <code className={styles.source}>{source}</code>}
          <span className={styles.count}>{t('skill.count', { count: skills.length })}</span>
        </p>
      </header>

      {skills.length === 0 ? (
        <p className={styles.hint}>{t('skill.empty')}</p>
      ) : (
        <ul className={styles.list}>
          {skills.map((s) => (
            <li key={s.dir} className={styles.item} data-skill={s.dir} data-status={s.status}>
              <div className={styles.row}>
                <span className={styles.name}>{s.name}</span>
                {s.version && <span className={styles.version}>v{s.version}</span>}
                <span className={styles.status} data-ok={s.validation_ok}>
                  {t(STATUS_KEY[s.status] ?? 'skill.status.draft')}
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
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
