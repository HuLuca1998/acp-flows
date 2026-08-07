# 发布与自动更新

> **M1 优先级最高的功能。** 设计稿里已经画完（设置 → 应用更新），照着实现即可。
>
> ⚠️ **本文的触发方式、版本号来源、检查时机三项已被
> [`adr/0007`](../adr/0007-release-revision-from-prior-art.md) 修订** ——
> 按前一个项目 `ai-workflows` 的实证改过。下面的内容已同步。
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

`.github/workflows/ci.yml`。**分支保护只 required `ci` 这一个汇总门禁**——
列单个 job 会因为路径过滤跳过而让 PR 永远 pending，见 [`ci.md`](../rules/ci.md) 规则 2。

| job | 内容 | 什么时候跑 |
|---|---|---|
| `changes` | 变更探测 + 脚手架探测 | 永远 |
| `guard` | 文档完整性 · 索引一致性 · 命名规范 · 提交信息格式 | 永远 |
| `contract` | `redocly lint` · `make check-gen`（生成物与 spec 一致） | `api/**` 变动 |
| `backend` | `golangci-lint` · `go test -race` · 集成测试 · 覆盖率门槛 | `backend/**` 或契约变动 |
| `frontend` | `tsc --noEmit` · ESLint · Stylelint（设计合规）· `vitest --run` | `frontend/**` 或契约变动 |
| `shell` | `cargo clippy -D warnings` · `cargo test` | `shell/**` 变动 |
| `ci` | ★ 汇总门禁，跳过算通过、失败不放过 | 永远 |

E2E（Playwright）在 `main` 上跑，不阻塞 PR —— 它慢，且依赖构建产物。

---

## 3. 发版：手动触发，双通道

**不用 release-please**（`adr/0007` 修订 2）。发布只手动触发，且只在 `main` 上跑：

```bash
gh workflow run release -f version=0.2.0   # 正式版
gh workflow run release                    # 预发布快照
```

| 填了 version | 版本号 | Release 类型 | 谁会收到 |
|---|---|---|---|
| 是（`0.2.0`） | `0.2.0`，tag 由流水线创建 | **latest** | 所有已安装用户 |
| 留空 | `0.0.0-snapshot.<日期>.<短 sha>` | **prerelease** | 只有手动下载的人 |

更新端点指向 `releases/latest/download/latest.json`——
**prerelease 不会成为 latest，所以快照不会推给已安装用户**。

### 为什么不用 tag 触发（前一个项目实测）

**GitHub Actions 的缓存有 ref 作用域**：tag 触发时缓存写在 `refs/tags/vX.Y.Z` 名下，
下次发版换了 tag 就读不到，等于每次从零编译整个依赖树（实测 6 分钟）。
在 `main` 上跑，作用域一直是 `main`。

顺带省额度：macOS runner 按 10 倍计费。

### 版本号只在发布时注入

仓库里的 `tauri.conf.json` 的 `version` **始终是 `0.0.0`**，
发布时用 `jq` 注入——避免每次发版都产生一次「bump version」提交。

release notes 由 `scripts/release/release-notes.sh` 从 conventional commits 生成，
写进 GitHub Release 正文。**`CHANGELOG.md` 这个文件不再维护**——
GitHub Release 页面就是变更日志。

**「人手动触发并填版本号」就是那个 D3 人工闸门。**

---

## 4. 构建与签名

`.github/workflows/release.yml`，`on: push: tags: ['v*']`。

```
macos-14 ─┬─ 构建 aarch64-apple-darwin ─┐
          └─ 交叉编译 x86_64-apple-darwin ┴─▶ lipo 合成 universal
                                            ─▶ 打包 .app / .dmg ─▶ minisign 签名
```

两个 target 都在 `macos-14`（arm64）上构建：Rust 交叉编译到 x86_64 只需
`rustup target add`，Go 侧 `CGO_ENABLED=0` 更是零成本。
用两个不同架构的 runner 反而更慢更贵，且 x64 runner 排队更久。

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

Apple 公证的开关在 workflow 里已预留，将来买了开发者账号只需填 secrets，不改流水线。

**两个实测踩过的坑**（`adr/0007` 修订 4、5）：

