import { useTranslation } from 'react-i18next'

import styles from './TimelineFilter.module.css'
import { EVENT_KINDS, FILTER_GROUPS } from './event-registry'

export type TimelineFilterProps = {
  /** 被关掉的事件类型。 */
  hidden: ReadonlySet<string>
  onChange: (hidden: ReadonlySet<string>) => void
}

/**
 * 「时间线显示」面板——照 `design/ACP Duet 1a.dc.html` 的过滤器。
 *
 * 两组的分法对用户是有意义的：**ACP 事件是 AI 说的，应用事件是 Duet 自己做的**，
 * 出问题时该找谁不一样。
 *
 * ★ 面板内容由 `FILTER_GROUPS` 驱动，加一项只加一条注册。
 */
export function TimelineFilter({ hidden, onChange }: TimelineFilterProps) {
  const { t } = useTranslation()

  return (
    <div className={styles.panel}>
      <div className={styles.head}>
        <span className={styles.title}>{t('timeline.filter.title')}</span>
        <div className={styles.spacer} />
        <button type="button" className={styles.preset} onClick={() => onChange(new Set())}>
          {t('timeline.filter.all')}
        </button>
        <button type="button" className={styles.preset} onClick={selectMinimal}>
          {t('timeline.filter.minimal')}
        </button>
      </div>

      {FILTER_GROUPS.map((group) => (
        <div key={group.titleKey} className={styles.group}>
          <div className={styles.groupTitle}>{t(group.titleKey)}</div>
          {group.items.map((item) => {
            // 这一项管的类型全被关掉才算「关」——部分关掉时仍显示为开，
            // 免得用户以为自己关了却还看得见。
            const off = item.types.every((type) => hidden.has(type))
            return (
              <label key={item.id} className={styles.item}>
                <input
                  type="checkbox"
                  className={styles.checkbox}
                  checked={!off}
                  onChange={() => toggle(item.types, off)}
                />
                <span className={styles.itemLabel}>{t(item.labelKey)}</span>
              </label>
            )
          })}
        </div>
      ))}
    </div>
  )

  function toggle(types: readonly string[], currentlyOff: boolean) {
    const next = new Set(hidden)
    for (const type of types) {
      if (currentlyOff) {
        next.delete(type)
      } else {
        next.add(type)
      }
    }
    onChange(next)
  }

  /**
   * 「极简」：只留 AI 说的话与轮次结束。
   *
   * 这是用户「我只想知道它说了什么」时的一键切换——
   * 工具调用、状态变动这些是排查时才看的。
   */
  function selectMinimal() {
    const keep = new Set(['message_chunk', 'turn_end'])
    onChange(new Set(EVENT_KINDS.filter((k) => !keep.has(k))))
  }
}
