# ADR 0007 · 按前一个项目的实证修订发布方案

- **日期**：2026-08-07
- **状态**：已接受
- **修订**：[`0002-release-and-auto-update.md`](0002-release-and-auto-update.md) 的触发方式、
  版本号来源、检查时机三项。**其余部分不变。**

## 背景

`~/work/ai-workflows`（同一作者的前一个项目，形态相同：**本地优先的
macOS App + Web**）已经把「GitHub 自动发布 + 应用内一键更新」跑通并用了一段时间，
经验沉淀在它的 `docs/RELEASE.md` 与 `docs/adr/0006`。

对照之后，**Duet 的 ADR 0002 有六处不如它**，其中两处是它实测踩过、
我凭推理不可能想到的坑。这份 ADR 逐条修订。

> 教训：**同一作者的前一个项目是 A 级证据**，优先级仅次于官方规范。
> 写方案前应该先去看它，而不是从零推导。这条已写进 `AGENTS.md` §7 的文档索引。

---

## 先回答一个前置问题：为什么必须有私钥

**私钥不是我们加的需求，是 Tauri updater 插件的强制机制。**

```
构建时：用 minisign 私钥给更新包签名  →  产出 .sig
运行时：客户端用内嵌的公钥校验签名   →  签名不匹配就拒绝安装
```

公钥硬编码在 `tauri.conf.json` 的 `plugins.updater.pubkey` 里，随应用一起分发。
**没有签名对，Tauri updater 根本不工作。**

它防的是：有人替换掉 GitHub Release 上的更新包，或者中间人劫持下载。
用户的应用只认那一个公钥，别人签的包装不上。

**代价是不可逆的**：私钥丢了 → 你再也签不出能被老版本接受的包 →
所有已安装用户永远收不到更新。只能让他们手动重新下载安装。

> 不用 Tauri updater 也是一个选项（前一个项目的参考实现 `codex-ui` 就是
> 手写 zip + 替换脚本），但那样就没有签名校验，任何人都能替换更新包。
> 对一个**会写用户代码的工具**来说，这个风险不能接受。

---

## 修订 1 · 触发方式：tag → `workflow_dispatch` on `main` ★

**原方案**：打 tag 触发 `release.yml`。
**改为**：只在 `main` 上手动触发，tag 由 `tauri-action` 在发布时创建。

### 理由（前一个项目实测）

**GitHub Actions 的缓存有 ref 作用域**：一个 run 只能读「自己这个 ref」
和「默认分支」的缓存。

tag 触发时缓存写在 `refs/tags/vX.Y.Z` 名下，**下一次发版换了 tag 就读不到**——
等于每次发版都从零编译整个依赖树。前一个项目实测 **6 分钟**。

Duet 的构建是 Go + Rust(Tauri) + TS，Rust 那部分只会更慢。

顺带省额度：macOS runner 按 **10 倍**计费，而绝大多数提交没人会去装。

### 双通道

| 触发方式 | 版本号 | Release 类型 | 谁会收到 |
|---|---|---|---|
| 填了 `version=0.2.0` | `0.2.0`，tag 由流水线创建 | **latest** | 所有已安装用户 |
| 留空 | `0.0.0-snapshot.<日期>.<短 sha>` | **prerelease** | 只有手动下载的人 |

更新端点指向 `releases/latest/download/latest.json`——
**prerelease 不会成为 latest，所以快照不会被推给已安装用户**。

---

## 修订 2 · 版本号：不用 release-please，发布时 `jq` 注入 ★

**原方案**：release-please 读 conventional commits，开 Release PR，
自动维护 `CHANGELOG.md` 与 `tauri.conf.json` 的 `version`。
**改为**：仓库里的 `version` **始终是 `0.0.0`**，发布时用 `jq` 注入。

```bash
jq --arg v "$VERSION" '.version = $v' "$conf" > "$tmp" && mv "$tmp" "$conf"
```

### 理由

release-please 每次发版都产生一次 `chore(release): vX.Y.Z` 提交，
**只为改一个数字**。前一个项目的评价很直接：「避免每次发版都产生一次
bump version 提交」。

放弃 release-please 也就放弃了自动 CHANGELOG。**补法**：发布时用
`git log` 从 conventional commits 生成 release notes，写进 GitHub Release 正文——
`CHANGELOG.md` 这个文件本身不再维护（GitHub Release 页面就是变更日志）。

### 与 ADR 0002 的「发版由人点」不冲突

原方案里「人合并 Release PR」是那个 D3 人工闸门。
改为 `workflow_dispatch` 后，**闸门变成「人手动触发并填版本号」**——
同样是人点一下，而且更直接。

```bash
gh workflow run release -f version=0.2.0    # 正式版
gh workflow run release                     # 快照
```

---

## 修订 3 · 检查时机：轮询 → 进设置页时检查 ★

**原方案**：后台每 6 小时 + 启动时检查。
**改为**：**用户进入设置页时才检查**。

### 理由

前一个项目的原话：「轮询对一个常驻工具是持续的网络与注意力开销，收益极低」。

Duet 也是常驻工具，而且**更极端**——它会挂在等审批上几小时。
后台轮询发现新版本后，如果不打断用户（ADR 0002 已定：绝不自动安装），
那这次检查就没有任何用处；如果打断，又违反了「不打断执行中的单元」。

**结论：检查只在用户主动来看的时候做。** 这同时消掉了「发现新版本要不要
发一条事件」这个问题——不需要事件（呼应 ADR 0006 Q33 的裁定）。

---

