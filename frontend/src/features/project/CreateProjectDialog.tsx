import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { addProject, previewProject } from '@/api/system'
import type { ProjectPreview } from '@/models/preview'
import { isAppend } from '@/models/preview'
import { capabilities, pickDirectory } from '@/platform'

import styles from './CreateProjectDialog.module.css'

/** `gh` 状态 → 词条 key。★ 显式映射，不动态拼。 */
const GH_KEY: Record<string, string> = {
  ready: 'project.gh.ready',
  not_installed: 'project.gh.not_installed',
  not_authenticated: 'project.gh.not_authenticated',
  probe_failed: 'project.gh.probe_failed',
}

export type CreateProjectDialogProps = {
  open: boolean
  onClose: () => void
  /** 创建成功后回调，参数是新项目的 id。 */
  onCreated: (projectID: string) => void
}

/**
 * 创建项目对话框。设计稿标题：
 * **「添加本地仓库 → 初始化 .acpflows → 导入已有 skills」**。
 *
 * ★★ **这个对话框的全部意义是「先说再做」。**
 *
 * 用户交出来的是他自己的代码仓库。所以在他点「创建项目」之前，
 * 这里要把 Duet 将要动的每一样东西都列出来——**包括为什么**。
 * 内容全部来自后端的预演，前端一个字都不编。
 */
