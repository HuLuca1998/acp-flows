-- 0010 · 单元契约与写入边界（M7 U7.1.2）
--
-- ★★ **没有 UPDATE 路径**（INV-UC-2）：契约冻结后不能改，要改就出新版本。
-- 冻结的那一版被证据、验收、决策引用着——改它就是改历史。
--
-- ★ 边界与验收标准各自一张表：一条边界前缀里出现分隔符不是不可能，
--   而拆错的后果是「允许改 internal/」变成两条谁都不认识的规则。

CREATE TABLE IF NOT EXISTS contracts (
  unit_id    TEXT    NOT NULL,
  version    INTEGER NOT NULL,
  frozen     INTEGER NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  PRIMARY KEY (unit_id, version)
);

CREATE TABLE IF NOT EXISTS contract_criteria (
  unit_id TEXT    NOT NULL,
  version INTEGER NOT NULL,
  id      TEXT    NOT NULL,
  text    TEXT    NOT NULL DEFAULT '',
  ord     INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (unit_id, version, id)
);

CREATE TABLE IF NOT EXISTS contract_boundaries (
  unit_id TEXT    NOT NULL,
  version INTEGER NOT NULL,
  -- kind 取 allowed | forbidden。★ 分开存而不是一列加标记：
  --   查「这个单元能改什么」时不用先过滤。
  kind    TEXT    NOT NULL,
  prefix  TEXT    NOT NULL,
  ord     INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (unit_id, version, kind, prefix)
);
