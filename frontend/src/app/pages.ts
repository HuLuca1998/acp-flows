import type { ComponentType } from 'react'

import { MemoryPage } from '@/features/memory'
import { ReportPage } from '@/features/report'
import { RolesPage } from '@/features/roles'
import { SettingsPage } from '@/features/settings'
import { SkillPage } from '@/features/skill'

/**
 * 左栏导航的页面注册表。**加一个页面 = 往这张表加一行**，不改任何组件。
 *
 * ★ 这里**只有 5 项**，与设计稿一致（design/ACP Duet 1a.dc.html）。
 * 「对话」不在其中——它是**默认主区**本身，不是一个可导航的页面；
 * 「计划」也不在——它是窗口栏 ▤ 唤出的**悬浮面板**。
 * 把这两个做成导航项是对信息架构的误读：左栏回答的是
 * 「你在哪个项目的哪个工作里」，不是「你要去哪个功能页」。
 */
export type PageId = 'chat' | 'report' | 'memory' | 'skill' | 'roles' | 'settings'

export type PageDef = {
  id: PageId
  /** i18n key。**不放中文字面量** —— 侧栏标题要跟着语言切换。 */
  titleKey: string
  /** Phosphor 图标类名。只从 regular 集取，禁止 emoji 与自绘 SVG。 */
  icon: string
  Component: ComponentType
}

/** 左栏顶部的导航项，顺序与设计稿一致。 */
export const NAV_PAGES: readonly PageDef[] = [
  { id: 'report', titleKey: 'nav.report', icon: 'ph-chart-bar', Component: ReportPage },
  { id: 'memory', titleKey: 'nav.memory', icon: 'ph-brain', Component: MemoryPage },
  { id: 'skill', titleKey: 'nav.skill', icon: 'ph-book-open', Component: SkillPage },
  { id: 'roles', titleKey: 'nav.roles', icon: 'ph-users-three', Component: RolesPage },
  { id: 'settings', titleKey: 'nav.settings', icon: 'ph-gear', Component: SettingsPage },
] as const

/**
 * 默认主区：对话。选中某个工作时回到它。
 * 未知页面标识也回到它——**绝不白屏**。
 */
export const DEFAULT_PAGE: PageId = 'chat'

/**
 * 三栏（左 + 主 + 右）只在对话时出现。
 * 报表 / 记忆 / Skill / 角色 / 设置都是**全宽内容页**，没有右栏。
 */
export function hasContextPanel(id: PageId): boolean {
  return id === DEFAULT_PAGE
}

/**
 * 把任意字符串规整成合法的页面标识。
 *
 * 持久化的值可能被手工改坏，也可能是上个版本遗留的旧标识。
 * 不规整的话会出现「主区回落到对话、右栏却按未知页收起」这种不一致。
 */
export function normalizePageId(raw: unknown): PageId {
  if (raw === DEFAULT_PAGE) return DEFAULT_PAGE
  return NAV_PAGES.find((p) => p.id === raw)?.id ?? DEFAULT_PAGE
}

/** 按 id 取导航页定义；不是导航页（含对话与未知值）返回 null。 */
export function navPageById(id: string | null): PageDef | null {
  return NAV_PAGES.find((p) => p.id === id) ?? null
}
