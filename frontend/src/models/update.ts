import type { components } from '@/api/gen/schema'

/**
 * 更新状态。**类型直接取自 OpenAPI 生成物**，不手写一份平行的定义——
 * 手写的那份会在契约变化时静默失配（铁律 2）。
 */
export type UpdateStatus = components['schemas']['UpdateStatus']

/** 更新前准备的结果。`blocked` 时前端**不得继续安装**。 */
export type UpdatePrepareResult = components['schemas']['UpdatePrepareResult']
