import type { components } from '@/api/gen/schema'

/** 一个单元的契约。形状由 api/openapi.yaml 决定。 */
export type Contract = components['schemas']['Contract']

/** 写入边界。★ `allowed` 为空表示**什么都不许改**，不是什么都许。 */
export type WriteBoundary = components['schemas']['WriteBoundary']
