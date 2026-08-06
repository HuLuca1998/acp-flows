import type { ButtonHTMLAttributes, ReactNode } from 'react'

import styles from './Button.module.css'

/**
 * 按钮。设计规范 §05。
 *
 * 三条硬约束（违反即不合规）：
 *   1. **主操作永不填充实心** —— accent-700 描边 + accent-300 字
 *   2. **危险动作用 --color-fail 文字色，不是红底**
 *   3. **纯图标按钮必须有中文 tooltip**（title + data-tt），带文字的不加
 */
export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger'

export type ButtonProps = Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'type'> & {
  variant?: ButtonVariant
  /** 激活/选中态：accent-900 底 + accent-300 字 */
  active?: boolean
  /** Phosphor 图标类名（如 ph-sidebar）。只从 Phosphor regular 取，禁止 emoji 与自绘 SVG。 */
  icon?: string
  /**
   * 中文名。**纯图标按钮必填** —— 它同时是 tooltip 与可访问名称。
   * 带文字的按钮不要传，否则会冒出重复的浮层（设计规范 §08）。
   */
  label?: string
  /** 快捷键，会拼进 tooltip：「折叠侧栏 ⌘B」 */
  shortcut?: string
  children?: ReactNode
}

export function Button({
  variant = 'secondary',
  active = false,
  icon,
  label,
  shortcut,
  className,
  children,
  ...buttonProps
}: ButtonProps) {
  const isIconOnly = children === undefined && label !== undefined

  // 带文字的按钮不加 tooltip；纯图标按钮的 tooltip 与可访问名称同源，
  // 不会出现「看得见的名字和读出来的名字不一样」。
  const tooltip = isIconOnly ? (shortcut === undefined ? label : `${label} ${shortcut}`) : undefined

  return (
    <button
      type="button"
      className={[styles.btn, styles[variant], isIconOnly && styles.iconOnly, className]
        .filter(Boolean)
        .join(' ')}
      data-active={active || undefined}
      aria-label={isIconOnly ? label : undefined}
      title={tooltip}
      data-tt={tooltip}
      {...buttonProps}
    >
      {icon !== undefined && <i className={`ph ${icon}`} aria-hidden="true" />}
      {children}
    </button>
  )
}
