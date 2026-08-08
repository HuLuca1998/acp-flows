import { useTranslation } from 'react-i18next'

import styles from './PermissionCard.module.css'
import type { PermissionRequest, PermissionToolKind } from './model'

export type PermissionCardProps = {
  ask: PermissionRequest
  /** 用户选了某个选项。参数是 **Agent 给的 optionId**，原样传回去。 */
  onDecide: (optionID: string) => void
  /** 应答提交中：禁用全部按钮。 */
  pending?: boolean
}

/**
 * 权限卡片：AI 想动你的东西，先问一句。
 *
 * ★ **不是弹窗**（本单元的 forbidden_changes 明写禁止用弹窗打断）。
 * 它长在时间线里，跟其余进展排在一起——用户能看到「它为什么要动这个文件」
 * 的上下文，而弹窗会把上下文盖住。
 *
 * ★ 按钮照 **Agent 给的 options** 渲染，不自己造一套「允许 / 拒绝」。
 * 自己造的话，Agent 提供的第三种选项（比如「这个目录以后都允许」）就消失了，
 * 而用户根本不知道自己少了一个选择。
 */
export function PermissionCard({ ask, onDecide, pending = false }: PermissionCardProps) {
  const { t } = useTranslation()

  return (
    <div className={styles.card} data-permission-ask={ask.id} data-kind={ask.kind}>
      <i className={`ph ph-shield-check ${styles.icon}`} aria-hidden="true" />

      <span className={styles.headline}>
        {t(actionKey(ask.kind), { runtime: ask.runtime })}
        {ask.path !== undefined && ask.path !== '' && (
          <span className={styles.path} data-path>
            {ask.path}
          </span>
        )}
      </span>

      {/* ★ 只在真的越界时才说。没依据就说的话，用户会对所有提示脱敏，
          真正越界那次他也不会看。 */}
      {ask.outOfBounds === true && <span className={styles.warn}>{t('permission.outOfBounds')}</span>}

      <div className={styles.spacer} />

      {ask.options.length === 0 ? (
        // 一个选项都没给：说清楚用户没法处理，而不是显示一张点不动的空卡片让他一直等
        <span className={styles.warn}>{t('permission.noOptions')}</span>
      ) : (
        ask.options.map((o) => (
          <button
            key={o.optionId}
            type="button"
            className={isAllow(o.kind) ? styles.allow : styles.reject}
            disabled={pending}
            // ★ 原样回传 Agent 给的 id，不按 kind 重新匹配——
            // 搞错的话，用户点「拒绝」而 Agent 收到「允许」
            onClick={() => onDecide(o.optionId)}
          >
            {o.name}
          </button>
        ))
      )}

      <span className={styles.type}>request_permission</span>
    </div>
  )
}

/**
 * 工具类别 → 说法的**显式映射**。
 *
 * ★ 不许写成 `permission.action.${kind}`：动态拼接之后静态分析查不出词条缺失
 * （docs/rules/i18n.md §4，check-i18n 会拦）。
 *
 * ★ 认不出的类别用兜底文案，**不把原始码显示给用户**——
 * `switch_mode` 这种字符串对他没有意义。
 */
const ACTION_KEY: Record<string, string> = {
  read: 'permission.action.read',
  edit: 'permission.action.edit',
  delete: 'permission.action.delete',
  move: 'permission.action.move',
  search: 'permission.action.search',
  execute: 'permission.action.execute',
  fetch: 'permission.action.fetch',
}

function actionKey(kind: PermissionToolKind): string {
  return ACTION_KEY[kind] ?? 'permission.action.other'
}

/** allow 类的按钮用强调色，reject 类用中性色——照设计稿。 */
function isAllow(kind: string): boolean {
  return kind === 'allow_once' || kind === 'allow_always'
}
