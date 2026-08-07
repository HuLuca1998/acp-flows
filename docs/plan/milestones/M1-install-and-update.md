# M1 · 装得上，能一直更新

> 对应验收点 **V1 · V2 · V3**（[`../acceptance.md`](../acceptance.md)）。
>
> **为什么排第一**：用户验收发现问题 → 我改 → 他得拿到改完的版本。
> 没有一键更新，这个循环每次都要卸载重装。
> 所以**第一个交到他手上的版本里就必须已经带着一键更新**。

> **进度（2026-08-07）**：S1.1 三个单元已完成（平台适配层 / 更新卡片 / 一键更新流程）。
> 壳已能拉起 `duetd` 并常驻菜单栏，本地打出的 `.app` 跑通了四类产物与签名校验。
> **卡点在 `U1.2.1` 第 ③ 步**：私钥进 GitHub Secrets 必须由人执行，见下。

## 目标

**用户下载一个 `.dmg`，装上能打开；之后我每发一个版本，他在设置页点一下就更新完了，工作不丢。**

## 完成标志

用户自己做这三件事，全部成功：

1. 下载 `.dmg` → 拖进「应用程序」→ 双击 → **窗口打开了**
2. 我发一个新版本 → 他进「设置 → 应用更新」→ 看到「发现新版本」→ 点「⟳ 一键更新并重启」→ **应用重启后版本号变了**
3. 进「设置 → 环境检测」→ **看得出 Claude / Codex 装没装、登没登录**

机器侧的对应检查：

```bash
make check                                                   # 全绿
gh release view v<X.Y.Z> --json assets --jq '[.assets[].name]'
        # 含 .dmg / .app.tar.gz / .app.tar.gz.sig / latest.json 四类
minisign -Vm Duet_<X.Y.Z>_universal.app.tar.gz \
  -P "$(jq -r .plugins.updater.pubkey shell/src-tauri/tauri.conf.json)"
```

## 已经就绪的地基

这些在本次里程碑重建之前就做完了，**不再作为单元**，但 M1 直接依赖它们：

| 已完成 | 提交 | M1 用它来做什么 |
|---|---|---|
| `duetd serve` + 回环鉴权 + SQLite | `8cf900f` | 承载 `/v1/system/*` 端点 |
| 工程门禁与 CI 四个 job | `117bd73` | 发版前的质量闸门 |
| 日志双去处（stderr + 落库） | `800d84f` | 更新失败时能查为什么 |
| 版本比较 + 检查更新 + 更新前准备（后端） | `ab30ec7` | V2 的后端一半 |
| Tauri 壳骨架 + release workflow 骨架 | — | V1 的起点 |

## 全局停止条件

触发任一条 **立刻停下来上报**：

1. 需要把私钥、口令或任何密钥材料写进仓库（含测试夹具）
2. 需要让客户端**自动下载 / 自动安装 / 自动重启**——更新必须由用户点
3. `prepare` 返回 `blocked` 而前端仍要继续安装
4. 需要删除或重打一个已发布的 tag
5. 需要手改 `tauri.conf.json` 的 `version`（它永远是 `0.0.0`，发布时注入）
6. 撞上 [`../open-questions.md`](../open-questions.md) 的 **Q5**（Apple 证书）——它**不阻塞**，ad-hoc 签名能跑通

---

## S1.1 · 让用户看到「有新版本」并点得动

**用户拿到什么**：设置页里的更新卡片。这是 V2 的界面一半。

### ✓ U1.1.1 · 平台适配层：Tauri 有 updater，浏览器没有  ·  `aba9010`

| | |
|---|---|
| `goal` | 让界面代码不必知道自己跑在 Tauri 里还是浏览器里，两种形态都能给出**真的可用**的更新路径 |
| `allowed_changes` | `frontend/src/platform/**` · `frontend/src/platform/*.test.ts` |
| `forbidden_changes` | 除 `platform/` 外任何文件出现 `@tauri-apps/*`；Web 实现写成空函数 |
| `stop_conditions` | 发现 Tauri updater 的 API 与 `adr/0007` 修订 3 冲突 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 桌面形态返回可用的 updater | 断言 `capabilities().canSelfUpdate === true` |
| R2 | **Web 降级必须真的可用**，不是空函数 | 断言 Web 实现返回指向 GitHub Release 的下载地址，而不是 `undefined` |
| R3 | 检测形态**不靠 UA 猜**，靠壳注入的标记 | 断言没有 `window.__TAURI__` 时判为 Web |
| R4 | 非 `platform/` 文件 import `@tauri-apps/*` 会被拦 | 故意写一行，`make lint-frontend` 必须红 |

