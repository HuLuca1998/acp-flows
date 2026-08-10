import type { Acceptance } from '@/models/acceptance'
import type { Contract } from '@/models/contract'
import type { Decision } from '@/models/decision'
import type { Plan } from '@/models/plan'
import type { ProjectPreview } from '@/models/preview'
import type { Project } from '@/models/project'
import type { Requirement } from '@/models/requirement'
import type { Runtime } from '@/models/runtime'
import type { UpdatePrepareResult, UpdateStatus } from '@/models/update'
import type { Work } from '@/models/work'
import type { WorkPreparation } from '@/models/workprep'
import type { WorktreeState } from '@/models/worktree'

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
 * ★★ **默认往用户的项目目录里写零个字节**，只登记路径。
 *
 * ★ `initialize` 为真时照预演的计划创建 `.acpflows/` 并追加 `.gitignore`——
 * 而**传 true 之前必须先让他看过 `previewProject` 的结果**。
 * 静默往用户的仓库里写东西是最快失去信任的方式。
 */
export async function addProject(path: string, initialize = false): Promise<Project> {
  return unwrap(await api.POST('/projects', { body: { path, initialize } }))
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
export async function startWork(
  project: string,
  prompt: string,
  baseRef = '',
): Promise<Work> {
  // ★ baseRef 留空时后端用仓库当前 HEAD。
  // 用户在弹层里选了 `develop` 却没传下去的话，工作还是从当前分支开的——
  // 而当前分支上可能正躺着他没提交完的东西。
  return unwrap(await api.POST('/works', { body: { project, prompt, base_ref: baseRef } }))
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
 * 读一个单元的验收：标准与证据对照。
 */
export async function getAcceptance(workID: string, unitID: string): Promise<Acceptance> {
  return unwrap(
    await api.GET('/works/{id}/units/{unitId}/acceptance', {
      params: { path: { id: workID, unitId: unitID } },
    }),
  )
}

/**
 * 采集这个单元的 diff 证据。
 *
 * ★★ **应用自己去读 git，不问 AI**：让 AI 报告自己改了什么，等于让被
 * 考核的人填自己的考勤表——它不需要撒谎，只需要「记错了」一次。
 */
export async function collectEvidence(workID: string, unitID: string): Promise<Acceptance> {
  return unwrap(
    await api.POST('/works/{id}/units/{unitId}/acceptance', {
      params: { path: { id: workID, unitId: unitID } },
    }),
  )
}

/**
 * 验收通过：提交改动并落检查点。
 *
 * ★★ **由用户点，不由 AI 判断。** 返回那次提交的短 sha。
 */
export async function acceptUnit(workID: string, unitID: string): Promise<string> {
  const body = unwrap(
    await api.POST('/works/{id}/units/{unitId}/accept', {
      params: { path: { id: workID, unitId: unitID } },
    }),
  )
  return body.commit
}

/**
 * 列出一个工作**还没答**的决策。
 *
 * ★★ 左栏那个亮蓝点靠它：不列的话，用户不知道有件事在等他。
 */
export async function listPendingDecisions(workID: string): Promise<Decision[]> {
  const body = unwrap(
    await api.GET('/works/{id}/decisions', { params: { path: { id: workID } } }),
  )
  return body.decisions
}

/**
 * 回答一条决策。**只能答一次。**
 */
export async function answerDecision(
  workID: string,
  decisionID: string,
  optionID: string,
): Promise<void> {
  const result = await api.POST('/works/{id}/decisions/{decisionId}', {
    params: { path: { id: workID, decisionId: decisionID } },
    body: { option_id: optionID },
  })
  if (result.error !== undefined && result.error !== null) {
    const problem = result.error as Problem
    throw new Error(
      typeof problem.type === 'string' && problem.type !== '' ? problem.type : 'request_failed',
    )
  }
}

/**
 * 读一个单元的契约。
 *
 * ★ 还没有契约时后端回 404 —— 单元设计师还没跑，那是常态。
 */
export async function getContract(workID: string, unitID: string): Promise<Contract> {
  return unwrap(
    await api.GET('/works/{id}/units/{unitId}/contract', {
      params: { path: { id: workID, unitId: unitID } },
    }),
  )
}

/**
 * 冻结一个单元的契约。**由用户点。**
 *
 * ★ 冻结之后 AI 才能照着它开工，而冻结前它一个字都不该改——
 * 所以这一下是用户把关的那一次。
 */
export async function freezeContract(workID: string, unitID: string): Promise<Contract> {
  return unwrap(
    await api.POST('/works/{id}/units/{unitId}/contract/freeze', {
      params: { path: { id: workID, unitId: unitID } },
    }),
  )
}

/**
 * 读一个工作当前的计划（子计划 DAG + 单元）。
 *
 * ★ 还没规划时后端回 404 —— 那是新工作的常态，调用方据此不显示计划面板。
 */
export async function getPlan(workID: string): Promise<Plan> {
  return unwrap(await api.GET('/works/{id}/plan', { params: { path: { id: workID } } }))
}

/**
 * 计划的变更历史，**从新到旧**——设计稿计划面板的「变更历史」。
 */
export async function getPlanHistory(workID: string): Promise<Plan[]> {
  const body = unwrap(
    await api.GET('/works/{id}/plan/history', { params: { path: { id: workID } } }),
  )
  return body.versions
}

/**
 * 读一个工作当前的需求快照。
 *
 * ★ 还没有需求时后端回 404 —— 那是**新工作的常态**，
 * 调用方据此不显示标签，而不是显示一个「v0」。
 */
export async function getRequirement(workID: string): Promise<Requirement> {
  return unwrap(await api.GET('/works/{id}/requirement', { params: { path: { id: workID } } }))
}

/**
 * 冻结当前这一版需求。
 *
 * ★★ **由用户点，不由 AI 判断。** AI 说「我觉得问清楚了」和用户说
 * 「就这样」是两件事——而冻结之后这一版就进了计划与契约，改不动了。
 *
 * ★ 抛出的 Error 的 message 是机器可读的错误码
 * （`requirement_open_facts_remain` 之类），界面按它查 i18n 词条。
 */
export async function freezeRequirement(workID: string): Promise<Requirement> {
  return unwrap(await api.POST('/works/{id}/requirement', { params: { path: { id: workID } } }))
}

/**
 * 在一个**已有**的工作里接着说一句。
 *
 * ★★ 同一个工作、同一条会话——这正是「连着说三句，AI 记得前两句」的那条路。
 * 走 `startWork` 的话，用户说第二句时开的是一个**新工作**：
 * 新 worktree、新会话、新时间线，前一句彻底不在上下文里，
 * 而他以为自己只是补充了一句。
 *
 * ★ 抛出的 Error 的 message 是**机器可读的错误码**，界面按它查 i18n 词条。
 */
export async function sayInWork(workID: string, text: string): Promise<void> {
  const result = await api.POST('/works/{id}/messages', {
    params: { path: { id: workID } },
    body: { text },
  })
  // 202 没有响应体，unwrap 会因为 data === undefined 而抛「empty_response」，
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
 * 创建项目前的预演：**只看不动**。
 *
 * ★★ 用户交出来的是他自己的代码仓库——「先说再做」是这一步的全部意义。
 * 拿到的每一步都带 `reason`，界面必须把它显示出来。
 */
export async function previewProject(path: string): Promise<ProjectPreview> {
  return unwrap(await api.POST('/projects/preview', { body: { path } }))
}

/**
 * 开工前的仓库状态：**只看不动**。
 *
 * ★ 未提交改动分「已跟踪 / 未跟踪」两个数——合成一条的话，
 * 「新建了几个还没 add 的文件」和「改了正在跟踪的代码」会长得一模一样。
 */
export async function prepareWork(project: string): Promise<WorkPreparation> {
  return unwrap(await api.POST('/works/prepare', { body: { project } }))
}

/**
 * 一个工作的 git 现场，右栏「工作区」照它渲染。**只读。**
 *
 * ★ `base_commit` 为空表示不知道基线——那时 `ahead` 与 `commits` 不可信，
 * 界面不该显示它们。显示「领先 0 个」会让用户以为 AI 什么都没干。
 */
export async function getWorkWorktree(id: string): Promise<WorktreeState> {
  return unwrap(await api.GET('/works/{id}/worktree', { params: { path: { id } } }))
}

