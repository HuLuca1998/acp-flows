import { describe, expect, it } from 'vitest'

import { EVENT_KINDS, FILTER_GROUPS, rendererFor } from './event-registry'

// M2 U2.3.2 · 时间线的事件注册表（验收点 V6）
//
// ★ forbidden_changes 明写「用 switch 分发事件类型（用注册表）」。
// 这份测试守的就是那条：加一类事件**只加一行注册**，不改任何既有代码。

describe('事件注册表', () => {
  // 与 api/openapi.yaml 的 Event.type 枚举一一对应。
  // 少一类，那类事件到了界面上就什么也不显示——而没有任何报错。
  const CONTRACT_TYPES = [
    // 来自 ACP Runtime
    'message_chunk',
    'thought_chunk',
    'tool_call',
    'request_permission',
    'turn_end',
    // 来自应用控制层
    'plan_version',
    'unit_contract',
    'state_change',
    'injection',
    'memory_candidate',
    'decision',
    'evidence',
    'checkpoint',
  ] as const

  it('契约里的 13 类事件每一类都有渲染器', () => {
    expect(CONTRACT_TYPES).toHaveLength(13)

    for (const type of CONTRACT_TYPES) {
      const renderer = rendererFor(type)
      // ★ 断言「不是兜底」而不是「defined」。只看 defined 的话，
      // 漏注册一类事件时兜底渲染器会接住它，这条照样绿——
      // 第一版就是这么写的，造负例才发现。
      expect(
        renderer.fallback,
        `事件 ${type} 没有注册，落到了兜底上——用户会看到「暂时看不懂的记录」`,
      ).toBeUndefined()
      expect(renderer.labelKey).toBeTruthy()
    }
  })

  it('注册表里不多出契约没有的类型', () => {
    // 多出来的话，说明契约改了而这里没跟上——那一类永远不会到达
    for (const type of EVENT_KINDS) {
      expect(CONTRACT_TYPES).toContain(type)
    }
  })

  // ★ R3：没见过的类型**不能让页面炸**。
  //
  // 后端加一类事件、前端还没跟上时，用户看到的应该是「有一条我暂时看不懂的记录」，
  // 而不是一片白屏——白屏会让他以为整个应用坏了。
  it('未知类型返回兜底渲染器而不是 undefined', () => {
    const renderer = rendererFor('something_from_the_future')
    expect(renderer, '未知类型没有兜底——页面会白屏').toBeDefined()
    expect(renderer.fallback).toBe(true)
  })

  // ★ R2：过滤器分组与设计稿逐字一致。
  //
  // 文案对不上的话，用户在设计稿截图与实际界面之间对照会以为点错了地方。
  it('过滤器按设计稿分成两组，标题逐字一致', () => {
    // 用 map 取而不是下标：下标在类型上可能是 undefined，
    // 而 `!` 会把「数组实际为空」这种失败静默成 TypeError
    expect(FILTER_GROUPS.map((g) => g.titleKey)).toEqual([
      'timeline.filter.groupAcp',
      'timeline.filter.groupApp',
    ])
  })

  it('每个过滤项都指向真实存在的事件类型', () => {
    const covered = new Set<string>()
    for (const group of FILTER_GROUPS) {
      for (const item of group.items) {
        for (const type of item.types) {
          expect(CONTRACT_TYPES, `过滤项 ${item.id} 指向了不存在的 ${type}`).toContain(type)
          covered.add(type)
        }
      }
    }

    // 每一类都能被某个过滤项管到——管不到的那类，用户没办法把它关掉
    for (const type of CONTRACT_TYPES) {
      expect(covered, `事件 ${type} 不属于任何过滤项，用户关不掉它`).toContain(type)
    }
  })

  // R1：加一类事件只需注册一行。
  //
  // 这条用「注册表是数据」来保证：EVENT_KINDS 由注册表推导，
  // 没有任何地方写 switch。真有人改成 switch 的话，
  // 上面「13 类都有渲染器」那条会在他漏掉一个 case 时红。
  it('注册表是数据，事件类型由它推导而不是各处硬编码', () => {
    expect(EVENT_KINDS.length).toBe(13)
    // 顺序无所谓，但内容必须与契约一致
    expect([...EVENT_KINDS].sort()).toEqual([...CONTRACT_TYPES].sort())
  })
})
