# AGENTS.md · backend/internal/store/entity

GORM 行结构，`store` 包私有。仓库总纲见根 [`AGENTS.md`](../../../../AGENTS.md)。

---

## 这些不是领域模型

| | 放哪 | 长什么样 |
|---|---|---|
| 领域模型 | `internal/domain/model` | 充血、有方法与不变量，**零 gorm 标签** |
| 行结构 | 这里 | 纯数据、无方法（除 `TableName`） |
| 映射 | `internal/store/mapper` | 两者之间的唯一接触点 |

**给领域模型挂 gorm 标签是本项目最容易犯的错**，理由见
[`database.md`](../../../../docs/rules/database.md) §1。

## 两条硬规矩

**`TableName()` 必须显式写。** 不写就依赖 GORM 的自动推导，
而推导规则变化或类型改名时，表结构会**静默漂移**——不会报错，只会读不到数据。

**主键默认用带类型前缀的字符串**（`work-08`、`proj-01`），见
[`adr/0005`](../../../../docs/adr/0005-persistence.md)。
只有两处例外用自增：`logs` 与 `events`——它们高频写入，
且 `events.seq` 本身对外有意义（前端按它去重与续传）。

## 加字段时

行结构改了，**迁移也要改**（`../migration/`）。GORM 不会自动建列，
少写迁移的表现是「本地好好的，装了旧版本的机器上读不到这一列」。
