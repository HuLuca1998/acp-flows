import { useCallback, useEffect, useRef } from 'react'

import styles from './Resizer.module.css'

/**
 * 分栏拖拽手柄。设计规范 §06 规则② / §08「拖拽」。
 *
 * 5px 宽、neutral-900 底、hover accent-700、`cursor: col-resize`。
 * 宽度范围写死在调用方，拖到边界时**钳制**而不是继续跟随——
 * 让栏宽超出范围会把主区挤没，且用户很难恢复。
 */
export type ResizerProps = {
  /** 当前宽度。 */
  width: number
  min: number
  max: number
  /** 往哪个方向拖增加宽度：左栏是 'right'，右栏是 'left'。 */
  grow: 'right' | 'left'
  onResize: (next: number) => void
  /** 可访问名称，走 i18n。 */
  label: string
}

export function Resizer({ width, min, max, grow, onResize, label }: ResizerProps) {
  // 用 ref 存拖拽起点：放 state 会让每次 mousemove 都重渲染整棵树。
  const drag = useRef<{ startX: number; startWidth: number } | null>(null)

  const clamp = useCallback(
    (value: number) => Math.min(max, Math.max(min, value)),
    [min, max],
  )

  useEffect(() => {
    function onMove(e: MouseEvent) {
      if (drag.current === null) return
      const delta = e.clientX - drag.current.startX
      onResize(clamp(drag.current.startWidth + (grow === 'right' ? delta : -delta)))
    }
    function onUp() {
      drag.current = null
      // 拖动时禁掉全局文本选择，否则整页文字会被刷蓝
      document.body.style.removeProperty('user-select')
    }
    window.addEventListener('mousemove', onMove)
    window.addEventListener('mouseup', onUp)
    return () => {
      window.removeEventListener('mousemove', onMove)
      window.removeEventListener('mouseup', onUp)
    }
  }, [clamp, grow, onResize])

  return (
    <div
      className={styles.handle}
      role="separator"
      aria-orientation="vertical"
      aria-label={label}
      aria-valuenow={width}
      aria-valuemin={min}
      aria-valuemax={max}
      tabIndex={0}
      onMouseDown={(e) => {
        drag.current = { startX: e.clientX, startWidth: width }
        document.body.style.userSelect = 'none'
      }}
      // 键盘也能调：纯鼠标的拖拽对键盘用户等于不存在
      onKeyDown={(e) => {
        if (e.key === 'ArrowLeft') onResize(clamp(width + (grow === 'right' ? -16 : 16)))
        if (e.key === 'ArrowRight') onResize(clamp(width + (grow === 'right' ? 16 : -16)))
      }}
    />
  )
}
