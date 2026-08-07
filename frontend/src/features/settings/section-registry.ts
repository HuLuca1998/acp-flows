/**
 * 设置页的六个分区。照 `design/ACP Duet 1a.dc.html` 的设置页左栏，逐字。
 *
 * ★ **这是注册表，不是 switch。** 加一个分区 = 加一条记录 + 加一个组件，
 * 不改导航渲染一行（与 `app/pages.ts` 同一套做法）。
 *
 * ★ 副标题里**不许出现数字**。设计稿写的是「1.4.2 → 1.5.0」「3 个项目」，
 * 那是示意数据——照抄的话，一个还没接项目功能的应用会告诉用户他有 3 个项目。
 * 编造数据比空白更糟（`App.test.tsx` 已有同款规则）。
 */
export const SETTINGS_SECTIONS = [
  { id: 'runtime', titleKey: 'settings.runtime.title', subKey: 'settings.nav.runtime' },
  { id: 'env', titleKey: 'settings.env.title', subKey: 'settings.nav.env' },
  { id: 'update', titleKey: 'settings.update.title', subKey: 'settings.nav.update' },
  { id: 'projects', titleKey: 'settings.projects.title', subKey: 'settings.nav.projects' },
  { id: 'github', titleKey: 'settings.github.title', subKey: 'settings.nav.github' },
  { id: 'general', titleKey: 'settings.general.title', subKey: 'settings.nav.general' },
] as const

export type SettingsSectionId = (typeof SETTINGS_SECTIONS)[number]['id']

/** 默认落在第一项，与设计稿一致。 */
export const DEFAULT_SECTION: SettingsSectionId = SETTINGS_SECTIONS[0].id
