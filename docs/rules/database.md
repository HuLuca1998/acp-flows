# 数据库与模型设计规范

> 选型理由见 [`adr/0005-persistence.md`](../adr/0005-persistence.md)。
> 调试与改数据的操作手册在 `db-operate` skill，**不在本文**——本文只管「怎么设计」。

| | |
|---|---|
| ORM | **GORM** |
| 数据库 | **SQLite**，纯 Go 驱动 `github.com/glebarez/sqlite`（无 CGO，交叉编译与 CI 简单） |
| 位置 | `~/.acpflows/duet.db` |
| 迁移 | GORM AutoMigrate **不用**，走**版本化 SQL 迁移**，见 §5 |

---

## 1. 最重要的一条：GORM 实体 ≠ 领域模型 ★

**AI 最容易犯的错，是给领域模型挂上 `gorm:"..."` 标签。** 那一瞬间：

- `domain` 包 import 了 `gorm.io/gorm` → 违反铁律与 depguard
- 持久化细节（主键策略、软删除、索引）渗进业务规则
- 换存储、改表结构会连带改业务代码

**边界写死：**

```
internal/domain/model/work.go     ← 领域模型。零 gorm 标签，零 gorm import
internal/store/entity/work.go     ← GORM 实体。store 包私有，不导出到包外
internal/store/mapper/work.go     ← 两者之间的双向映射
```

| | 领域模型 | GORM 实体 |
|---|---|---|
| 位置 | `internal/domain/model/` | `internal/store/entity/` |
| 可见性 | 全仓库可用 | **`store` 包内私有**，不出现在任何 port 接口签名里 |
| 字段 | 领域概念（`state constant.WorkState`） | 表结构（`State string` + `gorm:"index"`） |
| 方法 | 状态流转、不变量校验 | **无方法**，纯数据 |
| 导出字段 | 一律小写私有 + 访问器 | 必须导出（GORM 要求） |

```go
// ✗ 绝对禁止：领域模型挂 gorm 标签
package model
type Work struct {
    ID    string `gorm:"primaryKey"`   // ← depguard 会拦，但更重要的是这本身就错了
}

// ✓ 分开
package entity                          // internal/store/entity
type Work struct {
    ID        string    `gorm:"column:id;primaryKey;size:64"`
    State     string    `gorm:"column:state;size:32;index:idx_works_state"`
    CreatedAt time.Time `gorm:"column:created_at;index:idx_works_created_at"`
}
func (Work) TableName() string { return "works" }
```

**`app` 与 `api` 层永远看不到 `entity`。** port 接口收发的是 `*model.Work`。

---

## 2. 表命名

| 对象 | 规则 | 例 |
|---|---|---|
| 表名 | `snake_case` **复数** | `works` `plan_versions` `unit_contracts` `attempts` |
| 关联表 | `<A>_<B>` 字典序，复数 | `unit_evidences` |
| 列名 | `snake_case` 单数 | `work_id` `contract_version` `frozen_at` |
| 主键 | 一律 `id` | |
| 外键 | `<被引表单数>_id` | `work_id` `subplan_id` |
| 布尔 | `is_` / `has_` / `can_` 前缀 | `is_frozen` `has_evidence` |
| 时间 | `_at` 后缀，一律 `DATETIME` 存 **UTC** | `created_at` `updated_at` `frozen_at` |
| 计数 | `_count` 后缀 | `attempt_count` |
| 枚举列 | 存**字符串原值**，不存整数 | `state` 存 `'executing'` |

**必须显式写 `TableName()` 与 `column:` 标签。** 不要依赖 GORM 的自动推导——
推导规则一旦变化，或字段改名，表结构会静默漂移。

### 枚举列为什么存字符串

状态词在界面上原样等宽显示（`executing`），存整数意味着每次查询都要翻译，
而且 `sqlite3` 里直接看数据时全是数字，排查成本陡增。空间不是这个应用的瓶颈。

---

## 3. 主键：字符串 ID，不用自增整数

主键是**带类型前缀的字符串**：`work-08` · `unit-012` · `mem-203` · `ck-07`。

理由：
1. 界面上大量展示这些 ID（设计规范要求等宽显示），用整数会让前端到处拼字符串
2. 跨表引用一眼能看出引用的是什么
3. 导出/导入、日志、事件流里可读

**例外**：事件表用 ULID（`evt_01J...`），因为它需要**按时间可排序**且高频写入。

事件的 `seq` 是**单调递增整数**，与 ID 分开——「取消后最后事件游标可读」依赖它。

---

## 4. 索引

