# AGENTS.md · backend/internal/app/role

> **就近优先**。上层规矩见 [`../AGENTS.md`](../AGENTS.md)，总纲见根 [`AGENTS.md`](/AGENTS.md)。

## 负责什么

把**角色定义**（`domain/model`）与**Runtime 绑定**（adapter，经 `port.RoleBindings`）
拼成界面要显示的那张表。

## 为什么要有这一层

设计稿的原则是「**角色先定义、再绑定 Runtime**」——绑定是关系，不是角色自身的属性。
所以：

| 在哪 | 什么 |
|---|---|
| `domain/model.Role` | 角色是谁、干什么、要多大权限（**语义档**） |
| `acp/runtime` | 绑哪个 Runtime、那一端的档名叫什么（**品牌知识**） |
| 这里 | 把两边拼起来，且**任一边坏了都不让整张表消失** |

## 一条不能改的行为

**绑定查不到时不跳过那个角色**，而是把它连同 `Problem` 一起返回。

跳过的话，用户在界面上看到七个角色，而他不知道少了哪一个、为什么少——
这比「显示一个带错误说明的角色」糟得多。

## 检查命令

```bash
cd backend && go test ./internal/app/role/... ./internal/api/ -count=1
```
