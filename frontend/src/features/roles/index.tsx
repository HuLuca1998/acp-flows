import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { listRoles } from '@/api/system'
import type { Role } from '@/models/role'

import styles from './RolesPage.module.css'

/**
 * 权限裁决 → 词条 key。
 *
 * ★ 显式映射而不是 `t('roles.permission.' + p)`：动态拼 key 的话，
 * 静态分析查不出缺失词条——加一档权限裁决而忘了配文案时，
 * 界面上会直接显示原始的 `roles.permission.xxx`，而没有任何检查会红。
 */
const PERMISSION_KEY: Record<string, string> = {
  ask_each: 'roles.permission.ask_each',
  auto_allow_read: 'roles.permission.auto_allow_read',
}

/**
 * 角色与 Runtime 页。设计稿标题：**「角色先于 Runtime」**。
 *
 * 一张「角色 ↔ Runtime 绑定」表，列与 `design/INVENTORY.md` §八 一一对应：
 * 角色/承担的操作 · 性格与提示语气 · Runtime · 会话模式 · 权限裁决 · 提示词。
 *
 * ★ 这一页在 M2 之前一直是骨架占位。而八个预置角色是**内置的**——
 * 用户打开它看到空白，只会以为应用坏了。
 */
export function RolesPage() {
  const { t } = useTranslation()
  const [roles, setRoles] = useState<Role[] | null>(null)
  const [failed, setFailed] = useState(false)

  useEffect(() => {
    void (async () => {
      try {
        setRoles(await listRoles())
      } catch {
        // ★ 查不到要**说出来**。装作「一个角色都没有」的话，用户会去找
        // 「怎么添加角色」——而预置角色本来就该在那儿，问题在别处。
        setFailed(true)
      }
    })()
  }, [])

  if (failed) {
    return <p className={styles.hint}>{t('roles.failed')}</p>
  }
  if (roles === null) {
    return <p className={styles.hint}>{t('roles.loading')}</p>
  }

  return (
    <div className={styles.page}>
      <header className={styles.header}>
        {/* 设计稿的标题是两层：小字眉标 + 大标题 */}
        <p className={styles.eyebrow}>{t('roles.eyebrow')}</p>
        <h1 className={styles.title}>{t('roles.title')}</h1>
      </header>

      <div className={styles.tableWrap}>
        <table className={styles.table}>
          <thead>
            <tr>
              <th>{t('roles.col.role')}</th>
              <th>{t('roles.col.personality')}</th>
              <th>{t('roles.col.runtime')}</th>
              <th>{t('roles.col.mode')}</th>
              <th>{t('roles.col.permission')}</th>
              <th>{t('roles.col.prompt')}</th>
            </tr>
          </thead>
          <tbody>
            {roles.map((r) => (
              <tr key={r.id} data-role={r.id}>
                <td>
                  <div className={styles.roleName}>
                    {/* 设计稿每行左侧有一个圆点标记 */}
                    <span className={styles.dot} aria-hidden="true" />
                    {r.display_name}
                  </div>
                  {/* 承担的操作：设计稿里是 `clarify · snapshot` 这个形态 */}
                  <div className={styles.ops}>{(r.operations ?? []).join(' · ')}</div>
                </td>
                <td className={styles.personality}>{r.personality}</td>
                <td>
                  {/*
                    ★ 设计稿这里是下拉（`claude ⌄`）。M2 只读，所以做成
                    **禁用态的下拉**而不是纯文本——形态照设计稿，
                    但点下去不会给一个空菜单。可编辑在 M11。
                  */}
                  <span className={styles.select} data-disabled="true">
                    {r.runtime_name}
                    <i className="ph ph-caret-down" aria-hidden="true" />
                  </span>
                </td>
                <td>
                  {/*
                    ★ 显示的是后端翻译好的**实际档名**（`plan` / `read-only`），
                    同时用 data 属性带上语义档——只读与能写在界面上要分得出来，
                    而按档名判断的话每加一端就要加一串条件。
                  */}
                  <span className={styles.mode} data-mode={r.session_mode}>
                    {r.mode_name ?? '—'}
                    <i className="ph ph-caret-down" aria-hidden="true" />
                  </span>
                </td>
                <td>
                  <span className={styles.select} data-disabled="true">
                    {t(PERMISSION_KEY[r.permission_policy] ?? 'roles.permission.ask_each')}
                    <i className="ph ph-caret-down" aria-hidden="true" />
                  </span>
                </td>
                <td>
                  {/*
                    设计稿是「提示词 ✎」按钮。M11 才能编辑——
                    做成 disabled 而不是拿掉：拿掉的话用户不知道以后能改，
                    做成能点的又会点了没反应。
                  */}
                  <button type="button" className={styles.promptBtn} disabled title={t('roles.promptSoon')}>
                    {t('roles.col.prompt')}
                    <i className="ph ph-pencil-simple" aria-hidden="true" />
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/*
        ★★ 这两块是设计稿原文，**不是我加的说明**。
        它们回答了用户打开这一页最可能问的两个问题：
        这些开关到底在控制什么、为什么没有「选模型」。
        漏掉它们的话，这一页就只剩一张看不懂的表。
      */}
      <div className={styles.notes}>
        <section className={styles.note}>
          <h2 className={styles.noteTitle}>{t('roles.note.acpTitle')}</h2>
          <p className={styles.noteBody}>{t('roles.note.acpBody')}</p>
        </section>
        <section className={styles.note}>
          <h2 className={styles.noteTitle}>{t('roles.note.notProvidedTitle')}</h2>
          <p className={styles.noteBody}>{t('roles.note.notProvidedBody')}</p>
        </section>
      </div>

      <p className={styles.footnote}>{t('roles.footnote')}</p>

      {roles.some((r) => r.problem) && (
        <ul className={styles.problems}>
          {roles
            .filter((r) => r.problem)
            .map((r) => (
              <li key={r.id}>
                {r.display_name}：{r.problem}
              </li>
            ))}
        </ul>
      )}
    </div>
  )
}
