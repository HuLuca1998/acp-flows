<!--
条目不许删。填不出来说明流程没走完，回去补，别删条目。
规则见 docs/git-workflow.md §3。
-->

## 这个 PR 做了什么

<!-- 一到三句话。写「为什么」，不写「改了哪些文件」——那些看 diff。 -->

## 测试先行的证据（铁律 1）

**先红的测试：** `TestXxx`

<!-- 贴出你看见它失败时的输出，以及最终变绿的输出 -->

```
$ 
```

## 边界（铁律 4）

- 允许改动的范围：
- 实际改动是否超出：☐ 否 ☐ 是（说明原因）
- 触发过停止条件吗：☐ 没有 ☐ 有（说明如何处理的）

## 自查清单

- [ ] 有一个测试是先写的、先红过的
- [ ] 断言的是具体值，不是 `NotNil` / `NoError` 这类恒真式；对着「假测试图鉴」自查过
- [ ] 改了接口 → `api/openapi.yaml` 已同步且 `make gen` 跑过
- [ ] 改了 UI → 能指出对应 `design/Duet Spec.dc.html` 的哪一条；无硬编码 hex / 裸 px
- [ ] 新建关键目录 → 已补 `AGENTS.md` + `CLAUDE.md` 并填实
- [ ] 新增工具函数 → 已登记 `util/INDEX.md`；新增测试 → 已登记 `tests/INDEX.md`
- [ ] 查过索引，确认没有和已有工具/测试重复
- [ ] 没有读写 `~/.acpflows`、用户真实仓库或真实令牌
- [ ] 没有新增未经批准的第三方依赖
- [ ] `make check` 全绿

## 审查方（实现方不得审查自己的 PR）

- 实现：☐ Claude ☐ Codex ☐ 人
- 审查：☐ Claude ☐ Codex ☐ 人
- 审查结论：☐ `accepted` ☐ `implementation_fix` ☐ `contract_revision` ☐ `global_replan`