### ✓ U1.1.2 · 更新卡片：看得懂、点得动  ·  `6c301a4`

| | |
|---|---|
| `goal` | 用户在设置页能看到当前版本、有没有新版本、这次改了什么，并点一下开始更新 |
| `allowed_changes` | `frontend/src/features/settings/**` · `frontend/src/i18n/locales/*.json` |
| `forbidden_changes` | 硬编码用户可见文案（一律 `t('key')`）；硬编码 hex / 裸 px；自己发明设计稿没有的控件 |
| `stop_conditions` | 设计稿里找不到某个控件的样式条目——**先补设计条目再实现**（铁律 3） |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 文案与设计稿逐字一致 | 断言渲染出「发现新版本」「当前」「⟳ 一键更新并重启」「稍后」「完整更新日志」 |
| R2 | 已是最新时**不显示更新按钮** | 断言 `state=idle` 时查不到「一键更新」按钮 |
| R3 | Web 形态显示降级路径而不是死按钮 | 断言 `state=unsupported` 时出现「前往下载」而非「一键更新」 |
| R4 | 检查失败时**明确告诉用户失败了** | 断言出现错误提示，且**不出现**「已是最新版本」 |
| R5 | 中英双语齐备 | `make check-i18n` 绿 |
| R6 | 无硬编码颜色与尺寸 | `make lint-frontend`（stylelint）绿 |

> **R4 是这个单元最重要的一条。** 网络断了却显示「已是最新版本」，
> 用户永远不会知道自己在用旧版——这类故障没有任何症状。

### ✓ U1.1.3 · 一键更新的完整流程与失败处理  ·  `aba9010`

| | |
|---|---|
| `goal` | 用户点下按钮之后，要么更新成功重启，要么清楚地知道为什么没成 |
| `allowed_changes` | `frontend/src/features/settings/**` · `frontend/src/platform/**` |
| `forbidden_changes` | 跳过 `prepare` 直接安装；`blocked` 时仍继续安装 |
| `stop_conditions` | 发现 Tauri updater 无法在安装后自动重启 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | **先 `prepare` 再下载**，顺序不可颠倒 | 断言未调 `prepare` 时不会发起下载 |
| R2 | `blocked` 时**停下并列出卡住的工作** | 断言界面出现工作列表，且下载未被发起 |
| R3 | 下载过程有进度反馈 | 断言进度从 0 变化到 100 |
| R4 | 下载/校验失败时给出可操作的提示 | 断言出现重试入口，且不是一句英文堆栈 |
| R5 | 全程**不自动安装**：必须用户点 | 断言没有任何定时器会自行触发安装 |

---

## S1.2 · 让用户真的拿到安装包

**用户拿到什么**：一个能下载、能安装的 `.dmg`。这是 V1。

### ◐ U1.2.1 · minisign 密钥与 GitHub Secrets  ·  `2696c0b`（**卡在第 ③ 步：等人执行**）

| | |
|---|---|
| `goal` | 让发布出去的包能被客户端验签——没有它，updater 会拒绝一切更新 |
| `allowed_changes` | `docs/spec/release-and-update.md` 的操作步骤 · `shell/src-tauri/tauri.conf.json` 的 `pubkey` 字段 |
| `forbidden_changes` | **私钥或口令进仓库**（含测试夹具、含 `.gitignore` 掉的文件）；替用户保管私钥 |
| `stop_conditions` | 需要我持有私钥才能继续——**必须由用户自己生成并录入 Secrets** |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 公钥进仓库，私钥不进 | `git log -p` 全历史 grep 私钥特征串为空 |
| R2 | 给用户的操作步骤可照抄执行 | 断言文档里每条命令都能直接粘贴运行 |
| R3 | 私钥有离线备份方案 | 断言文档写明备份位置与丢失后的补救（重发公钥 = 老版本无法更新） |

> **这是 M1 唯一必须用户亲自操作的一步。** AI 不碰密钥材料。
>
> **当前状态（2026-08-07）**：
> - ✓ 密钥对已生成在 `~/.duet-updater/updater.key`（无口令），权限 0600
> - ✓ 公钥已写进 `shell/src-tauri/tauri.conf.json`，与签名 key ID 对得上
>   （`C8E416816471A7C9`，已用真包验证过）
> - ⬜ **私钥进 GitHub Secrets —— 等人执行**：
>   ```bash
>   gh secret set TAURI_SIGNING_PRIVATE_KEY < ~/.duet-updater/updater.key
>   gh secret set TAURI_SIGNING_PRIVATE_KEY_PASSWORD --body ""
>   ```
>
> ★ **接手的 AI 注意**：这一步**不要代做**。把私钥、口令、令牌这类凭据
> 输入到任何地方（含 `gh secret set`）是硬约束，用户明确授权也不解除。
> 没有这个 Secret，CI 签不出 `.sig`，`U1.2.2` 一定失败——
> 遇到就停下来找人，不要试图绕过。

