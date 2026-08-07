import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { listProjects, listWorks, startWork } from '@/api/system'
import type { Project } from '@/models/project'
import type { Work } from '@/models/work'

import { Timeline } from '../timeline/Timeline'
import { useEventStream } from '../timeline/use-event-stream'

import styles from './ChatPage.module.css'

/**
 * 对话页：提一个需求，看着 AI 干活。
 *
 * 这是 V5 与 V6 真正连起来的地方——用户在这里第一次看到「我说的话变成了
 * 一个正在进行的工作」。
 */
export function ChatPage() {
  const { t } = useTranslation()

  const [projects, setProjects] = useState<Project[]>([])
  const [current, setCurrent] = useState<Work | null>(null)
  const [prompt, setPrompt] = useState('')
  const [errorCode, setErrorCode] = useState<string | null>(null)
  const [starting, setStarting] = useState(false)

  const { events } = useEventStream(current?.id ?? null)

  useEffect(() => {
    void (async () => {
      try {
        setProjects(await listProjects())
        const works = await listWorks()
        // 已有工作就接着看，不用重新提一遍需求
        if (works.length > 0) {
          setCurrent(works[0] ?? null)
        }
      } catch (err) {
        setErrorCode(errorCodeOf(err))
      }
    })()
  }, [])

  const start = useCallback(async () => {
    const text = prompt.trim()
    const project = projects[0]
    // ★ 空需求不发请求。发出去的话后端会拒，而用户看到的是一句莫名其妙的
    // 错误——他明明什么都没输入。
    if (text === '' || project === undefined || project.path === undefined) {
      return
    }

    setStarting(true)
    setErrorCode(null)
    try {
      setCurrent(await startWork(project.path, text))
      setPrompt('')
    } catch (err) {
      // ★ 失败要说出来。静默的话用户点了「开始」之后界面毫无变化——
      // 他不知道是没点上、还是在转圈、还是失败了。
      setErrorCode(errorCodeOf(err))
    } finally {
      setStarting(false)
    }
  }, [prompt, projects])

  return (
    <div className={styles.page}>
      {renderComposer()}
      {errorCode !== null && <p className={styles.error}>{t(problemKey(errorCode))}</p>}
      <Timeline events={events} />
    </div>
  )

  function renderComposer() {
    // 一个项目都没有：让他先去加，而不是给一个点了没用的输入框
    if (projects.length === 0) {
      return <p className={styles.empty}>{t('chat.needProject')}</p>
    }

    return (
      <form
        className={styles.composer}
        onSubmit={(e) => {
          e.preventDefault()
          void start()
        }}
      >
        <input
          type="text"
          className={styles.input}
          value={prompt}
          placeholder={t('chat.placeholder')}
          aria-label={t('chat.inputLabel')}
          onChange={(e) => setPrompt(e.target.value)}
        />
        <button type="submit" className={styles.submit} disabled={starting}>
          {t(starting ? 'chat.starting' : 'chat.start')}
        </button>
      </form>
    )
  }
}

/**
 * 错误码 → 词条 key 的**显式映射**。
 *
 * ★ 不许写成 `chat.error.${code}`：动态拼接之后静态分析查不出词条缺失，
 * 删掉一条也没有任何检查会红（docs/rules/i18n.md §4）。
 *
 * ★ 认不出来的码用兜底文案，**不把原始码显示给用户**——
 * `work_project_not_a_repo` 这种字符串对他没有意义。
 */
const ERROR_KEY: Record<string, string> = {
  work_project_not_a_repo: 'chat.error.work_project_not_a_repo',
  work_project_not_found: 'chat.error.work_project_not_found',
  project_path_not_absolute: 'chat.error.project_path_not_absolute',
  work_prompt_required: 'chat.error.work_prompt_required',
}

function problemKey(code: string): string {
  return ERROR_KEY[code] ?? 'chat.error.unknown'
}

/** 从错误里取机器可读的原因码；取不到时给兜底码，绝不返回 null。 */
function errorCodeOf(err: unknown): string {
  if (err !== null && typeof err === 'object' && 'type' in err) {
    const type = (err as { type?: unknown }).type
    if (typeof type === 'string' && type !== '') {
      return type
    }
  }
  return 'work_operation_failed'
}
