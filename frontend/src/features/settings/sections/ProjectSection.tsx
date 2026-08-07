import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import type { Project } from '@/models/project'
import { Skeleton } from '@/ui/Skeleton'

import sectionStyles from '../Settings.module.css'

import styles from './ProjectSection.module.css'

export type ProjectSectionProps = {
  projects: Project[]
  loading: boolean
  /** 读列表失败的原因码；null 表示没失败。 */
  errorCode: string | null
  /** 能不能弹系统对话框选文件夹。浏览器里为 false。 */
  canPickDirectory: boolean
  /** path 为空表示走系统对话框；非空表示用户手填了路径。 */
  onAdd: (path?: string) => void
  onRemove: (id: string) => void
}

/**
 * 项目管理（验收点 V4）。照 `design/ACP Duet 1a.dc.html` 的「项目管理」分区。
 *
 * ★ 设计稿的表格有 worktree 与 Skill/记忆两列，那是 M2/M3 的内容——
 * 这里**留空而不是填假数字**（编造数据比空白更糟）。
 *
 * ★ 措辞上「移除」不是「删除」：用户交出的是自己的代码目录，
 * 写「删除」会让他以为文件没了。后端也确实只取消登记。
 */
export function ProjectSection({
  projects,
  loading,
  errorCode,
  canPickDirectory,
  onAdd,
  onRemove,
}: ProjectSectionProps) {
  const { t } = useTranslation()
  const [manualPath, setManualPath] = useState('')

  return (
    <section className={sectionStyles.section}>
      <div className={styles.head}>
        <h3 className={sectionStyles.heading}>{t('settings.projects.title')}</h3>
        <div className={styles.spacer} />
        {canPickDirectory && (
          <button type="button" className={styles.addButton} onClick={() => onAdd()}>
            {t('settings.projects.add')}
          </button>
        )}
      </div>

      {/* 浏览器里拿不到绝对路径（showDirectoryPicker 只给句柄），
          所以降级成手填——而不是给一个点了没反应的按钮。 */}
      {!canPickDirectory && (
        <form
          className={styles.manual}
          onSubmit={(e) => {
            e.preventDefault()
            const trimmed = manualPath.trim()
            if (trimmed !== '') {
              onAdd(trimmed)
              setManualPath('')
            }
          }}
        >
          <input
            type="text"
            className={styles.input}
            value={manualPath}
            placeholder={t('settings.projects.pathPlaceholder')}
            aria-label={t('settings.projects.pathLabel')}
            onChange={(e) => setManualPath(e.target.value)}
          />
          <button type="submit" className={styles.addButton}>
            {t('settings.projects.addManual')}
          </button>
        </form>
      )}

      {renderBody()}
    </section>
  )

  function renderBody() {
    if (loading) {
      return <Skeleton hintKey="settings.projects.loading" rows={2} />
    }

    // 读不到列表时说读不到。退化成空列表的话，用户会以为自己加的项目丢了。
    if (errorCode !== null) {
      return <p className={styles.error}>{t('settings.projects.unavailable')}</p>
    }

    if (projects.length === 0) {
      return <p className={styles.empty}>{t('settings.projects.empty')}</p>
    }

    return (
      <div className={styles.table}>
        <div className={styles.headRow}>
          <span className={styles.colName}>{t('settings.projects.colName')}</span>
          <span className={styles.colPath}>{t('settings.projects.colPath')}</span>
          <span className={styles.colBranch}>{t('settings.projects.colBranch')}</span>
          <span className={styles.colAction}>{t('settings.projects.colAction')}</span>
        </div>

        {projects.map((p) => (
          <div key={p.id} className={styles.row} data-row>
            <span className={styles.colName}>{p.name}</span>
            <span className={`${styles.colPath} ${styles.mono}`}>{p.path}</span>
            <span className={`${styles.colBranch} ${styles.mono}`}>
              {/* 非 git 仓库没有分支。这里给的是后端下发的命令，
                  前端不按名字拼——与 Runtime 检测同一套做法。 */}
              {p.is_git_repo ? (p.default_branch ?? '') : (p.remedy?.command ?? '')}
            </span>
            <button
              type="button"
              className={styles.removeButton}
              onClick={() => onRemove(p.id)}
            >
              {t('settings.projects.remove')}
            </button>
          </div>
        ))}
      </div>
    )
  }
}
