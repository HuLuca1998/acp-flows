import type { components } from '@/api/gen/schema'

/** 开工前的仓库状态。形状由 api/openapi.yaml 决定。 */
export type WorkPreparation = components['schemas']['WorkPreparation']

/**
 * 工作区脏不脏。
 *
 * ★ 两个数**分开看**：已跟踪的改动意味着「他改了正在跟踪的代码」，
 * 未跟踪意味着「他新建了几个还没 add 的文件」——
 * 后者常常只是临时文件，前者才是他真在做的事。
 */
export function isDirty(p: WorkPreparation): boolean {
  return p.tracked_dirty > 0 || p.untracked > 0
}