1. **必须写成两个互斥的构建步骤**，不能靠 `tauri-action` 判空——
   GitHub Actions 无法条件性地「不设置」一个 env，空的 `APPLE_CERTIFICATE`
   会让它报 `failed to import keychain certificate`
2. **ad-hoc 签名必须显式设 `APPLE_SIGNING_IDENTITY: '-'`** ——
   Apple Silicon 要求 arm64 可执行文件必须有签名，只靠链接器那份不覆盖 bundle，
   Gatekeeper 会判「**已损坏**」而不是「未认证开发者」。前者用户会以为下载坏了

配套 `scripts/release/install-app.sh` 解除隔离标记，把「未认证开发者」那层也过掉。

### Secrets 清单

| Secret | 用途 | 现在需要 |
|---|---|---|
| `TAURI_SIGNING_PRIVATE_KEY` | minisign 私钥 | ✅ |
| `TAURI_SIGNING_PRIVATE_KEY_PASSWORD` | 私钥口令 | ✅ |
| `APPLE_CERTIFICATE` / `APPLE_CERTIFICATE_PASSWORD` | 代码签名证书 | ⬜ 待定 |
| `APPLE_ID` / `APPLE_PASSWORD` / `APPLE_TEAM_ID` | 公证 | ⬜ 待定 |

### 为什么必须有私钥

**这不是我们加的需求，是 Tauri updater 的强制机制**：构建时用私钥签更新包，
运行时客户端用内嵌的公钥校验，**签名不匹配就拒绝安装**。
它防的是有人替换 Release 上的更新包或中间人劫持下载。

### 私钥存三处（`adr/0007`）

| 位置 | 用途 |
|---|---|
| GitHub Secret `TAURI_SIGNING_PRIVATE_KEY` | CI 签名 |
| 本地 `~/.duet-updater/updater.key` | 生成时的备份 |
| **密码管理器**（另存一份） | 前两处同时丢失时的最后防线 |

```bash
cd shell
# ① 生成密钥对。输出会把私钥打到 stdout —— 重定向掉，别让它进终端历史或日志
pnpm exec tauri signer generate -w ~/.duet-updater/updater.key -p "" --ci > /dev/null 2>&1
chmod 700 ~/.duet-updater && chmod 600 ~/.duet-updater/updater.key

# ② 公钥写进 tauri.conf.json 的 plugins.updater.pubkey（公钥是公开信息，随包分发）
#    .pub 文件的内容本身就是 base64，整段原样填进去

# ③ 私钥进 GitHub Secrets。**这一步必须由人自己跑**
gh secret set TAURI_SIGNING_PRIVATE_KEY < ~/.duet-updater/updater.key
gh secret set TAURI_SIGNING_PRIVATE_KEY_PASSWORD --body ""
```

> **AI 协作者注意**：① 与 ② 可以代做，**③ 不行**。
> 把私钥、口令、令牌这类凭据输入到任何地方（含 `gh secret set`）是硬约束，
> 用户明确授权也不解除。代做的部分务必把 stdout 重定向掉——
> `signer generate` 会把私钥打印出来，一旦进了对话记录或 CI 日志就等于泄露。

**当前状态（2026-08-07）**：① ② 已完成（公钥已在 `tauri.conf.json` 里，
私钥在 `~/.duet-updater/updater.key`，无口令）。**③ 待人工执行**——
在此之前 release workflow 签不出 `.sig`，客户端也就装不上更新。

> ⚠️ **私钥丢失 = 所有已安装客户端再也收不到更新**（公钥硬编码在旧版本里）。
> 只能让用户手动重新下载安装。`~/.duet-updater/` 要加进隔离守卫的受保护列表。

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

### 检查时机：进设置页时检查，**不轮询**（`adr/0007` 修订 3）

- **用户进入设置页时**才检查
- **不做后台轮询**、不在启动时检查
- **绝不自动下载、绝不自动安装、绝不自动重启**

前一个项目的理由：「轮询对一个常驻工具是持续的网络与注意力开销，收益极低」。

Duet 更极端——它会挂在等审批上几小时。后台轮询发现新版本后，
不打断用户就没有任何用处；打断又违反「不打断执行中的单元」。

> 这同时消掉了「发现新版本要不要发一条事件」这个问题——**不需要事件**
> （`adr/0006` Q33）。

本地构建（版本号是 `0.0.0` 或含 `dev`）**跳过检查**。

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