### ○ U1.2.2 · 发一个真实版本并验证四类产物

| | |
|---|---|
| `goal` | 跑通一次真实发版，产出用户能下载的 `.dmg` |
| `allowed_changes` | `scripts/release/**` · `.github/workflows/release.yml`（已停用，只改说明） |
| `forbidden_changes` | 用 `--admin` 绕过分支保护；重打已发布的 tag |
| `stop_conditions` | 构建产物缺任何一类；签名验证不过 |

> **2026-08-08 改成在本机发版。** macOS runner 按 10 倍计费，而 universal 包
> 要编两个架构的 Rust；真发版试了三次、两次挂在环境上，每次十几分钟额度只
> 换回一条错误信息。现在跑：
>
> ```bash
> bash scripts/release/publish-local.sh 0.0.1
> ```
>
> `release.yml` 已 `gh workflow disable`，文件保留——本机脚本与它的步骤
> 一一对应，将来恢复 CI 发版时那就是参照。**别顺手把它重新启用**，
> 原因写在文件头。

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | Release 上有四类产物 | `gh release view` 的 assets 含 `.dmg` / `.app.tar.gz` / `.sig` / `latest.json` |
| R2 | 签名可验证 | `minisign -Vm ... -P <pubkey>` 退出码 0 |
| R3 | `latest.json` 的版本号与 tag 一致 | 断言两者字符串相等 |
| R4 | 仓库里的 `version` 仍是 `0.0.0` | 断言发布后 `tauri.conf.json` 未被改动 |

### ○ U1.2.3 · 在一台干净的机器上装得上

| | |
|---|---|
| `goal` | 证明「没装过开发工具的 mac 也能装能开」，而不只是我的机器能跑 |
| `allowed_changes` | `docs/spec/release-and-update.md` 的安装说明 · `scripts/release/install-app.sh` |
| `forbidden_changes` | 要求用户装 Xcode / Node / Go 才能运行 |
| `stop_conditions` | 发现 sidecar 二进制没被打进包里 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | `.app` 内含 `duetd` sidecar | 断言 `Contents/MacOS/` 下存在该二进制 |
| R2 | 双击能打开并出现窗口 | 人工操作测试（第 ⑤ 步），留截图 |
| R3 | Gatekeeper 拦截时有可执行的解法 | 断言说明里给出 `xattr` 命令，一条即可解除 |
| R4 | 应用退出时 sidecar 一并退出 | 断言退出后 `pgrep duetd` 为空（不留僵尸） |

---

## S1.3 · 让用户看得出环境有没有配好

**用户拿到什么**：设置页的环境检测。这是 V3。

### ✓ U1.3.1 · Runtime 检测与可执行的修复提示  ·  `dc2248a`

| | |
|---|---|
| `goal` | 用户能看出 Claude / Codex 装没装、登没登录，没装时知道下一步敲什么 |
| `allowed_changes` | `backend/internal/acp/runtime/**` · `backend/internal/api/**` 的 `/v1/runtimes` · `frontend/src/features/settings/**` |
| `forbidden_changes` | 上层按 runtime 名字做判断（只能查能力）；写 `~/.claude`、`~/.codex` 一个字节 |
| `stop_conditions` | 检测需要真实发起模型调用（那会产生费用） |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 未安装 / 已安装未登录 / 已就绪三态可区分 | 三个用例各断言一种状态 |
| R2 | 提示**含具体命令**，不是「请检查配置」 | 断言未登录时的提示里含 `codex login` |
| R3 | 检测**零模型开销** | 断言全程未发 `session/prompt` |
| R4 | 检测失败不影响应用其余部分 | 断言检测超时后其他页面照常可用 |

---

## M1 验收

**六步全走完之外，还要满足：**

| # | 标准 | 怎么验 |
|---|---|---|
| A1 | 用户在一台干净 mac 上装上并打开 | 人工，留截图 |
| A2 | 用户点一下完成一次真实更新，版本号变了 | 人工，前后两张截图 |
| A3 | 有工作在跑时点更新，**被拦住并列出是哪几个** | 人工 |
| A4 | 网络断开时点检查更新，**明确报错**而不是「已是最新」 | 人工，拔网线试 |
| A5 | `make check` 全绿 | CI |

**A4 是 M1 真正的验收标准。** 其余都是过程；
一个会把故障伪装成「一切正常」的更新系统，比没有更新系统更危险。
