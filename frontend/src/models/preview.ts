import type { components } from '@/api/gen/schema'

/** 创建项目前的预演。形状由 api/openapi.yaml 决定。 */
export type ProjectPreview = components['schemas']['ProjectPreview']

/** 计划里的一步。 */
export type ProjectAction = components['schemas']['ProjectAction']

/** `gh` 的状态。★ 四态——只用两个布尔表达不了「检测本身失败了」。 */
export type GhStatus = NonNullable<components['schemas']['GhStatus']['status']>

/**
 * 这一步是新建东西还是追加到已有文件。
 *
 * ★ 设计稿把它们分成两块显示（「将创建」/「将追加」），
 * 因为对用户是两件事：前者是 Duet 自己的目录，
 * 后者**动的是他的文件**。
 */
export function isAppend(a: ProjectAction): boolean {
  return a.kind === 'append_lines'
}
