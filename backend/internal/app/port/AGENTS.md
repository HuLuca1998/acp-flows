# AGENTS.md · backend/internal/app/port

**app 层对外部世界的全部抽象。** 仓库总纲见根 [`AGENTS.md`](../../../../AGENTS.md)。

---

## 这里只能放什么

**只有 interface 和它们用到的领域类型。零 struct 实现、零第三方库 import。**

基础设施包反过来实现这些接口——Go 是结构化类型，它们甚至不需要 import 本包
（`acp/runtime.Detector` 就是这样实现 `RuntimeDetector` 的）。

依赖方向由 `depguard` 强制：`api → app → domain`，基础设施实现 `app/port`。
反向 import 会在 CI 变红，不是靠自觉。

---

## 接口要小

**一个用例只依赖它真正需要的两三个方法。**

巨型接口是假测试的温床：为了塞一个桩要实现十几个方法，于是有人图省事直接
`mock.On(...).Return(...)`，测试就退化成「mock 喂 mock」。

`RuntimeDetector` 只有一个方法，`Clock` 只有一个方法——这是常态，不是偷懒。

---

## 加接口时想清楚这件事

**这个抽象是为了替换实现，还是为了测试？**

如果只是为了测试而抽象，先问能不能用真实实例（真文件、真临时目录、真子进程）。
本仓库的测试策略是**真实实例真实数据**，接口是最后手段不是第一反应。

反例：`RuntimeDetector` 存在是因为 `api` 与 `app` 被 depguard 禁止 import `acp`——
这是**真实的架构约束**，不是为了好测。而检测逻辑本身在 `acp/runtime` 里
是用真脚本、真 PATH、真退出码测的，一个 mock 都没有。
