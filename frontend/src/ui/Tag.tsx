import type { ReactNode } from 'react'

import styles from './Tag.module.css'

/**
 * 类型标签。设计规范 §05 / 实现规格 §7.3。
 *
 * 10px 等宽小芯片，两种 tone：
 *   - `accent`：accent-900 底 + accent-300 字（版本号、类型）
 *   - `neutral`：surface 底 + neutral-400 字（次要标记）
 *
 * ★ 文件/条目**类型靠文字表达**，禁止用彩色圆点（§7.3）。
 */
export type TagProps = {
  tone?: 'accent' | 'neutral'
  children: ReactNode
}

export function Tag({ tone = 'neutral', children }: TagProps) {
  return (
    <span className={styles.tag} data-tone={tone}>
      {children}
    </span>
  )
}
