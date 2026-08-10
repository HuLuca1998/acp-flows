import type { ReactNode } from 'react'

import styles from './ListItem.module.css'

/**
 * 可选中的列表项。设计规范 §05 / 实现规格 §7.4。
 *
 * 三态固定，没有第四态：默认（inset 底）· 选中（raised 底 + accent 描边）·
 * 失效（透明底 + 60% 不透明度）。
 *
 * ★ 左侧 2px accent 竖条（`isCurrent`）**仅用于「当前工作 / 当前单元」**，
 * 这条竖条在代码里只有这一个出口（§7.4）。
 */
export type ListItemProps = {
  state?: 'default' | 'selected' | 'invalid'
  isCurrent?: boolean
  onSelect?: () => void
  children: ReactNode
}

export function ListItem({ state = 'default', isCurrent = false, onSelect, children }: ListItemProps) {
  return (
    <button
      type="button"
      className={styles.item}
      data-state={state}
      data-current={isCurrent || undefined}
      aria-pressed={state === 'selected'}
      onClick={onSelect}
    >
      {children}
    </button>
  )
}
