import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { checkUpdate } from '@/api/system'
import type { UpdateStatus } from '@/models/update'
import { Skeleton } from '@/ui/Skeleton'

import { LanguageSection } from './LanguageSection'
import styles from './Settings.module.css'
import { UpdateSection } from './UpdateSection'

/**
 * 设置页。五个分区，未实现的用骨架占位。
 *
 * ★ 更新检查是**进页面时查一次，不轮询**（docs/adr/0007 修订 3）：
 * 轮询既费流量又没意义——用户不看设置页的时候，提示他有更新也没用。
 */
export function SettingsPage() {
  const { t } = useTranslation()
  const [status, setStatus] = useState<UpdateStatus | null>(null)
  const [loading, setLoading] = useState(false)
  const [errorCode, setErrorCode] = useState<string | null>(null)

  const runCheck = useCallback(async () => {
    setLoading(true)
    setErrorCode(null)
    try {
      setStatus(await checkUpdate())
    } catch (err) {
      // ★ 绝不降级成「已是最新版本」：网络断了、后端挂了都会走到这里。
      // 静默的话用户永远不知道自己在用旧版——这类故障没有任何症状。
      setStatus(null)
      setErrorCode(err instanceof Error ? err.message : 'update_check_failed')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void runCheck()
  }, [runCheck])

  return (
    <div className={styles.page}>
      <h2 className={styles.title}>{t('nav.settings')}</h2>

      <section className={styles.section}>
        <h3 className={styles.heading}>{t('settings.env.title')}</h3>
        <Skeleton hintKey="settings.env.hint" rows={2} />
      </section>

      <UpdateSection
        status={status}
        loading={loading}
        errorCode={errorCode}
        onCheck={() => void runCheck()}
        onUpdate={() => {
          // 一键更新的完整流程（先 prepare 再下载）在 M1 的 U1.1.3。
          // 现在**什么都不做**比假装在更新诚实。
        }}
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
