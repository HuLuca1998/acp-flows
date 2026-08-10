-- 0012 · 验收证据（M8 U8.1.2）
--
-- ★★ 这张表**没有 UPDATE 路径**：证据被改写过就不再是证据了——
-- 「当时到底跑出了什么」没有第二个地方可查。
--
-- ★ `source` 分 app / agent 两种，**必填**：分不出来源的话，
-- 一条 AI 转述会和一份应用采集的 diff 长得一样，
-- 而用户判断「该不该信」全靠这一个字段。

CREATE TABLE IF NOT EXISTS evidence (
  id      TEXT    NOT NULL PRIMARY KEY,
  work_id TEXT    NOT NULL,
  unit_id TEXT    NOT NULL,
  kind    TEXT    NOT NULL,
  source  TEXT    NOT NULL,
  summary TEXT    NOT NULL DEFAULT '',
  -- body 是**原始输出**，原样存不截断——截断过的输出在排查时等于没有。
  body       TEXT     NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_evidence_unit ON evidence (work_id, unit_id);

-- 证据与验收标准是**多对多**：一次测试可能跑通三条标准，
-- 一条标准也可能要几条证据。
CREATE TABLE IF NOT EXISTS evidence_criteria (
  evidence_id TEXT NOT NULL,
  criterion_id TEXT NOT NULL,
  ord          INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (evidence_id, criterion_id)
);
