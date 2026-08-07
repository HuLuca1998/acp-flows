import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import styles from './Timeline.module.css'
import { rendererFor, type TimelineEvent } from './event-registry'

export type TimelineProps = {
  events: TimelineEvent[]
  /** 被关掉的事件类型；不传表示全显示。 */
  hidden?: ReadonlySet<string>
}

/** 合并后的一段：连续的同类文本片段并成一条。 */
type Segment = {
  key: string
  type: string
  /** 合并进来的全部文本 */
  text: string
  /** 段内最后一条事件的序号，用来做 key 与调试 */
  lastSeq: number
  count: number
}

/**
 * 时间线（验收点 V6）。
 *
 * ★ 渲染形态全部来自 `event-registry`，**这里没有一个 switch**
 * （`U2.3.2` 的 forbidden_changes 明写禁止）。加一类事件只加一条注册。
 */
export function Timeline({ events, hidden }: TimelineProps) {
  const { t } = useTranslation()

  const segments = useMemo(() => mergeEvents(events, hidden), [events, hidden])

  if (segments.length === 0) {
    return <p className={styles.empty}>{t('timeline.empty')}</p>
  }

  return (
    <div className={styles.list}>
      {segments.map((seg) => {
        const renderer = rendererFor(seg.type)
        return (
          <div
            key={seg.key}
            className={`${styles.item} ${styles[renderer.shape]}`}
            data-event-type={seg.type}
            data-shape={renderer.shape}
          >
            <span className={styles.label}>{t(renderer.labelKey)}</span>
            <span className={styles.text}>{seg.text}</span>
          </div>
        )
      })}
    </div>
  )
}

/**
 * 把事件列表合并成显示用的段。
 *
 * ★ **只有 `merge: true` 的类型才合并**（文本流）。工具调用两次就是两次——
 * 合并的话用户会以为 AI 只动了一个文件。
 *
 * 合并的意义在于「不闪烁」：流式文本一个字一个字地来，每片一个气泡的话，
 * 界面会在打字过程中疯狂重排。
 */
function mergeEvents(events: TimelineEvent[], hidden?: ReadonlySet<string>): Segment[] {
  const out: Segment[] = []

  for (const e of events) {
    const type = e.type ?? ''
    if (hidden?.has(type) === true) {
      continue
    }

    const renderer = rendererFor(type)
    const text = textOf(e)
    const last = out[out.length - 1]

    // 只有同类、且这一类允许合并时才接上去
    if (renderer.merge === true && last !== undefined && last.type === type) {
      last.text += text
      last.lastSeq = e.seq ?? last.lastSeq
      last.count += 1
      continue
    }

    out.push({
      key: `${type}-${e.seq ?? out.length}`,
      type,
      text,
      lastSeq: e.seq ?? 0,
      count: 1,
    })
  }

  return out
}

/**
 * 从载荷里取要显示的文本。
 *
 * 取不到时返回空串而不是抛——**一条载荷形状意外的事件不该让整个时间线白屏**。
 * 后端加字段、改结构时，用户看到的应该是「这条少了点东西」而不是整页没了。
 */
function textOf(e: TimelineEvent): string {
  const payload = e.payload as Record<string, unknown> | undefined
  if (payload === undefined) {
    return ''
  }
  const text = payload.text
  return typeof text === 'string' ? text : ''
}
