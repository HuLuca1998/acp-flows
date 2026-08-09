import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { listProjects, listWorks, sayInWork, startWork } from '@/api/system'
import type { Project } from '@/models/project'
import type { Work } from '@/models/work'

import { PermissionDock } from '../permission/PermissionDock'
import { usePermissions } from '../permission/use-permissions'
import { PlanPanel } from '../plan/PlanPanel'
import { Timeline } from '../timeline/Timeline'
import { useEventStream } from '../timeline/use-event-stream'
import { RequirementBar } from '../work/RequirementBar'
import { WorkStatus } from '../work/WorkStatus'

import styles from './ChatPage.module.css'

/**
 * 对话页：提一个需求，看着 AI 干活。
 *
 * 这是 V5 与 V6 真正连起来的地方——用户在这里第一次看到「我说的话变成了
 * 一个正在进行的工作」。
 */
/**
 * 左栏点过来的意图：在某个项目下开新对话，或打开一条已有的工作。
 */
export type ChatIntent =
  | {
      kind: 'new'
      projectPath: string
      /**
       * 用户在「新建工作」弹层里选的基线（分支名或 commit）。
       *
       * ★ 空串表示用仓库当前 HEAD。传不下去的话，他选了 `develop`
       * 而工作还是从当前分支开的——而当前分支上可能正躺着他没提交完的东西。
       */
      baseRef?: string
    }
  | { kind: 'open'; workID: string }

export type ChatPageProps = {
  /**
   * 当前工作变化时通知外面。
   *
   * ★ 右栏「工作区」要靠它知道该读哪个工作的 git 现场——
   * 让右栏自己去查「哪个工作是当前的」的话，两处会各有一份答案。
   */
  onWorkChange?: (work: Work | null) => void
  /** 左栏点过来的动作；为 null 表示用户直接进的对话页。 */
  intent: ChatIntent | null
  /**
   * 意图的序号。
   *
   * ★ 同一个项目连点两次「新建对话」，`intent` 的内容不变——
   * 只看内容的话第二次毫无动静，而用户明明点了两下。序号让每一次点击
   * 都是一个新事件。
   */
  intentSeq: number
}

