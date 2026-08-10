import type { components } from '@/api/gen/schema'

/** 一版计划。形状由 api/openapi.yaml 决定。 */
export type Plan = components['schemas']['Plan']

/** 计划里的一个子计划。 */
export type Subplan = components['schemas']['Subplan']

/** 一个开发单元。★ `role_id` 必填——每个单元都要派人（裁定三）。 */
export type PlanUnit = components['schemas']['PlanUnit']
