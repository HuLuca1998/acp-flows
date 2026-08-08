import { useCallback, useEffect, useMemo, useRef, useState } from 'react'

import { answerPermission } from '@/api/system'

import type { TimelineEvent } from '../timeline/event-registry'

import type { PermissionOption, PermissionRequest, PermissionToolKind } from './model'

export type PermissionsState = {
  /** 还没应答的请求，按事件顺序。 */
  asks: PermissionRequest[]
  /** 提交用户的选择。失败时抛出——上层据此把卡片留下并提示。 */
  decide: (askID: string, optionID: string) => Promise<void>
}

/**
 * 从事件流里挑出待裁决的权限请求。
 *
 * ★ **已应答的要自己记。** 事件流不会撤回历史事件——用户点完之后一刷新，
 * 同一条 `request_permission` 还在流里。不记的话卡片又回来了，
 * 而他会以为自己上次没点上。
 *
 * ★ 载荷缺字段时**跳过这一条**，不让整个时间线白屏。后端加字段、改结构时，
 * 用户看到的应该是「少了一张卡片」而不是整页没了。
 */
export function usePermissions(workID: string | null, events: TimelineEvent[]): PermissionsState {
  const [settled, setSettled] = useState<ReadonlySet<string>>(new Set())
  // 切工作时清空——「已应答」是上一个工作的事
  const lastWork = useRef(workID)
  useEffect(() => {
    if (lastWork.current !== workID) {
      lastWork.current = workID
      setSettled(new Set())
    }
  }, [workID])

  const asks = useMemo(() => {
    if (workID === null || workID === '') {
      return []
    }
    const out: PermissionRequest[] = []
    for (const e of events) {
      if (e.type !== 'request_permission') {
        continue
      }
      const ask = toRequest(e.payload)
      // 已应答的在下面过滤，这里只管解析
      if (ask !== null) {
        out.push(ask)
      }
    }
    return out
  }, [workID, events])

  const decide = useCallback(
    async (askID: string, optionID: string) => {
      if (workID === null || workID === '') {
        return
      }
      // ★ 先提交再记，**失败时不记**：记了的话卡片消失而 AI 还在等，
      // 用户对着不动的界面等下去。
      await answerPermission(workID, askID, optionID)
      setSettled((prev) => new Set(prev).add(askID))
    },
    [workID],
  )

  return { asks: asks.filter((a) => !settled.has(a.id)), decide }
}

/**
 * 把契约的载荷转成组件要的形状。
 *
 * ★ 字段名要从蛇形转成驼峰（`option_id` → `optionId`）。转错的话，
 * 按钮上的 id 是 undefined，用户点了什么都发不出去。
 *
 * 缺必要字段时返回 null——跳过一条总比白屏好。
 */
function toRequest(payload: unknown): PermissionRequest | null {
  if (payload === null || typeof payload !== 'object') {
    return null
  }
  const p = payload as Record<string, unknown>

  const id = str(p.ask_id)
  const toolCallId = str(p.tool_call_id)
  if (id === '' || toolCallId === '') {
    return null
  }
  // ★ 必须是数组。零个元素**不算错**——卡片会说清楚「没法处理」，
  // 而悄悄跳过的话 AI 挂在那儿，用户完全不知道。
  if (!Array.isArray(p.options)) {
    return null
  }

  const options: PermissionOption[] = []
  for (const raw of p.options) {
    if (raw === null || typeof raw !== 'object') {
      continue
    }
    const o = raw as Record<string, unknown>
    const optionId = str(o.option_id)
    if (optionId === '') {
      continue
    }
    options.push({
      optionId,
      name: str(o.name) || optionId,
      kind: str(o.kind) as PermissionOption['kind'],
    })
  }

  const req: PermissionRequest = {
    id,
    toolCallId,
    runtime: str(p.runtime) || 'AI',
    kind: (str(p.kind) || 'other') as PermissionToolKind,
    options,
  }
  // 没有的字段就不放：tsconfig 开着 exactOptionalPropertyTypes，
  // 而且空串会让界面画出一个空的等宽块
  const path = str(p.path)
  if (path !== '') {
    req.path = path
  }
  if (p.out_of_bounds === true) {
    req.outOfBounds = true
  }
  return req
}

function str(v: unknown): string {
  return typeof v === 'string' ? v : ''
}