export function ChatPage({ intent, intentSeq, onWorkChange }: ChatPageProps) {
  const { t } = useTranslation()

  const [projects, setProjects] = useState<Project[]>([])
  // 左栏指定的项目路径。为空时回落到第一个项目（用户直接进对话页的情形）。
  const [pickedProject, setPickedProject] = useState<string | null>(null)
  // ★ 用户在「新建工作」弹层里选的基线，跟着这一轮走到 startWork。
  const [baseRef, setBaseRef] = useState('')
  const [current, setCurrent] = useState<Work | null>(null)
  const [prompt, setPrompt] = useState('')
  const [errorCode, setErrorCode] = useState<string | null>(null)
  const [starting, setStarting] = useState(false)

  // ★ 把当前工作报给外面（右栏要用）。放在 effect 里而不是每次 setCurrent
  // 时手动调——手动调的话，漏掉任何一条赋值路径都会让右栏停在旧工作上。
  useEffect(() => {
    onWorkChange?.(current)
  }, [current, onWorkChange])

  const { events } = useEventStream(current?.id ?? null)
  // ★★ 新消息来了自动滚到底——聊天软件不这么做的话，用户得手动追着看，
  // 而 AI 说话是一个字一个字来的，他会一直在往下拖。
  const streamRef = useRef<HTMLDivElement>(null)
  // 待裁决的权限请求。★ 它排在时间线**上方**——AI 挂着等的时候，
  // 用户要一眼看到「有件事在等我」，而不是往下滚才发现。
  const { asks, decide } = usePermissions(current?.id ?? null, events)

  useEffect(() => {
    const el = streamRef.current
    if (el === null) {
      return
    }
    // ★ 只在**用户本来就在底部**时才跟着滚：他往上翻看历史时，
    // 新消息把他拽回底部是最烦人的一种行为。
    const nearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 120
    if (nearBottom) {
      el.scrollTop = el.scrollHeight
    }
  }, [events])

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

  // ★ 依赖里带上 intentSeq：连点两次同一个项目也要各响应一次
  useEffect(() => {
    if (intent === null) {
      return
    }
    if (intent.kind === 'new') {
      setPickedProject(intent.projectPath)
      setBaseRef(intent.baseRef ?? '')
      setCurrent(null) // 新的一轮：不接着看上一条工作
      setPrompt('')
      setErrorCode(null)
      return
    }
    void (async () => {
      try {
        const works = await listWorks()
        setCurrent(works.find((w) => w.id === intent.workID) ?? null)
      } catch (err) {
        setErrorCode(errorCodeOf(err))
      }
    })()
  }, [intent, intentSeq])

  const send = useCallback(async () => {
    const text = prompt.trim()
    // ★ 空需求不发请求。发出去的话后端会拒，而用户看到的是一句莫名其妙的
    // 错误——他明明什么都没输入。
    if (text === '') {
      return
    }

    // ★★ **已经有工作就接着说，不新建。**
    //
    // 每句都 startWork 的话，用户说第二句时开的是一个新工作：
    // 新 worktree、新会话、新时间线——前一句彻底不在上下文里，
    // 而他以为自己只是补充了一句。会话按「工作 + 角色」常驻（Q42），
    // 走这条路第二句才进得了上一句的上下文。
    const workID = current?.id
    if (workID !== undefined && workID !== '') {
      setStarting(true)
      setErrorCode(null)
      try {
        await sayInWork(workID, text)
        setPrompt('')
      } catch (err) {
        setErrorCode(errorCodeOf(err))
      } finally {
        setStarting(false)
      }
      return
    }

    // ★ 优先用左栏点的那个项目。固定取 projects[0] 的话，
    // 用户在 B 项目下点「新建对话」，工作却建到了 A 项目里——
    // 而他要到 AI 开始读错文件时才发现。
    const project =
      pickedProject === null
        ? projects[0]
        : (projects.find((p) => p.path === pickedProject) ?? projects[0])
    if (project === undefined || project.path === undefined) {
      return
    }

    setStarting(true)
    setErrorCode(null)
    try {
      setCurrent(await startWork(project.path, text, baseRef))
      setPrompt('')
    } catch (err) {
      // ★ 失败要说出来。静默的话用户点了「开始」之后界面毫无变化——
      // 他不知道是没点上、还是在转圈、还是失败了。
      setErrorCode(errorCodeOf(err))
    } finally {
      setStarting(false)
    }
  }, [prompt, projects, pickedProject, baseRef, current])

  return (
    <div className={styles.page}>
      {/*
        ★★ 消息在上、输入框在下——**聊天软件的基本形态**。
        输入框在顶部的话，用户读到最新一条要往下滚，回复又要往上翻，
        每说一句都在页面里来回跳。
      */}
      <div className={styles.stream} ref={streamRef}>
      {/* ★ 状态与「停下」排在时间线上方——用户要一眼看到「它在干什么、
          我能不能停」，而不是往下滚才发现。 */}
      {current !== null && (
        <>
          <WorkStatus
            workID={current.id ?? ''}
            state={String(current.state ?? '')}
            onCancelled={() => setCurrent({ ...current, state: 'paused' })}
          />
          {/* ★ 需求快照条排在时间线上方，和状态一起——用户要一眼看到
              「现在是第几版、冻没冻」，而不是往下滚才发现。 */}
          <RequirementBar workID={current.id ?? ''} />
          {/* ★ 计划面板：拆成了什么、每条谁做、走到哪了。
              还没规划时它自己不显示——新工作的常态。 */}
          <PlanPanel workID={current.id ?? ''} />
        </>
      )}
        <PermissionDock asks={asks} onDecide={decide} />
        <Timeline events={events} />
      </div>

      {/* 输入区在**底部**，照设计稿。 */}
      {renderComposer()}
      {errorCode !== null && <p className={styles.error}>{t(problemKey(errorCode))}</p>}
    </div>
  )

  function renderComposer() {
    // 一个项目都没有：让他先去加，而不是给一个点了没用的输入框
    if (projects.length === 0) {
      return <p className={styles.empty}>{t('chat.needProject')}</p>
    }

    // ★ 有工作时这个输入框是「接着说」，没有时是「开始一个新工作」。
    // 两者文案不同不是装饰：用户要看得出这句话会进已有的对话，
    // 还是会另起一个工作——后者会新开一个 worktree。
    const continuing = current !== null && (current.id ?? '') !== ''

    return (
      <form
        className={styles.composer}
        onSubmit={(e) => {
          e.preventDefault()
          void send()
        }}
      >
        <input
          type="text"
          className={styles.input}
          value={prompt}
          placeholder={t(continuing ? 'chat.sayMore' : 'chat.placeholder')}
          aria-label={t(continuing ? 'chat.sayMoreLabel' : 'chat.inputLabel')}
          onChange={(e) => setPrompt(e.target.value)}
        />
        <button type="submit" className={styles.submit} disabled={starting}>
          {t(submitKey(continuing, starting))}
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
  work_not_accepting_messages: 'chat.error.work_not_accepting_messages',
  work_message_required: 'chat.error.work_message_required',
}

function problemKey(code: string): string {
  return ERROR_KEY[code] ?? 'chat.error.unknown'
}

/**
 * 提交按钮的词条 key，**显式四选一**。
 *
 * ★ 不许写成 `chat.${continuing ? 'send' : 'start'}${...}`：
 * 动态拼接之后静态分析查不出词条缺失（docs/rules/i18n.md §4）。
 */
function submitKey(continuing: boolean, busy: boolean): string {
  if (continuing) {
    return busy ? 'chat.sending' : 'chat.send'
  }
  return busy ? 'chat.starting' : 'chat.start'
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
