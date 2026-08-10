import type { components } from '@/api/gen/schema'

/**
 * 一个能接着做的工作。
 *
 * ★ `unit_id` 是「停在哪个单元」——没有它用户看到的只是一串 `work-0N`。
 */
export type ResumableWork = components['schemas']['ResumableWork']
