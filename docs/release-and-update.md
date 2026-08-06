# 发布与自动更新

> **M1 优先级最高的功能。** 设计稿里已经画完（设置 → 应用更新），照着实现即可。
> 关键约束来自设计稿的一句话：**「更新前会暂停所有工作并保存检查点，重启后从检查点恢复。」**
> 这句话意味着更新不是"下载装包重启"，而是一次**跨前后端的领域操作**。

---

## 1. 全链路

```
 开发                    发布                          客户端
┌──────────┐   ┌────────────────────────────┐   ┌──────────────────────┐
│ PR 合入   │──▶│ release-please 累积变更     │   │ 定期检查 latest.json  │
│ main     │   │ 开 Release PR + CHANGELOG   │   │        │              │
└──────────┘   └─────────────┬──────────────┘   │        ▼              │
                             │ 人类合并 ← D3     │  发现新版本 → 提示     │
                             ▼                  │        │              │
               ┌────────────────────────────┐   │        ▼              │
               │ 打 tag vX.Y.Z              │   │  ① 暂停所有工作        │
               │ 构建 macOS arm64 + x64     │   │  ② 每个工作落检查点     │
               │ 合成 universal             │   │  ③ 下载 + 校验签名     │
               │ minisign 签名              │   │  ④ 安装 + 重启        │
               │ 生成 latest.json           │   │  ⑤ 从检查点恢复        │
               │ 发布 GitHub Release        │──▶│                      │
               └────────────────────────────┘   └──────────────────────┘
```

**两条独立的更新线，不要混：**

| | 更新对象 | 触发 | 需要重启 App |
|---|---|---|---|
| **应用更新** | `Duet.app` 自身 | Tauri updater ← GitHub Release | 是 |
| **Runtime 更新** | `claude-agent-acp` / `codex-acp` | duetd ← npm registry | 否，但需该 Runtime 无活跃会话 |

---

## 2. CI：每个 PR 跑什么

`.github/workflows/ci.yml`，四个必过检查（对应分支保护里的 required checks）：

| job | 内容 |
|---|---|
| `backend` | `go vet` · `golangci-lint` · `go test ./... -race` · 覆盖率门槛 |
| `frontend` | `tsc --noEmit` · ESLint · Stylelint（设计合规）· `vitest --run` |
| `contract` | `make check-gen`（生成物与 `api/openapi.yaml` 一致）· `redocly lint` |
| `docs` | `make check-docs`（关键目录都有填实的 AGENTS.md + CLAUDE.md）· 提交信息格式 |

E2E（Playwright）在 `main` 上跑，不阻塞 PR —— 它慢，且依赖构建产物。

---

## 3. 版本推导：release-please

`.github/workflows/release-please.yml` 监听 `main`。

- 读 `main` 上自上次 release 以来的 conventional commits
- 按 [`git-workflow.md`](git-workflow.md) §2 的表推导版本号
- 自动维护 `CHANGELOG.md` 与 `shell/src-tauri/tauri.conf.json` 里的 `version`
- 开一个 `chore(release): vX.Y.Z` 的 PR

**这个 Release PR 由人类合并。** 合并即打 tag，触发 `release.yml`。
理由：发版对外不可撤回，属于 D3。这是整条流水线上唯一需要人点的地方。

> 版本号是 **`tauri.conf.json` 说了算**，`package.json` / `go.mod` 不参与。
> 任何人不得手改版本号。

---

## 4. 构建与签名

`.github/workflows/release.yml`，`on: push: tags: ['v*']`。

```
macos-14 (arm64) ─┐
                  ├─▶ lipo 合成 universal ─▶ 打包 .app / .dmg ─▶ minisign 签名
macos-13 (x64)  ──┘
```

### 构建顺序

1. `make build` —— 编 `duetd`（两个架构）+ 前端 `dist`
2. `duetd` 二进制作为 Tauri **sidecar** 放进 `shell/src-tauri/binaries/`
   （文件名必须带 target triple 后缀，Tauri 的约定）
3. `pnpm tauri build` —— 前端 dist 内嵌，产出 `.app` 与 `.dmg`
4. Tauri 用 `TAURI_SIGNING_PRIVATE_KEY` 做 **minisign** 签名，产出 `.sig`
5. 生成 `latest.json`，连同安装包一起上传到 GitHub Release

