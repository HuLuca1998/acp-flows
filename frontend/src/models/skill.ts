import type { components } from '@/api/gen/schema'

/** 一个 Skill。形状由 api/openapi.yaml 决定。 */
export type Skill = components['schemas']['Skill']

/** 校验状态。draft 时 `validation_reason` 说明为什么没过。 */
export type SkillStatus = NonNullable<Skill['status']>
