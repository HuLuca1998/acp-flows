-- 0008 · 事件上「这一条是谁说的」与「当时需求是第几版」（M5 U5.3.1 / U5.2.1）
--
-- ★★ 这五列**只有真机验证才会暴露**：内存总线直推订阅者时角色是在的，
-- 而前端首次连接**总是**带 `Last-Event-ID: 0` 把历史要回来（不带的话
-- 用户重开应用后时间线是空的）——那条路径经过这张表。
--
-- 也就是说：不加这几列的话，用户看到的**第一屏永远没有角色标签**，
-- 而他正是靠这个标签判断「现在是谁在说话、他能不能动我的文件」。
--
-- ★ 全部允许为空：应用自己发的事件（state_change、checkpoint）没有角色，
-- 而 `requirement_version = 0` 表示「那时还没有需求快照」。
--   **空就是空**，别在读的时候填一个默认值上去。

ALTER TABLE events ADD COLUMN role TEXT NOT NULL DEFAULT '';
ALTER TABLE events ADD COLUMN role_display_name TEXT NOT NULL DEFAULT '';
ALTER TABLE events ADD COLUMN runtime TEXT NOT NULL DEFAULT '';
ALTER TABLE events ADD COLUMN requirement_version INTEGER NOT NULL DEFAULT 0;
ALTER TABLE events ADD COLUMN requirement_frozen INTEGER NOT NULL DEFAULT 0;
