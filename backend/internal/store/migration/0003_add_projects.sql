-- 0003 · 项目表（U2.1.1，验收点 V4）
--
-- 用户加进来的本地代码目录。**这张表里只有登记信息**——
-- Duet 不往用户的项目目录里写任何东西，所以这里也没有任何
-- 「Duet 在他项目里放了什么」的字段。
--
-- worktree 建在 `~/.acpflows/worktrees`（open-questions Q30），
-- 与项目目录无关，因此不在这张表里。

CREATE TABLE IF NOT EXISTS projects (
    id             TEXT     NOT NULL PRIMARY KEY,          -- proj-01，带类型前缀的字符串（adr/0005）
    name           TEXT     NOT NULL,                      -- 显示名，默认取目录名，用户可改
    path           TEXT     NOT NULL,                      -- 规整后的绝对路径
    is_git_repo    INTEGER  NOT NULL DEFAULT 0,
    default_branch TEXT     NOT NULL DEFAULT '',
    created_at     DATETIME NOT NULL,
    updated_at     DATETIME NOT NULL
);

-- ★ path 唯一。用户从 Finder 拖两次是很常见的，落成两条一模一样的记录会让他
-- 以为自己点错了，而删掉一条又不知道会不会连带删掉另一条的数据。
-- 应用层已经先查后写，这条约束是最后一道防线——并发添加时只有它拦得住。
CREATE UNIQUE INDEX IF NOT EXISTS idx_projects_path ON projects (path);
