-- 0009 · 计划版本、子计划、单元（M6 U6.1.2）
--
-- ★★ 三张表**都没有 UPDATE 路径**（INV-PLAN-4）：计划改了就出新版本，
-- 旧版本一个字不动。用户打开计划面板要能回答「上周那版拆成了什么、
-- 为什么改」——覆盖掉的话那个问题永远没有答案。
--
-- ★ 复合主键把「同一个工作的同一版」钉死，重复插入会撞主键而不是静默覆盖。

CREATE TABLE IF NOT EXISTS plans (
  work_id     TEXT    NOT NULL,
  version     INTEGER NOT NULL,
  title       TEXT    NOT NULL DEFAULT '',
  -- 重规划时每一项已验收工作的处置，`id=disposition` 用换行连接。
  -- ★ 用换行而不是逗号：标识里出现逗号不是不可能，而拆错的后果是
  --   一条处置凭空消失，且没有任何报错。
  dispositions TEXT   NOT NULL DEFAULT '',
  created_at  DATETIME NOT NULL,
  PRIMARY KEY (work_id, version)
);

CREATE TABLE IF NOT EXISTS subplans (
  work_id  TEXT    NOT NULL,
  version  INTEGER NOT NULL,
  id       TEXT    NOT NULL,
  title    TEXT    NOT NULL DEFAULT '',
  -- ord 保住**显示顺序**：按 id 排的话，subplan-10 会排在 subplan-02 前面。
  ord      INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (work_id, version, id)
);

CREATE TABLE IF NOT EXISTS units (
  work_id    TEXT    NOT NULL,
  version    INTEGER NOT NULL,
  subplan_id TEXT    NOT NULL,
  id         TEXT    NOT NULL,
  title      TEXT    NOT NULL DEFAULT '',
  -- ★★ role_id 是**必填**（裁定三）：不写的话到执行时才发现没人认领，
  --    而那时用户已经等了几分钟。
  role_id    TEXT    NOT NULL,
  -- 依赖的单元标识，换行连接。理由同 dispositions。
  depends_on TEXT    NOT NULL DEFAULT '',
  contract_frozen INTEGER NOT NULL DEFAULT 0,
  accepted        INTEGER NOT NULL DEFAULT 0,
  ord        INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (work_id, version, id)
);

CREATE INDEX IF NOT EXISTS idx_units_subplan ON units (work_id, version, subplan_id);
