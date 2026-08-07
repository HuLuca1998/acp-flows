import { afterEach, describe, expect, it } from 'vitest'

import { capabilities, downloadAndInstall, RELEASES_URL } from './index'

// M1 U1.1.1 · 平台适配层
//
// 验收标准见 docs/plan/milestones/M1-install-and-update.md S1.1。

const w = globalThis as { __TAURI_INTERNALS__?: unknown }

afterEach(() => {
  delete w.__TAURI_INTERNALS__
})

describe('运行形态检测', () => {
  it('浏览器里不能自更新', () => {
    expect(capabilities().canSelfUpdate).toBe(false)
  })

  it('壳注入标记后可以自更新', () => {
    w.__TAURI_INTERNALS__ = {}
    expect(capabilities().canSelfUpdate).toBe(true)
  })

  // ★ 靠壳注入的标记判断，不靠 UA 猜。
  // Tauri 的 WebView UA 与 Safari 极像，猜错的代价是
  // 「在浏览器里显示了一个点不动的更新按钮」。
  it('不看 User-Agent', () => {
    const original = navigator.userAgent
    Object.defineProperty(navigator, 'userAgent', {
      value: 'Mozilla/5.0 (Macintosh) Tauri/2.0',
      configurable: true,
    })
    expect(capabilities().canSelfUpdate).toBe(false)
    Object.defineProperty(navigator, 'userAgent', { value: original, configurable: true })
  })
})

describe('Web 降级', () => {
  // ★ 降级必须**真的可用**，不能是个空函数。
  it('给出真实的发布页地址', () => {
    expect(RELEASES_URL).toMatch(/^https:\/\/github\.com\/.+\/releases/)
  })

  // 浏览器里调自更新要明确失败，不能静默什么都不做——
  // 静默会让用户点了按钮以为在更新，其实什么都没发生。
  it('浏览器里调自更新会抛出可辨识的错误', async () => {
    await expect(downloadAndInstall()).rejects.toThrow('update_not_supported')
  })
})