### 两种签名，别搞混

| 签名 | 作用 | 现在的策略 |
|---|---|---|
| **minisign**（Tauri updater） | 保证更新包没被篡改，客户端强制校验 | **立刻做，必须做**。私钥在 GitHub Secrets，公钥硬编码进 `tauri.conf.json` |
| **Apple 代码签名 + 公证** | 让 Gatekeeper 放行 | **暂不做**（ad-hoc 签名）。首次安装需用户手动放行 |

Apple 公证的开关在 workflow 里已预留（`APPLE_*` secrets 存在时才执行），
将来买了开发者账号只需填 secrets，不改流水线。见 [`adr/0002`](adr/0002-release-and-auto-update.md)。

### Secrets 清单

| Secret | 用途 | 现在需要 |
|---|---|---|
| `TAURI_SIGNING_PRIVATE_KEY` | minisign 私钥 | ✅ |
| `TAURI_SIGNING_PRIVATE_KEY_PASSWORD` | 私钥口令 | ✅ |
| `APPLE_CERTIFICATE` / `APPLE_CERTIFICATE_PASSWORD` | 代码签名证书 | ⬜ 待定 |
| `APPLE_ID` / `APPLE_PASSWORD` / `APPLE_TEAM_ID` | 公证 | ⬜ 待定 |

> **minisign 私钥丢失 = 所有已安装的客户端再也收不到更新**（公钥硬编码在旧版本里）。
> 生成后立刻离线备份，不要只存在 GitHub Secrets 里。

### latest.json

Tauri updater 的清单，托管在 GitHub Release 的 asset 上：

```json
{
  "version": "1.5.0",
  "notes": "修复主管会话 id 丢失；codex 权限档默认收权；worktree 恢复更稳。",
  "pub_date": "2026-08-07T10:00:00Z",
  "platforms": {
    "darwin-aarch64": { "signature": "<minisign>", "url": "https://github.com/.../Duet_1.5.0_universal.app.tar.gz" },
    "darwin-x86_64":  { "signature": "<minisign>", "url": "https://github.com/.../Duet_1.5.0_universal.app.tar.gz" }
  }
}
```

`notes` 直接取 CHANGELOG 里该版本的条目 —— 设计稿的更新卡片会原样显示它。

---

## 5. 客户端更新流程

### 状态机

```
idle ──检查──▶ checking ──有新版──▶ available
                   │                    │ 用户点「一键更新并重启」
                   │无新版               ▼
                   └────────▶ idle   preparing   ← ★ 后端在这里暂停工作、落检查点
                                          │
                                    ┌─────┴─────┐
                                 失败│           │成功
                                    ▼           ▼
                                 blocked    downloading ──▶ installing ──▶ restarting
                                                                              │
                                                                    重启后 ──▶ resuming
```

### `preparing` 是重点，不是形式

用户点「一键更新并重启」后，**先调后端，不是先下载**：

```
POST /v1/system/update/prepare
```

后端对每个非终态的 Work 做：

| Work 当前状态 | 处理 |
|---|---|
| `executing` | ① 发 ACP `session/cancel`（两段式：发请求 → 等 `stopReason` 落盘）<br>② 采集当前证据与最后事件游标<br>③ 落 checkpoint<br>④ Work → `paused` |
| `reviewing_unit` | 同上，但当前 attempt 标记 `superseded` |
| `waiting_user` | 直接落 checkpoint，保留待决策项 |
| `paused` `completed` `failed` | 无需处理 |

响应：

```json
{
  "status": "ready" | "blocked",
  "prepared": [{"work_id": "work-08", "checkpoint_id": "ck-09"}],
  "blocked":  [{"work_id": "work-11", "reason": "cancel 超时，Runtime 无响应"}]
}
```

**`blocked` 非空时前端不得继续安装**，要把哪个工作卡住了显示给用户，
让用户选择「强制更新（丢弃 work-11 的 2:14 工作）」还是「稍后」。
破坏性按钮上要写清后果——这是设计规范第 09 节的要求。

