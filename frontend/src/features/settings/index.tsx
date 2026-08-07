import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'

import { Skeleton } from '@/ui/Skeleton'

import { LanguageSection } from './LanguageSection'
import { RuntimeSection } from './RuntimeSection'
import styles from './Settings.module.css'
import { UpdateSection } from './UpdateSection'
import { useRuntimes } from './use-runtimes'
import { useUpdateFlow } from './use-update-flow'

/**
 * 设置页。五个分区，未实现的用骨架占位。
 *
 * ★ 更新检查是**进页面时查一次，不轮询**（docs/adr/0007 修订 3）：
 * 轮询既费流量又没意义——用户不看设置页的时候，提示他有更新也没用。
 */
export function SettingsPage() {
  const { t } = useTranslation()
  const update = useUpdateFlow()
  const runtimes = useRuntimes()
  const { check } = update

  // 进设置页时检查一次，**不轮询**（docs/adr/0007 修订 3）
  useEffect(() => {
    void check()
  }, [check])

  return (
    <div className={styles.page}>
      <h2 className={styles.title}>{t('nav.settings')}</h2>

      {/* 设计稿里「ACP Runtime」排在「环境检测」之前，且是两个不同的分区：
          前者是 claude / codex，后者是 node / git / cargo。 */}
      <RuntimeSection
        runtimes={runtimes.runtimes}
        loading={runtimes.loading}
        errorCode={runtimes.errorCode}
        onRetry={() => void runtimes.refresh()}
      />

      <section className={styles.section}>
        <h3 className={styles.heading}>{t('settings.env.title')}</h3>
        <Skeleton hintKey="settings.env.hint" rows={2} />
      </section>

      <UpdateSection
        status={update.status}
        phase={update.phase}
        errorCode={update.errorCode}
        blocked={update.blocked}
        progress={update.progress}
        onCheck={() => void update.check()}
        onUpdate={() => void update.applyUpdate()}
      />

      <section className={styles.section}>
        <h3 className={styles.heading}>{t('settings.projects.title')}</h3>
        <Skeleton hintKey="settings.projects.hint" rows={2} />
      </section>

      <section className={styles.section}>
        <h3 className={styles.heading}>{t('settings.github.title')}</h3>
        <Skeleton hintKey="settings.github.hint" rows={2} />
      </section>

      <LanguageSection />
    </div>
  )
}
