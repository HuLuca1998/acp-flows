# AGENTS.md · backend/internal/app/system

> 本目录的规则。**就近优先**：与根 [`AGENTS.md`](/AGENTS.md) 冲突时以本文件为准。

## 负责什么

系统级用例：检查更新、判断现在更新会不会打断用户。

## 不负责什么

- **不下载、不安装、不重启。** 全归 Tauri updater（[`adr/0002`](../../../../docs/adr/0002-release-and-auto-update.md)）。
- **不做 HTTP。** 网络在 `internal/release`，本包只依赖 `port.ReleaseSource` 接口。
- **不碰持久化。** 只经 `port.WorkLister` 做只读查询。

## 依赖方向

| | |
|---|---|
| 允许 import | `domain` · `app/port` · `constant` · `util` |
| 禁止 import | `store` `acp` `api` `release` 的具体类型 |

由 `.golangci.yml` 的 `app` 规则强制。

## 检查命令

```bash
cd backend && go test ./internal/app/... -count=1 -race
```

## 改这里之前必读

- [`M1-install-and-update.md`](../../../../docs/plan/milestones/M1-install-and-update.md) S1.1（更新界面与流程）
- [`adr/0007`](../../../../docs/adr/0007-release-revision-from-prior-art.md) 修订 3 —— **进设置页时检查，不轮询**

## 本域特有的坑

这一层的失败模式**全是静默的**。判错了不报错，只是用户永远收不到更新，
或者更新时丢掉正在跑的工作。所以每条错误路径都要有断言。

- **`Prepare` 失败安全：查询失败按 `blocked` 处理，不是 `ready`。**
  查不到工作列表 = 不知道有没有工作在跑；不知道的时候重启用户的应用是最坏的选择。
  M1 里程碑写死了这条：「先放行、以后再补暂停逻辑」会真实地丢掉用户几十分钟的工作。
- **`Check` 出错绝不降级成「已是最新版本」。** 网络断了、URL 写错了都会走到这里，
  静默的话用户永远不知道自己在用半年前的版本 —— 这类故障没有任何症状。
- **远端更旧也要报 `idle`。** 回滚发布之后如果还提示更新，
  用户会被反复劝着装回旧版。
- **Web 形态直接返回 `unsupported`，且不查发布源。**
  浏览器里没有 updater，「提示了更新却点不动」会把用户卡在没有出路的界面上。
- **`blocked` 是业务结论不是错误**，HTTP 层要回 200：
  前端要拿着这个列表告诉用户「这几个工作还在跑」。
