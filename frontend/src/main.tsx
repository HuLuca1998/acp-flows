import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

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
