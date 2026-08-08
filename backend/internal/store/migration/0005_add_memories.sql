-- 0005 · 记忆索引表（U2.3.1）
--
-- ★★ **这张表里没有正文**（INV-MEM-8）。
--
-- 正文只存在于 md 文件里（`<project>/.acpflows/memory/<id>.md` 或
-- `~/.acpflows/memory/<id>.md`）——人可读可编辑、可入 git。
-- DB 只存索引与状态，用来筛选、统计、判定注入资格。
--
-- 两边各存一份的话它们迟早对不上，而到时候「哪一份是真的」没有答案。
-- 有反射测试守着领域模型不含正文字段。
--
-- ★ **没有 DELETE 路径**（INV-MEM-6）：失效 ≠ 删除。
-- 删掉的话，半年前那次运行「当时用的是哪条记忆」就永远查不到了。

CREATE TABLE IF NOT EXISTS memories (
    id           TEXT     NOT NULL PRIMARY KEY,   -- mem-203，带类型前缀（adr/0005）
    kind         TEXT     NOT NULL,               -- constraint / experience / fact
    scope        TEXT     NOT NULL,               -- 项目名（L2）或 '*'（L3 跨项目）
    status       TEXT     NOT NULL,               -- candidate / active / discarded / invalid / obsolete
    source_refs  TEXT     NOT NULL DEFAULT '',    -- 依据，'ev-412,unit-009' 这样存
    created_by   TEXT     NOT NULL DEFAULT '',    -- 产出者角色，如 memory_curator
    confirmed_by TEXT     NOT NULL DEFAULT '',    -- ★ 是谁确认的。空 = 还没人拍板
    reason       TEXT     NOT NULL DEFAULT '',    -- 废弃理由（obsolete 才有）
    supersedes   TEXT     NOT NULL DEFAULT '',    -- 被本条取代的记忆
    history_len  INTEGER  NOT NULL DEFAULT 1,     -- 变更历史条数，只增不减
    created_at   DATETIME NOT NULL,
    updated_at   DATETIME NOT NULL
);

-- 按 scope + status 筛是这张表最常见的查法（记忆页的两个筛选器就是它们）。
CREATE INDEX IF NOT EXISTS idx_memories_scope_status ON memories (scope, status);
