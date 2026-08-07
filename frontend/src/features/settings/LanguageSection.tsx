import { useTranslation } from 'react-i18next'

import { Button } from '@/ui/Button'
import { STORAGE_KEYS, writePersisted } from '@/utils/persisted'

import styles from './Settings.module.css'

/** 支持的语言。加一种语言 = 往这张表加一行。 */
const LANGUAGES = [
  { code: 'zh-CN', labelKey: 'settings.general.langZh' },
  { code: 'en-US', labelKey: 'settings.general.langEn' },
] as const

/**
 * 通用设置 · 语言切换。
 *
 * 切换**立刻生效、不需要重启**——要用户重启才能换语言是上个时代的做法。
 */
export function LanguageSection() {
  const { t, i18n } = useTranslation()

  return (
    <section className={styles.section} aria-labelledby="general-heading">
      <h3 id="general-heading" className={styles.heading}>
        {t('settings.general.title')}
      </h3>
      <div className={styles.langRow}>
        <span className={styles.langLabel}>{t('settings.general.language')}</span>
        {LANGUAGES.map((lang) => (
          <Button
            key={lang.code}
            variant="secondary"
            active={i18n.language === lang.code}
            onClick={() => {
              void i18n.changeLanguage(lang.code)
              // 记住选择：下次打开还是这个语言。
              writePersisted(STORAGE_KEYS.locale, lang.code)
            }}
          >
            {t(lang.labelKey)}
          </Button>
        ))}
      </div>
    </section>
  )
}
