import { fileURLToPath, URL } from 'node:url'

import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'

// dev-web 形态：vite 把 /v1 代理到 duetd，前端不需要知道端口。
// 见 docs/architecture.md §2。
const DUETD_PORT = process.env.DUET_PORT ?? '7777'

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  server: {
    port: 5173,
    proxy: {
      '/v1': {
        target: `http://127.0.0.1:${DUETD_PORT}`,
        changeOrigin: true,
      },
    },
  },
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: ['./tests/setup.ts'],
    css: true,
    coverage: { reporter: ['text', 'lcov'], include: ['src/**'] },
  },
})
