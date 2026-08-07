import { useCallback, useState } from 'react'

/**
 * 存到 localStorage 的键。集中在这里，避免各处拼字符串拼错。
 *
 * 前缀 `duet.` 是为了在浏览器形态下与同域的其他应用隔开。
 */
export const STORAGE_KEYS = {
  locale: 'duet.locale',
  railOpen: 'duet.rail.open',
  contextOpen: 'duet.context.open',
  page: 'duet.page',
  railWidth: 'duet.rail.width',
  contextWidth: 'duet.context.width',
} as const

/**
 * 读一个持久化的值。**读失败一律返回 fallback，绝不抛**——
 * 隐私模式、存储配额满、值被手工改坏，都不该让整个界面白屏。
 */
export function readPersisted<T>(key: string, fallback: T): T {
  try {
    const raw = globalThis.localStorage?.getItem(key)
    return raw === null || raw === undefined ? fallback : (JSON.parse(raw) as T)
  } catch {
    return fallback
  }
}

/** 写一个持久化的值。写失败静默忽略——存不下不是功能故障。 */
export function writePersisted(key: string, value: unknown): void {
  try {
    globalThis.localStorage?.setItem(key, JSON.stringify(value))
  } catch {
    // 隐私模式下 setItem 会抛。界面照常工作，只是下次打开回到默认值。
  }
}

/**
 * 像 useState，但值会存进 localStorage，重开应用后还在。
 *
 * 用于「用户调过的界面偏好」——折叠了哪些栏、选了什么语言。
 * 每次重开都被打回默认值的界面会让人觉得自己的调整不被尊重。
 */
export function usePersistedState<T>(key: string, initial: T): [T, (next: T) => void] {
  const [value, setValue] = useState<T>(() => readPersisted(key, initial))

  const set = useCallback(
    (next: T) => {
      setValue(next)
      writePersisted(key, next)
    },
    [key],
  )

  return [value, set]
}
