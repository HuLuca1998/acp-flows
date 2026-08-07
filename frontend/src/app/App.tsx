import { useTranslation } from 'react-i18next'

import { WORK_STATE } from '@/constants/state'
import { Button } from '@/ui/Button'

import styles from './App.module.css'

/**
 * 应用骨架。当前是 M0 的冒烟页面——只证明令牌、i18n、组件、构建这条链是通的。
 *
 * 真正的三栏布局（窗口栏 42 / 左栏 252 / 主区 800 / 右栏 300）在 M2 的 S2.9，
 * 规格见 docs/spec/frontend-guide.md §8。
 */
export function App() {
  const { t } = useTranslation()

  return (
    <div className={styles.shell}>
      <header className={styles.titlebar}>
        <Button icon="ph-sidebar-simple" label="折叠侧栏" shortcut="⌘B" />
        <span className={styles.brand}>{t('app.title')}</span>
        {/* 状态词是标识符不是文案：中英两版都保持英文原值、等宽显示 */}
        <code className={styles.state}>{WORK_STATE.executing}</code>
      </header>

      <main className={styles.main}>
        <p className={styles.hint}>{t('common.state.empty')}</p>
        <div className={styles.actions}>
          <Button variant="secondary">{t('common.action.later')}</Button>
          <Button variant="primary">{t('common.action.confirm')}</Button>
        </div>
      </main>
    </div>
  )
}