export function CreateProjectDialog({ open, onClose, onCreated }: CreateProjectDialogProps) {
  const { t } = useTranslation()
  const [path, setPath] = useState('')
  const [preview, setPreview] = useState<ProjectPreview | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  // 关掉时清干净：留着上一次的预演的话，下次打开会先闪一下别人的项目。
  useEffect(() => {
    if (!open) {
      setPath('')
      setPreview(null)
      setError('')
    }
  }, [open])

  const loadPreview = useCallback(
    async (p: string) => {
      if (!p) return
      setBusy(true)
      setError('')
      try {
        setPreview(await previewProject(p))
      } catch (e) {
        // ★ 预演失败要**说出来并且不给创建**：让他对着一个空对话框
        // 点「创建」是最坏的——那等于回到了静默写。
        setPreview(null)
        setError(e instanceof Error ? e.message : t('project.previewFailed'))
      } finally {
        setBusy(false)
      }
    },
    [t],
  )

  const choose = useCallback(async () => {
    const picked = await pickDirectory()
    if (!picked) return
    setPath(picked)
    await loadPreview(picked)
  }, [loadPreview])

  const create = useCallback(async () => {
    if (!preview) return
    setBusy(true)
    setError('')
    try {
      // ★★ `initialize: true` 只在用户看过上面那份清单之后才发得出去。
      const p = await addProject(preview.path, true)
      onCreated(p.id ?? '')
      onClose()
    } catch (e) {
      setError(e instanceof Error ? e.message : t('project.createFailed'))
    } finally {
      setBusy(false)
    }
  }, [preview, onCreated, onClose, t])

  if (!open) return null

  const canPick = capabilities().canPickDirectory

  const creates = (preview?.actions ?? []).filter((a) => !isAppend(a))
  const appends = (preview?.actions ?? []).filter(isAppend)

  return (
    <div className={styles.backdrop} role="presentation" onClick={onClose}>
      <div
        className={styles.dialog}
        role="dialog"
        aria-modal="true"
        aria-label={t('project.dialogTitle')}
        onClick={(e) => e.stopPropagation()}
      >
        <header className={styles.header}>
          <h2 className={styles.title}>{t('project.dialogTitle')}</h2>
          <p className={styles.subtitle}>{t('project.dialogSubtitle')}</p>
        </header>

        <section className={styles.block}>
          <h3 className={styles.blockTitle}>{t('project.repoDir')}</h3>
          {canPick ? (
            <div className={styles.pathRow}>
              <code className={styles.path}>{path || t('project.noPath')}</code>
              <button type="button" className={styles.pick} onClick={() => void choose()}>
                {t('project.choose')}
              </button>
            </div>
          ) : (
            /*
              ★★ 浏览器形态下 `showDirectoryPicker` 出于安全**只给句柄不给路径**，
              而后端要的是绝对路径。所以这里降级成手动粘贴——
              装作能选、然后拿一个假路径去请求的话，用户会在后端拿到
              「路径不存在」，而他明明刚从对话框里选过。那种错误没人能自己解决。
            */
            <div className={styles.pathRow}>
              <input
                type="text"
                className={styles.path}
                aria-label={t('project.repoDir')}
                placeholder={t('project.pathPlaceholder')}
                value={path}
                onChange={(e) => setPath(e.target.value)}
                onBlur={() => void loadPreview(path)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') void loadPreview(path)
                }}
              />
              <button
                type="button"
                className={styles.pick}
                disabled={!path || busy}
                onClick={() => void loadPreview(path)}
              >
                {t('project.inspect')}
              </button>
            </div>
          )}
        </section>

        {error && <p className={styles.error}>{error}</p>}

        {preview && (
          <>
            <section className={styles.block}>
              <h3 className={styles.blockTitle}>{t('project.willCreate')}</h3>
              <ul className={styles.list}>
                {creates.map((a) => (
                  <li key={a.path} className={styles.item} data-already={a.already_there}>
                    <code className={styles.itemPath}>{shortPath(a.path, preview.path)}</code>
                    {/* ★ 每一步都要说得出为什么——不然用户凭什么点确认 */}
                    <span className={styles.reason}>{a.reason}</span>
                    {a.already_there && <span className={styles.already}>{t('project.already')}</span>}
                  </li>
                ))}
              </ul>
            </section>

            {appends.length > 0 && (
              <section className={styles.block}>
                {/*
                  ★★ 「将追加」单独一块，因为它**动的是用户自己的文件**——
                  和「建 Duet 自己的目录」不是一件事。
                */}
                <h3 className={styles.blockTitle}>{t('project.willAppend')}</h3>
                <ul className={styles.list}>
                  {appends.map((a) => (
                    <li key={a.path} className={styles.item} data-already={a.already_there}>
                      <code className={styles.itemPath}>{shortPath(a.path, preview.path)}</code>
                      <span className={styles.reason}>{a.reason}</span>
                      {(a.lines ?? []).map((l) => (
                        <code key={l} className={styles.line}>
                          {l}
                        </code>
                      ))}
                      {a.already_there && (
                        <span className={styles.already}>{t('project.already')}</span>
                      )}
                    </li>
                  ))}
                </ul>
              </section>
            )}

            {!preview.is_git_repo && (
              // ★ 不是 git 仓库**如实说**，且**不擅自 git init**——
              // 在别人的目录里建仓库是不可逆的。
              <p className={styles.note}>{t('project.notARepo')}</p>
            )}

            {preview.remote && (
              <section className={styles.block}>
                <h3 className={styles.blockTitle}>{t('project.github')}</h3>
                <p className={styles.remote}>
                  <code>{preview.remote.url}</code>
                  {preview.remote.slug && <span className={styles.slug}>{preview.remote.slug}</span>}
                </p>
                {preview.gh && (
                  <p className={styles.gh} data-status={preview.gh.status}>
                    {t(GH_KEY[preview.gh.status] ?? 'project.gh.probe_failed')}
                    {preview.gh.account && <span className={styles.account}>{preview.gh.account}</span>}
                    {/* ★ 修复命令由后端给，前端只原样显示 */}
                    {preview.gh.remedy && <code className={styles.remedy}>{preview.gh.remedy}</code>}
                  </p>
                )}
              </section>
            )}

            <section className={styles.block}>
              <h3 className={styles.blockTitle}>
                {t('project.foundSkills', { count: preview.skills.length })}
              </h3>
              {preview.skills.length === 0 ? (
                <p className={styles.reason}>{t('project.noSkills')}</p>
              ) : (
                <ul className={styles.list}>
                  {preview.skills.map((s) => (
                    <li key={s.source + '/' + s.dir} className={styles.item} data-skill={s.dir}>
                      <code className={styles.itemPath}>{s.source}</code>
                      <span className={styles.skillName}>{s.name}</span>
                      {/* ★ 校验没过的要说清缺什么（INV-SKL-2） */}
                      {!s.validation_ok && s.validation_reason && (
                        <span className={styles.invalid}>{s.validation_reason}</span>
                      )}
                    </li>
                  ))}
                </ul>
              )}
            </section>
          </>
        )}

        {/* ★ 设计稿原文的提示，照做：用户最怕的是「加进去它就自己开始干了」 */}
        <p className={styles.hint}>{t('project.wontStartAnything')}</p>

        <footer className={styles.footer}>
          <button type="button" className={styles.cancel} onClick={onClose}>
            {t('common.action.cancel')}
          </button>
          <button
            type="button"
            className={styles.confirm}
            // ★ 没有预演就不给创建——那等于回到了静默写
            disabled={!preview || busy}
            onClick={() => void create()}
          >
            {busy ? t('project.creating') : t('project.create')}
          </button>
        </footer>
      </div>
    </div>
  )
}

/** 把绝对路径缩成项目内的相对形态，界面上一长串前缀全是噪声。 */
function shortPath(full: string, root: string): string {
  if (full.startsWith(root)) {
    const rest = full.slice(root.length).replace(/^\//, '')
    return rest === '' ? '.' : rest
  }
  return full
}
