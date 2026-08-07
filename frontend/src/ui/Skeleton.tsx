import { useTranslation } from 'react-i18next'

import styles from './Skeleton.module.css'

/**
 * 骨架占位。用于「这块将来会有东西，但现在还没做」。
 *
 * ★ **占位就是占位，绝不编造看起来像真的内容。**
 * 一个显示「一次通过率 87%」但其实是编的界面，比空白更糟——
 * 用户会在假信息上做判断。所以这个组件只画**无意义的灰条**，
 * 并要求调用方用 `hint` 说清这里将来是什么。
 *
 * 设计规范 §07「加载与空态」。
 */
export type SkeletonProps = {
  /**
   * 这块将来是什么。**必填** —— 没有它，占位就退化成了「页面坏了」。
   * 传 i18n key，不传中文字面量。
   */
  hintKey: string
  /** 画几条灰条，默认 3。 */
  rows?: number
}

export function Skeleton({ hintKey, rows = 3 }: SkeletonProps) {
  const { t } = useTranslation()

  return (
    <div className={styles.wrap} data-testid="skeleton">
      <div className={styles.bars} aria-hidden="true">
        {Array.from({ length: rows }, (_, i) => (
          <div key={i} className={styles.bar} data-row={i} />
        ))}
      </div>
      <p className={styles.hint}>{t(hintKey)}</p>
      <p className={styles.badge}>{t('common.state.comingSoon')}</p>
    </div>
  )
}
