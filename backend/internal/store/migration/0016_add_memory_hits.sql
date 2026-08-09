-- 记忆的**命中计数**：这条记忆被注入过几次。
--
-- ★★ 计数由**应用自己数**，不问 AI（与 M8 的证据同理）。AI 自报的话，
-- 它会把「我读到了这条」说成「我用上了这条」——而用户看这个数字是为了
-- 判断「哪些记忆真的在起作用、哪些该清掉」。
--
-- ★ 默认 0 而不是 NULL：NULL 在界面上会显示成空白，
-- 而「从没被用过」和「不知道用过几次」对用户是两件事。
ALTER TABLE memories ADD COLUMN hit_count INTEGER NOT NULL DEFAULT 0;

-- 注入清单：哪一轮注入了哪些东西。
--
-- ★★ 清单由**应用记**，不解析 AI 的自由文本：它说「我参考了那条记忆」
-- 时可能根本没收到那条——而用户正是靠这份清单判断
-- 「它是不是带着我的规矩在干活」。
CREATE TABLE IF NOT EXISTS injections (
    id          TEXT PRIMARY KEY,
    work_id     TEXT NOT NULL,
    unit_id     TEXT NOT NULL DEFAULT '',
    role_id     TEXT NOT NULL DEFAULT '',
    -- memory_ids / skill_refs 存 JSON 数组；一轮可能注入多条
    memory_ids  TEXT NOT NULL DEFAULT '[]',
    skill_refs  TEXT NOT NULL DEFAULT '[]',
    created_at  DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_injections_work ON injections(work_id, created_at);
