# AGENTS.md · backend/internal/acp/runtime

**检测本机装了哪些 ACP Runtime、能不能用。** 只看，不改。

仓库总纲见根 [`AGENTS.md`](../../../../AGENTS.md)；本目录规则覆盖同名条款。

---

## 三条硬规则

### 1. 零模型开销

只跑 `--version` 和登录态查询。**绝不发起会话、绝不发 prompt**——
用户只是打开了一下设置页，不该产生费用。

`TestDetectNeverPrompts` 守着这条：假 runtime 记下每次 argv，
出现任何未声明的子命令就红。

### 2. 不确定就说不确定

状态有**四态**，`probe_failed` 不是可选的：

| 状态 | 含义 |
|---|---|
| `ready` | 装好了、登录了 |
| `not_installed` | PATH 上找不到 |
| `not_authenticated` | 装了没登录 |
| `probe_failed` | **检测本身没跑成** |

没有第四态的话，「检测失败」只能并进 `not_installed`——
界面会对着一个装好的 Runtime 说「请先安装」，用户照做还装不上。

同理，`probe_failed` 时**不给修复命令**：不知道要修什么，给一条瞎猜的比不给更糟。

### 3. 加 Runtime 只改 registry.go

`Registered()` 里加一条就完了。**任何地方出现 `if name == "..."` 都是设计失败**
（[`design-principles.md`](../../../../docs/rules/design-principles.md) §4.4）。
「codex 没登录该敲什么」是数据，不是代码。

---

## 改之前必须知道的三个真实行为

都是 2026-08-07 真机实测的，不是照文档抄的。**改注册表前先自己跑一遍。**

| 事实 | 后果 |
|---|---|
| 适配器与 CLI 是**两个可执行文件**（`claude-agent-acp` vs `claude`） | 所以有 `AuthBin` |
| `claude auth status` **未登录也返回 exit 0**，结论在 JSON 的 `loggedIn` 里 | 所以有 `AuthOKSubstring`；只看退出码会把没登录报成就绪 |
| `codex login status` 把结论写在 **stderr** | 所以 `run()` 返回合并输出；只读 stdout 会把已登录判成没登录 |

另外 `--version` 的输出格式**没有一家一样**（`0.63.0` / `@agentclientprotocol/codex-acp 1.1.7`
/ `codex-cli 0.145.0` / `2.1.224 (Claude Code)`），所以有 `extractVersion`。

---

## exec 的两个坑

**`cmd.WaitDelay` 必须设。** ctx 到期时 `CommandContext` 只 kill 直接子进程；
子进程 fork 过的话，孙子进程继承着 stdout 管道活下去，`Output()` 要等管道关闭才返回——
超时就此被架空。实测 `/bin/sleep 30` 配 1.5 秒超时跑满了 30 秒。
`check-naming.sh` 第 8 节守着这条。

**`cmd.Stdin = nil` 必须写。** ACP Runtime 的正常形态是从 stdin 读 JSON-RPC。
某个子命令不认识参数就进交互模式时，继承来的 stdin 会让它一直等下去。

---

## 测试怎么写

**不 mock exec。** 造真的 shell 脚本、真的执行位、真的 PATH、真的退出码——
把 exec 换成接口再塞假实现，测的就只剩「假实现会返回我塞进去的值」。

PATH 要**整个替换**而不是追加：追加的话，开发机上真装了 codex 的人会命中真的 codex，
测试结果取决于谁的机器。代价是脚本里要写 `/bin/sleep` 这样的绝对路径。

超时**别设太小**：macOS 首次执行新建的可执行文件要过代码签名检查，实测 >100ms。
设成 80ms 的话所有用例都会 `probe_failed`，包括本该 `ready` 的。
