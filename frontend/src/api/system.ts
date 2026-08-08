import type { Memory, MemoryStatus } from '@/models/memory'
import type { Project } from '@/models/project'
import type { Role } from '@/models/role'
import type { Runtime } from '@/models/runtime'
import type { Skill } from '@/models/skill'
import type { UpdatePrepareResult, UpdateStatus } from '@/models/update'
import type { Work } from '@/models/work'

import { api, unwrap, type Problem } from './client'

/**
 * 检查应用更新。**只检查，绝不下载、绝不安装**（docs/adr/0002）。
 *
 * 失败时抛错——调用方必须显式处理，不能当成「已是最新」。
 */
export async function checkUpdate(): Promise<UpdateStatus> {
  return unwrap(await api.POST('/system/update/check'))
}

/**
 * 更新前准备：判断现在更新会不会打断用户。
 *
 * ★ 返回 `blocked` 时前端**不得继续安装**——那意味着有工作在跑。
 * `blocked` 是业务结论不是错误，HTTP 状态仍是 200。
 */
export async function prepareUpdate(): Promise<UpdatePrepareResult> {
  return unwrap(await api.POST('/system/update/prepare'))
}

/**
 * 查本机装了哪些 ACP Runtime、能不能用。
 *
 * ★ **只看，不改**：不写用户的 `~/.claude` 与 `~/.codex`，
 * 也不发起任何会产生费用的模型调用。
 *
 * 失败时抛错——调用方要把「检测不了」和「一个都没装」分开显示，
 * 后者会让用户去安装已经装好的东西。
 */
export async function listRuntimes(): Promise<Runtime[]> {
  const body = unwrap(await api.GET('/runtimes'))
  return body.runtimes
}

/** 已添加的本地项目。 */
export async function listProjects(): Promise<Project[]> {
  const body = unwrap(await api.GET('/projects'))
  return body.projects
}

/**
 * 把一个本地文件夹加进来。
 *
 * ★ 这个动作**往用户的项目目录里写零个字节**，只登记路径。
 */
export async function addProject(path: string): Promise<Project> {
  return unwrap(await api.POST('/projects', { body: { path } }))
}

/** 移除项目。**只取消登记，不删用户的文件。** */
export async function removeProject(id: string): Promise<void> {
  unwrap(await api.DELETE('/projects/{id}', { params: { path: { id } } }))
}

/** 全部工作。 */
export async function listWorks(): Promise<Work[]> {
  const body = unwrap(await api.GET('/works'))
  return body.works
}

/**
 * 对一个项目提需求，开一个工作。
 *
 * ★ 会切一个独立 worktree，**建在用户项目之外**（`~/.acpflows/worktrees`）。
 */
export async function startWork(project: string, prompt: string): Promise<Work> {
  return unwrap(await api.POST('/works', { body: { project, prompt } }))
}

/**
 * 应答一次权限请求。
 *
 * ★ `optionID` 是 **Agent 定义的不透明字符串**，从事件载荷原样取、原样送。
 * 这一层不做任何加工——搞错的话，用户点「拒绝」而 Agent 收到「允许」。
 */
export async function answerPermission(
  workID: string,
  askID: string,
  optionID: string,
): Promise<void> {
  const result = await api.POST('/works/{id}/permission', {
    params: { path: { id: workID } },
    body: { ask_id: askID, option_id: optionID },
  })
  // 204 没有响应体，unwrap 会因为 data === undefined 而抛「empty_response」，
  // 所以这里只把错误挑出来。
  if (result.error !== undefined && result.error !== null) {
    const problem = result.error as Problem
    throw new Error(
      typeof problem.type === 'string' && problem.type !== '' ? problem.type : 'request_failed',
    )
  }
}

/**
 * 停下一个工作正在跑的那一轮。
 *
 * ★ 抛出的 Error 的 message 是**机器可读的错误码**（`work_cancel_not_allowed`
 * 之类），界面按它查 i18n 词条。不要把它直接显示给用户。
 */
export async function cancelWork(workID: string): Promise<void> {
  const result = await api.POST('/works/{id}/cancel', {
    params: { path: { id: workID } },
  })
  if (result.error !== undefined && result.error !== null) {
    const problem = result.error as Problem
    throw new Error(
      typeof problem.type === 'string' && problem.type !== '' ? problem.type : 'request_failed',
    )
  }
}

/**
 * 角色与 Runtime 绑定表。八个预置角色，**顺序就是设计稿的行序**。
 *
 * ★ 后端没装配时会返回 503 而不是空列表——预置角色是内置的，
 * 空表只会让用户以为应用坏了。所以这里的失败**必须**显示出来。
 */
export async function listRoles(): Promise<Role[]> {
  const body = unwrap(await api.GET('/roles'))
  return body.roles
}

/**
 * Skill 库。不传 scope 时是全局库（`~/.acpflows/skills`）。
 *
 * ★ 扫不动时后端返回错误而不是空列表——装作「一个都没有」的话，
 * 用户以为自己的 skill 丢了，而实际是目录读不了。
 */
export async function listSkills(): Promise<Skill[]> {
  const body = unwrap(await api.GET('/skills'))
  return body.skills
}

/**
 * 记忆库。不传 scope 时返回全部（含跨项目与各项目的）。
 *
 * ★ 查不动时后端返回错误而不是空列表——装作「一条都没有」的话，
 * 用户以为 Duet 把记忆忘光了。
 */
export async function listMemories(params?: {
  scope?: string
  // ★ 用契约里的枚举而不是 string：写错一个状态名时编译器会红，
  // 而用 string 的话只会在运行时静默筛出空列表。
  status?: MemoryStatus
}): Promise<Memory[]> {
  const body = unwrap(await api.GET('/memories', { params: { query: params ?? {} } }))
  return body.memories
}

/**
 * 审核一条候选记忆。
 *
 * ★★ 这是 `candidate → active` 的**唯一入口**（INV-MEM-2），
 * 且 `actor` 必填——AI 没有任何路径能自己把候选变成生效。
 */
export async function reviewMemory(
  id: string,
  decision: 'confirm' | 'reject',
  actor: string,
): Promise<Memory> {
  return unwrap(
    await api.POST('/memories/{id}/review', {
      params: { path: { id } },
      body: { decision, actor },
    }),
  )
}
