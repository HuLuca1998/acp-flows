import { defineConfig } from '@playwright/test'

/**
 * 端到端：跑**真实 duetd**（临时数据目录 + Fake ACP Runtime），不是 mock 服务。
 *
 * 只测跨越整个系统才能验证的行为。单个模块能测出来的一律下沉——
 * E2E 慢且脆，塞进去的每条用例都要付长期维护成本。见 e2e/AGENTS.md。
 */
const WEB_PORT = 5173
const API_PORT = 7777

export default defineConfig({
  testDir: './specs',
  fullyParallel: false, // 共用一个 duetd 实例，并行会互相污染数据
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  reporter: process.env.CI ? [['html'], ['github']] : 'list',

  use: {
    baseURL: `http://localhost:${WEB_PORT}`,
    trace: 'on-first-retry',
    // 失败时留截图，但**截图不是唯一证据**——断言要基于 role/text
    screenshot: 'only-on-failure',
  },

  // ★ 两个服务都要起。只起前端的话，所有打后端的用例都会撞
  // `ECONNREFUSED 127.0.0.1:7777` —— 而那看起来像「后端挂了」，
  // 实际是「后端从来没被启动过」。这个配置曾经就是只起前端，
  // 导致 e2e 在 main 上一直是红的。
  webServer: [
    {
      // 数据目录指向临时路径，绝不碰 ~/.acpflows（铁律 6）。
      // DUET_DATA_DIR 是**家目录替身**，duetd 会在它下面再建 .acpflows/。
      // ★ `-dev` 不能省：不带它时 duetd 监听**内核随机分配的端口**
      // （生产形态：端口与 token 写进 session.json）。
      // 随机端口下 vite 代理与下面的 port 探活都对不上，
      // 症状是 `ECONNREFUSED 127.0.0.1:7777` 与 webServer 超时。
      command: 'go run ./cmd/duetd -dev',
      cwd: '../backend',
      env: {
        DUET_DATA_DIR: process.env.DUET_E2E_DATA_DIR ?? '/tmp/duet-e2e',
        DUET_LOG: 'info',
      },
      // ★ 用 port 而不是 url：全部端点都要 Bearer token，
      // 拿 url 探活会一直收到 401，Playwright 会一直等到超时。
      port: API_PORT,
      reuseExistingServer: !process.env.CI,
      timeout: 120_000, // 首次 go run 要编译
      stdout: 'pipe',
      stderr: 'pipe',
    },
    {
      command: 'pnpm -C ../frontend dev',
      url: `http://localhost:${WEB_PORT}`,
      reuseExistingServer: !process.env.CI,
      timeout: 60_000,
    },
  ],
})
