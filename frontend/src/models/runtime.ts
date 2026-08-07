import type { components } from '@/api/gen/schema'

/** 一个 ACP Runtime 的检测结论。形状由 api/openapi.yaml 决定。 */
export type Runtime = components['schemas']['Runtime']

/** 检测状态。四态，不是三态。 */
export type RuntimeStatus = NonNullable<Runtime['status']>

/**
 * 界面上一个 Runtime 需要用户做点什么吗。
 *
 * ★ 判断依据是 `status`，**不是 `name`**。
 * `if (name === 'codex')` 出现在这里就意味着加第三个 Runtime 要改前端，
 * 而修复命令本来就是后端给的（见 openapi 的 Runtime.remedy）。
 */
export function needsAttention(runtime: Runtime): boolean {
  return runtime.status !== 'ready'
}
