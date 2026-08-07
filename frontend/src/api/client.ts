import createClient from 'openapi-fetch'

import type { paths } from './gen/schema'

/**
 * 后端 API 客户端。
 *
 * ★ 用 `openapi-fetch` 而不是裸 `fetch`：路径与请求/响应类型由
 * `api/openapi.yaml` 的生成物约束，契约一变编译就红（铁律 2）。
 * 裸 fetch 会让「后端改了字段名」这类问题一路溜到运行时。
 */

/** 开发态 token。生产形态由 Tauri 壳注入（M1 的 U1.1.1）。 */
const DEV_TOKEN = 'dev-local-token'

function token(): string {
  const injected = (globalThis as { __DUET__?: { token?: string } }).__DUET__
  return injected?.token ?? DEV_TOKEN
}

export const api = createClient<paths>({
  baseUrl: '/v1',
  headers: {
    get Authorization() {
      return `Bearer ${token()}`
    },
  },
})

/** 后端返回的 RFC 9457 Problem。`type` 是机器可读错误码，前端据此查 i18n 词条。 */
export type Problem = { type?: string; title?: string }

/**
 * 把 openapi-fetch 的 `{ data, error }` 收敛成「成功返回值 / 失败抛异常」。
 *
 * ★ 失败一律**抛出**，绝不返回一个「看起来正常」的默认值。
 * 检查更新那条路径尤其致命：静默降级会让网络故障伪装成「已是最新版本」。
 */
export function unwrap<T>(result: { data?: T; error?: unknown }): T {
  if (result.error !== undefined && result.error !== null) {
    const problem = result.error as Problem
    throw new Error(
      typeof problem.type === 'string' && problem.type !== '' ? problem.type : 'request_failed',
    )
  }
  if (result.data === undefined) throw new Error('empty_response')
  return result.data
}
