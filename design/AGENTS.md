# AGENTS.md · design

> **就近优先**：与根 [`AGENTS.md`](../AGENTS.md) 冲突时以本文件为准。

## 负责什么

目前只剩 [`icon/`](icon/)——应用图标的**构建源**（`duet.svg` / `duet-tray.svg`），
`scripts/gen/gen-icons.sh` 用它生成 Tauri 打包所需的全部图标产物，
`make check-icons` 守着产物与源同步。

## 不负责什么

- 旧设计稿（`*.dc.html`、`_ds/`、对照表）已于 2026-08-10 随重新设计整体移除。
  新设计真源建立后再回到这里。

## 本域特有的坑

- 改 `icon/duet.svg` 后必须跑 `./scripts/gen/gen-icons.sh`，否则 `check-icons` 红
