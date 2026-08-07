/**
 * 13 类事件的封闭枚举。与 api/openapi.yaml 的 `Event.type` 一一对应。
 *
 * ★ 新增第 14 类要**同时改四处**，缺一不可：
 *   1. api/openapi.yaml 的 Event.type 枚举
 *   2. docs/spec/architecture.md §4 的表
 *   3. design/Duet Spec.dc.html 第 07 节
 *   4. features/conversation/renderers 的注册表
 */
export const EVENT_SOURCE = { acp: 'acp', app: 'app' } as const
export type EventSource = (typeof EVENT_SOURCE)[keyof typeof EVENT_SOURCE]

/** 来自 ACP Runtime 的事件：只做展示与折叠。 */
export const ACP_EVENT_TYPE = {
  messageChunk: 'message_chunk',
  thoughtChunk: 'thought_chunk',
  toolCall: 'tool_call',
  requestPermission: 'request_permission',
  turnEnd: 'turn_end',
} as const

/** 来自应用控制层的事件：**永远可点开到对应的结构化产物**。 */
export const APP_EVENT_TYPE = {
  planVersion: 'plan_version',
  unitContract: 'unit_contract',
  stateChange: 'state_change',
  injection: 'injection',
  memoryCandidate: 'memory_candidate',
  decision: 'decision',
  evidence: 'evidence',
  checkpoint: 'checkpoint',
} as const

export const EVENT_TYPE = { ...ACP_EVENT_TYPE, ...APP_EVENT_TYPE } as const
export type EventType = (typeof EVENT_TYPE)[keyof typeof EVENT_TYPE]

export const ALL_EVENT_TYPES: readonly EventType[] = Object.values(EVENT_TYPE)