### 重启后恢复

App 启动时：

```
GET /v1/system/resume
→ { "resumable": [{"work_id": "work-08", "checkpoint_id": "ck-09", "unit_id": "unit-012"}] }
```

设置里的开关「启动时恢复 · 从检查点恢复未完成的工作」控制是自动恢复还是只提示。

### 「自动检查更新（不自动安装）」

设计稿写死了这个语义，**不要做成自动安装**：

- 后台每 6 小时检查一次 + 启动时检查一次
- 发现新版本 → 设置页出角标 + 一条 `app` 事件
- **绝不自动下载、绝不自动安装、绝不自动重启**
- 用户关掉这个开关后完全静默

理由：这个 App 在跑长任务写用户的代码。任何未经确认的重启都可能丢失几十分钟的工作。

---

## 6. Runtime 更新（第二条线）

设计稿的「Runtime 更新」区：`claude-agent-acp 0.63.0 → 0.64.1 [更新]`。

- duetd 查 npm registry 拿最新版本
- 安装到 `~/.acpflows/runtimes/<name>/<version>/`（**多版本并存**，不覆盖）
- 装完跑**能力探针**（12 项），全过才允许切换为默认版本
- 探针不过 → 保留旧版本，把探针失败项显示给用户
- 有活跃会话在用旧版本时，等会话结束再切；不打断执行中的单元

多版本并存让「切换版本」按钮可用，也让升级出问题时能一键退回。

---

## 7. 回滚

| 场景 | 手段 |
|---|---|
| 新版本有严重问题 | 删除该 Release 的 `latest.json`，或发布一个版本号更高的修复版（**推荐**） |
| 客户端装坏了 | GitHub Release 里保留全部历史安装包，用户手动装回旧版 |
| Runtime 升级出问题 | 设置页「切换版本」退回，旧版本还在磁盘上 |

**不要把已发布的 tag 删掉重打。** 已经更新的客户端无法回退，只会制造版本混乱。

---

## 8. 相关 API（契约以 `api/openapi.yaml` 为准）

| 端点 | 用途 |
|---|---|
| `GET /v1/system/version` | 当前版本、构建信息、平台 |
| `GET /v1/system/update/check` | 主动检查（对应「检查更新」按钮） |
| `POST /v1/system/update/prepare` | ★ 暂停所有工作 + 落检查点 |
| `GET /v1/system/resume` | 启动时列出可恢复的工作 |
| `POST /v1/system/resume/{workId}` | 从检查点恢复某个工作 |
| `GET /v1/runtimes` | Runtime 列表、版本、探针结果 |
| `POST /v1/runtimes/{name}/install` | 安装/升级某个 Runtime |
| `POST /v1/runtimes/{name}/probe` | 重新探测 |
| `POST /v1/runtimes/{name}/activate` | 切换默认版本 |

下载与安装动作本身由 Tauri updater 完成，**duetd 不下载 App 安装包**。
duetd 只负责"能不能更新"和"更新前后的状态"。

---

## 9. 测试怎么做

按铁律 1，这些都要先写测试：

| 测什么 | 怎么测 |
|---|---|
| `prepare` 对各种 Work 状态的处理 | domain 层表驱动单测，覆盖 6 种状态 |
| Runtime 无响应时 `prepare` 返回 `blocked` | Fake ACP Runtime 配置成不回 `stopReason` |
| 取消的幂等性（连续 prepare 两次） | 集成测试，断言只发一次协议 cancel |
| 检查点落盘后可恢复 | 临时 SQLite + 临时 git 仓库，落点后重启 duetd 再读 |
| `latest.json` 格式 | golden 文件对比 |
| 前端更新状态机 | Vitest + MSW mock 各种响应 |
| 全链路 | Playwright：mock updater endpoint，走完 available → prepare → 恢复 |

**不要用真实的 GitHub Release 做测试。** mock 掉 updater endpoint。

---

## 10. 开放项

1. Apple 开发者证书与公证要不要做（$99/年）——见 `adr/0002`
2. 要不要做 beta 通道（`latest-beta.json`）——等有外部用户再说
3. 增量更新（差分包）——Tauri 暂不支持，先不考虑
