import { beforeEach, describe, expect, it, vi } from 'vitest'

import { readPersisted, STORAGE_KEYS, writePersisted } from './persisted'

// M0 U0.1.1 R4 / U0.2.1 R4 · 界面偏好的持久化
//
// ★ 这里的契约是「坏掉也不能让界面崩」：隐私模式、配额满、值被手工改坏，
// 都只该退回默认值，绝不抛异常——一个存储问题不该导致整页白屏。

describe('持久化偏好', () => {
  beforeEach(() => {
    globalThis.localStorage.clear()
  })

  it('写进去能读回来', () => {
    writePersisted(STORAGE_KEYS.locale, 'en-US')
    expect(readPersisted(STORAGE_KEYS.locale, 'zh-CN')).toBe('en-US')
  })

  it('没存过时返回默认值', () => {
    expect(readPersisted(STORAGE_KEYS.railOpen, true)).toBe(true)
    expect(readPersisted(STORAGE_KEYS.contextOpen, false)).toBe(false)
  })

  // ★ 值被手工改成非法 JSON 时退回默认，不抛。
  it('存储里的值坏掉时退回默认值，不抛异常', () => {
    globalThis.localStorage.setItem(STORAGE_KEYS.locale, '{不是合法 JSON')
    expect(() => readPersisted(STORAGE_KEYS.locale, 'zh-CN')).not.toThrow()
    expect(readPersisted(STORAGE_KEYS.locale, 'zh-CN')).toBe('zh-CN')
  })

  // ★ 隐私模式下 setItem 会抛。存不下不是功能故障，界面要照常工作。
  it('写入失败时静默忽略，不影响界面', () => {
    const spy = vi.spyOn(globalThis.localStorage, 'setItem').mockImplementation(() => {
      throw new Error('QuotaExceededError')
    })
    expect(() => writePersisted(STORAGE_KEYS.railOpen, false)).not.toThrow()
    spy.mockRestore()
  })

  it('布尔值往返不丢类型', () => {
    writePersisted(STORAGE_KEYS.railOpen, false)
    expect(readPersisted(STORAGE_KEYS.railOpen, true)).toBe(false)
  })
})