| 规则 | |
|---|---|
| 命名 | `idx_<表>_<列>[_<列>]`，唯一索引用 `uq_` 前缀 |
| 必建 | 全部外键列；所有出现在 `WHERE` / `ORDER BY` 里的列 |
| 复合索引 | 最左前缀原则，**顺序写进注释说明为什么是这个顺序** |
| 禁止 | 建了但没有查询用到的索引（写入成本白付） |

```go
State string `gorm:"column:state;size:32;index:idx_works_state"`
// 复合：按 work 查事件并按 seq 排序 —— 这个顺序不能反
Seq   int64  `gorm:"column:seq;index:idx_events_work_id_seq,priority:2"`
WorkID string `gorm:"column:work_id;index:idx_events_work_id_seq,priority:1"`
```

**加索引必须同时说明它服务哪个查询。** 说不出来就不要加。

---

## 5. 迁移：版本化 SQL，不用 AutoMigrate

```
internal/store/migration/
├── 0001_init.sql
├── 0002_add_checkpoint_commit.sql
└── migration.go        # 嵌入 + 顺序执行 + 版本表
```

**为什么不用 `AutoMigrate`**：

- 它不删列、不改类型、不重命名——真实演进它做不了
- 它的行为随 GORM 版本变化，是隐式的
- **没有 down 路径**，出问题无法回退
- 用户机器上的数据库是**用户的数据**，不能让一个隐式推导去改它

**规则：**

1. 文件名 `NNNN_<动词>_<对象>.sql`，编号**只增不复用**
2. 每个迁移**幂等**：`CREATE TABLE IF NOT EXISTS` / 先查 `pragma_table_info` 再 `ALTER`
3. 迁移记录写 `schema_migrations` 表（`version` `applied_at` `checksum`）
4. **checksum 变了直接报错退出**——已应用的迁移被人改过是严重问题，不能静默继续
5. 启动时按版本顺序执行，全部在**一个事务**里
6. **每个迁移必须有测试**：空库跑通 + 从上一版本升级跑通

> 破坏性迁移（删列、改类型）在桌面应用上尤其危险——用户没有 DBA，没有备份。
> 迁移前自动备份 `duet.db` 到 `duet.db.bak-<version>`。

---

## 6. SQLite 的必设项

```go
// 连接串必须带这些参数，缺一个都会出问题
dsn := "file:" + path + "?" + strings.Join([]string{
    "_pragma=journal_mode(WAL)",      // 读写不互斥；桌面应用会并发跑多个 Work
    "_pragma=busy_timeout(5000)",     // 锁等待而不是立即 SQLITE_BUSY
    "_pragma=foreign_keys(ON)",       // ★ SQLite 默认是关的，不开等于没有外键
    "_pragma=synchronous(NORMAL)",    // WAL 下的推荐值
}, "&")
```

**`foreign_keys` 默认关闭**是 SQLite 最著名的坑：外键约束写了但不生效，
删了父行子行还在。**必须显式打开，且要有测试断言它开着。**

### 并发

- SQLite 同一时刻只允许**一个写者**（WAL 下读者不阻塞）
- 写操作必须短，长事务会把其他 Work 卡住
- **禁止在事务里做 IO**（调 ACP、跑 git、读文件）——事务只包 DB 操作

---

## 7. 查询规范

| 规则 | |
|---|---|
| **禁止 `SELECT *`** | 显式列出列。加列时不会静默改变返回结构 |
| **禁止裸 SQL 字符串拼接** | 一律用 GORM 的参数绑定或 `?` 占位 |
| **禁止 N+1** | 关联查询用 `Preload` / `Joins`，且**要有断言查询次数的测试** |
| 分页 | cursor 分页（`WHERE seq > ? LIMIT ?`），不用 `OFFSET` |
| 软删除 | **不用 GORM 的 `gorm.DeletedAt`**。领域里「失效 ≠ 删除」有明确语义（`status` 列），别再叠一层 |
| 事务 | 事务边界在 `app` 层用例里，不在 `store` 里 |

### 复杂查询放子包

```
internal/store/query/   # 报表统计、DAG 展开这类复杂查询
```

`store` 根目录只放按聚合分的 **repo**（`work_repo.go` `plan_repo.go`…），一个聚合一个文件。

**行结构与映射不放在 repo 文件里**，而是分包：

```
store/
├── work_repo.go     ← 查询与写入（一个聚合一个文件）
├── entity/work.go   ← 行结构（§1 要求与领域模型分离）
└── mapper/work.go   ← 双向映射
```

`design-principles.md` §5.2 的「按领域概念切、不按技术种类切」在这里依然成立——
切的是 `work` / `plan` / `unit`，只是每个概念横跨三个包。
**这三个包的存在是为了守住 §1 的边界**（领域模型不许挂 gorm 标签），
是一条有明确理由的例外，不是「按技术种类切」的复辟。

