import styles from './StatusText.module.css'

/**
 * 状态文字。设计规范 §05 / 实现规格 §7.3。
 *
 * 10.5px 等宽，**不加底色**，只着色——状态词一律英文原值（术语表）。
 */
export type StatusTextProps = {
  value: string
  tone: 'pass' | 'fail' | 'muted'
}

export function StatusText({ value, tone }: StatusTextProps) {
  return (
    <span className={styles.status} data-tone={tone}>
      {value}
    </span>
  )
}
