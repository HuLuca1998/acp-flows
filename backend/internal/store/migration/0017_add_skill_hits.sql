-- Skill 的**命中计数**。
--
-- ★★ 单独一张表，而不是给 skills 加列——**没有 skills 表**。
-- Skill 是扫盘产物（`~/.acpflows/skills` 下的目录），用户可以随时增删改，
-- 我们不留它的副本（与记忆正文同理：那是他的东西）。
-- 这里只存我们自己观察到的一件事：它被注入过几次。
--
-- ★ ref 用 `<scope>:<dir>` 而不是 name：name 来自 frontmatter，
-- 用户改一次名计数就断了，而目录才是那个 Skill 的身份。
CREATE TABLE IF NOT EXISTS skill_hits (
    ref        TEXT PRIMARY KEY,
    hit_count  INTEGER NOT NULL DEFAULT 0,
    updated_at DATETIME NOT NULL
);
