import type { components } from '@/api/gen/schema'

/** 一条记忆。形状由 api/openapi.yaml 决定。 */
export type Memory = components['schemas']['Memory']

/** 状态。**五态**，不是设计稿筛选器上那三档。 */
export type MemoryStatus = components['schemas']['MemoryStatus']

/**
 * 界面上的筛选档位。
 *
 * ★★ 设计稿的筛选器只有四个按钮（`全部` / `active` / `候选` / `已失效`），
 * 而后端有五态——「已失效」这一档**同时装着 `invalid` 与 `obsolete`**。
 * 两者对用户长得一样（都是不再注入的旧记忆），对系统不一样
 * （废弃要带理由、可指向 supersedes）。
 */
export type MemoryFilterTab = 'all' | 'active' | 'candidate' | 'retired'

/**
 * 一个筛选档覆盖哪些后端状态。
 *
 * ★ `all` **不含候选**（Q25 裁定）：设计稿的计数是「全部 12 · active 9 ·
 * 候选 2 · 已失效 3」，9+3=12——候选是待办不是库存，混进「全部」里
 * 会让用户以为它们已经生效了。
 */
export const TAB_STATUSES: Record<MemoryFilterTab, readonly MemoryStatus[]> = {
  all: ['active', 'invalid', 'obsolete'],
  active: ['active'],
  candidate: ['candidate'],
  retired: ['invalid', 'obsolete'],
}

/** 这条记忆属于哪个筛选档（用于计数与过滤）。 */
export function matchesTab(m: Memory, tab: MemoryFilterTab): boolean {
  return TAB_STATUSES[tab].includes(m.status)
}
