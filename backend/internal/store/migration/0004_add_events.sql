-- 0004 · 事件表（U2.3.1，验收点 V6）
--
-- 时间线上的每一条都在这里。**先落库再扇出**——前端收到而库里没有的话，
-- 用户会看到一条重启后就消失的记录，那比丢事件更糟。
--
-- ★ seq 用自增主键，序号因此由数据库发放而不是内存计数器。
-- 内存计数器一重启就归零，而前端按 seq 去重——归零之后新事件会被当成旧的丢掉。
-- 这是全库第二处用自增的地方（另一处是 logs），理由相同：高频写入且序号本身有意义。

CREATE TABLE IF NOT EXISTS events (
    seq      INTEGER  NOT NULL PRIMARY KEY AUTOINCREMENT,
    id       TEXT     NOT NULL,                    -- evt_<ulid>，对外暴露的标识
    work_id  TEXT     NOT NULL DEFAULT '',
    source   TEXT     NOT NULL,                    -- acp | app
    type     TEXT     NOT NULL,                    -- 13 类之一，见 api/openapi.yaml 的 Event
    ts       DATETIME NOT NULL,
    payload  TEXT     NOT NULL DEFAULT '{}'        -- JSON
);

-- 「这个工作的时间线，按顺序」—— 界面打开一个工作时的唯一查询
CREATE INDEX IF NOT EXISTS idx_events_work_id_seq ON events (work_id, seq);

-- 断线重连补发：「seq 大于 N 的都给我」。没有这条索引时它是全表扫，
-- 而重连恰恰发生在事件最多的时候。
CREATE INDEX IF NOT EXISTS idx_events_seq ON events (seq);
