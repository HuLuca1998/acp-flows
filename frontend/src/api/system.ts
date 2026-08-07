import type { UpdateStatus } from '@/models/update'

import { api, unwrap } from './client'

/**
 * 检查应用更新。**只检查，绝不下载、绝不安装**（docs/adr/0002）。
 *
 * 失败时抛错——调用方必须显式处理，不能当成「已是最新」。
 */
export async function checkUpdate(): Promise<UpdateStatus> {
  return unwrap(await api.POST('/system/update/check'))
}
