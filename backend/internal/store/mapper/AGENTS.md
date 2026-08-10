# AGENTS.md · backend/internal/store/mapper

> **就近优先**。上层规矩见 [`../AGENTS.md`](../AGENTS.md)，总纲见根 [`AGENTS.md`](/AGENTS.md)。

## 负责什么

领域模型 ↔ 行结构的双向映射。**只做搬运，不做判断。**

## 两条铁的规矩

### 1. 不取时间

时间戳由 repo 用注入的 `Clock` 填，映射本身不碰 `time.Now()`——
碰了就没法写确定性测试。

### 2. 回程走 `Restore*` 而不是构造函数

| 方向 | 用什么 | 为什么 |
|---|---|---|
| 模型 → 行 | 直接读字段 | — |
| 行 → 模型 | `model.Restore*` | 构造函数校验的是**用户输入**；这里读的是已经存过的东西。<br>用构造函数的话，校验规则一变严，用户列表里的老数据就突然读不出来了 |

★★ `RestoreMemory` 能造出 `active` 的记忆，是绕过 INV-MEM-2 的**唯一口子**。
它只许在这里被调用——`app` / `api` 想新建记忆只有一条路：
`ProposeCandidate` 然后由人 `Confirm`。

## 一条来自负例的结论

`splitRefs` 里**没有**「空串早返回」那一段：造负例时发现拆掉它测试照样绿
（`strings.Split("", ",")` 给出 `[""]`，正好被下面的 `p != ""` 过滤掉）。
测不到的分支留着只会让人以为它在防什么。

## 检查命令

```bash
cd backend && go test ./internal/store/... -count=1
```
