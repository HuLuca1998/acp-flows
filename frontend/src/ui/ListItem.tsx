import type { KeyboardEvent, ReactNode } from 'react'

import styles from './ListItem.module.css'

/**
 * 可选中的列表项。设计规范 §05 / 实现规格 §7.4。
 *
 * 三态固定，没有第四态：默认（inset 底）· 选中（raised 底 + accent 描边）·
 * 失效（透明底 + 60% 不透明度）。
 *
 * ★ 用 `role="button"` 的 div 而不是 `<button>`：列表卡里可以嵌动作按钮
 * （候选的「收下 / 不要」），而 HTML 不允许按钮套按钮。
 * 可访问性靠 role + tabIndex + Enter/Space 补（§7.5 同款做法）。
 *
 * ★ 左侧 2px accent 竖条（`isCurrent`）**仅用于「当前工作 / 当前单元」**，
 * 这条竖条在代码里只有这一个出口（§7.4）。
 */
export type ListItemProps = {
  state?: 'default' | 'selected' | 'invalid'
  isCurrent?: boolean
  /**
   * 可访问名称（条目的 id 或名字）。★ 必填——不给的话可访问名会把
   * 卡里**所有文字连同嵌套按钮的文字**拼在一起，读屏用户听到的是一锅粥，
   * 测试里按名字定位也会撞车。
   */
  label: string
  onSelect?: () => void
  children: ReactNode
}

export function ListItem({
  state = 'default',
  isCurrent = false,
  label,
  onSelect,
  children,
}: ListItemProps) {
  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    // 只认落在自己身上的键：嵌在卡里的动作按钮有自己的键盘行为。
    if (e.target !== e.currentTarget) {
      return
    }
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault()
      onSelect?.()
    }
  }

  return (
    <div
      role="button"
      tabIndex={0}
      aria-label={label}
      className={styles.item}
      data-state={state}
      data-current={isCurrent || undefined}
      aria-pressed={state === 'selected'}
      onClick={onSelect}
      onKeyDown={onKeyDown}
    >
      {children}
    </div>
  )
}
