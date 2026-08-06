---
name: db-operate
description: 连接、查询、调试、修改 Duet 的 SQLite 数据库时使用。触发场景：想看某张表里有什么、排查「数据对不上」、验证迁移是否生效、检查索引有没有被用到、确认外键约束是否打开、需要手动改一条测试数据、或用户说「查一下数据库」「看看这条记录」。**只读操作可直接做；写操作有硬性前置条件，见本文。** 不负责数据库设计（那是 docs/database.md）。
---

# 数据库操作

> 设计规范在 [`docs/database.md`](../../docs/database.md)（表怎么建、模型怎么写）。
> **本文只管「怎么连、怎么查、怎么调、怎么改」。**

## 三条红线

1. **绝不在测试里碰 `~/.acpflows/duet.db`** —— 铁律 6。`testutil` 的守卫会 `t.Fatal`
2. **改用户真实数据库前必须备份**，且要让用户知道你在改什么
3. **不要用 `sqlite3` CLI 去改结构** —— 结构变更一律走版本化迁移，否则下次启动 checksum 校验会炸

---

## 连接

| 环境 | 路径 |
|---|---|
| 用户真实数据 | `~/.acpflows/duet.db` |
| `make dev-web` 开发态 | `~/.duet-dev/duet.db`（由 `DUET_DATA_DIR` 指定，**不碰真实数据**） |
| 测试 | `t.TempDir()` 下的临时文件，测试结束自动清理 |

```bash
sqlite3 ~/.duet-dev/duet.db          # 开发库
sqlite3 -readonly ~/.acpflows/duet.db  # ★ 看真实库一律加 -readonly
```

**看真实库一律 `-readonly`。** 忘了加而手滑执行了 `UPDATE`，用户的工作记录就没了。

### 好用的 CLI 设置

```bash
sqlite3 ~/.duet-dev/duet.db <<'SQL'
.headers on
.mode box
.timer on
SQL
```

或写进 `~/.sqliterc` 一次性搞定。

---

## 先跑这四条自检

排查任何「数据对不上」之前，**先确认环境本身是对的**：

```sql
PRAGMA foreign_keys;      -- 必须是 1。★ SQLite 默认是 0，外键写了不生效
PRAGMA journal_mode;      -- 必须是 wal
PRAGMA integrity_check;   -- 必须是 ok
SELECT * FROM schema_migrations ORDER BY version;   -- 迁移到哪一版了
```

> `foreign_keys` 返回 0 是**最常见的假象来源**：外键约束写了，
> 删父行子行还在，然后开始怀疑业务逻辑。先查这个。
>
> ⚠️ **`foreign_keys` 是「每连接」的，`journal_mode` 是「每库」的** —— 实测确认过：
>
> | pragma | 在 sqlite3 CLI 里查 | 说明 |
> |---|---|---|
> | `foreign_keys` | **总是 0** | CLI 自己的连接没带 DSN 参数。**这不代表应用侧是 0** |
> | `journal_mode` | `wal` | 库属性，`duetd` 建库时写进文件了，谁连都一样 |
>
> 所以：**在 CLI 里看到 `foreign_keys = 0` 是正常的，不要据此判断应用有问题。**
> 要验证应用侧，跑 `TestOpen_R1_ForeignKeysPragmaIsOn`。
>
> 反过来，在 CLI 里手动删数据时**外键约束不生效** —— 想让它生效要先
> `PRAGMA foreign_keys = ON;`，否则可能删出孤儿行。

---

## 常用查询

```sql
-- 表清单与建表语句（看真实索引与约束，比读代码可靠）
.tables
.schema works

-- 某个工作的全貌
SELECT id, state, branch, created_at FROM works WHERE id = 'work-08';

-- 事件流游标（排查「取消后游标可读」时用）
SELECT MAX(seq) FROM events WHERE work_id = 'work-08';

-- 计划版本链（append-only，版本号必须连续）
SELECT version, created_at FROM plan_versions WHERE work_id='work-08' ORDER BY version;

-- 契约冻结状态
SELECT unit_id, contract_version, frozen_at FROM unit_contracts WHERE frozen_at IS NOT NULL;
```

**不要 `SELECT *` 之后靠眼睛找列。** 列多了看不清，显式写列名。

---

## 调试

### 查询没走索引？

```sql
EXPLAIN QUERY PLAN
SELECT id, state FROM works WHERE project_id = 'acp-engine' ORDER BY created_at DESC;
```

看输出里有没有 `USING INDEX idx_...`。出现 **`SCAN works`** 就是全表扫。

