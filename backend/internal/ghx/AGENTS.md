# AGENTS.md · backend/internal/ghx

> **就近优先**。总纲见根 [`AGENTS.md`](/AGENTS.md)。

## 负责什么

检测本机的 GitHub CLI：**装了吗、登录了吗**。就这两个问题。

## ★★ Duet 从头到尾不碰令牌（Q41）

`gh` 自己把令牌存在 macOS keychain 里。我们不读、不存、不传——
不进数据库、不进日志、不进 git 历史、不进任何配置文件。

**没有令牌可泄漏，是比「小心保管令牌」强得多的性质。**

代价：用户必须自己装 `gh` 并登录一次。这比「在应用里贴一个令牌」多一步，
但换来的是**我们永远不需要为一个泄漏的令牌负责**。

★ `gh auth status` 的输出里有一行 `Token: gho_***`。
`TestDetect_NeverLeaksToken` 守着 Result 的**每个字段**都不含 `gho_`——
把整段输出塞进 Detail 或 Account 的话，令牌会进日志、进界面。

## 四态，不是两个布尔

| 状态 | 含义 | 给什么修复命令 |
|---|---|---|
| `ready` | 装了且登录了 | 无 |
| `not_installed` | 没装 | `brew install gh` |
| `not_authenticated` | 装了没登录 | `gh auth login` |
| `probe_failed` | **检测本身失败了** | 无——说不清要修什么 |

★ 第四态是必须的：只用「装了/登录了」两个布尔表达不了「检测失败」，
那时 `installed=false` 与「确实没装」无法区分，界面会把一个可能是假的
结论告诉用户，还附上一句「请先安装」。

## 两条判定上的坑

### 「没登录」与「说不清」要分开

`gh auth status` 在没登录时以非 0 退出——**那是正常结论不是故障**。
但网络不通、配置坏了也是非 0。认不出时**当成 probe_failed**：
给一句「请运行 gh auth login」而实际问题是别的，会让用户照着做一遍然后发现没用。

### 文件在但跑不起来 ≠ 没装

权限不对、架构不对、装坏了——报成「没装」的话，
用户会去 `brew install` 一个已经在那儿的东西。

## 合并 stdout 与 stderr

`gh auth status` 把结论写在 **stderr** 上。只读 stdout 的话，
「没登录」这条永远判不出来。

## 检查命令

```bash
cd backend && go test ./internal/ghx/ -count=1
```
