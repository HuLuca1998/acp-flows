-- 0006 · 工作的基线 commit（U4.2.2）
--
-- ★★ 右栏的「领先几个 commit」与验收时的 diff 都拿它当起点。
-- 不记的话，「AI 到底干了什么」只能靠猜——而猜出来的答案
-- 会随着仓库变化而漂移。
--
-- ★ `branch` 与 `worktree` 两列 0001 里已经有了，这里只补基线。
-- 允许为空：工作在 initializing 阶段还没切 worktree。

ALTER TABLE works ADD COLUMN base_commit TEXT NOT NULL DEFAULT '';
