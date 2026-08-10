-- 0007 · 需求快照的版本链（U5.2.1）
--
-- ★★ **版本链只增不改**（INV-REQ-2）：冻结后可改的话，
-- 计划、契约、单元全是照着某一版做的，而那一版已经不存在了——
-- 事后没人说得清「当时到底要做什么」。
--
-- 所以这张表**没有 UPDATE 路径**：改需求就插一条新版本。

CREATE TABLE IF NOT EXISTS requirements (
    work_id    TEXT     NOT NULL,
    version    INTEGER  NOT NULL,
    -- items 与 open_facts 用换行连接存。
    --
    -- ★ 不建关联表：它们是**整体读取、整体替换**的（一个版本一套），
    -- 没有任何按单条反查的需求。建表只会让「取一版需求」从一次查询变成三次。
    -- ★ 用换行而不是逗号：需求条目里出现逗号是很正常的事。
    items      TEXT     NOT NULL DEFAULT '',
    open_facts TEXT     NOT NULL DEFAULT '',
    frozen     INTEGER  NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,

    -- ★ 复合主键：同一个工作的同一版本只能有一条。
    -- 少了它的话，一次重试会插出两条 v2，而「当时是哪一版」又没了答案。
    PRIMARY KEY (work_id, version)
);

-- 取某个工作的最新一版是最常见的查法。
CREATE INDEX IF NOT EXISTS idx_requirements_work ON requirements (work_id, version DESC);
