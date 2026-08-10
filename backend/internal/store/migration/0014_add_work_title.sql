-- 0014 · 工作的标题（左栏与面包屑要显示它）
--
-- ★★ 左栏显示 `work-01` 的话，用户看不出那条工作是干嘛的——
-- 而他可能同时开着五六条。设计稿那一行是
-- 「取消运行中的 Agent turn / executing 3/7」，标题是主体。
--
-- ★ 标题取自**用户提的那句需求**（截断），不是 AI 起的名字：
-- AI 起的名字与他说的话对不上时，他找不到自己那条工作。

ALTER TABLE works ADD COLUMN title TEXT NOT NULL DEFAULT '';
