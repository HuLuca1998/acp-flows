# AGENTS.md · backend/internal/fsstore

> **就近优先**。仓库总纲见根 [`AGENTS.md`](../../../AGENTS.md)。

## 负责什么

`.acpflows/` 下的 **md 文件读写、frontmatter 解析、与 DB 索引对账**
（[`架构 §129`](../../../docs/spec/architecture.md)）。

```
fsstore/
└── skill/     扫 skills 目录 → 解析 frontmatter → 交 domain 判定
```

## ★★ 这一层最重要的一条：碰用户的文件要极其小心

Skill 与记忆的目录**是用户自己的产物**。红线 3 明写：不改用户项目的 skill。

| ✗ 绝不做 | 为什么 |
|---|---|
| 「顺手补个默认 frontmatter」 | 那是替他改了他的文件，而他不会知道 |
| 「格式不对就改名/移动」 | 同上，且不可逆 |
| 跟着符号链接扫出去 | 项目里一个指向 `~` 的链接就能让扫描漫游整个家目录 |
| 一条坏就整批失败 | 用户会连修复它的入口都找不到 |

**扫描类函数要有「扫完内容哈希不变」的测试**，判据是全目录指纹，
不是「我们没写 O_WRONLY」——前者才管得住那些出于好意的改动。

## 不负责什么

| ✗ 不该出现在这里 | 该去哪 |
|---|---|
| 判断「这个 skill 合不合格」 | `domain/model`——那是业务规则，纯计算 |
| 事务、并发编排 | `app` |
| HTTP / JSON-RPC | `api` / `acp` |
| SQL | `store` |

分工的判据：**这里回答「盘上有什么」，domain 回答「它算不算数」。**

## 依赖方向

| | |
|---|---|
| 允许 import | 标准库 · `domain/model` · `platform` |
| 禁止 import | `app` · `api` · `store` · `acp` |

## 不引 YAML 库解 frontmatter

frontmatter 是 `key: value` 的平铺结构。为四个字符串键引一个 YAML 库，
换来的是**一个能执行任意锚点展开的解析器去读用户提供的文件**。不值。

解不出来时**不报错**，返回空值让 `domain.ValidateSkill` 去说「缺什么」——
在这里报错的话，错误信息只能是「YAML 解析失败」，
而用户想知道的是「缺 description」。

## 测试要求

| | |
|---|---|
| 覆盖率门槛 | **≥ 70%**（`testing-strategy.md` §84） |
| 形态 | **真目录真文件**（`t.TempDir()`），不 mock 文件系统 |
| 必覆盖 | 只读性（内容哈希）· 符号链接 · 单条坏不影响其余 · 目录不存在 |

★ mock 一个 fs 测不出「符号链接不跟出去」和「扫完一字未动」——
而这两条正是这一层最该守的。

## 检查命令

```bash
cd backend && go test ./internal/fsstore/... -count=1
```
