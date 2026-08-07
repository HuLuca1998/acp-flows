/**
 * 生成物的**编译期**契约断言。
 *
 * 为什么需要：`make gen` 跑成功 ≠ 生成的类型有用。
 * 生成器把枚举退化成 `string`、把必填字段生成成可选，都不会让生成失败，
 * 但会让整个"契约先行"失去意义——前端拿到的类型什么都不挡。
 *
 * 本文件的断言主要靠 `@ts-expect-error`：如果被标注的那行**不再报错**
 * （说明类型放宽了），tsc 会报「未使用的 ts-expect-error」——**编译不过**。
 * 所以这些断言在 `make lint-frontend`（tsc --noEmit）阶段就会红，
 * 不需要等运行时。
 *
 * 契约真源：api/openapi.yaml。生成物：src/api/gen/schema.d.ts（不许手改）。
 */
import { describe, expect, it } from 'vitest'

import type { components, operations, paths } from './gen/schema'

type UpdateStatus = components['schemas']['UpdateStatus']
type UpdatePrepareResult = components['schemas']['UpdatePrepareResult']
type Problem = components['schemas']['Problem']
type Runtime = components['schemas']['Runtime']
type VersionInfo = components['schemas']['VersionInfo']

describe('生成的 TS 类型与 openapi.yaml 一致', () => {
  it('R1 · 端点齐备：spec 里的每个 path 都生成出来了', () => {
    // 运行时拿不到类型，所以这里断言的是"键名写错会编译不过"
    const covered: (keyof paths)[] = [
      '/system/version',
      '/system/update/check',
      '/system/update/prepare',
      '/system/resume',
      '/runtimes',
      '/runtimes/{name}/probe',
    ]
    expect(covered).toHaveLength(6)
  })

  it('R2 ★ · 枚举是字面量联合，没有退化成 string', () => {
    const states: UpdateStatus['state'][] = ['idle', 'available', 'unsupported']
    expect(states).toHaveLength(3)

    // @ts-expect-error 'bogus' 不在联合里。若这行不再报错，说明 state 变成了 string
    const bogus: UpdateStatus['state'] = 'bogus'
    expect(bogus).toBe('bogus')

    const prepare: UpdatePrepareResult['status'][] = ['ready', 'blocked']
    expect(prepare).toHaveLength(2)

    // @ts-expect-error 同上：prepare 的 status 必须是两值联合
    const badPrepare: UpdatePrepareResult['status'] = 'pending'
    expect(badPrepare).toBe('pending')
  })

  it('R3 ★ · 必填字段是必填的，没有被生成成可选', () => {
    // 少写一个必填字段就编译不过。这条挡的是"生成器把 required 丢了"。
    // ★ 字段是 snake_case —— 契约的命名约定见 api/AGENTS.md，
    //   前端的 camelCase 规范不适用于**来自契约的类型**（那是外部形状不是我们的模型）
    const v: VersionInfo = { version: '0.1.0', platform: 'macOS 15.3', arch: 'arm64' }
    expect(v.version).toBe('0.1.0')

    // @ts-expect-error 缺 arch（required）—— 若这行不报错说明 required 被丢了
    const missing: VersionInfo = { version: '0.1.0', platform: 'macOS 15.3' }
    expect(missing.version).toBe('0.1.0')

    const p: Problem = { type: 'work_not_found', title: 'Work not found', status: 404 }
    expect(p.status).toBe(404)
  })

  it('R4 · Problem.type 是机器可读错误码，不是给人看的文案', () => {
    // 断言的是用法而非类型：后端绝不返回用户可见文案（docs/rules/i18n.md §3）。
    // 这条是"活文档"——把约定固定在测试里，比只写在文档里更难被忽略。
    const p: Problem = { type: 'runtime_not_installed', title: 'Runtime not installed', status: 409 }
    expect(p.type).toMatch(/^[a-z][a-z0-9_]*$/)
  })

  it('R5 ★ · Runtime.name 不是封闭枚举，能力矩阵才是降级依据', () => {
    // name 必须是 string 而非 'claude' | 'codex' —— Runtime 是注册表，
    // 加第三个 runtime 不该改契约（docs/adr/0006 Q13）。
    // 生成器若把 examples 误当 enum，下面这行会报错。
    const third: Runtime['name'] = 'gemini'
    expect(third).toBe('gemini')

    const r: Runtime = {
      name: 'claude',
      status: 'ready',
      installed: true,
      capabilities: {
        passed: 1,
        total: 2,
        probes: [
          { id: 'streaming_thoughts', passed: true },
          { id: 'permission_request', passed: false, detail: 'not observed' },
        ],
      },
    }
    // 上层靠这个数字降级，绝不靠 r.name 判断（design-principles §4.4）
    expect(r.capabilities?.passed).toBe(1)
  })

  it('R5b ★ · status 有第四态 probe_failed，且修复命令由后端给', () => {
    // 只有 installed/authenticated 两个布尔的话，「检测本身失败了」
    // 只能并进 not_installed——界面会对着一个装好的 Runtime 说「请先安装」。
    const failed: Runtime = { name: 'codex', status: 'probe_failed', installed: false }
    expect(failed.status).toBe('probe_failed')

    // remedy.command 是后端给的整条命令。前端按 name 拼命令的话，
    // 加第三个 Runtime 就要改两处，迟早漂移（design-principles §4.4）。
    const missing: Runtime = {
      name: 'gemini',
      status: 'not_installed',
      installed: false,
      remedy: { command: 'npm i -g @agentclientprotocol/gemini-acp' },
    }
    expect(missing.remedy?.command).toContain('npm i -g')
  })

  it('R6 · operations 类型可用（生成客户端时要靠它推导响应体）', () => {
    type VersionResponse =
      operations['getVersion']['responses']['200']['content']['application/json']
    const v: VersionResponse = { version: '0.1.0', platform: 'macOS 15.3', arch: 'arm64' }
    expect(v.version).toBe('0.1.0')
  })
})
