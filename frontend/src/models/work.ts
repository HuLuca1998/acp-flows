import type { components } from '@/api/gen/schema'

/** 一次工作。形状由 api/openapi.yaml 决定。 */
export type Work = components['schemas']['Work']

/** 工作状态。**封闭枚举**，与后端 constant.WorkState 一一对应。 */
export type WorkState = NonNullable<Work['state']>

/**
 * 这个工作还在动吗。
 *
 * ★ `initializing_failed` 是**终态、不可恢复**——worktree 没切成就没有
 * 可执行的现场（docs/adr/0006 Q1）。界面上不该给它「重试」按钮。
 */
export function isTerminal(state: WorkState): boolean {
  return state === 'completed' || state === 'failed' || state === 'initializing_failed'
}
