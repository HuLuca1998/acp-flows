import type { components } from '@/api/gen/schema'

/** 一条事件。形状由 api/openapi.yaml 决定。 */
export type TimelineEvent = components['schemas']['Event']

/** 事件在时间线上的显示形态。 */
export type EventShape =
  /** 气泡：连续的文本片段合并进同一个气泡（消息、思考摘要）。 */
  | 'bubble'
  /** 卡片：有结构的一坨（工具调用、权限请求、契约）。 */
  | 'card'
  /** 单行：一句话说完的状态变化。 */
  | 'line'

/** 一类事件的渲染规格。 */
export type EventRenderer = {
  /** i18n 词条 key，**不写死文案**（forbidden_changes）。 */
  labelKey: string
  shape: EventShape
  /**
   * 连续同类事件是否合并。
   *
   * ★ 只有文本流该合并。工具调用两次就是两次——合并的话，
   * 用户会以为 AI 只动了一个文件。
   */
  merge?: boolean
  /**
   * 归并键在载荷里的字段名。
   *
   * ★ 有它的话，**同一个键值的多条事件合成一条**。工具调用需要：
   * ACP 会为同一次调用连发 `tool_call` + 若干 `tool_call_update`（状态变化），
   * 共用一个 `toolCallId`。不归并的话用户看到四条一模一样的「工具调用」，
   * 以为 AI 动了四个文件。
   *
   * 与 `merge` 的区别：`merge` 按「连续同类」并文本（流式消息），
   * 这个按「同一个业务对象」并成一条记录，中间可以隔着别的事件。
   */
  mergeKey?: string
  /**
   * 摘要取自哪些字段，**按顺序取第一个有值的**。
   *
   * 支持 `a.b` 形式的嵌套路径。写在注册表里而不是写提取函数，
   * 是为了守住「注册表是数据不是代码」——加一类事件仍然只加一条记录。
   */
  detailFrom?: readonly string[]
  /** 状态取自哪个字段；后来的覆盖先前的。 */
  statusFrom?: string
  /** 兜底渲染器的标记，正常注册的都没有这个字段。 */
  fallback?: boolean
}

/**
 * 事件注册表 —— **这是数据，不是代码**。
 *
 * ★ 加一类事件只加一条记录，不改任何既有代码
 * （`U2.3.2` 的 forbidden_changes 明写「禁止 switch 分发」）。
 *
 * 键与 `api/openapi.yaml` 的 `Event.type` 枚举一一对应；
 * 少一条，那类事件到了界面上会什么都不显示，而没有任何报错。
 */
const RENDERERS = {
  // ── 来自 ACP Runtime ────────────────────────────────
  message_chunk: { labelKey: 'timeline.event.messageChunk', shape: 'bubble', merge: true },
  thought_chunk: { labelKey: 'timeline.event.thoughtChunk', shape: 'bubble', merge: true },
  tool_call: {
    labelKey: 'timeline.event.toolCall',
    shape: 'card',
    // 同一次调用的开始与若干次状态更新合成一张卡片
    mergeKey: 'toolCallId',
    // Agent 给的 title 最好懂（「Read README.md」）；没有就退到文件路径，
    // 总比只显示「工具调用」四个字强
    detailFrom: ['title', 'rawInput.file_path', 'locations.0.path', 'kind'],
    statusFrom: 'status',
  },
  request_permission: {
    labelKey: 'timeline.event.requestPermission',
    shape: 'card',
    // ★ 时间线上留痕：用户回头要能看到「我当时被问了什么」。
    // 待裁决的那张可点的卡片由 features/permission 的 Dock 负责，
    // 这里只是历史记录——不显示路径的话，它就是一条空行。
    detailFrom: ['path', 'tool_call_id'],
  },
  turn_end: { labelKey: 'timeline.event.turnEnd', shape: 'line' },

  // ── 来自应用控制层（这些永远可点开到对应的结构化产物）──
  plan_version: { labelKey: 'timeline.event.planVersion', shape: 'card' },
  unit_contract: { labelKey: 'timeline.event.unitContract', shape: 'card' },
  state_change: { labelKey: 'timeline.event.stateChange', shape: 'line' },
  injection: { labelKey: 'timeline.event.injection', shape: 'line' },
  memory_candidate: { labelKey: 'timeline.event.memoryCandidate', shape: 'card' },
  decision: { labelKey: 'timeline.event.decision', shape: 'card' },
  evidence: { labelKey: 'timeline.event.evidence', shape: 'card' },
  checkpoint: { labelKey: 'timeline.event.checkpoint', shape: 'line' },
} as const satisfies Record<string, EventRenderer>

/** 已登记的事件类型。由注册表推导，不另外维护一份列表。 */
export const EVENT_KINDS = Object.keys(RENDERERS) as (keyof typeof RENDERERS)[]

/**
 * 兜底渲染器。
 *
 * ★ 后端加了一类事件而前端还没跟上时，用户看到的应该是
 * 「有一条我暂时看不懂的记录」，**而不是白屏**——白屏会让他以为整个应用坏了。
 */
const FALLBACK: EventRenderer = {
  labelKey: 'timeline.event.unknown',
  shape: 'line',
  fallback: true,
}

/** 取一类事件的渲染规格；没见过的返回兜底而不是 undefined。 */
export function rendererFor(type: string): EventRenderer {
  return (RENDERERS as Record<string, EventRenderer>)[type] ?? FALLBACK
}

/** 过滤器里的一项，可以管一类或几类事件。 */
export type FilterItem = {
  id: string
  labelKey: string
  /** 这一项管住的事件类型。 */
  types: readonly string[]
}

export type FilterGroup = {
  titleKey: string
  items: readonly FilterItem[]
}

/**
 * 过滤器分组 —— 照 `design/ACP Duet 1a.dc.html` 的「时间线显示」面板。
 *
 * 设计稿把两组分别标为「ACP 事件 · 来自 Runtime」与「应用事件 · 来自控制层」。
 * 这个分法对用户是有意义的：**前者是 AI 说的，后者是 Duet 自己做的**，
 * 出问题时该找谁不一样。
 */
export const FILTER_GROUPS: readonly FilterGroup[] = [
  {
    titleKey: 'timeline.filter.groupAcp',
    items: [
      { id: 'msg', labelKey: 'timeline.filter.msg', types: ['message_chunk'] },
      { id: 'think', labelKey: 'timeline.filter.think', types: ['thought_chunk'] },
      { id: 'tool', labelKey: 'timeline.filter.tool', types: ['tool_call'] },
      { id: 'perm', labelKey: 'timeline.filter.perm', types: ['request_permission'] },
      { id: 'stop', labelKey: 'timeline.filter.stop', types: ['turn_end'] },
    ],
  },
  {
    titleKey: 'timeline.filter.groupApp',
    items: [
      { id: 'plan', labelKey: 'timeline.filter.plan', types: ['plan_version'] },
      { id: 'contract', labelKey: 'timeline.filter.contract', types: ['unit_contract'] },
      { id: 'state', labelKey: 'timeline.filter.state', types: ['state_change', 'checkpoint'] },
      {
        id: 'inject',
        labelKey: 'timeline.filter.inject',
        types: ['injection', 'memory_candidate'],
      },
      { id: 'decision', labelKey: 'timeline.filter.decision', types: ['decision', 'evidence'] },
    ],
  },
] as const
