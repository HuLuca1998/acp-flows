# M3 · 把我的项目交给它

> ★ **2026-08-08 里程碑按引用关系重排**（见 `design/DEPENDENCIES.md`）。
> 旧的 `M3`（能管住 AI）连同它的 `U3.*` 编号已归档到
> [`archive/M3-control.md`](archive/M3-control.md)——里面做完的权限与取消
> 没有白做，落位见 [`roadmap.md`](../roadmap.md) 的对照表。
>
> 本文件的 `U3.*` 是**重排后的新编号**。

## 目标

**用户点一下「创建项目」，选一个本地文件夹，Duet 就接管了它——
而且在点确认之前，他就知道 Duet 会往他的仓库里放什么。**

★ 这是用户流程的第一步（创建项目 → 创建对话 → 观测对话）。
2026-08-08 之前这个按钮是死的，用户打开应用第一句话就是
「为什么菜单没有显示项目列表和对话记录」。

## 完成标志

用户自己做这七件事，全部成功：

1. 点左栏「＋ 创建项目」→ **弹出对话框**，能选一个本地文件夹

   > 实操（2026-08-09）：浏览器点左栏「创建项目」→ 弹层出来了。
   > ★ Web 形态下 `showDirectoryPicker` 只给句柄不给路径，
   > 所以降级成手动粘贴（壳形态下才是「选择文件夹…」）。

2. 对话框里看到「将创建 `<项目>/.acpflows/`（skills/ · memory/ · project.yaml）」

   > 实操：粘 `/tmp/duet-demo2` → 五条「将创建」逐条列出，
   > **每条都带为什么**（如 `.acpflows/runs` → 「每次执行的过程记录，不入 git」）。

3. 对话框里看到「将追加 `.gitignore` — 忽略 `.acpflows/runs/`」

   > 实操：「将追加」单独一块（它动的是**用户自己的文件**），
   > 显示 `.gitignore` + 那一行 `.acpflows/runs/`。

4. 对话框里看到「**发现已有 Skill 目录 · N**」，列出扫到的 `.claude/skills/` 等，
   并标出哪些**校验没过**（如「缺 description」）

   > 实操：第一个演示项目里放了个缺 description 的 skill →
   > 显示「.claude/skills · my-skill · 校验未通过：frontmatter 缺 description」。
   > 第二个项目的 skill 放在 `tools/agent/skills/` 也扫得到——
   > 按设计稿扫的是 `**/skills`，不是两个固定目录。

5. 有 git remote 时，看到 GitHub 仓库与对应账号

   > 实操：`https://github.com/acme/second-repo.git` → 显示 URL + `acme/second-repo`，
   > 下面一行 `gh 已登录 HuLuca1998`。

6. 点「创建项目」→ 项目出现在左栏，**磁盘上真的多了 `.acpflows/`**

   > 实操：点确认后左栏立刻多了 `duet-demo2`（**没刷新页面**）；
   > 磁盘上 `.acpflows/` 里 memory / runs / skills / project.yaml 都在；
   > `.gitignore` 变成 `dist/` `*.log` `.acpflows/runs/`——
   > **用户原有的两行一行不少**；他的 `tools/agent/skills/good-one` 也没被动。

7. 提示明确：「创建项目不会开始任何工作」

   > 实操：弹层底部那句话在，且是设计稿原文
   > （「之后在项目下『＋ 新建对话』才会切 worktree」）。

★ 第 2–5 条是**确认之前**就看得到的。「先告知再动手」是这一步的全部意义：
用户交出来的是他自己的代码仓库。

## 已经就绪的地基

| 已完成 | 提交 | M3 用它来做什么 |
|---|---|---|
| `/v1/projects` 增删查 | `M2` 旧编号 | 弹层确认后调它落库 |
| 左栏项目树 | `120f019` 之后 | 创建成功后项目出现在这里 |
| Skill 库与校验 | `U2.2.1` | 扫到的 skill 用同一套校验，**不另起一套** |
| 记忆库 | `U2.3.1` | 初始化的 `memory/` 接进去 |

## 全局停止条件

1. 需要在用户仓库里写 `.acpflows/` 以外的任何东西 —— 停下来问
2. 用户选的目录不是 git 仓库 —— 如实告知，**不擅自 `git init`**
3. 扫到的 skill 需要**改写**才能用 —— 红线 3：不改用户项目的 skill

---

## S3.1 · 项目初始化（后端）

### ✓ U3.1.1 · 初始化预演与执行

| | |
|---|---|
| `goal` | 确认之前就能列出「将要做什么」，确认之后照单执行 |
| `allowed_changes` | `backend/internal/app/project/**` · `backend/internal/fsstore/project/**` |
| `forbidden_changes` | 预演与执行走两套代码（必然漂移）；在预演阶段写盘 |
| `stop_conditions` | 目标目录已有 `.acpflows/` 且内容与预期不符 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | **预演一个字节都不写** | 预演后断言目录树与 mtime 与调用前完全一致 |
| R2 | 预演列出的条目与执行实际创建的**逐条一致** | 用同一份计划分别跑预演与执行，断言两者的路径集合相等 |
| R3 | `.gitignore` 是**追加**不是覆盖 | 断言原有内容一行不少，且不重复追加第二次 |
| R4 | 非 git 仓库如实报告 | 断言返回明确标记，**且没有执行 `git init`** |
| R5 | 中途失败**不留半成品** | 注入写失败，断言已创建的部分被清掉 |
| R6 | 覆盖率 ≥ 85% | `make cover` |

