# AGENTS.md · backend/internal/store

> **就近优先**。设计规范在 [`../../../docs/rules/database.md`](../../../docs/rules/database.md)，
> 选型理由在 [`../../../docs/adr/0005-persistence.md`](../../../docs/adr/0005-persistence.md)，
> 调试与改数据用 `db-operate` skill。**动手前先读 database.md，它比本文详细。**

## 负责什么

SQLite 持久化：GORM 实体、版本化迁移、按聚合分的 repo、复杂查询。

```
store/
├── store.go        包门面：Store 结构体、New()、DSN 与连接池装配
├── errors.go       GORM 错误 → 领域错误的翻译
├── tx.go           事务封装（事务边界由 app 层决定，这里只提供机制）
├── entity/       ★ GORM 实体，本包私有
├── mapper/       ★ 领域模型 ↔ 实体 的双向映射
├── migration/      版本化 SQL + 嵌入 + checksum 校验
├── query/          报表统计、DAG 展开这类复杂查询
├── work_repo.go    一个聚合一个文件
├── plan_repo.go
└── unit_repo.go
```

## 不负责什么

- **不写业务规则。** 状态能不能从 A 到 B 是 `domain` 的事
- **不定事务边界。** 边界在 `app` 用例里，本包只提供 `InTx` 机制
- **不做 IO 以外的编排。** 调 ACP、跑 git、读 md 文件都不在这里
- **不把 GORM 实体暴露出去。** port 接口收发的一律是 `*model.X`

## 依赖方向

| | |
|---|---|
| 允许 import | `domain/model` · `constant` · `util` · `app/port`（只为实现接口）· `gorm.io/gorm` |
| 禁止 import | `app/usecase` · `api` · `acp` · `gitx` · `ghx` · 其他基础设施包 |

**`domain` 禁止 import gorm** —— 由 depguard 强制。
反过来 `store` import `domain` 是允许的（映射需要）。

## 三条最容易犯的错

### 1. 给领域模型挂 gorm 标签

```go
// ✗ 一挂上去，domain 就 import 了 gorm，铁律与 depguard 同时破
package model
type Work struct { ID string `gorm:"primaryKey"` }
```

实体在 `entity/`，领域模型在 `domain/model/`，中间隔一个 `mapper/`。
理由见 `database.md` §1。

### 2. `Updates` 传 struct

零值字段会被**静默忽略**。状态机把状态改成看起来像零值的值时，更新会丢。

```go
db.Model(&e).Updates(map[string]any{"state": s, "updated_at": now})   // ✓ 一律用 map
```

### 3. 用 `Find` 查单条

`Find` 记录不存在时 `err` 是 `nil`。查单条一律 `First`，并把
`gorm.ErrRecordNotFound` 翻译成 `model.ErrNotFound`。

**GORM 错误不出本包。** `app` 层用 `errors.Is(err, model.ErrNotFound)` 判断。

## 检查命令

```bash
cd backend && go test ./internal/store/... -count=1
cd backend && go test -tags=integration ./tests/integration/... -run Store
make -C ../../.. cover        # store 覆盖率门槛 70%
```

## 本域特有的坑

- **`foreign_keys` pragma 默认关闭。** 不显式打开等于外键白写：删父行子行还在。
  连接串必须带 `_pragma=foreign_keys(ON)`，**且要有断言它开着的测试**。
- **测试用临时文件，不用 `:memory:`。** `:memory:` 测不出 WAL 与并发行为，
  而产品会并发跑多个 Work。
- **事务里禁止做 IO。** SQLite 同一时刻只允许一个写者，
  在事务里调 ACP 或跑 git 会把其他所有 Work 卡死。这是设计错误不是性能问题。
- **`SetMaxOpenConns(1)`** 是刻意的：把并发写序列化在应用层，换取零 `SQLITE_BUSY`。
  改这个值前先跑并发测试。
- **迁移 checksum 变了会直接报错退出。** 这是特性——已应用的迁移被改过是严重问题。
  要改结构就加新迁移文件，不要动旧的。
- **别用 `AutoMigrate`**，别用 `gorm.DeletedAt`，别用 GORM 关联标签，别写 Hooks。
  每一条的理由都在 `database.md` §8–§9。
