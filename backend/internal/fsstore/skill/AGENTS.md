# AGENTS.md · backend/internal/fsstore/skill

> **就近优先**。上层规矩见 [`../AGENTS.md`](../AGENTS.md)，总纲见根 [`AGENTS.md`](/AGENTS.md)。

## 负责什么

扫描磁盘上的 Skill 目录，解析 frontmatter，把结果交 `domain` 判定。

```
Scan(Options) → []Entry     纯扫描，任何来源目录都能用
Store.ScanGlobal()          port.SkillScanner 的实现，定位 ~/.acpflows/skills
```

## ★★ 一个字节都不写

用户的 skill 是**他自己的产物**（红线 3）。这个包只读——
不改名、不移动、不「顺手补个默认 frontmatter」。

`TestScan_R5_DoesNotTouchUserFiles` 用**全目录内容哈希 + 文件清单**守这条。
判据不能换成「我们没写 `O_WRONLY`」——那管不住出于好意的改动。

## 两条来自负例的结论

| 结论 | 怎么发现的 |
|---|---|
| **符号链接有两道防线**（显式判 `ModeSymlink` + `ReadDir` 的 Lstat 语义），各自都独立有效 | 分别拆掉验过：任一道单独存在时都挡得住。留着 ① 是因为它写的是**意图**，② 只是 Lstat 语义的副作用 |
| `TrimRight(line, "\r")` 是**冗余**的 | 造负例时拆掉它测试照样绿——下游每处比较都走了 `TrimSpace`。测不到的代码要么删掉要么说清守什么 |

## 不引 YAML 库

frontmatter 是 `key: value` 平铺结构。为四个字符串键引一个能执行任意锚点展开的
解析器去读**用户提供的文件**，不值。

解不出来**不报错**，返回空值交给 `model.ValidateSkill` 去说「缺什么」——
在这里报错的话，错误信息只能是「YAML 解析失败」，而用户想知道的是「缺 description」。

## 三条不能改的行为

1. **目录不存在 = 空列表，不是错误。** 绝大多数项目没有 `.claude/skills`
2. **一条坏的不影响其余。** 整批失败会让用户连修复它的入口都找不到
3. **扫出来一律 `draft`**（INV-SKL-1）。扫盘就直接 `active` 的话，
   用户往目录里丢个文件就等于让它进了注入清单

## 检查命令

```bash
cd backend && go test ./internal/fsstore/skill/ -count=1
```
