# AGENTS.md · backend/internal/fsstore/memory

> 本目录的规则。**就近优先**：与根 [`AGENTS.md`](../../../../AGENTS.md) 冲突时以本文件为准。

## 负责什么

**记忆的正文**——一条记忆的 md 文件读写、frontmatter 拆分。

## 不负责什么

| 不在这里 | 去哪 | 为什么 |
|---|---|---|
| 记忆的索引（类型/状态/时间） | `store/memory_repo.go` | ★★ INV-MEM-8：正文不进数据库 |
| 状态迁移（candidate → active） | `domain/model/memory.go` | 规则是领域的事 |
| 候选从哪来 | `app/work` | 那是解析 AI 回复 |

## 依赖方向

| | |
|---|---|
| 允许 import | 标准库 |
| 禁止 import | `store` · `app` · 任何 ORM |

## 检查命令

```bash
go test ./internal/fsstore/memory/
```

## 改这里之前必读

- [`M10 施工图`](../../../../docs/plan/milestones/M10-memory-and-skills.md) · `U10.1.1`
- 根 `AGENTS.md` 的 INV-MEM-8

## 本域特有的坑

- ★★ **正文只在 md 文件里。** 用户要能用任何编辑器打开它、改它、
  用 git 管它。塞进数据库的话，那条记忆就只能通过 Duet 的界面看——
  而记忆是他自己的资产，不该被一个应用扣住。
- ★★ **每次都从磁盘读，不缓存。** 他可能刚用编辑器改过，
  我们给他看的必须是他刚写下的那一版。
- ★★ **解析失败不丢内容**：`Malformed` 置位，`Text` 给整个原文。
  一条 frontmatter 少了个引号就吞掉用户写的三百字，是最糟的处理方式。
- ★ **文件不见了要说清楚**（带路径），不当成空正文——
  空正文看起来像「这条记忆没内容」，而真相是「文件丢了」。
- ★ 挡住越界写的**只有 `ErrBadID` 那一道字符检查**。原来跟着一段
  `filepath.Rel` 的「双保险」，但它永远进不去（能走到那行的 id 已经不含
  `/` `\` `..`）。留着会让人以为有两层防御，而实际只有一层。
