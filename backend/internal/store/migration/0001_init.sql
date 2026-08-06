-- 0001 · 初始表结构
--
-- 规范见 docs/database.md：表名 snake_case 复数、列名 snake_case 单数、
-- 主键是带类型前缀的字符串、枚举存字符串原值、时间存 UTC。
--
-- 幂等：全部用 IF NOT EXISTS，重复执行不出错。

CREATE TABLE IF NOT EXISTS works (
    id         TEXT     NOT NULL PRIMARY KEY,
    project_id TEXT     NOT NULL DEFAULT '',
    state      TEXT     NOT NULL,
    branch     TEXT     NOT NULL DEFAULT '',
    worktree   TEXT     NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

-- 左栏按项目分组列工作，且按创建时间倒序
CREATE INDEX IF NOT EXISTS idx_works_project_id ON works (project_id);
-- 「进行中的工作」「等待你决策」这类筛选
CREATE INDEX IF NOT EXISTS idx_works_state      ON works (state);
CREATE INDEX IF NOT EXISTS idx_works_created_at ON works (created_at);
