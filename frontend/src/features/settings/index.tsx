import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Skeleton } from '@/ui/Skeleton'

import styles from './Settings.module.css'
import { SettingsNav } from './SettingsNav'
import { DEFAULT_SECTION, type SettingsSectionId } from './section-registry'
import { LanguageSection } from './sections/LanguageSection'
import { ProjectSection } from './sections/ProjectSection'
import { RuntimeSection } from './sections/RuntimeSection'
import { UpdateSection } from './sections/UpdateSection'
import { useProjects } from './use-projects'
import { useRuntimes } from './use-runtimes'
import { useUpdateFlow } from './use-update-flow'

/**
 * 设置页。左侧二级导航 + 右侧内容，六个分区，照 `design/ACP Duet 1a.dc.html`。
 *
 * ★ **两个数据 hook 都挂在这一层，不挂进分区组件里。**
 * 挂进去的话，每切一次分区就 unmount / remount 一次，于是重新请求——
 * 而 Runtime 探测要拉起子进程，用户会感到「点一下卡一下」。
 *
 * ★ 更新检查是**进页面时查一次，不轮询**（docs/adr/0007 修订 3）：
 * 用户不看设置页的时候，提示他有更新也没用。
 */
export function SettingsPage() {
  const { t } = useTranslation()
  const [current, setCurrent] = useState<SettingsSectionId>(DEFAULT_SECTION)
  const update = useUpdateFlow()
  const runtimes = useRuntimes()
  const projects = useProjects()
  const { check } = update

  // 进设置页时检查一次，**不轮询**（docs/adr/0007 修订 3）
  useEffect(() => {
    void check()
  }, [check])

  return (
    <div className={styles.page}>
      <h2 className={styles.title}>{t('nav.settings')}</h2>

      <div className={styles.body}>
        <SettingsNav current={current} onSelect={setCurrent} />
        <div className={styles.pane}>{renderSection()}</div>
      </div>
    </div>
  )

  function renderSection() {
    switch (current) {
      case 'runtime':
        return (
          <RuntimeSection
            runtimes={runtimes.runtimes}
            loading={runtimes.loading}
            errorCode={runtimes.errorCode}
            onRetry={() => void runtimes.refresh()}
          />
        )
      case 'env':
        return <PlaceholderSection titleKey="settings.env.title" hintKey="settings.env.hint" />
      case 'update':
        return (
          <UpdateSection
            status={update.status}
            phase={update.phase}
            errorCode={update.errorCode}
            blocked={update.blocked}
            progress={update.progress}
            onCheck={() => void update.check()}
            onUpdate={() => void update.applyUpdate()}
          />
        )
      case 'projects':
        return (
          <ProjectSection
            projects={projects.projects}
            loading={projects.loading}
            errorCode={projects.errorCode}
            canPickDirectory={projects.canPickDirectory}
            onAdd={(path) => void projects.add(path)}
            onRemove={(id) => void projects.remove(id)}
          />
        )
      case 'github':
        return <PlaceholderSection titleKey="settings.github.title" hintKey="settings.github.hint" />
      case 'general':
        return <LanguageSection />
    }
  }
}

/** 还没做的分区。骨架里**不含任何数字**——编造数据比空白更糟。 */
function PlaceholderSection({ titleKey, hintKey }: { titleKey: string; hintKey: string }) {
  const { t } = useTranslation()
  return (
    <section className={styles.section}>
      <h3 className={styles.heading}>{t(titleKey)}</h3>
      <Skeleton hintKey={hintKey} rows={2} />
    </section>
  )
}
