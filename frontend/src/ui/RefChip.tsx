import styles from './RefChip.module.css'

/**
 * 可移除的引用芯片。设计规范 §05 / 实现规格 §7.3。
 *
 * 11px 等宽，raised 底 + neutral-800 描边；前置小图标，尾部 `✕`
 * （✕ 是 §7.3 规格原文，不算「emoji 当图标」）。
 */
export type RefChipProps = {
  /** Phosphor 图标类名（如 ph-file）。 */
  icon?: string
  label: string
  onRemove?: () => void
  /** 移除按钮的可访问名（如「移除引用 notes/cancel.md」）。有 onRemove 时必填。 */
  removeLabel?: string
}

export function RefChip({ icon, label, onRemove, removeLabel }: RefChipProps) {
  return (
    <span className={styles.chip}>
      {icon !== undefined && <i className={`ph ${icon}`} aria-hidden="true" />}
      <code className={styles.label}>{label}</code>
      {onRemove && (
        <button
          type="button"
          className={styles.remove}
          aria-label={removeLabel ?? label}
          onClick={onRemove}
        >
          ✕
        </button>
      )}
    </span>
  )
}
