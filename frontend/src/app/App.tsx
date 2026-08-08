import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { listProjects } from '@/api/system'
import { ChatPage, type ChatIntent } from '@/features/chat'
import { ContextPanel } from '@/features/context'
import { Rail } from '@/features/rail'
import type { Project } from '@/models/project'
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

  // 面包屑要显示**真实的**当前项目。null 表示还没查到或一个都没有。
  const [project, setProject] = useState<Project | null>(null)
  // 左栏点「新建对话」/「打开工作」时，把意图传给对话页。
  // ★ 用一个带序号的对象而不是裸字符串：同一个项目连点两次「新建对话」，
  // 裸字符串不变，对话页不会有反应——而用户明明点了两下。
  const [intent, setIntent] = useState<ChatIntent | null>(null)
  const [intentSeq, setIntentSeq] = useState(0)

  const openIntent = (next: ChatIntent) => {
    setIntent(next)
    setIntentSeq((n) => n + 1)
    setPageId(DEFAULT_PAGE) // 左栏点的是对话，就切回对话主区
  }

  useEffect(() => {
    void (async () => {
      try {
        const list = await listProjects()
        setProject(list[0] ?? null)
      } catch {
        // 查不到就当没有——**绝不让整个窗口白掉**。后端没起来时，
        // 用户最需要的正是这条窗口栏（他要点进设置页看看怎么回事）。
        setProject(null)
      }
    })()
  }, [])

  const navPage = navPageById(pageId)
  const showContext = hasContextPanel(pageId) && contextOpen

  return (
    <div className={styles.shell}>
      {/* ★ data-tauri-drag-region：按住这一条能拖动窗口。
          Overlay 窗口没有系统标题栏可抓，不标的话整个应用拖不动。

          ★★ 注意：Tauri 检查的是 **mousedown 的目标元素本身**有没有这个属性，
          **不会向上找父元素**。所以窗口栏里每一个非交互的子元素都要单独标，
          否则会出现「按空白处能拖、按到文字上就拖不动」这种时灵时不灵的现象。
          交互元素（按钮）**不要标** —— 标了点击会变成拖拽。 */}
      <header className={styles.titlebar} data-tauri-drag-region>
        {/* macOS 交通灯的占位。真窗口里由系统绘制，Web 形态下留出等宽空间，
            否则从浏览器切到 App 时整条窗口栏会横向跳一下。 */}
        <span className={styles.trafficLights} aria-hidden="true" data-tauri-drag-region />

        <Button
          icon="ph-sidebar-simple"
          label={t('nav.toggleRail')}
          shortcut="⌘B"
          active={railOpen}
          onClick={() => setRailOpen(!railOpen)}
        />

        <nav className={styles.breadcrumb} aria-label={t('nav.breadcrumb')} data-tauri-drag-region>
          {/* ★ 显示**真实的**当前项目。写死占位的话界面会说谎——
              真机上后端明明有项目，顶栏却一直写着「还没有项目」。
              而且原本那句还带着「项目管理即将上线」，可它早就在设置页上线了。 */}
          {project === null ? (
            <span className={styles.crumbMuted} data-tauri-drag-region>
              {t('common.state.noProject')}
            </span>
          ) : (
            <>
              <span className={styles.crumbRoot} data-tauri-drag-region>
                {project.name}
              </span>
              <span className={styles.crumbSep} aria-hidden="true" data-tauri-drag-region>
                /
              </span>
              <span className={styles.crumbCurrent} data-tauri-drag-region>
                {t(navPage?.titleKey ?? 'nav.chat')}
              </span>
            </>
          )}
        </nav>

        {/* 靠视口右缘：tooltip 必须右对齐，否则会把页面撑出横向滚动条 */}
        <div className={styles.titlebarRight} data-tt-align="end" data-tauri-drag-region>
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
          onNewWork={(projectPath) => openIntent({ kind: 'new', projectPath })}
          onOpenWork={(workID) => openIntent({ kind: 'open', workID })}
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
          {navPage === null ? (
            <ChatPage intent={intent} intentSeq={intentSeq} />
          ) : (
            <navPage.Component />
          )}
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
