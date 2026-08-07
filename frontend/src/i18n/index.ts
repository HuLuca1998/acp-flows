import i18next from 'i18next'
import { initReactI18next } from 'react-i18next'

import { readPersisted, STORAGE_KEYS } from '@/utils/persisted'

import enUS from './locales/en-US.json'
import zhCN from './locales/zh-CN.json'

/**
 * 中英双语。规则见 docs/rules/i18n.md。
 *
 * 默认与回退都是 zh-CN：设计稿的一手文案是中文，
 * 英文缺词条时显示中文，**不显示 key**（那对用户毫无意义）。
 */
export const SUPPORTED_LOCALES = ['zh-CN', 'en-US'] as const
export type Locale = (typeof SUPPORTED_LOCALES)[number]

export const DEFAULT_LOCALE: Locale = 'zh-CN'

/** 读回用户上次选的语言；没选过或值不合法时用默认。 */
export function preferredLocale(): Locale {
  const saved = readPersisted<string | null>(STORAGE_KEYS.locale, null)
  return SUPPORTED_LOCALES.includes(saved as Locale) ? (saved as Locale) : DEFAULT_LOCALE
}

export async function initI18n(locale: Locale = preferredLocale()) {
  await i18next.use(initReactI18next).init({
    resources: {
      'zh-CN': { translation: zhCN },
      'en-US': { translation: enUS },
    },
    lng: locale,
    fallbackLng: DEFAULT_LOCALE,
    interpolation: { escapeValue: false }, // React 已经转义了
    returnNull: false,
  })
  return i18next
}

export default i18next
