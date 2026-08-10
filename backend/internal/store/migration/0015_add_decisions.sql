-- 0015 · 决策（M9 U9.1.2）
--
-- ★★ 答过的决策**一个字都不能改**：改了的话「他当时选了什么」就没有答案，
-- 而后面几十个文件的改动都是照着那个选择做的。
--
-- ★ `answered_with` 空表示**还没答**（「稍后决定」）——左栏那个亮点靠它。

CREATE TABLE IF NOT EXISTS decisions (
  id       TEXT    NOT NULL PRIMARY KEY,
  work_id  TEXT    NOT NULL,
  unit_id  TEXT    NOT NULL DEFAULT '',
  level    TEXT    NOT NULL,
  question TEXT    NOT NULL,
  -- recommended 空表示 AI 也拿不准。
  recommended   TEXT NOT NULL DEFAULT '',
  answered_with TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_decisions_work ON decisions (work_id, answered_with);

CREATE TABLE IF NOT EXISTS decision_options (
  decision_id TEXT NOT NULL,
  id          TEXT NOT NULL,
  text        TEXT NOT NULL DEFAULT '',
  -- ★★ impact 是「选了它会怎样」。没有它用户在盲选。
  impact TEXT    NOT NULL DEFAULT '',
  ord    INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (decision_id, id)
);