---

## 8. GORM 模型定义规范

### 标准模板 —— 照抄这个形状

```go
// internal/store/entity/work.go
package entity

// Work 是 works 表的行结构。
//
// 这是 store 包的私有类型，不出现在任何 port 接口签名里。
// 与领域模型的映射在 internal/store/mapper/work.go。
type Work struct {
	ID        string    `gorm:"column:id;primaryKey;size:64"`
	ProjectID string    `gorm:"column:project_id;size:64;not null;index:idx_works_project_id"`
	State     string    `gorm:"column:state;size:32;not null;index:idx_works_state"`
	Branch    string    `gorm:"column:branch;size:255;not null"`
	Worktree  string    `gorm:"column:worktree;size:1024;not null"`
	CreatedAt time.Time `gorm:"column:created_at;not null;index:idx_works_created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null"`
}

// TableName 显式指定表名，不依赖 GORM 的自动推导。
func (Work) TableName() string { return "works" }
```

### 每个字段必写四项

| 项 | 为什么 |
|---|---|
| `column:<列名>` | 字段改名时表结构不会静默漂移 |
| `size:<长度>` | 迁移 SQL 与实体保持一致；不写会退化成无界文本 |
| `not null` 或明确可空 | 可空性是业务语义，不能靠默认 |
| 索引（若需要） | 见 §4 |

**可空列用指针或 `sql.Null*`，不要用零值当"空"。**
`""` 与「没填」在业务上是两件事——`frozen_at` 为零值到底是「没冻结」还是「1970 年冻结的」？

```go
FrozenAt *time.Time `gorm:"column:frozen_at"`   // ✓ nil = 尚未冻结
```

### 禁止在实体上做的事

| ✗ | 为什么 | 正确做法 |
|---|---|---|
| 嵌入 `gorm.Model` | 塞进 `ID uint` 自增主键 + `DeletedAt` 软删除，两个都与本项目冲突 | 显式写字段 |
| 定义方法（除 `TableName`） | 实体是纯数据；有方法说明业务逻辑漏到持久化层了 | 方法写在 `model` 上 |
| GORM 关联标签（`foreignKey` / `many2many`） | 见下 | 显式外键列 + 显式查询 |
| GORM Hooks（`BeforeSave` / `AfterFind`…） | **业务逻辑藏在 ORM 回调里，测试与阅读都找不到** | 逻辑写在 `app` 用例里 |
| `gorm.DeletedAt` | 领域里「失效 ≠ 删除」已有 `status` 语义 | 用 `status` 列 |

### 关联：只存外键列，不用 GORM 关联

```go
// ✗ 隐式加载、N+1、级联删除行为不可控
type Work struct {
    Units []Unit `gorm:"foreignKey:WorkID"`
}

// ✓ 只存外键，要用时显式查
type Unit struct {
    WorkID string `gorm:"column:work_id;size:64;not null;index:idx_units_work_id"`
}
```

理由：GORM 关联的加载时机是隐式的，很容易在循环里触发 N+1；级联行为跨版本不稳定。
**外键约束交给数据库（`foreign_keys` pragma 打开），加载交给显式查询。**

需要一次取父子时用 `Preload`，并且**必须有断言查询次数的测试**（见 §7）。

---

## 9. GORM 使用规范

### 必须 `WithContext`

```go
// ✓ 所有查询都要能被取消 —— M1 的 update prepare 依赖这个
if err := r.db.WithContext(ctx).Where("id = ?", id).First(&e).Error; err != nil { … }

// ✗ 无法取消，请求断了查询还在跑
r.db.Where("id = ?", id).First(&e)
```

由 `golangci-lint` 的自定义规则拦：`store` 包内出现 `r.db.` 后面不接 `WithContext` 即报错。

### 四个必知的 GORM 陷阱 ★

这些是 AI 写 GORM 时最常踩的，**每一条都要有测试守着**。

#### ① `Updates` 传 struct 会忽略零值

```go
// ✗ State 是 "" 或 0 时这个字段根本不会被更新，静默失败
db.Model(&e).Updates(entity.Work{State: newState})

// ✓ 用 map，零值也会写
db.Model(&e).Updates(map[string]any{"state": newState, "updated_at": now})

// ✓ 或显式 Select 要更新的列
db.Model(&e).Select("state", "updated_at").Updates(&e)
```

**本项目一律用 map 形式。** 状态机把状态改成某个"看起来像零值"的值时，
struct 形式会静默丢更新——这类 bug 极难排查。

#### ② `Find` 不返回 `ErrRecordNotFound`