## 修订 4 · Apple 证书必须用两个互斥的构建步骤 ★（实测踩过的坑）

**不能**靠 `tauri-action` 自己判断证书是否为空。

```
bundle 阶段报 failed to import keychain certificate
```

原因：**GitHub Actions 无法条件性地"不设置"一个 env**——
写了 `APPLE_CERTIFICATE: ${{ secrets.APPLE_CERTIFICATE }}`，
secret 不存在时它就是空字符串，而 `tauri-action` 会拿这个空串去导入钥匙串。

**正确做法**：两个 `if` 互斥的步骤，一个带 Apple env、一个不带。

```yaml
- id: signing
  run: |
    if [ -n "${CERT:-}" ]; then echo "apple=true" >> "$GITHUB_OUTPUT"
    else echo "apple=false" >> "$GITHUB_OUTPUT"; fi

- name: 构建并发布（含 Apple 签名与公证）
  if: steps.signing.outputs.apple == 'true'
  # ... 带全部 APPLE_* env

- name: 构建并发布（ad-hoc 签名，未公证）
  if: steps.signing.outputs.apple != 'true'
  # ... 不带 APPLE_CERTIFICATE
```

---

## 修订 5 · ad-hoc 签名必须显式 `APPLE_SIGNING_IDENTITY: '-'` ★（实测踩过的坑）

没有 Apple 证书时**不能什么都不签**。

前一个项目的原文注释：

> Apple Silicon 要求 arm64 可执行文件必须有签名 —— 只靠链接器那份
> linker-signed 不覆盖 bundle，Gatekeeper 会直接判「已损坏」。

「已损坏」比「未认证开发者」糟糕得多：后者右键打开就能过，
前者用户会以为下载坏了。

```yaml
# ad-hoc 签名（identity "-"）：免费、不需要任何 Apple 账号
APPLE_SIGNING_IDENTITY: '-'
```

再配一个 `scripts/install-app.sh` 解除隔离标记，把「未认证开发者」那层也过掉。

**这个坑我凭推理不可能想到** —— ADR 0002 写的是「ad-hoc 签名，首次安装需手动放行」，
没意识到不显式设 identity 会退化成「已损坏」。

---

## 修订 6 · 幂等检查放 Linux，重发时 macOS runner 不启动

版本解析与「这个版本是不是已经发过了」放在 `ubuntu-latest`（1 倍计费）的
独立 job 里。重复触发同一个版本时，**macOS runner 根本不会启动**。

---

## 保持不变的部分

ADR 0002 的这些结论**经前一个项目验证是对的**，不改：

| 结论 | 前一个项目的做法 |
|---|---|
| minisign 签名必须做 | 一致，且明确对比过「手写 zip + 替换脚本」的参考实现并否决了它 |
| Apple 公证暂缓，CI 预留开关 | 一致，靠 secret 存在与否自动切换 |
| 绝不自动安装 | 一致，且理由相同（会挂在审批上数小时） |
| 安装前要确认未结束的运行 | 一致（他们叫 `setBlockers`） |
| 更新状态机独立于 Tauri、可注入、可普通单测 | 一致 |
| 下载失败退回 `available` 而不是停在 `downloading` | 一致（用户能直接重试，不必重启应用） |
| 本地构建（`dev` 版本）跳过检查 | 一致（`isDevVersion`） |

### Duet 比它多做的一件事

前一个项目的 `setBlockers` 只是**确认**（"有未结束的运行，确定要重启吗"）。

Duet 的 `prepare` 更重：**暂停所有工作 + 落检查点**。
这是设计稿写死的（「更新前会暂停所有工作并保存检查点，重启后从检查点恢复」），
不能退化成只确认。

两者形态兼容：`prepare` 返回的 `blocked` 列表 ≈ 他们的 `blockers`，
前端同样提供「强制更新（丢弃 N 分钟工作）」的逃生门。

---

## 私钥备份方案（Q6 就此解决）

照抄前一个项目，只改路径：

| 存放位置 | 用途 |
|---|---|
| GitHub Secret `TAURI_SIGNING_PRIVATE_KEY` | CI 签名用 |
| 本地 `~/.duet-updater/updater.key` | 生成时的备份 |
| **密码管理器**（另存一份） | 前两处同时丢失时的最后防线 |

```bash
cd shell
pnpm exec tauri signer generate -w ~/.duet-updater/updater.key --password ""
gh secret set TAURI_SIGNING_PRIVATE_KEY --body "$(cat ~/.duet-updater/updater.key)"
# 把输出的公钥写回 shell/src-tauri/tauri.conf.json 的 plugins.updater.pubkey
```

`~/.duet-updater/` 要加进隔离守卫的受保护列表（测试不许碰）。

> **Q6 从「需要人拍板」降级为「已决」** —— 方案是现成的，不需要发明。
> 剩下的只是执行：生成密钥 + 存三处。

---

## 后果

**得到**

- 发版从「每次重编译 6 分钟」变成缓存命中
- 不再有 bump version 提交污染历史
- 避开两个实测踩过的坑（keychain 导入失败、Gatekeeper 判「已损坏」）
- macOS runner 额度只在真发版时消耗
- Q6 不再阻塞

**付出**

- 放弃自动 CHANGELOG，改为发布时从 git log 生成 release notes
- 版本号要人填，不再从 commits 推导（但这本来就是那个人工闸门）

**风险**

- 人填版本号可能填错/跳号。缓解：`plan` job 做幂等检查，
  已存在的版本直接拒绝，不会覆盖
