import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ChatPage } from '@/features/chat'
import { ContextPanel } from '@/features/context'
import { Rail } from '@/features/rail'
import { Button } from '@/ui/Button'
import { Resizer } from '@/ui/Resizer'
import { STORAGE_KEYS, usePersistedState } from '@/utils/persisted'

import styles from './App.module.css'
import {
  DEFAULT_PAGE,
  hasContextPanel,
  navPageById,
  normalizePageId,
  type PageId,
} from './pages'

/**
 * 应用骨架：窗口栏 + 左栏 + 主区 + 右栏。
 *
 * 结构严格照 design/ACP Duet 1a.dc.html：
 *   - 窗口栏统一收纳**三个**折叠开关（⧉ 左栏 · ▤ 计划 · ◫ 右栏）+ 面包屑
 *   - 左栏 252：5 项导航 + 项目树 + 最近 + Runtime 状态
 *   - 右栏 300：**只在对话主区出现**，其余页面是全宽内容页
 *
 * 尺寸全部取自设计令牌，不写裸 px —— stylelint 会拦。
 */
export function App() {
  const { t } = useTranslation()
  // 刷新后停在原地：被打回首页会让人以为自己的操作丢了。
  const [savedPage, setPageId] = usePersistedState<string>(STORAGE_KEYS.page, DEFAULT_PAGE)
  // 存储里的值可能被手工改坏或是旧版本遗留——规整一次，绝不白屏。
  const pageId: PageId = normalizePageId(savedPage)
  // 栏的折叠状态要留住：每次重开都被打回默认值，会让人觉得自己的调整不被尊重。
  const [railOpen, setRailOpen] = usePersistedState(STORAGE_KEYS.railOpen, true)
  const [contextOpen, setContextOpen] = usePersistedState(STORAGE_KEYS.contextOpen, true)
  const [planOpen, setPlanOpen] = useState(false)
  // 栏宽可拖，范围写死在设计规范 §06：左栏 180–420、右栏 220–460
  const [railWidth, setRailWidth] = usePersistedState(STORAGE_KEYS.railWidth, 252)
  const [contextWidth, setContextWidth] = usePersistedState(STORAGE_KEYS.contextWidth, 300)

  const navPage = navPageById(pageId)
  const showContext = hasContextPanel(pageId) && contextOpen

  return (
    <div className={styles.shell}>
      <header className={styles.titlebar}>
        {/* macOS 交通灯的占位。真窗口里由系统绘制，Web 形态下留出等宽空间，
            否则从浏览器切到 App 时整条窗口栏会横向跳一下。 */}
        <span className={styles.trafficLights} aria-hidden="true" />

        <Button
          icon="ph-sidebar-simple"
          label={t('nav.toggleRail')}
          shortcut="⌘B"
          active={railOpen}
          onClick={() => setRailOpen(!railOpen)}
        />

        <nav className={styles.breadcrumb} aria-label={t('nav.breadcrumb')}>
          <span className={styles.crumbMuted}>{t('common.state.noProject')}</span>
        </nav>

        {/* 靠视口右缘：tooltip 必须右对齐，否则会把页面撑出横向滚动条 */}
        <div className={styles.titlebarRight} data-tt-align="end">
          <Button
            icon="ph-tree-structure"
            label={t('nav.togglePlan')}
            active={planOpen}
            onClick={() => setPlanOpen((v) => !v)}
          />
          <Button
            icon="ph-sidebar"
            label={t('nav.toggleContext')}
            active={contextOpen}
            onClick={() => setContextOpen(!contextOpen)}
            disabled={!hasContextPanel(pageId)}
          />
        </div>
      </header>

      <div className={styles.body}>
        <Rail
          currentPage={pageId}
          onNavigate={setPageId}
          collapsed={!railOpen}
          width={railWidth}
        />
        {railOpen && (
          <Resizer
            width={railWidth}
            min={180}
            max={420}
            grow="right"
            onResize={setRailWidth}
            label={t('nav.resizeRail')}
          />
        )}

        <main className={styles.main}>
          {navPage === null ? <ChatPage /> : <navPage.Component />}
        </main>

        {showContext && (
          <Resizer
            width={contextWidth}
            min={220}
            max={460}
            grow="left"
            onResize={setContextWidth}
            label={t('nav.resizeContext')}
          />
        )}
        {showContext && <ContextPanel width={contextWidth} />}
      </div>

      {planOpen && (
        <div className={styles.planPanel} role="dialog" aria-label={t('nav.planPanel')}>
          <header className={styles.planHeader}>
            <span>{t('nav.planPanel')}</span>
            <Button icon="ph-x" label={t('common.action.close')} onClick={() => setPlanOpen(false)} />
          </header>
          <p className={styles.planHint}>{t('page.plan.hint')}</p>
        </div>
      )}
    </div>
  )
}
