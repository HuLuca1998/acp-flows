import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

// 图标字体。设计稿用的就是这一套（design/*.dc.html 引的 @phosphor-icons/web@2.1.1）。
// **走 npm 本地打包而不是 CDN**：离线要能用，Tauri 打包进 .app 也不能依赖外网。
// 只引 regular 一套 —— 设计规范禁止混用其他字重。
import '@phosphor-icons/web/regular'

import { App } from '@/app/App'
import '@/design/tokens.css'
import '@/design/duet.css'
import { initI18n } from '@/i18n'

const root = document.getElementById('root')
if (!root) throw new Error('#root not found')

// i18n 先初始化再渲染：否则首帧会闪出 key
void initI18n().then(() => {
  createRoot(root).render(
    <StrictMode>
      <App />
    </StrictMode>,
  )
})
