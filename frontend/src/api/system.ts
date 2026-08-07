import type { UpdatePrepareResult, UpdateStatus } from '@/models/update'

import { api, unwrap } from './client'

/**
 * 检查应用更新。**只检查，绝不下载、绝不安装**（docs/adr/0002）。
 *
 * 失败时抛错——调用方必须显式处理，不能当成「已是最新」。
 */
export async function checkUpdate(): Promise<UpdateStatus> {
  return unwrap(await api.POST('/system/update/check'))
}

/**
 * 更新前准备：判断现在更新会不会打断用户。
 *
 * ★ 返回 `blocked` 时前端**不得继续安装**——那意味着有工作在跑。
 * `blocked` 是业务结论不是错误，HTTP 状态仍是 200。
 */
export async function prepareUpdate(): Promise<UpdatePrepareResult> {
  return unwrap(await api.POST('/system/update/prepare'))
}