```go
// ✗ 记录不存在时 err 是 nil，list 是空切片 —— 以为查到了
db.Where("id = ?", id).Find(&e)
if err != nil { … }

// ✓ 单条查询用 First / Take，它们才返回 ErrRecordNotFound
if err := db.Where("id = ?", id).First(&e).Error; err != nil {
    if errors.Is(err, gorm.ErrRecordNotFound) {
        return nil, fmt.Errorf("find work %s: %w", id, model.ErrNotFound)
    }
    return nil, fmt.Errorf("find work %s: %w", id, err)
}
```

**规则**：查单条一律 `First`；`Find` 只用于查列表，且**列表为空不是错误**。

#### ③ `Save` 会更新全部字段

`Save` 把整行覆盖写，包括你没打算改的字段。并发下会覆盖别人的更新。

**本项目禁止 `Save`。** 新增用 `Create`，更新用 `Updates` + map。

#### ④ 批量操作会全表扫

GORM 对没有 `WHERE` 的 `Delete` / `Update` 默认会报 `ErrMissingWhereClause`（好事），
**不要用 `AllowGlobalUpdate` 关掉它**。

### 错误映射：GORM 错误不出 store 包

```go
// store 内部把 GORM 错误翻译成领域错误
errors.Is(err, gorm.ErrRecordNotFound) → model.ErrNotFound
// 唯一约束冲突 → model.ErrAlreadyExists
```

`app` 层用 `errors.Is(err, model.ErrNotFound)` 判断，**不许 import gorm**。
由 depguard 强制。

### 事务：边界在 app 层

```go
// app 层用例定义事务边界
func (uc *FreezeContract) Execute(ctx context.Context, …) error {
    return uc.tx.InTx(ctx, func(ctx context.Context) error {
        // 这里面的 repo 调用共用同一个事务
    })
}
```

- 事务通过 `context` 传递，repo 方法从 ctx 里取，**签名上看不到 `*gorm.DB`**
- **事务里禁止做 IO**：调 ACP、跑 git、读写文件、发 HTTP
- 事务要短。SQLite 同一时刻只允许一个写者，长事务会卡住其他 Work

### Logger

```go
gorm.Config{
    Logger: logger.New(slogAdapter, logger.Config{
        SlowThreshold:             200 * time.Millisecond,  // 慢查询告警
        IgnoreRecordNotFoundError: true,                    // 它是正常控制流，不是错误
        LogLevel:                  logger.Warn,             // 生产不打全部 SQL
    }),
}
```

开发时可用 `DUET_DB_TRACE=1` 环境变量把 LogLevel 提到 `Info` 看全部 SQL。

### `PrepareStmt` 与连接池

```go
gorm.Config{ PrepareStmt: true }        // 预编译缓存，SQLite 上收益明显

sqlDB.SetMaxOpenConns(1)                // ★ SQLite 只允许一个写者
sqlDB.SetMaxIdleConns(1)
sqlDB.SetConnMaxLifetime(0)
```

> `SetMaxOpenConns(1)` 是 SQLite 上的常见做法：把并发写序列化在应用层，
> 换取零 `SQLITE_BUSY`。配合 WAL，读不受影响。
> **改这个值前先跑并发测试**，别凭感觉调大。

---

## 10. 测试

| 规则 | |
|---|---|
| 用**临时文件** SQLite，**不用 `:memory:`** | `:memory:` 测不出 WAL 与并发行为，而产品会并发跑多个 Work |
| 每个测试独立的 `t.TempDir()` | 测试间不共享状态 |
| **禁止碰 `~/.acpflows/duet.db`** | `testutil` 的守卫会 `t.Fatal`（铁律 6） |
| 迁移测试 | 空库跑通 + 逐版本升级跑通 + checksum 变更被拒 |
| 外键测试 | 断言 `PRAGMA foreign_keys` 为 1，且删父行时子行约束真的生效 |
| 覆盖率门槛 | `store` ≥ 70% |

---

## 11. 禁止清单

- ✗ **领域模型挂 `gorm` 标签**，或 `domain` 包 import gorm
- ✗ GORM 实体泄漏到 `app` / `api` 层
- ✗ 用 `AutoMigrate` 代替版本化迁移
- ✗ 不显式写 `TableName()` 与 `column:`，依赖自动推导
- ✗ 不开 `foreign_keys` pragma
- ✗ `SELECT *`
- ✗ 裸 SQL 字符串拼接
- ✗ 用 `gorm.DeletedAt` 软删除（领域已有 `status` 语义）
- ✗ 事务里做 IO（ACP / git / 文件）
- ✗ 枚举存整数
- ✗ 自增整数主键（事件表的 `seq` 除外，它不是主键）
- ✗ 加索引说不出它服务哪个查询
- ✗ 测试用 `:memory:` 或碰用户真实数据库
