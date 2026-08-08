# M2 · 它有了角色、技能和记性

> ★ **2026-08-08 里程碑按引用关系重排**（见 `design/DEPENDENCIES.md`）。
> 旧的 `M2`（提需求、看 AI 干活）连同它的 `U2.*` 编号已归档到
> [`archive/M2-talk-and-observe.md`](archive/M2-talk-and-observe.md)——
> 那里面做完的东西没有白做，落位见 [`roadmap.md`](../roadmap.md) 的对照表。
>
> 本文件的 `U2.*` 是**重排后的新编号**。查旧记录时以归档目录为准。

## 目标

**用户看得到 Duet 有哪几个「员工」、每个员工用什么 Runtime、能读到哪些技能和记忆。**

★ 这一步用户**看得到但还用不上**——它是后面所有事的地基：
创建项目要扫 skills、对话 AI 要读记忆、计划里每条要派角色。

## 完成标志

用户自己做这五件事，全部成功：

1. 打开「角色与 Runtime」页 → 看到**八个内置角色**的表，每行有
   「承担的操作」「性格与语气」「Runtime」「会话模式」「权限裁决」
2. 打开「Skill」页 → 看到**全局 Skill 库**（`~/.acpflows`），每条带版本号、
   状态（active / draft），**校验没过时说明为什么**
3. 打开「记忆」页 → 看到**跨项目记忆**（`~/.acpflows`），每条带类型
   （constraint / experience）与状态（active / 候选 / 已失效）
4. 三个页面都能筛选，且**不再是骨架占位**
5. 角色的「会话模式」能选档位，选了 `read-only` 的角色**开会话时真的收权**——
   让它改文件，**磁盘上不会有那个改动**

## 已经就绪的地基

| 已完成 | 提交 | M2 用它来做什么 |
|---|---|---|
| ACP 传输层与会话 | `29a45c1` `1dc00bf` | 发 `set_config_option` 收权 |
| Runtime 探测注册表 | `ba73425` | 角色绑定 Runtime 时列出可选项 |
| 权限裁决三策略 | `1dc00bf` | ★ **要从会话层迁到角色层** |
| `platform.Paths` | `M0` | `~/.acpflows` 的位置 |

## 全局停止条件

1. 发现某个 Runtime 不支持 `set_config_option` **也不支持** `set_mode` ——
   那意味着收不了权，**必须停下来问用户**，不能假装收了
2. 需要写 `~/.claude` 或 `~/.codex` —— 那是红线（`acp-field-notes.md` §4）
3. 角色数量或职责与 `design/INVENTORY.md` §八 对不上 → 先补设计条目

---

## S2.1 · 角色库

### ○ U2.1.1 · 角色模型与内置八角色

| | |
|---|---|
| `goal` | 有一份「谁是谁、用什么 Runtime、什么权限」的可查数据 |
| `allowed_changes` | `backend/internal/domain/model/role.go` · `backend/internal/app/role/**` |
| `forbidden_changes` | `domain` 出现 `context.Context` 或 `time.Now()`；把权限档硬编码成某一端的取值 |
| `stop_conditions` | 设计稿的角色职责与 `docs/spec/` 里的描述冲突 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 内置**八**个角色，与设计稿逐字一致 | 断言八个名称与 `operations` 逐条对上 `domain-model.md` §16.1 |
| R1b | **11 个 AI 操作全被覆盖**（INV-ROLE-6） | 断言八个角色的 `operations` 并集 == 11 个操作全集 |
| R2 | 角色 → Runtime 的绑定可查可改 | 断言改完读回来是新值 |
| R3 | **权限档是两端映射，不是硬编码** | 断言同一个「只读」在 claude 上是 `plan`、在 codex 上是 `read-only` |
| R4 | 认不出的角色名一律拒绝 | 断言返回明确错误，**不静默回落到默认角色** |
| R5 | 覆盖率 ≥ 90%（domain 层） | `make cover` |