**已完成**（89.2%）。★ R5 那条测试**先红发现了一个真 bug**：
`os.MkdirAll` 对已存在的目录成功返回，于是我们把它记成「这次创建的」，
回滚时 `RemoveAll` 掉——**连同里面用户已有的东西**。
计划里的 `AlreadyThere` 挡不住它：那是算计划**那一刻**的快照，而执行是后来发生的。
修法是动手前再 stat 一次。

★ R5 的判据也修正了：是「**我们自己建的东西**回到原样」，不是「整个目录回到原样」。
`.gitignore` 里追加的那一行**不回滚**——撤销它要读-改-写用户自己的文件，
而这中间他可能已经改过。一次失败的初始化毁掉他手写的规则，
比留下一行无害的忽略规则糟得多。

### ✓ U3.1.2 · 已有 Skill 扫描

| | |
|---|---|
| `goal` | 用户原来就有的 skill 被认出来，坏的说得出坏在哪 |
| `allowed_changes` | `backend/internal/app/skill/**`（扫描入口）· `backend/internal/fsstore/skill/**` |
| `forbidden_changes` | 写、移动、改名用户项目里的任何 skill 文件（红线 3） |
| `stop_conditions` | 发现某个来源目录的 skill 格式与 `U2.2.1` 的解析器不兼容 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 扫得到 `.claude/skills/` 与 `.acpflows/skills/` | 断言两个来源的条目都在结果里，且**标出来源** |
| R2 | 校验复用 `U2.2.1` 那一套 | 断言同一个坏 frontmatter 在两条路径下给出**同样的原因文本** |
| R3 | **只读**：扫完用户文件一字未动 | 扫描前后断言全目录的 mtime 与内容哈希不变 |
| R4 | 符号链接不跟出项目外 | 造一个指向项目外的链接，断言不被收录 |
| R5 | 一个都没有时是**空**不是错 | 断言返回空列表且无错误 |

**已完成**。★ 按设计稿扫的是 `**/skills`（跳过 node_modules 与 target），
不是只看两个固定目录——真实项目里它可能在 `.claude/skills`、
`tools/agent/skills`、`.acpflows/skills` 任何一处。
另外加了「找到 `skills/` 就不再下探」：继续下探的话，
每个 skill 自己的 `scripts/`、`references/` 都会被当成候选。

### ✓ U3.1.3 · GitHub remote 与账号识别

| | |
|---|---|
| `goal` | 用户看得到 Duet 认出的是哪个仓库、哪个账号 |
| `allowed_changes` | `backend/internal/app/project/**` · `backend/internal/platform/git/**` |
| `forbidden_changes` | 读取或存储任何凭据；发起需要鉴权的网络请求 |
| `stop_conditions` | 需要 token 才能识别账号 —— 那就只显示 remote，不猜账号 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 解析 `origin` 的 https 与 ssh 两种写法 | 断言两种 URL 得到同一个 `owner/repo` |
| R2 | 没有 remote 时如实说「无」 | 断言返回空值而非编造 |
| R3 | **不碰凭据** | 断言实现里没有读 `~/.gitconfig` 之外的凭据源，且无网络调用 |
| R4 | 非 GitHub 的 remote 照常显示 URL | 断言 GitLab 的 remote 不被丢弃 |
| R5 | 检测 `gh` 装没装、登没登录（**Q41**） | 断言两种缺失分别给出 `brew install gh` / `gh auth login` |

**已完成**（2026-08-09 真机验过：`ready` / 2.96.0 / HuLuca1998）。
★ 补了两条计划里没有的：
① **令牌绝不外泄**——`gh auth status` 的输出里有一行 `Token: gho_***`，
测试守着 Result 的**每个字段**都不含它。Q41 的全部意义就是不碰令牌。
② `https://user:token@github.com/...` 这种 remote 写法里夹的密码要摘掉——
那个字段会显示在界面上、写进日志。

---

## S3.2 · 创建项目弹层（前端）

### ✓ U3.2.1 · 创建项目对话框

| | |
|---|---|
| `goal` | 「＋ 创建项目」按钮活了，而且点确认前把要做的事说清楚 |
| `allowed_changes` | `frontend/src/features/project/**` · `frontend/src/features/rail/**` · i18n |
| `forbidden_changes` | 编造预演内容（必须来自后端）；偏离 `INVENTORY.md` §五的结构 |
| `stop_conditions` | 设计稿里找不到某个字段的展示形态 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 弹层四块齐全（路径 / 将创建 / 将追加 / 已有 Skill） | 断言四个区块标题逐字对上 `INVENTORY.md` §五 |
| R2 | 「将创建」的内容来自**后端预演**，不是前端写死 | mock 后端改一条，断言界面跟着变 |
| R3 | 校验没过的 skill **标出来并说原因** | 断言那一条显示原因文本 |
| R4 | 创建成功后项目出现在左栏，**不用刷新** | 断言左栏列表新增一项 |
| R5 | 创建失败时说清楚失败在哪一步 | 断言错误文案含后端返回的步骤名 |
| R6 | 「创建项目不会开始任何工作」这句话在界面上 | 断言该提示存在 |
| R7 | 能切英文 | 断言无硬编码中文 |

**已完成**。★ 补了计划里没有的一条：**浏览器形态降级成手动粘贴路径**。
`showDirectoryPicker` 出于安全只给句柄不给路径，而后端要绝对路径——
装作能选、然后拿一个假路径去请求的话，用户会在后端拿到「路径不存在」，
而他明明刚从对话框里选过。那种错误没人能自己解决。

★ 还补了一条负例才发现的：**换目录预演失败时要清掉上一次的清单**。
留着的话，用户选了 A 看到清单、又选了 B 但预演失败，他点确认——
而 `create` 用的是 `preview.path`，于是**对 A 执行了**，而他以为在建 B。
