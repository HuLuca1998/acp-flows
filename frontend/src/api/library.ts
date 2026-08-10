// 角色库、Skill 库、记忆库的接口。
//
// ★ 从 `system.ts` 拆出来：那个文件把整个后端的端点堆在一起，
// 改一个域要在四百行里翻——而这三个库本来就各自演化。

import type { Memory, MemoryStatus } from '@/models/memory'
import type { ResumableWork } from '@/models/resume'
import type { Role } from '@/models/role'
import type { Skill } from '@/models/skill'


import { api, unwrap } from './client'

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
 * 读一个 Skill 的 SKILL.md 正文（frontmatter 与正文分开给）。
 *
 * ★ 后端**从磁盘现读**——Skill 是用户的产物，他随时可能用编辑器改它。
 * 读不到时后端报错并带路径，不返回空正文（空正文看起来像「没写内容」）。
 */
export async function getSkillBody(
  dir: string,
): Promise<{ dir: string; frontmatter: string; text: string }> {
  const body = unwrap(await api.GET('/skills/{dir}/body', { params: { path: { dir } } }))
  return { dir: body.dir, frontmatter: body.frontmatter, text: body.text }
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
 * 读一条记忆的正文。
 *
 * ★★ 正文在 **md 文件**里，不在数据库（INV-MEM-8）。审核候选时用户要读到
 * 它才决定得了收不收——只给标题的话他在**盲选**，而收下之后这条会影响
 * 后面每一轮。
 */
export async function getMemoryBody(id: string): Promise<{ title: string; text: string }> {
  const body = unwrap(await api.GET('/memories/{id}/body', { params: { path: { id } } }))
  return { title: body.title, text: body.text }
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

/**
 * 列出能接着做的工作。
 *
 * ★★ 后端这条链路早就通了，而界面上一直没有入口——用户打开应用
 * 永远看不到「有 2 个工作可以接着做」，那整套检查点代码等于没用。
 */
export async function listResumable(): Promise<ResumableWork[]> {
  const body = unwrap(await api.GET('/system/resume', {}))
  return body.resumable
}

/** 恢复失败的原因，`worktree_dirty` 要问用户一句再来。 */
export class ResumeBlocked extends Error {
  constructor(readonly code: string) {
    super(code)
  }
}

/**
 * 恢复一个工作。
 *
 * ★★ 工作区脏时抛 `ResumeBlocked('worktree_dirty')`——**先问用户一句**，
 * 不静默恢复：他手工改过那个 worktree，而状态推回可跑之后 AI 会接着
 * 往上写。先问，他才有机会去看看自己改了什么。
 *
 * ★ `force` 由用户显式确认后才传——默认永远是「先检查」。
 */
export async function resumeWork(workID: string, force = false): Promise<void> {
  const result = await api.POST('/system/resume/{id}', {
    params: { path: { id: workID }, query: force ? { force: true } : {} },
  })
  if (result.error !== undefined && result.error !== null) {
    const problem = result.error as { type?: string }
    throw new ResumeBlocked(
      typeof problem.type === 'string' && problem.type !== '' ? problem.type : 'resume_failed',
    )
  }
}
