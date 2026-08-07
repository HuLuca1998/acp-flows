import { useTranslation } from 'react-i18next'

import styles from './SettingsNav.module.css'
import { SETTINGS_SECTIONS, type SettingsSectionId } from './section-registry'

export type SettingsNavProps = {
  current: SettingsSectionId
  onSelect: (id: SettingsSectionId) => void
}

/**
 * 设置页左侧的二级导航。照设计稿：196px 定宽、右侧一条分隔线、
 * 每项两行（名字 + 副标题）。
 *
 * 用 tablist/tab 而不是一堆 div：选中态在无障碍树上要能表达出来，
 * 键盘用户也得能走到（设计规范 §09）。
 */
export function SettingsNav({ current, onSelect }: SettingsNavProps) {
  const { t } = useTranslation()

  return (
    <nav className={styles.nav} role="tablist" aria-label={t('settings.nav.label')}>
      {SETTINGS_SECTIONS.map((section) => {
        const selected = section.id === current
        return (
          <button
            key={section.id}
            type="button"
            role="tab"
            aria-selected={selected}
            className={`${styles.item} ${selected ? styles.itemActive : ''}`}
            onClick={() => onSelect(section.id)}
          >
            <span className={styles.itemTitle}>{t(section.titleKey)}</span>
            <span className={styles.itemSub} data-sub>
              {t(section.subKey)}
            </span>
          </button>
        )
      })}
    </nav>
  )
}