### ○ U2.1.2 · 用 `set_config_option` 收权

| | |
|---|---|
| `goal` | 选了只读的角色，Agent 那侧真的写不了文件 |
| `allowed_changes` | `backend/internal/acp/session/**` · `backend/internal/acp/runtime/**` |
| `forbidden_changes` | 用已废弃的 `set_mode` 作为首选；收权失败却继续开工 |
| `stop_conditions` | 两个方法都不支持 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | **首选 `session/set_config_option`** | 断言发出的方法名是它，不是 `set_mode` |
| R2 | 参数名是 `configId` 不是 `optionId` | 断言线上帧的字段名 |
| R3 | 按 `category` 取选项，不按 `id` | 断言两端 `id` 不同（`effort` vs `reasoning_effort`）时都能取到 |
| R4 | 不支持时**降级**到 `set_mode` | 断言降级路径发出的是 `set_mode` |
| R5 | **两个都不支持 → 报错拒绝开工** | 断言返回错误且**没有**发出 `session/prompt` |
| R6 | 设了档位当场回读校验 | 断言响应里的 `configOptions` 含设定值 |

---

## S2.2 · Skill 库

### ○ U2.2.1 · 全局 Skill 库与校验

| | |
|---|---|
| `goal` | 用户看得到有哪些技能可用，坏的那些说得出坏在哪 |
| `allowed_changes` | `backend/internal/fsstore/skill/**` · `backend/internal/app/skill/**` |
| `forbidden_changes` | 写用户项目里的 skill 文件（只读，红线 3） |
| `stop_conditions` | SKILL.md 的必填字段与 `docs/spec/` 不一致 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 扫 `~/.acpflows/skills`，解析 frontmatter | 断言 name / description / version 都读得出 |
| R2 | **缺 `description` 判 draft 并说明原因** | 断言状态是 draft 且原因文本含「description」 |
| R3 | frontmatter 坏了**不影响其余条目** | 断言一条坏的旁边那条照样 active |
| R4 | 版本号可比较 | 断言 `v2.1` > `v1.4` |
| R5 | **不写用户的文件** | 扫完断言目录的 mtime 未变 |

---

## S2.3 · 记忆库

### ○ U2.3.1 · 跨项目记忆库

| | |
|---|---|
| `goal` | 用户看得到 Duet 记住了什么，能筛掉不要的 |
| `allowed_changes` | `backend/internal/store/**` 的 memory 表 · `backend/internal/app/memory/**` |
| `forbidden_changes` | 记忆内容由 AI 直接写库（必须经审核，见 `M10`） |
| `stop_conditions` | 记忆类型的全集与设计稿对不上 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 类型是封闭枚举（constraint / experience …） | 断言穷举，新增时测试会红 |
| R2 | 状态三态：active / 候选 / 已失效 | 断言按状态筛选各得其所 |
| R3 | 跨项目与项目级**分开存** | 断言项目级的不出现在跨项目列表里 |
| R4 | **候选态的不参与注入** | 断言注入清单里没有候选条目 |

---

## S2.4 · 三个页面

### ○ U2.4.1 · 角色 / Skill / 记忆页面

| | |
|---|---|
| `goal` | 三页不再是骨架占位 |
| `allowed_changes` | `frontend/src/features/{roles,skill,memory}/**` · i18n |
| `forbidden_changes` | 编造数据（没有就显示空态）；偏离设计稿的表结构 |
| `stop_conditions` | 设计稿找不到某个字段的展示形态 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 角色表七列齐全 | 断言表头逐字对上 `INVENTORY.md` §八 |
| R2 | Skill 条目显示版本与校验原因 | 断言 draft 那条显示「缺 description」 |
| R3 | 记忆按状态筛选 | 断言点「候选」后只剩候选条目 |
| R4 | **查询失败时说出来**，不显示空列表 | 断言出现错误提示而非「还没有」 |
| R5 | 三页都能切英文 | 断言无硬编码中文 |
