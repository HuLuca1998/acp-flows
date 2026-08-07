-- 0002 · 日志表
--
-- 日志是 AI 调试时的唯一观测面（docs/logging.md）。落库的意义在于**可查询**——
-- tail -f 只能看时间序，AI 真正需要的是「这个 Work 的所有 ERROR」
-- 「这次 attempt 里 acp 说了什么」，那是查询不是滚动。

CREATE TABLE IF NOT EXISTS logs (
    seq        INTEGER  NOT NULL PRIMARY KEY AUTOINCREMENT,  -- 唯一用自增的地方：日志高频写且不对外暴露
    ts         DATETIME NOT NULL,
    level      INTEGER  NOT NULL,        -- slog 的数值：trace -8 / debug -4 / info 0 / warn 4 / error 8
    component  TEXT     NOT NULL DEFAULT '',
    msg        TEXT     NOT NULL,
    attrs      TEXT     NOT NULL DEFAULT '{}',  -- JSON
    work_id    TEXT     NOT NULL DEFAULT '',
    unit_id    TEXT     NOT NULL DEFAULT '',
    attempt_id TEXT     NOT NULL DEFAULT '',
    trace_id   TEXT     NOT NULL DEFAULT ''
);

-- 「这个 Work 的日志，按时间倒序」—— 最高频的排查查询
CREATE INDEX IF NOT EXISTS idx_logs_work_id_seq ON logs (work_id, seq);
-- 「只看错误」
CREATE INDEX IF NOT EXISTS idx_logs_level_seq   ON logs (level, seq);
-- 「一次请求 / 一轮 turn 的全过程」
CREATE INDEX IF NOT EXISTS idx_logs_trace_id    ON logs (trace_id);
-- 保留策略按时间删
CREATE INDEX IF NOT EXISTS idx_logs_ts          ON logs (ts);
