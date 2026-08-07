/**
 * Work 状态。取值与后端 `internal/constant/state.go` 及 AGENTS.md §8 术语表一字不差。
 *
 * ★ 这些是**标识符不是文案**：中英两版界面里都保持英文原值、等宽显示、不翻译。
 * 把它们塞进 i18n 词条文件是错的，见 docs/i18n.md §2。
 *
 * 用 `as const` 对象而非 `enum`：enum 会生成运行时代码，
 * 且与 openapi 生成的字符串联合类型对不上。
 *
 * 共 11 个。设计规范 §09 只列了后 9 个——那是「对话状态行显示的」子集，
 * `initializing` 阶段还没有对话。见 docs/adr/0006 Q1。
 */
export const WORK_STATE = {
  /** 正在切 worktree 与分支；此时对话还没开始。新建工作的初始状态。 */
  initializing: 'initializing',
  /** worktree 创建失败。终态——没切成就没有可执行的现场，只能删掉重建。 */
  initializingFailed: 'initializing_failed',
  clarifying: 'clarifying',
  planning: 'planning',
  ready: 'ready',
  executing: 'executing',
  reviewingUnit: 'reviewing_unit',
  waitingUser: 'waiting_user',
  paused: 'paused',
  completed: 'completed',
  failed: 'failed',
} as const

export type WorkState = (typeof WORK_STATE)[keyof typeof WORK_STATE]

/** 全部合法取值，顺序即状态机的推进顺序。 */
export const ALL_WORK_STATES: readonly WorkState[] = Object.values(WORK_STATE)

/** 终态：没有任何出边。 */
export const TERMINAL_WORK_STATES: readonly WorkState[] = [
  WORK_STATE.initializingFailed,
  WORK_STATE.completed,
  WORK_STATE.failed,
]
