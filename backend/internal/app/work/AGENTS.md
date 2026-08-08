# AGENTS.md · backend/internal/app/work

上级规则见 [`backend/internal/app/AGENTS.md`](../AGENTS.md)，仓库总纲见根 [`AGENTS.md`](../../../../AGENTS.md)。
本文件只写本目录独有的东西。

---

## 这个包干什么

**「一次工作」的用例**：新建、列出、停下。

一个工作 = 一个独立 worktree + 一条 ACP 会话 + 一条时间线。
这三样必须一起活、一起死。

---

## 本目录的铁律

### 1. 不许 import `acp`

depguard 挡着。要用 Agent 的能力，走 `app/port` 定义的接口，装配在 `cmd/duetd`。

**跨层传信息用返回值，不用哨兵错误**——拿不到那边的 error 类型。
`AgentCanceller.CancelTurn` 返回 `(mustKill bool, err error)` 就是这个原因：
「必须杀进程」这件事太重要，不能靠调用方去 `errors.Is` 一个它看不见的类型。

### 2. 后台那一轮脱开请求的 ctx

`runTurn` 用 `context.WithoutCancel`。挂在请求 ctx 上的话，HTTP 一返回
AI 就被砍掉，而用户看到的是时间线停在半截、没有任何报错。

★ 同理：**开完 goroutine 就别再碰那个领域对象**。`Start` 先把 View 取出来
再开后台轮次——反过来的话两个 goroutine 同时读写同一个 `*model.Work`，
race detector 会红，而在用户那儿的表现是返回的状态时对时不对。

### 3. 取消：先问规则再动手 ★

```
model.CanCancel(state) → 不行就直接拒，**不碰 Agent**
                      → 行就发协议取消
                          ├ 停下了 → paused + checkpoint 事件
                          └ 停不下来 → 杀进程 + failed + 原因码
```

反过来的话，审查中的工作已经被掐掉了才发现不该掐——那个单元既没通过
也没被驳回，卡在一个说不清的状态里。

**「停不下来就杀」是「界面说已取消、后台还在烧钱改文件」的唯一防线。**
只报错不杀的话，用户以为什么都没发生。

### 4. 失败的工作也要落库

不落的话，用户点了「开始」之后什么都没发生，他不知道是没点上还是失败了。

### 5. 错误码是封闭枚举

`ErrorCode(err)` 把错误翻成机器可读的码，界面按它查 i18n 词条。
**认不出的一律给兜底码**，绝不把 error 的文字摊给用户——
那里面有路径、有内部状态名，对他没有意义。

---

## 测试怎么写

**worktree 用真 `gitx` 实现，不塞假的**：本包最要紧的一条是「不往用户项目里写」，
而假实现什么都不写——那条断言会永远绿。

内存仓储的 `FindWork` 要**重建新对象**，不能交出指针。真 store 每次都经
`mapper.WorkToModel` 重建，交指针的话这个替身比真实现「更共享」，
测出来的竞态在生产里根本不存在。

写 `*_test.go` 前先调 `go-unit-testing` skill。
每加一条断言，问一遍「把实现改坏，它会不会红」——**造负例，别猜**。