处理顺序：
1. 先确认这个查询**真的需要**索引（数据量小的表全表扫更快）
2. 需要 → 加**版本化迁移**建索引，不要用 CLI 直接 `CREATE INDEX`
3. 加完在 `docs/database.md` §4 的意义上说明它服务哪个查询

### 看应用发出的真实 SQL

```bash
DUET_DB_TRACE=1 make dev-web
```

把 GORM 的 LogLevel 提到 `Info`。**排查 N+1 时最有用**——
循环里冒出一串几乎相同的 `SELECT` 就是它。

### 慢查询

GORM 的 `SlowThreshold` 是 200ms，超了会 warn。SQLite 上超过 200ms 基本都是缺索引或全表扫。

### 数据库被锁住（`SQLITE_BUSY` / `database is locked`）

```bash
lsof ~/.duet-dev/duet.db            # 谁占着
ls -la ~/.duet-dev/duet.db*         # 有 -wal / -shm 说明有活跃连接
```

常见原因，**按可能性排序**：

1. **事务里做了 IO**（调 ACP、跑 git、读文件）—— 这是设计错误，见 `database.md` §9
2. `sqlite3` CLI 开着一个未提交的事务 —— 退出 CLI
3. `duetd` 还在跑 —— 先停掉

---

## 改数据

### 只读排查优先

**大多数「需要改数据」的场景其实是「需要看懂数据」。** 先把只读手段用尽。

### 真要改：三个前置条件

改**开发库**（`~/.duet-dev/`）：随便改，它就是用来糟蹋的。

改**用户真实库**（`~/.acpflows/duet.db`）必须三条全满足：

1. **先备份**：`cp ~/.acpflows/duet.db ~/.acpflows/duet.db.bak-$(date +%s)`
2. **告诉用户你要改什么、为什么**，等确认
3. **在事务里改，改完先 `SELECT` 验证再 `COMMIT`**

```sql
BEGIN;
UPDATE works SET state = 'paused' WHERE id = 'work-08';
SELECT id, state FROM works WHERE id = 'work-08';   -- 先看对不对
-- 对了才 COMMIT；不对就 ROLLBACK;
COMMIT;
```

### 绝不用 CLI 做的事

| ✗ | 正确做法 |
|---|---|
| `CREATE TABLE` / `ALTER TABLE` / `CREATE INDEX` | 写版本化迁移，见 `database.md` §5 |
| `DELETE FROM <表>` 无 WHERE | 想清楚再说 |
| 改 `schema_migrations` | checksum 校验会炸，且掩盖真实问题 |
| 直接改状态绕过状态机 | 状态机存在就是为了挡非法迁移；绕过它等于制造脏数据 |

> 最后一条最重要：**把 `state` 直接 UPDATE 成 `completed`，
> 就绕过了「必须经过 reviewing_unit」这条不变量。**
> 数据库里看着对了，但业务上它是脏的。走 API 或写测试，不要手改状态。

---

## 迁移相关

```bash
# 迁移状态
sqlite3 ~/.duet-dev/duet.db "SELECT version, applied_at FROM schema_migrations ORDER BY version;"

# 从零重建开发库（开发期最省事的"回滚"）
rm -f ~/.duet-dev/duet.db* && make dev-web
```

**开发期不要写 down 迁移**，删库重建更快也更可靠。
用户数据的回退靠迁移前的自动备份。

写新迁移的检查清单：

- [ ] 文件名 `NNNN_<动词>_<对象>.sql`，编号只增不复用
- [ ] 幂等（`IF NOT EXISTS` / 先查 `pragma_table_info`）
- [ ] 有测试：空库跑通 + 从上一版本升级跑通
- [ ] **改过的已应用迁移会被 checksum 拦下** —— 这是特性不是 bug，要加新迁移

---

## 排查套路

遇到「数据对不上」，按这个顺序，**不要跳步**：

1. `PRAGMA foreign_keys` 是 1 吗？→ 不是的话很多"怪现象"都有解释了
2. `schema_migrations` 到哪一版？→ 与代码期望的版本一致吗
3. `DUET_DB_TRACE=1` 看应用**真实发出**的 SQL → 与你以为的一样吗
4. 在 `sqlite3` 里**手跑那条 SQL** → 结果一样吗？不一样说明是代码问题不是数据问题
5. `EXPLAIN QUERY PLAN` → 是不是全表扫导致的超时
6. **写一个能复现它的测试**（用临时库）→ 修完这个测试就是回归防线

**第 6 步不能省。** 手动查明白了但没留下测试，下一轮 AI 会再踩一次
（见 `docs/tech-debt.md`）。
