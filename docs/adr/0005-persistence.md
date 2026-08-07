# ADR 0005 · 持久化：GORM + SQLite

- **日期**：2026-08-07
- **状态**：已接受
- **相关**：[`../database.md`](../rules/database.md)（设计规范）· `db-operate` skill（操作手册）

## 背景

Duet 需要存 Work / Plan / Unit / Contract / Attempt / Evidence / Decision / Checkpoint
这些结构化状态，以及记忆与 Skill 的**索引**（正文在 md 文件里）。

设计稿明确写了两件事：

> 数据目录 `~/.acpflows` · 「系统数据库记录 · **SQLite**」
> 「**md 是内容，DB 只存索引与状态**」

## 决策

### 1. ORM 用 GORM

理由：生态最大、AI 参考实现最多、迁移与查询的常见需求都覆盖。

代价是 GORM 有一批**著名的隐式行为**（`Updates` 忽略零值、`Find` 不返回
`ErrRecordNotFound`、`Save` 全字段覆盖、Hooks 藏业务逻辑）。
这些不是「注意一下」能解决的——[`../database.md`](../rules/database.md) §9 逐条写了规则，
**每条都要有测试守着**。

### 2. 只用 SQLite，不做 MySQL

一度考虑「SQLite 默认 + MySQL 可切」，**已放弃**。

Duet 是**本地优先的 macOS 桌面应用**。要求用户装 Docker + MySQL 才能用一个桌面 App
是不合理的；而且自动更新的「暂停工作 + 落检查点 + 重启恢复」会因此依赖外部服务可用性。

放弃 dialect 中立换来的好处很实在：可以放心用 SQLite 的特性（WAL、pragma、
`INSERT OR REPLACE`），不必为一个可能永远不会来的迁移付抽象税。

> 将来真要做服务端/团队版，那是**另一个部署形态**，届时重新做架构决策，
> 写新的 ADR。不要现在为它预留。

### 3. 驱动用 `github.com/glebarez/sqlite`（纯 Go）

| 候选 | 否决理由 |
|---|---|
| **`glebarez/sqlite`**（基于 `modernc.org/sqlite`）✅ | 纯 Go 无 CGO，交叉编译与 CI 简单 |
| `gorm.io/driver/sqlite`（基于 `mattn/go-sqlite3`） | 需要 CGO：交叉编译要装工具链，`CGO_ENABLED=0` 的静态二进制做不了 |

M1 要在 CI 上交叉编译 `duetd` 给两个架构再合 universal —— **CGO 会让这件事变得很痛**。

### 4. 迁移用版本化 SQL，不用 `AutoMigrate`

`AutoMigrate` 不删列、不改类型、不重命名，没有 down 路径，行为随 GORM 版本变化。

**用户机器上的数据库是用户的数据**，不能让一个隐式推导去改它。
版本化迁移 + checksum 校验 + 迁移前自动备份，见 [`../database.md`](../rules/database.md) §5。

### 5. GORM 实体与领域模型严格分离 ★

**这条是本 ADR 里最重要的。**

```
internal/domain/model/     领域模型。零 gorm 标签、零 gorm import
internal/store/entity/     GORM 实体。store 包私有
internal/store/mapper/     双向映射
```

AI 最自然的写法是给领域模型挂 `gorm:"primaryKey"` —— 那一瞬间 `domain` 就
import 了 gorm，铁律与 depguard 同时被破坏，持久化细节渗进业务规则。

由 `depguard` 强制：`domain` 不许 import 任何第三方库。

### 6. 主键是带类型前缀的字符串

`work-08` · `unit-012` · `mem-203` · `ck-07`，不用自增整数。

界面上大量展示这些 ID 且要求等宽显示；用整数会让前端到处拼字符串。
例外是事件表用 ULID（需要按时间可排序 + 高频写入），事件的 `seq` 是单调递增整数、
与主键分开——「取消后最后事件游标可读」依赖它。

## 后果

**得到**

- 单文件数据库，`.app` 可独立分发，零外部依赖
- 无 CGO，交叉编译与 CI 简单
- 领域层保持纯粹，可 100% 单测覆盖

**付出**

- GORM 的隐式行为需要一整套规则 + 测试来约束
- 放弃 dialect 中立，将来若要服务端版本需要重做数据层
- SQLite 单写者：写事务必须短，**事务里禁止做 IO**

**风险**

- `foreign_keys` pragma 默认关闭 —— 不显式打开等于外键白写。
  必须有断言它开着的测试
- 长事务会卡住其他并发的 Work。事务边界放在 `app` 用例里，且要短
