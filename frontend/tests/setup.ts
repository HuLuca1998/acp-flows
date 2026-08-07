import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterEach, beforeAll } from 'vitest'

import { initI18n } from '../src/i18n'

// 每个测试之间清干净 DOM——残留的节点会让 getByRole 查到上一个测试的元素，
// 症状是「单跑绿、全跑红」，很难排查。
afterEach(cleanup)

// ★ jsdom 给的 localStorage 是个**空对象**：属性在，setItem/getItem/clear 全没有。
// 直接用会报 `localStorage.clear is not a function`。
//
// 这里补一个符合 Storage 语义的内存实现——**不是 mock**，是把 jsdom 缺的那块补齐，
// 测试仍然在测真实的读写契约（存进去能读回来、坏值退回默认、写失败不抛）。
if (typeof globalThis.localStorage?.setItem !== 'function') {
  const store = new Map<string, string>()
  Object.defineProperty(globalThis, 'localStorage', {
    configurable: true,
    value: {
      getItem: (k: string) => store.get(k) ?? null,
      setItem: (k: string, v: string) => void store.set(k, String(v)),
      removeItem: (k: string) => void store.delete(k),
      clear: () => store.clear(),
      key: (i: number) => [...store.keys()][i] ?? null,
      get length() {
        return store.size
      },
    },
  })
}

// ★ 用**真实词条**跑测试，不是 mock 掉 i18n。
//
// 不初始化的话 t() 会回退成 key，测试里断言的就变成了 `settings.update.check`
// 这种字符串——那既测不出文案对不对，也让「文案与设计稿逐字一致」这类
// 验收标准彻底失效。词条缺失、key 写错、中英不对齐都要在这里就暴露。
beforeAll(async () => {
  await initI18n()
})
