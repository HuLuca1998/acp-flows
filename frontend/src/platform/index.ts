/**
 * 平台适配层。**全仓库唯一可以 import `@tauri-apps/*` 的地方**
 * （其余文件 import 会被 ESLint 拦下，见 frontend/AGENTS.md）。
 *
 * 界面代码不该知道自己跑在 Tauri 壳里还是浏览器里——
 * 它只问「能不能自更新」，拿到什么就走什么路径。
 */

/** 发布页地址。Web 形态下的降级出口。 */
export const RELEASES_URL = 'https://github.com/HuLuca1998/acp-flows/releases/latest'

export type Capabilities = {
  /** 能不能就地自更新（下载 → 安装 → 重启）。浏览器里恒为 false。 */
  canSelfUpdate: boolean
}

/**
 * 检测运行形态。
 *
 * ★ **靠壳注入的标记判断，不靠 User-Agent 猜。**
 * UA 可以被改、可以被代理伪造，而且 Tauri 的 WebView UA 与 Safari 极像——
 * 猜错的代价是「在浏览器里显示了一个点不动的更新按钮」。
 */
export function capabilities(): Capabilities {
  const w = globalThis as { __TAURI_INTERNALS__?: unknown }
  return { canSelfUpdate: w.__TAURI_INTERNALS__ !== undefined }
}

/** 下载进度回调。`total` 为 0 表示服务端没给 Content-Length。 */
export type ProgressHandler = (downloaded: number, total: number) => void

/**
 * 下载并安装更新，然后重启应用。
 *
 * ★ 只有用户点了按钮才会走到这里——**绝不自动下载、绝不自动安装**
 * （docs/adr/0002 与 M1 全局停止条件第 2 条）。
 *
 * 调用方必须**先调 `/v1/system/update/prepare` 并确认放行**，
 * 拿到 `blocked` 时不得继续（那意味着有工作在跑，装下去会丢掉它）。
 *
 * 在 Web 形态下调用会抛——界面本就不该给出这个入口。
 */
export async function downloadAndInstall(onProgress?: ProgressHandler): Promise<void> {
  if (!capabilities().canSelfUpdate) {
    throw new Error('update_not_supported')
  }

  // 动态 import：浏览器形态下这些模块根本不会被加载，
  // 静态 import 会把 Tauri 的运行时代码打进 Web 版的首屏包里。
  const { check } = await import('@tauri-apps/plugin-updater')
  const { relaunch } = await import('@tauri-apps/plugin-process')

  const update = await check()
  if (update === null) {
    // 后端说有更新、updater 说没有 —— 两个真源不一致，如实报错而不是假装成功
    throw new Error('update_not_found_by_updater')
  }

  let downloaded = 0
  let total = 0
  await update.downloadAndInstall((event) => {
    switch (event.event) {
      case 'Started':
        total = event.data.contentLength ?? 0
        break
      case 'Progress':
        downloaded += event.data.chunkLength
        onProgress?.(downloaded, total)
        break
      case 'Finished':
        onProgress?.(total, total)
        break
    }
  })

  await relaunch()
}
