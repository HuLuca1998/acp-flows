# AGENTS.md · backend/internal/release

> 本目录的规则。**就近优先**：与根 [`AGENTS.md`](/AGENTS.md) 冲突时以本文件为准。

## 负责什么

读发布源上的 `latest.json`（Tauri updater 的 manifest），告诉上层「远端最新是哪个版本」。

**查的 URL 必须与 `shell/src-tauri/tauri.conf.json` 的
`plugins.updater.endpoints` 完全一致。** 分成两个真源的话，
界面说「有更新」而 updater 说「没有」，用户会点一个永远不动的按钮。

## 不负责什么

- **不下载安装包。** 检查更新的全部网络开销就是拉一个几百字节的 manifest。
  下载与安装是 Tauri updater 的事 —— duetd 会在更新时**被替换掉**，
  让它管自己的替换过程是错的（[`adr/0002`](../../../docs/adr/0002-release-and-auto-update.md)）。
- **不比较版本。** 那是 `domain/model.Version` 的职责。本包只把字符串原样带上来。
- **不解析 `platforms` 段。** 下载地址与签名归 updater 管，
  解析它们没有意义，还会在格式变化时无谓地失败。

## 依赖方向

| | |
|---|---|
| 允许 import | 标准库 · `app/port`（实现 `port.ReleaseSource`） |
| 禁止 import | `api` · `store` · 其他基础设施包 |

由 `.golangci.yml` 的 `infra` 规则覆盖（`internal/release/**` 已登记）。

## 检查命令

```bash
cd backend && go test ./internal/release/... -count=1 -race
```

## 改这里之前必读

- [`docs/adr/0002-release-and-auto-update.md`](../../../docs/adr/0002-release-and-auto-update.md) —— 为什么 duetd 不碰安装包
- [`docs/adr/0007-release-revision-from-prior-art.md`](../../../docs/adr/0007-release-revision-from-prior-art.md) 修订 1、2 —— 版本号形态与双通道

## 本域特有的坑

- **404 是最常见的一种失败**：还没发过任何 release 时，
  `releases/latest/download/latest.json` 就是 404。
  **必须报错**——静默当成「已是最新」的话，第一个版本发出去之前
  没人会发现这条链路是断的。
- **响应体必须设上限。** `latest.json` 正常几百字节；
  一个被劫持或写错的端点返回几 GB，不设限就能把 duetd 拖死。
- **`pub_date` 解析失败不致命**：发布时间只是展示用，没有它照样能更新。
  但 `version` 缺失是致命的 —— 那样的 manifest 对更新检查毫无意义。
- **测试一律用 `httptest.Server`**，绝不碰真实网络：CI 上没网也要绿。
