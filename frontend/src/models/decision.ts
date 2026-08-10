import type { components } from '@/api/gen/schema'

/** 一条待决策。★ 每个选项都带 `impact`——没有它用户在盲选。 */
export type Decision = components['schemas']['Decision']
