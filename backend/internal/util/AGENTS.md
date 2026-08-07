# AGENTS.md · backend/internal/util

> **就近优先**。这个目录有本仓库最严格的准入规则——它最容易腐化成垃圾桶。

## 负责什么

生产代码用的**纯工具函数**。按主题分文件：`slice.go` `ptr.go` `strx.go` `pathx.go` `retry.go`。

## 准入门槛：四条全满足才能进来

1. **纯函数** —— 无 IO、无全局状态、无时间/随机源（要用就注入）
2. **零业务语义** —— 不认识 Work / Unit / Contract / Runtime 这些概念
3. **已有 ≥ 2 个真实调用方，且跨包** —— 只有一个调用方就放在调用方旁边
4. **有单元测试** —— 无测试的工具函数不许合并

**第 3 条是重点：抽象的触发条件是第 2 个真实调用方出现，不是预感会复用。**
猜测中的复用是负债。

## 不负责什么

| ✗ 不该进来的 | 该去哪 |
|---|---|
| 带业务语义（`FormatUnitID`） | `domain/model`，做成类型的方法 |
| 碰 IO（`ReadConfig`） | `platform` 或 `fsstore` |
| 只有一个调用方 | 就放在调用方文件里 |
| 测试辅助 | `backend/tests/testutil` |
| 常量、枚举 | `internal/constant` |

## 禁止的文件名

`util.go` `utils.go` `helper.go` `helpers.go` `common.go` `misc.go`

**文件名不说明内容 = 垃圾桶。** 这四个名字由 `scripts/check-naming.sh` 在 CI 拦下。

文件名即主题：一眼看不出里面装什么，就是名字起错了。

## 索引是强制的

**写任何新工具函数前，先搜 [`INDEX.md`](INDEX.md)。**

新增 / 删除 / 改签名后必须同步索引：

```bash
make check-util-index
```

导出了没登记、登记了没实现、签名对不上——三种情况都会红。

## 检查命令

```bash
cd backend && go test ./internal/util/... -count=1
make -C ../../.. check-util-index
```

## 改这里之前必读

- [`../../../docs/coding-standards.md`](../../../docs/rules/coding-standards.md) §1.3–§1.4

## 本域特有的坑

- **`pathx` 不许碰文件系统。** 路径的纯计算（拼接、规范化、判断包含关系）在这里；
  真的去 `os.Stat` 就该去 `platform`。
- **泛型别写过头。** `Chunk[T any]` 好；三个类型参数加一堆约束的通常说明抽错了层。
- **别为了让覆盖率好看而把私有逻辑挪进来导出。** 那是把内部实现变成公开契约。
