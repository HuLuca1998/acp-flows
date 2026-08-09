import type { components } from '@/api/gen/schema'

/** 一个工作区的 git 现场。形状由 api/openapi.yaml 决定。 */
export type WorktreeState = components['schemas']['WorktreeState']

/** 一个文件的改动。 */
export type FileChange = components['schemas']['FileChange']

/** 一条 commit。 */
export type CommitInfo = components['schemas']['CommitInfo']

/**
 * 合计增删行数。
 *
 * ★ 设计稿右栏底部显示 `合计 +184 −12`——
 * 用户扫一眼就知道这次改动有多大，不用自己加。
 */
export function totalOf(changes: FileChange[]): { added: number; removed: number } {
  return changes.reduce(
    (acc, c) => ({ added: acc.added + c.added, removed: acc.removed + c.removed }),
    { added: 0, removed: 0 },
  )
}
