import type { Project } from '@/models/project'
import type { Runtime } from '@/models/runtime'
import type { UpdatePrepareResult, UpdateStatus } from '@/models/update'
import type { Work } from '@/models/work'

import { api, unwrap } from './client'

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
