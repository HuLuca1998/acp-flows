# AGENTS.md · backend/internal/gitx

git 的基础设施封装。仓库总纲见根 [`AGENTS.md`](../../../AGENTS.md)。

---

## 一条压倒一切的规则

**对用户的仓库只读，除非调用方明确要求写。**

用户把自己的代码目录交给 Duet。加进来这个动作让他的 `git status` 多出东西，
是最快失去信任的方式——他还没开始用，就已经发现你动了他的东西。

探测类函数（`Probe`）一个字节都不写。写操作只发生在**用户明确发起一个工作**之后，
且写在 worktree 里。

## 两个位置别搞混

| 放什么 | 放哪 |
|---|---|
| worktree | `~/.acpflows/worktrees/`（**用户主目录**，见 `open-questions.md` Q30） |
| 记忆 / Skill | `<project>/.acpflows/`（用户项目里，M3，用户主动创建第一条时才写） |

## 起 git 子进程的三件事

每一条都真出过问题，缺一个就会表现成「偶尔卡住」：

```go
cmd.Stdin = nil                                        // git 要凭据时会读终端
cmd.WaitDelay = time.Second                            // 见 check-naming.sh 第 8 节
cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0") // 服务进程里没人来输密码
```

`WaitDelay` 是硬性的，`check-naming.sh` 会拦：ctx 到期时 `CommandContext`
只 kill 直接子进程，而 git 会 fork（credential helper、pager），
孙子进程握着 stdout 管道能把超时完全架空。

## 「不是 git 仓库」不是错误

返回 `IsRepo: false` 就好。当成错误的话，用户得先去命令行 `git init`
才能用这个产品——而他很可能正是不想碰命令行才来用的。

**路径不存在或不是目录才是错误**：静默当成「普通目录」的话，用户会把一个
打错的路径加进列表，直到真正开工时才发现，那时错误离原因已经很远了。

## 测试

用 `testutil.NewGitRepo(t)` 现场 git init 一个真仓库，**不要 mock git**——
mock 掉之后「有没有污染用户仓库」这个问题就永远测不出来了。

证明「什么都没写」用 `testutil.SnapshotDir` + `AssertUnchanged`：
它同时比 `git status` 与完整文件列表。只比 status 是不够的——
往 `.gitignore` 加一行、或写进已被忽略的路径，status 依然干净。
