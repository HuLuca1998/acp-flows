import { defineConfig } from '@playwright/test'

/**
 * 端到端：跑**真实 duetd**（临时数据目录 + Fake ACP Runtime），不是 mock 服务。
 *
 * 只测跨越整个系统才能验证的行为。单个模块能测出来的一律下沉——
 * E2E 慢且脆，塞进去的每条用例都要付长期维护成本。见 e2e/AGENTS.md。
 */
const PORT = 5173

export default defineConfig({
  testDir: './specs',
  fullyParallel: false, // 共用一个 duetd 实例，并行会互相污染数据
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  reporter: process.env.CI ? [['html'], ['github']] : 'list',

  use: {
    baseURL: `http://localhost:${PORT}`,
    trace: 'on-first-retry',
    // 失败时留截图，但**截图不是唯一证据**——断言要基于 role/text
    screenshot: 'only-on-failure',
  },

  // 起真实服务：duetd 的数据目录指向临时路径，绝不碰 ~/.acpflows（铁律 6）
  webServer: {
    command: 'DUET_DATA_DIR=$(mktemp -d) pnpm -C ../frontend dev',
    url: `http://localhost:${PORT}`,
    reuseExistingServer: !process.env.CI,
    timeout: 60_000,
  },
})
