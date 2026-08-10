import { useEffect, useRef, useState } from 'react'

import styles from './Dropdown.module.css'

/**
 * 下拉选择器。设计规范 §05 / 实现规格 §7.6。
 *
 * 结构＝小标签 + 当前值（等宽）+ `⌄`；面板列条目，右侧可带 meta。
 *
 * ★ 硬约束（§7.6）：**同类选择器在不同页面必须放同一位置**——
 * 页头标题行下方第一行。记忆页的「范围」、Skill 页的「项目」都遵守。
 */
export type DropdownItem = { value: string; label: string; meta?: string }

export type DropdownProps = {
  /** 小标签，如「项目」「范围」。也是触发器的可访问名。 */
  label: string
  /** 当前值的显示文本，等宽显示。 */
  value: string
  items: DropdownItem[]
  onSelect: (value: string) => void
  footerAction?: { label: string; onClick: () => void }
}

export function Dropdown({ label, value, items, onSelect, footerAction }: DropdownProps) {
  const [open, setOpen] = useState(false)
  const rootRef = useRef<HTMLDivElement>(null)

  // 点外面关掉。★ 挂在 document 上而不是遮罩层：§7.6 的面板没有遮罩。
  useEffect(() => {
    if (!open) {
      return
    }
    const onDown = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', onDown)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onDown)
      document.removeEventListener('keydown', onKey)
    }
  }, [open])

  return (
    <div className={styles.root} ref={rootRef}>
      <button
        type="button"
        className={styles.trigger}
        aria-haspopup="listbox"
        aria-expanded={open}
        onClick={() => setOpen((o) => !o)}
      >
        <span className={styles.label}>{label}</span>
        <span className={styles.value}>{value}</span>
        <span className={styles.chevron} aria-hidden>
          ⌄
        </span>
      </button>

      {open && (
        <div className={styles.panel} role="listbox" aria-label={label}>
          {items.map((it) => (
            <button
              key={it.value}
              type="button"
              role="option"
              aria-selected={it.label === value}
              className={styles.option}
              data-selected={it.label === value}
              onClick={() => {
                setOpen(false)
                onSelect(it.value)
              }}
            >
              <span className={styles.optionLabel}>{it.label}</span>
              {it.meta && <span className={styles.meta}>{it.meta}</span>}
            </button>
          ))}
          {footerAction && (
            <button
              type="button"
              className={styles.footer}
              onClick={() => {
                setOpen(false)
                footerAction.onClick()
              }}
            >
              {footerAction.label}
            </button>
          )}
        </div>
      )}
    </div>
  )
}
