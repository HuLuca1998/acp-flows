# AGENTS.md · backend/internal/app/memory

> **就近优先**。上层规矩见 [`../AGENTS.md`](../AGENTS.md)，总纲见根 [`AGENTS.md`](/AGENTS.md)。

## 负责什么

记忆的查询与**人工审核**。

## ★★ 这一层存在的唯一理由：INV-MEM-2「绝不自动写入」

`candidate → active` 只能走 `Confirm(ctx, id, actor)`，**且 actor 必填**。

允许匿名确认的话，一个后台任务就能把 AI 提的候选变成长期记忆——
而 `AGENTS.md` §9 把「自动把一次成功经验写成长期记忆」列为**明令反例**。

错了的后果不是「多一条记忆」，而是 **AI 把自己的一次臆断变成了
以后每一轮的前提**，而用户从没看过那句话。

这条在四个地方守着：

| 层 | 怎么守 |
|---|---|
| `domain` | `ProposeCandidate` 只造得出 candidate；`Confirm` 空 actor 报错 |
| 这里 | `review` 先查 actor 再动状态机 |
| `api` | 空 actor 回 400，且**状态没变** |
| 前端 | 候选条目上那两个按钮就是「用户的那一下」 |

## 不重复 domain 的守卫

状态机判定在 `model.Memory` 上，这里**只调不判**。判两遍必然漂移，
而漂移的那一侧会静默放行。

## 检查命令

```bash
cd backend && go test ./internal/app/memory/... ./internal/api/ -run Memor -count=1
```
