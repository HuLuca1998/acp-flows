# M1 · 发布与自动更新

> ⚠️ **本章的 S1.2（分支保护）与 S1.5（构建签名发布）已被
> [`../adr/0007`](../adr/0007-release-revision-from-prior-art.md) 修订**：
> 触发方式改为 `workflow_dispatch` on main、弃用 release-please、
> 检查时机改为进设置页时检查。开工前先读那份 ADR。
>
> 体系与编号规则见 [`README.md`](README.md)。动手前必读
> [`../release-and-update.md`](../release-and-update.md)（规格）、
> [`../adr/0002-release-and-auto-update.md`](../adr/0002-release-and-auto-update.md)（已定的决策，不重新讨论）、
> [`../ci.md`](../ci.md)（CI 设计规范）与 [`../git-workflow.md`](../git-workflow.md) §6（版本推导）。

> **读法**：本文 ~15k token，**不要整篇读**。一个里程碑章是子计划的菜单，
> 你一次只做其中**一个**子计划。标准读法是三段：
>
> ```
> ① 目标 + 完成标志 + 全局停止条件     ~1k tok，必读
> ② 子计划 DAG                        看清你这个子计划的前置有没有做完
> ③ 你自己那一个 S1.x                 只读这一节
> ```
>
> ```bash
> grep -n '^## ' docs/milestones/M1-release-and-update.md   # 全部子计划一览
> grep -n '^## S1.7'  docs/milestones/M1-release-and-update.md   # 定位到你的那一节
> ```
>
> | 子计划 | 一句话 |
> |---|---|
> | S1.1 | 工程门禁可信（先把检查本身修对） |
> | S1.2 | 版本推导与 Release PR（**已被 adr/0007 修订**） |
> | S1.3 | Tauri 壳最小可用 |
> | S1.4 | 前端骨架与平台适配层 |
> | S1.5 | 构建、签名与发布（**已被 adr/0007 修订，签名有两个坑**） |
> | S1.6 | 系统端点：version / check / 检查调度 |
> | S1.7 ★ | `prepare`：更新前暂停工作并落检查点 |
> | S1.8 | 更新界面 · 一键更新重启 · 恢复 |
> | S1.9 | Runtime 更新（第二条线） |

## 目标

**合一个 PR 就能发出一个用户装得上、且能自动更新的版本。**

「装得上」= 一台没装过 Duet 的 mac 下载 `.dmg` 能打开并跑起来。
「能自动更新」= 下一次发版时，这台机器收到提示，点一下完成更新，**且不丢工作**。

## 完成标志

```bash
make check                                                    # 全绿
gh pr checks <M1 的任一 PR>                                    # ci 汇总门禁 pass，不是 queued
gh release view v<X.Y.Z> --json assets --jq '[.assets[].name]'
        # 含 .dmg / .app.tar.gz / .app.tar.gz.sig / latest.json 四类
minisign -Vm Duet_<X.Y.Z>_universal.app.tar.gz \
  -P "$(jq -r .plugins.updater.pubkey shell/src-tauri/tauri.conf.json)"   # 退出码 0
curl -sf -H "Authorization: Bearer $DUET_DEV_TOKEN" \
  http://127.0.0.1:7777/v1/system/update/check                # state=available，全程零下载
pnpm -C e2e test -g 'update'                                  # available → prepare → blocked 阻断 → resume
```

## 为什么 M1 是这个顺序

**先修 CI，再干别的。** 当前 GitHub Actions 的 run 一直停在 `queued`、runner 从不接单
（[issue #3](https://github.com/HuLuca1998/acp-flows/issues/3)）。这不是一个可以绕开的背景噪音：
M1 的绝大多数验收标准的断言对象**就是 CI 的结论**。CI 报不出结果时，所有验收都只能靠
`--admin` 绕过分支保护 —— **绕过换来的绿不是验收，是伪造验收**。
`git-workflow.md` §3 已经写明「连续两次需要绕过说明 CI 本身坏了」，而 PR #1、#2 已经绕过两次。

**发布链路上每一环只在下一环存在时才可验证**，所以顺序是被物理决定的：

```
没有 tauri.conf.json  →  没有版本号真源
没有版本号            →  release-please 开不出 Release PR
没有 Release PR       →  没有 tag
没有 tag              →  没有构建产物
没有产物              →  没有 minisign 签名、没有 latest.json
没有 latest.json      →  客户端根本检测不到更新
```

**`prepare` 反着来。** 它的完整语义横跨 M0（`Work` 状态机、两段式取消）与 M2（Checkpoint 聚合），
不可能在 M1 前期做完。所以先做简化版，但简化版的默认分支是**失败安全**的：
遇到非终态 Work 一律返回 `blocked`，而不是放行。
「先放行、以后再补暂停逻辑」会在中间那段时间里真实地丢掉用户几十分钟的工作 ——
而「更新不丢工作」正是这个产品能被信任的前提。

## 依赖

**M1 与 M0 可并行**（不同 worktree）。并行的前提是文件不重叠，边界如下表：

| 路径 | M0 归属 | M1 归属 |
|---|---|---|
| `backend/internal/acp/**` | 全部 | 仅 `runtime/install.go` · `runtime/activate.go`（S1.9 新建） |
| `backend/internal/api/**` | 路由骨架 · 鉴权中间件 · `sse/` | `api/system/**` · `api/runtime/**` |
| `backend/internal/app/**` | 用例编排骨架 | `app/system/**` · `app/runtime/**` |
| `backend/internal/domain/**` | 全部 | 仅 `policy/update_prepare.go` |
| `api/openapi.yaml` | U0.10.1 先落骨架与 `Event` | `/system/*` · `/runtimes/*` · `/settings` 各节 |
| `frontend/**` | — | **全部**（M0 没有前端单元，M1 负责脚手架） |
| `shell/**` | — | 全部 |
| `.github/**` · `release-please-config.json` | — | 全部 |
| `scripts/**` | `probe.sh` · `dev-web.sh` | 其余 |

### M1 单元对 M0 单元的具体依赖

| M1 单元 | 依赖 M0 | 依赖的是什么 | M0 未就绪时 |
|---|---|---|---|
| U1.3.2 | U0.10.1 | 可执行的 `duetd` 二进制 | 用桩进程完成本单元，真机联调等 M0 |
| U1.6.1 | U0.10.1 | HTTP 路由骨架 + bearer 鉴权 | 停，等 M0 |
| U1.6.2 | U0.1.2 · U0.10.2 | 注入式 `Clock` · eventbus | 停，等 M0 |
| U1.7.1 | — | 只经 port 做只读查询 | **可立刻开工** |
| U1.7.2 | U0.6.1 · U0.9.1 · U0.5.2 | 两段式取消 · `Work` 状态机 · 事件游标持久化 | 停，留在 U1.7.1 的简化版 |
| U1.7.2 | **M0 无对应单元** | Checkpoint 聚合与落盘 | ★ 里程碑拆分缺口，见全局停止条件第 7 条 |
| U1.8.3 | U0.5.2 | 会话恢复；恢复失败不伪造「会话仍连续」 | 停，等 M0 |
| U1.9.1 | U0.7.2 | Runtime 注册表与多版本并存目录 | 停，等 M0 |
| U1.9.2 | U0.7.1 | 12 项能力探针与能力矩阵 | 停，等 M0 |

## 全局停止条件

触发任一条 **立刻停下来上报**，不要自行扩大范围：

1. issue #3 未解决，而当前验收标准的断言对象是 CI 的结论 → **停，不要用 `--admin` 把验收糊过去**
2. 需要手改 `shell/src-tauri/tauri.conf.json` 的 `version` 或 `CHANGELOG.md`
3. 需要把私钥、口令或任何密钥材料写进仓库（含测试夹具）
4. 需要让客户端自动下载 / 自动安装 / 自动重启
5. `prepare` 返回 `blocked` 而前端仍需继续安装
6. 需要删除或重打一个已发布的 tag（`adr/0002`「不做」第 3 条明确禁止）
7. 需要在 M1 里发明 Checkpoint 聚合 —— `roadmap.md` 与 `adr/0002` 都写「1.7 依赖 M0 的 Work 状态机与 checkpoint」，
   但 M0 章节里没有 Checkpoint 单元，它排在 M2 的 2.8。这是拆分缺口，找人裁定，不要顺手造一个
8. 需要新增第 14 类 `Event.type` —— 它是封闭枚举，新增要同时改四处（`frontend-guide.md` §9.5）
9. 撞上 [`../open-questions.md`](../open-questions.md) 的 **Q3 · Q5 · Q6 · Q8 · Q13 · Q14 · Q17 · Q19**

---

## 子计划 DAG

```
                     S1.1 工程门禁可信 ★
             （U1.1.1 解 issue #3 —— 全里程碑取证的前置）
   ┌──────────────┬──────────────┬──────────────┬──────────────┐
   ▼              ▼              ▼              ▼              ▼
S1.2 版本推导  S1.3 Tauri 壳  S1.4 前端骨架  S1.6 系统端点  S1.9 Runtime 更新
   │              │              │          ◀ M0 U0.10.1   ◀ M0 U0.7.1 / U0.7.2
   └──────┬───────┘              │              │
          ▼                      │              ▼
  S1.5 构建·签名·发布             │        S1.7 prepare
          │                      │        ◀ M0 U0.6.1 / U0.9.1（仅 U1.7.2）
          └───────────┬──────────┴──────────────┘
                      ▼
        S1.8 更新界面 · 一键更新并重启 · 恢复
```

**可并行**：`S1.1` 之后 `S1.2` / `S1.3` / `S1.4` / `S1.6` / `S1.9` 五条同时开，
写入范围分别是 `.github/**`+`release-please-config.json` / `shell/**` / `frontend/**` /
`backend/internal/{api,app}/system/**` / `backend/internal/acp/runtime/{install,activate}.go`，**互不重叠**。

`S1.9` 与 M1 其余部分零交集 —— 它是 Runtime 更新线，只等 M0 的 `S0.7`。

---

## S1.1 · 工程门禁可信

**阶段交付物**：CI 能真的跑、能真的拦、分支保护只认汇总门禁，且这三件事各被反向验证过一次。

> 对应 `roadmap.md` 的 1.1 与 1.2。
> **本子计划完成前，M1 其余单元不得把「CI 绿」写进验收断言。**

### ○ U1.1.1 · 解除 GitHub Actions 排队阻塞（issue #3）★

| | |
|---|---|
| `goal` | 让 workflow run 能被 runner 接单并跑完，使 M1 后续每一条验收标准都能在 CI 上取证，而不是靠 `--admin` 绕过 |
| `allowed_changes` | `.github/workflows/ci.yml`（仅在确认根因在 workflow 侧时）· `docs/ci.md` 新增排障小节 · `docs/tech-debt.md` 登记行 |
| `forbidden_changes` | 不改分支保护规则本身；不删检查、不加 `continue-on-error`、不放宽断言以求变绿；本单元不夹带任何业务改动 |
| `stop_conditions` | 排查后确认根因在**账号级**（billing / Actions 配额 / 组织策略 / 账号门禁）→ 停下来上报。改账号设置需要仓库所有者操作，AI 不得代为进行 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 一次 push 触发的 run 在 10 分钟内离开 `queued` | `gh api repos/HuLuca1998/acp-flows/actions/runs --jq '.workflow_runs[0].status'` 输出 `in_progress` 或 `completed` |
| R2 | `changes` job 被真的分配到 runner | `gh api repos/HuLuca1998/acp-flows/actions/runs/<id>/jobs --jq '.jobs[].runner_name'` 每行非空、非 `null` |
| R3 | `ci` 汇总门禁在一个 PR 上报出结论 | `gh pr checks <n> --json name,state --jq '.[]\|select(.name=="ci").state'` 输出 `SUCCESS` |
| R4 | issue #3 关闭，且关闭评论里贴了 R1–R3 的实际命令输出 | `gh issue view 3 --json state --jq .state` 输出 `CLOSED` |
| R5 | 根因与复现命令写进 `docs/ci.md` 的排障小节 | 该小节含「现象 / 根因 / 验证命令」三项，缺一即不通过 |

**测试**：本单元的「先红的测试」就是 issue #3 里那条复现命令 ——
修复前它输出 `queued`（红），修复后输出 `in_progress`（绿）。两次输出都要贴进 PR。

### ○ U1.1.2 · 负例夹具：证明检查真的会红

| | |
|---|---|
| `goal` | 为 `guard` / `contract` 两个 job 与 `ci` 汇总门禁各准备一个**故意违规**的夹具，把「检查会红」本身变成一条常驻断言 |
| `allowed_changes` | `scripts/test-guards.sh`（新增）· `Makefile` 的 `check` 目标链 · `.github/workflows/ci.yml` 的 `guard` job · `scripts/AGENTS.md` · `docs/ci.md` §7 |
| `forbidden_changes` | 负例夹具不得留在工作树里（一律在 `mktemp -d` 的副本上制造违规）；不修改被测检查脚本的判定逻辑；不给任何检查加豁免名单 |
| `stop_conditions` | 某条检查无法在不污染工作树的前提下被反向验证 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 删掉任一关键目录的 `AGENTS.md` → `make check-docs` 红 | `scripts/test-guards.sh` 断言退出码 `!= 0`，且 stderr 含被删目录的路径 |
| R2 | 改 `api/openapi.yaml` 而不跑 `make gen` → `make check-gen` 红 | 同上手法，断言退出码 `!= 0` |
| R3 | 提交信息里 `先红的测试: 不适用` 而 type 是 `feat` → `check-commit-msg.sh` 红 | 构造一个夹具提交，断言退出码 `!= 0` |
| R4 | `ci` 汇总门禁对 `failure` 判红 | 临时分支上让 `guard` 必失败，断言 `gh run view <id> --json jobs --jq '.jobs[]\|select(.name=="ci").conclusion'` 为 `failure` |
| R5 | `ci` 汇总门禁对 `skipped` 判绿 | 只改 `docs/**` 的 PR（`backend`/`frontend`/`shell` 全跳过），断言同一条命令输出 `success` |
| R6 | 尚未脚手架的 job **显式跳过并打印原因**，不静默通过 | `gh run view <id> --log \| grep '尚未脚手架'` 有输出 |
| R7 | `scripts/test-guards.sh` 挂进 `make check` 与 `guard` job | `make -n check \| grep test-guards` 有输出；`grep -n test-guards .github/workflows/ci.yml` 有输出 |

**测试**：R1–R3 是「**故意制造违规，确认检查会红**」；R4/R5 直接验证 `ci.md` 规则 2
最容易踩的那一条 —— 汇总门禁把 `skipped` 当通过、把 `failure` 当不通过。

### ○ U1.1.3 · `main` 分支保护落地与反向验证

| | |
|---|---|
| `goal` | `main` 只能经 PR + `ci` 汇总门禁合入，且每条规则都被反向验证过一次，而不是「配了就以为生效」 |
| `allowed_changes` | `scripts/setup-branch-protection.sh` · `docs/git-workflow.md` §5 的表 |
| `forbidden_changes` | **不把单个 job 设为 required check**（`ci.md` 禁止清单第 2 条）；不改 `enforce_admins`；不放宽 `required_linear_history` |
| `stop_conditions` | `git-workflow.md` §5 的 required checks 表（`ci / backend` `ci / frontend` `ci / docs` `ci / contract` 四项）与 `ci.md` 规则 2（只 required `ci` 一项）**直接矛盾** —— 按 `docs/AGENTS.md`「一件事只在一处写」合并到 `ci.md`，`git-workflow.md` 改成链接。合并后仍存疑则停下来上报 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 直接 push `main` 被拒 | `git push origin main` 退出码 `!= 0`，且 stderr 含 `protected branch` |
| R2 | required check **恰为** `ci` 一项 | `gh api repos/HuLuca1998/acp-flows/branches/main/protection --jq '.required_status_checks.contexts'` 输出 `["ci"]` |
| R3 | 仅 squash 可合并 | `gh api repos/HuLuca1998/acp-flows --jq '[.allow_squash_merge,.allow_merge_commit,.allow_rebase_merge]'` 输出 `[true,false,false]` |
| R4 | 线性历史；禁止强推与删除 | `gh api .../branches/main/protection --jq '[.required_linear_history.enabled,.allow_force_pushes.enabled,.allow_deletions.enabled]'` 输出 `[true,false,false]` |
| R5 | 合并后自动删分支 | `gh api repos/HuLuca1998/acp-flows --jq .delete_branch_on_merge` 输出 `true` |
| R6 | 两份文档不再冲突 | `grep -n 'ci / backend' docs/git-workflow.md` 无输出 |
| R7 | 一个只改 `docs/**` 的 PR 能正常合入（跳过的 job 不卡它） | `gh pr merge <n> --squash --delete-branch` 退出码 0，且**未加** `--admin` |

> R7 是 `ci.md` 规则 2 的最终验证。它红，说明 required check 配错了，
> 后面每一个只改单个子项目的 PR 都会永远 pending。

---

## S1.2 · 版本推导与 Release PR

**阶段交付物**：一个 `feat:` 提交合进 `main`，自动出现一个 `chore(release): vX.Y.Z` 的 PR；
版本号只有一个真源，手改会红。

### ○ U1.2.1 · release-please 产出 Release PR 与 CHANGELOG

| | |
|---|---|
| `goal` | 版本号完全由 conventional commits 推导，Release PR 同时维护 `CHANGELOG.md` 与 `shell/src-tauri/tauri.conf.json` 的 `version` |
| `allowed_changes` | `release-please-config.json` · `.release-please-manifest.json` · `.github/workflows/release-please.yml` · `CHANGELOG.md`（首次由 release-please 生成，人不写正文） |
| `forbidden_changes` | 不手写 `CHANGELOG.md` 正文；不手改任何位置的版本号；**不给 Release PR 加自动合并**（`adr/0002` 决策 2：这是整条流水线上唯一需要人点的地方） |
| `stop_conditions` | `extra-files` 指向的 `shell/src-tauri/tauri.conf.json` 尚不存在 → 先做 U1.3.1 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 合入一个 `feat(...)` 提交后自动开 Release PR | `gh pr list --search 'chore(release) in:title' --json headRefName --jq '.[0].headRefName'` 输出 `release-please--branches--main` |
| R2 | 版本号按 `git-workflow.md` §2 的表推导 | `feat` → minor +1、patch 归 0：断言 PR 标题里的 `X.Y.Z` 与上一 tag 的差值 |
| R3 | Release PR 恰好改两个文件 | `gh pr diff <n> --name-only` 输出恰为 `CHANGELOG.md` 与 `shell/src-tauri/tauri.conf.json`（首次可含 `.release-please-manifest.json`） |
| R4 | `tauri.conf.json` 的 `version` 与 PR 标题一致 | `gh pr diff <n> \| grep '"version"'` 的新值 == 标题中的 `X.Y.Z` |
| R5 | CHANGELOG 分节用配置里的中文节名，隐藏节不出现 | 断言 diff 含 `### 新功能`；`grep -c '### 文档\|### 杂项'` 为 0 |
| R6 | **Release PR 不会被任何自动化合并** | `grep -rn 'pr merge\|automerge\|auto-merge' .github/workflows/` 无输出 |
| R7 | 只有 `docs`/`chore`/`ci` 提交时不开 Release PR | 断言 `gh pr list --search 'chore(release) in:title'` 为空数组 |

### ○ U1.2.2 · 版本号真源守卫

| | |
|---|---|
| `goal` | 任何人（含 AI）手改 `tauri.conf.json` 的 `version` 或手写 `CHANGELOG.md` 时，`guard` job 直接红 |
| `allowed_changes` | `scripts/check-version-source.sh`（新增）· `Makefile` 的 `check` 目标链 · `.github/workflows/ci.yml` 的 `guard` job · `scripts/AGENTS.md` · `docs/git-workflow.md` §6 |
| `forbidden_changes` | 不把违规降级为警告；`release-please--branches--main` 之外不设任何豁免；不改 `release-please-config.json`（那是 U1.2.1 的边界） |
| `stop_conditions` | 在 PR 上无法区分 release-please 的提交与人的提交 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 普通分支上改 `version` → 红 | 临时分支改一位，`bash scripts/check-version-source.sh <base> <head>` 退出码 `!= 0` |
| R2 | `release-please--branches--main` 上改 `version` → 放行 | 同一脚本对该分支退出码 `== 0` |
| R3 | 普通分支上改 `CHANGELOG.md` → 红 | 同 R1 手法，退出码 `!= 0` |
| R4 | 失败信息给出修正方式 | 断言 stderr 含「版本号由 release-please 维护」与撤销该改动的具体命令 |
| R5 | 脚本挂进 `make check` 与 `guard` job | `make -n check \| grep check-version-source` 有输出；`grep -n check-version-source .github/workflows/ci.yml` 有输出 |
| R6 | `package.json` / `go.mod` / `Cargo.toml` 里的版本字段不参与推导 | 改 `shell/package.json` 的 `version`，断言脚本退出码 `== 0`（不是真源，不管） |

---

## S1.3 · Tauri 壳最小可用

**阶段交付物**：无边框窗口能开、`duetd` 能被拉起并守住、端口与令牌能注入 WebView。

> 规格在 [`../../shell/AGENTS.md`](../../shell/AGENTS.md) 与
> [`../architecture.md`](../architecture.md) §2、§6。**壳只做四件事**，第五件不要写进来。

### ○ U1.3.1 · `src-tauri` 骨架与无边框窗口

| | |
|---|---|
| `goal` | `shell/src-tauri` 存在且 `cargo clippy` / `cargo test` 可跑，`make dev-app` 开出一个无边框窗口，`tauri.conf.json` 成为版本号真源的落点 |
| `allowed_changes` | `shell/package.json` · `shell/pnpm-lock.yaml` · `shell/src-tauri/Cargo.toml` · `shell/src-tauri/tauri.conf.json` · `shell/src-tauri/src/main.rs` · `shell/src-tauri/src/constants/**` · `shell/src-tauri/icons/**` |
| `forbidden_changes` | 壳里不写业务逻辑、不做持久化、不直接和 ACP Runtime 说话；非测试代码禁用 `unwrap()` / `expect()`；本单元不新增任何 Tauri command；不配置 updater（那是 U1.5.2） |
| `stop_conditions` | `decorations: false` 下的拖拽区与设计稿的 42px 窗口栏冲突 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | CI 的 `shell` job 不再被脚手架守卫跳过 | `gh run view <id> --json jobs --jq '.jobs[]\|select(.name=="shell").conclusion'` 输出 `success` |
| R2 | `cargo clippy --all-targets -- -D warnings` 退出码 0 | CI `shell` job 的该 step |
| R3 | 非测试代码里的 `unwrap()` 被 deny | 在 `main.rs` 临时写一行 `.unwrap()`，`cargo clippy` 退出码 `!= 0` |
| R4 | 窗口无边框，窗口栏高度常量为 42 | `jq -r .app.windows[0].decorations shell/src-tauri/tauri.conf.json` 输出 `false`；Rust 单测断言 `constants::TITLEBAR_HEIGHT == 42.0` |
| R5 | `version` 初值与 manifest 一致 | `jq -r .version shell/src-tauri/tauri.conf.json` == `jq -r '.["."]' .release-please-manifest.json` |
| R6 | bundle 目标含 `app` 与 `dmg` | `jq -r '.bundle.targets' shell/src-tauri/tauri.conf.json` 含这两项 |
| R7 | 壳里零业务 command | `ls shell/src-tauri/src/commands 2>/dev/null \| wc -l` 输出 0 |

### ○ U1.3.2 · sidecar 生命周期与僵尸清理

| | |
|---|---|
| `goal` | 壳负责拉起 `duetd`、健康检查、崩溃重启、退出时优雅关闭；App 崩溃后残留的 `duetd` 在下次启动被回收 |
| `allowed_changes` | `shell/src-tauri/src/sidecar.rs` · `shell/src-tauri/src/models/**` · `shell/src-tauri/src/utils/**` · `shell/src-tauri/tauri.conf.json` 的 `bundle.externalBin` · `scripts/build.sh` · `shell/src-tauri/tests/**` |
| `forbidden_changes` | **不得通过 Rust IPC 绕过 HTTP 调后端逻辑**（`shell/AGENTS.md` 最重要的一条）；壳里不解析业务响应体；不改 `backend/**`；测试不访问 `$HOME/.acpflows` |
| `stop_conditions` | `duetd` 尚不存在（M0 U0.10.1 未合入）→ 用 `shell/src-tauri/tests/fixtures/stub-duetd` 桩进程完成本单元，真机联调等 M0；需要按进程组杀孙进程而当前抽象做不到 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | sidecar 拉起后健康检查通过 | 集成测试：起桩进程 → 断言 `GET /v1/system/version` 返回 200 |
| R2 | 桩进程崩溃后被重启，且重启次数有上限 | 连续杀 4 次，断言前 3 次被重启、第 4 次后状态置为 `sidecar_failed` 且不再重启 |
| R3 | 退出时先 `SIGTERM`、超时再 `SIGKILL` | 用忽略 SIGTERM 的桩进程，断言最终退出且耗时 ≈ 宽限期（±200ms） |
| R4 | 上次崩溃残留的 `duetd` 在下次启动被回收 | 写一个指向存活桩进程的 pid 文件 → 启动壳 → 断言 `kill -0 <pid>` 退出码 `!= 0` |
| R5 | sidecar 二进制文件名带 target triple 后缀 | 跑 `bash scripts/build.sh --target aarch64-apple-darwin`，断言 `shell/src-tauri/binaries/duetd-aarch64-apple-darwin` 存在 |
| R6 | 壳与后端只经 HTTP 通信 | `grep -rn 'invoke\|IpcMessage' shell/src-tauri/src` 的结果全部落在 `commands/` 内；`commands/` 内无任何业务端点调用 |
| R7 | 桩进程与真 `duetd` 的接口断言完全相同 | 同一批断言用表驱动跑两遍（真 `duetd` 用例带构建标签，M0 未就绪时跳过） |

### ○ U1.3.3 · 端口令牌注入与 `window.__DUET__`

| | |
|---|---|
| `goal` | 壳读 `session.json` 并把 `{port, token}` 注入 WebView，使前端用与浏览器完全一样的 `fetch` / `EventSource` 访问后端 |
| `allowed_changes` | `shell/src-tauri/src/commands/session.rs` · `shell/src-tauri/src/models/session.rs` · `shell/src-tauri/src/utils/paths.rs` · `shell/src-tauri/tests/**` |
| `forbidden_changes` | **令牌绝不写进日志**、绝不进入前端可持久化存储、绝不放进 URL 查询串；不读写用户真实 `~/.acpflows`（测试一律 `tempdir` + 环境变量覆盖）；不动 `frontend/**` |
| `stop_conditions` | `session.json` 的字段与 `architecture.md` §2「本地回环安全」不一致 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | `session.json` 权限不是 `0600` 时拒绝启动 | 夹具文件 `chmod 0644`，断言返回 `session_file_insecure` 且窗口不加载前端 |
| R2 | 注入的 `window.__DUET__` 键集合**恰为** `port` / `token` / `platform` | 单测断言注入脚本的 JSON 顶层键排序后等于该三元组 |
| R3 | 令牌不出现在任何日志 | `RUST_LOG=trace` 下跑一遍集成测试，断言全部输出不含夹具 token 字面量 |
| R4 | 测试全程不触碰 `$HOME/.acpflows` | 铁律 6 守卫：访问该路径直接 fail，且信息指向铁律 6 |
| R5 | `session.json` 缺失时给出可执行提示，不白屏 | 断言错误码 `sidecar_not_ready`，且界面渲染出重试入口 |
| R6 | 端口是从文件读的，不是硬编码 | 夹具写入 `48231`，断言注入值为 `48231` |

---

## S1.4 · 前端骨架与平台适配层

**阶段交付物**：`frontend/` 可 build / test / lint / typecheck，设计合规 lint 会拦人，
自动更新在 Web 形态下有**真的能用**的降级。

> M0 没有前端单元，`frontend/**` 由 M1 全权负责。

### ○ U1.4.1 · frontend 脚手架与设计合规 lint

| | |
|---|---|
| `goal` | 建立 `frontend/` 的构建、测试、lint、类型检查与契约生成物，使 CI 的 `frontend` / `contract` 两个 job 不再被脚手架守卫跳过 |
| `allowed_changes` | `frontend/package.json` · `frontend/pnpm-lock.yaml` · `frontend/vite.config.ts` · `frontend/tsconfig.json` · `frontend/eslint.config.js` · `frontend/.stylelintrc.json` · `frontend/src/{app,design,api,i18n}/**` · `frontend/tests/**` · `scripts/gen-api.sh` |
| `forbidden_changes` | 不写任何业务页面（S1.8）；`src/platform/` 之外不得 import `@tauri-apps/*`；不新增未经批准的第三方依赖；不改 `api/openapi.yaml` |
| `stop_conditions` | 撞上 `open-questions.md` **Q14 / Q15 / Q17 / Q19**（设置页 tab 选中态、输入框、空/错误态、toast 均无设计条目）—— 本单元只做骨架，**不实现这四类控件** |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | CI 的 `frontend` job 不再被跳过且全绿 | `gh run view <id> --json jobs --jq '.jobs[]\|select(.name=="frontend").conclusion'` 输出 `success` |
| R2 | 硬编码 hex 被 lint 拦下 | 临时写 `color: #ff0000`，`pnpm -C frontend lint` 退出码 `!= 0` |
| R3 | 裸 px 被 lint 拦下 | 临时写 `padding: 12px`，同一命令退出码 `!= 0` |
| R4 | `src/platform/` 之外 import `@tauri-apps/*` 被 ESLint 拦下 | 在 `src/app/` 临时 import，同一命令退出码 `!= 0` |
| R5 | 中英词条同进同退 | 只往 `zh-CN.json` 加一条，`make check-i18n` 退出码 `!= 0` |
| R6 | TS 客户端生成物与 spec 一致 | 改 `api/openapi.yaml` 不跑 `make gen`，`make check-gen` 退出码 `!= 0` |
| R7 | `make dev-web` 能起来并打开 | `scripts/dev-web.sh` 起服务后 `curl -sf http://localhost:5173` 返回 200 |

### ○ U1.4.2 · 平台适配层与自动更新的 Web 降级

| | |
|---|---|
| `goal` | 组件侧只认 `@/platform` 单例；Web 形态下自动更新降级为「可见但按钮 `disabled` + 下载链接」，而不是空实现 |
| `allowed_changes` | `frontend/src/platform/{index.ts,types.ts,tauri/**,web/**}` · `frontend/tests/platform/**` |
| `forbidden_changes` | `src/platform/` 之外不得出现 `@tauri-apps/*`；降级实现里禁止 `throw new Error('not supported')`、禁止静默 no-op、禁止空函数体（`frontend-guide.md` §12.2）；不在本单元写设置页 |
| `stop_conditions` | 某个能力在 Web 下给不出「用户能看到什么、点了之后发生什么」的降级 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 运行时按 `window.__DUET__` 是否存在选实现 | 两个用例分别断言 `platform.kind === 'tauri'` / `'web'` |
| R2 | `PlatformAdapter` 的每个方法在 web 实现里都有非空实现 | 反射遍历接口方法名，断言 web 实现的每个方法体长度 > 0 且不抛 `not supported` |
| R3 | Web 形态下更新按钮 `disabled` 且给出下载链接 | 组件测试断言按钮 `disabled === true`，且链接 `href` 指向 GitHub Release |
| R4 | Web 形态下窗口控制按钮**整体隐藏**，不是置灰 | 断言 DOM 里该节点不存在（`queryByTestId` 返回 `null`） |
| R5 | 降级表五行各有 ≥ 1 个断言「用户能看到什么 / 点了之后发生什么」的测试 | 测试名带 `frontend-guide.md` §12.2 表的行号，`grep -c` 输出 ≥ 5 |
| R6 | 新增一个 `PlatformAdapter` 方法而 web 实现未补时测试红 | 加一个方法，断言 R2 的反射测试变红 |

---

## S1.5 · 构建、签名与发布

**阶段交付物**：tag 触发后，GitHub Release 上出现 universal 安装包 + `.sig` + `latest.json`。

> 决策见 `adr/0002` 决策 3：**minisign 立刻做，Apple 公证暂缓**（CI 预留开关）。
> 这两条已定，不重新讨论。

### ○ U1.5.1 · universal 合成与 sidecar 打包

| | |
|---|---|
| `goal` | tag 触发后产出一个 universal 的 `Duet.app` / `.dmg`，内嵌两个架构的 `duetd` |
| `allowed_changes` | `scripts/make-universal.sh`（新增）· `scripts/build.sh` · `.github/workflows/release.yml` 的 `build` / `publish` job · `shell/src-tauri/tauri.conf.json` 的 `bundle` |
| `forbidden_changes` | **不在 PR 上跑构建**（`ci.md` §3）；不改 `.github/workflows/ci.yml`；不把构建产物提交进仓库；不改 `plugins.updater`（U1.5.2 的边界） |
| `stop_conditions` | `lipo` 无法合成两个架构的产物；`release-and-update.md` §4 的图写着 `macos-14` + `macos-13` 两个 runner，而 `release.yml` 的 matrix 两行都是 `macos-14` —— 二者不一致，裁定后同步修正文档或 workflow，不要两边各留一套 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 主二进制是 universal | `lipo -archs dist/Duet.app/Contents/MacOS/Duet` 输出同时含 `x86_64` 与 `arm64` |
| R2 | 内嵌的 `duetd` 同样是双架构 | 对 sidecar 二进制跑同一条 `lipo -archs`，输出同 R1 |
| R3 | `.dmg` 与 `.app.tar.gz` 各恰好一份 | `ls dist/*.dmg \| wc -l` 与 `ls dist/*.app.tar.gz \| wc -l` 均输出 1 |
| R4 | x64 产物是真的按 x64 编的，不是 arm64 冒充 | `file artifacts/bundle-x86_64-apple-darwin/**/Duet` 输出含 `x86_64`，不含 `arm64` |
| R5 | 构建只在 tag 与 `workflow_dispatch` 上触发 | `yq '.on \| keys' .github/workflows/release.yml` 输出恰为 `[push, workflow_dispatch]`，且 `.on.push` 只有 `tags` |
| R6 | 产物不入库 | `git status --porcelain dist shell/src-tauri/binaries` 无输出 |

### ○ U1.5.2 · minisign 密钥、离线备份与签名

| | |
|---|---|
| `goal` | 更新包被 minisign 签名、客户端强制校验；私钥有离线备份，**备份这件事本身是一条验收标准** |
| `allowed_changes` | `shell/src-tauri/tauri.conf.json` 的 `plugins.updater`（`pubkey` / `endpoints`）· `.github/workflows/release.yml` 的签名与公证 step · `docs/release-and-update.md` §4 的 Secrets 清单 · `docs/open-questions.md` 的 Q6 行 |
| `forbidden_changes` | **私钥、口令、任何密钥材料一律不得进仓库**，测试夹具也不行；不把 Apple 公证改成默认开启；不改构建矩阵（U1.5.1 的边界） |
| `stop_conditions` | 撞上 `open-questions.md` **Q6**（私钥离线备份放哪、谁保管）—— 这是人拍板项，**AI 不得替它决定备份位置**；撞上 **Q5**（要不要买 Apple 开发者证书） |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 每个更新包旁有一份 `.sig` | `ls dist/*.app.tar.gz.sig \| wc -l` == `ls dist/*.app.tar.gz \| wc -l` |
| R2 | 签名能被 `tauri.conf.json` 里的公钥验证 | `minisign -Vm dist/Duet_<X.Y.Z>_universal.app.tar.gz -P "$(jq -r .plugins.updater.pubkey shell/src-tauri/tauri.conf.json)"` 退出码 0 |
| R3 | 篡改安装包后校验失败 | 往 `.tar.gz` 追加 1 字节，同一条 `minisign -V` 退出码 `!= 0` |
| R4 | `endpoints` 指向 GitHub Release 的 `latest.json` | `jq -r '.plugins.updater.endpoints[0]' shell/src-tauri/tauri.conf.json` 匹配 `.../releases/latest/download/latest.json` |
| R5 | 仓库里没有任何私钥材料 | `git grep -n 'untrusted comment: minisign secret key'` 无输出；密钥扫描工具退出码 0 |
| R6 | **私钥已离线备份**，位置与保管人由人指定 | `docs/open-questions.md` 的 Q6 行已标注「结论 + 日期 + 出处」并移入「已决」表；**未标注前本单元不得标 `✓`** |
| R7 | Apple 公证保持关闭，且开关可用 | 断言公证 step 带 `if: env.APPLE_ID != ''` 守卫；不填 `APPLE_*` secrets 时该 step 结论为 `skipped` 而不是 `failure` |

> **R6 不是文书工作。** 公钥硬编码在每个已发布的旧版本里；私钥丢了，
> 这些客户端**再也收不到任何更新**，且无法远程补救 —— 只能让每个用户手动重装。
> 这是 M1 唯一一个不可逆、且无技术手段兜底的风险点。

### ○ U1.5.3 · `latest.json` 与 GitHub Release 发布

| | |
|---|---|
| `goal` | 每个 tag 产出一份 `latest.json`，Tauri updater 据它发现新版本；`notes` 直接取 CHANGELOG 里该版本的条目 |
| `allowed_changes` | `scripts/make-updater-manifest.sh`（新增）· `scripts/test-make-updater-manifest.sh`（新增）· `scripts/testdata/updater/**` · `.github/workflows/release.yml` 的 `publish` job · `Makefile` 的 `check` 目标链 |
| `forbidden_changes` | 不手写 `latest.json` 的 `notes`；不把安装包 URL 指向 `latest` 之外的浮动地址；**不删除或重打已发布的 tag**；不用真实 GitHub Release 做测试 |
| `stop_conditions` | CHANGELOG 里找不到该版本的小节（说明 release-please 链路断了）→ 回 S1.2 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 顶层键**恰为** `version` `notes` `pub_date` `platforms` | `jq -S 'keys' dist/latest.json` 输出与 `scripts/testdata/updater/latest.golden.json` 的同一表达式逐字相同 |
| R2 | `platforms` 恰含 `darwin-aarch64` 与 `darwin-x86_64` | `jq -r '.platforms\|keys\|sort' dist/latest.json` 输出 `["darwin-aarch64","darwin-x86_64"]` |
| R3 | 两个平台指向同一个 universal 包 | `jq -r '[.platforms[].url]\|unique\|length' dist/latest.json` 输出 `1` |
| R4 | `version` 与 tag 去掉 `v` 后一致 | `jq -r .version dist/latest.json` == `${GITHUB_REF_NAME#v}` |
| R5 | `notes` 与 CHANGELOG 该版本小节正文逐字一致，且非空 | 脚本比对两段文本，差异非空即退出码 `!= 0` |
| R6 | `signature` 是 `.sig` 的**内容**而不是路径 | `jq -r '.platforms["darwin-aarch64"].signature' dist/latest.json \| head -c 18` 输出 `untrusted comment` |
| R7 | 生成脚本本地可跑且有 golden 对比 | `bash scripts/test-make-updater-manifest.sh` 退出码 0；改 golden 里任一字段后退出码 `!= 0` |
| R8 | Release 的 asset 清单齐全 | `gh release view v<X.Y.Z> --json assets --jq '[.assets[].name]'` 同时含 `.dmg` · `.app.tar.gz` · `.app.tar.gz.sig` · `latest.json` |

---

## S1.6 · 系统端点：version / check / 自动检查调度

**阶段交付物**：duetd 能报版本、能在**不下载不安装**的前提下报出有没有新版本，并按固定节律自动检查。

> 依赖 M0 U0.10.1（路由骨架与 bearer 鉴权）。它未合入时本子计划不开工。

### ○ U1.6.1 · `GET /v1/system/version` 与 `POST /v1/system/update/check`

| | |
|---|---|
| `goal` | duetd 报出自己的版本与构建信息，并在零下载的前提下报出 `idle` / `available` / `unsupported` |
| `allowed_changes` | `backend/internal/api/system/**` · `backend/internal/app/system/**` · `backend/internal/platform/buildinfo.go` · `backend/tests/system/**` · `scripts/build.sh` 的 ldflags · `api/openapi.yaml` 的 `/system/version` 与 `/system/update/check` 两节 |
| `forbidden_changes` | 不改 M0 U0.10.1 拥有的路由骨架与鉴权中间件；不在 `api` 层写业务判断；**duetd 不下载安装包**（`release-and-update.md` §8 末句）；后端不返回中文文案，只返回 `Problem.type` 错误码 |
| `stop_conditions` | M0 U0.10.1 未合入 → 停，等它；需要改 `UpdateStatus` schema 的既有字段 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | `version` 与 `tauri.conf.json` 一致 | 响应的 `version` == `jq -r .version shell/src-tauri/tauri.conf.json` |
| R2 | 无 `Authorization` 一律 401 | 断言状态码 401，且响应体不含版本、commit、路径任一信息 |
| R3 | `check` 全程零下载 | httptest mock updater endpoint，断言收到的请求路径集合 ⊆ `{/latest.json}`，对 `.tar.gz` 的请求数 == 0 |
| R4 | 无新版本 → `state == idle`；有新版本 → `state == available` 且带 `latest_version` / `notes` / `size_bytes` / `published_at` | 两个用例，四个字段逐个断言非空 |
| R5 | Web 形态 → `state == unsupported` | 断言 `unsupported`，且不返回 `latest_version` |
| R6 | 响应通过 `openapi.yaml` schema 校验 | `kin-openapi` 校验 `UpdateStatus` |
| R7 | 拉取 `latest.json` 失败时返回 `Problem`，**不伪装成「无更新」** | mock 500，断言 `Problem.type == update_check_failed`，且响应不是 `state: idle` |
| R8 | 签名字段缺失的 `latest.json` 被拒 | 去掉 `signature`，断言 `Problem.type == updater_manifest_invalid` |

> R7 是「不静默失败」的落点。返回 `idle` 掩盖网络错误，会让用户永远收不到更新却毫无察觉。

### ○ U1.6.2 · 自动检查调度：6 小时 + 启动时，绝不自动安装

| | |
|---|---|
| `goal` | 后台每 6 小时 + 启动时各检查一次，发现新版本发一条 `source: app` 的事件；开关关掉后完全静默 |
| `allowed_changes` | `backend/internal/app/system/updatecheck.go` · `backend/internal/eventbus/**`（仅新增发布调用）· `backend/tests/system/**` · `api/openapi.yaml` 新增 `/settings` 一节 |
| `forbidden_changes` | **绝不自动下载、绝不自动安装、绝不自动重启**（`adr/0002` 决策 5）；不在 `domain` 层写定时器；不用裸 `time.Now()` / `time.Ticker`（一律经 M0 U0.1.2 的注入式 `Clock`）；测试不访问真实网络 |
| `stop_conditions` | ① 「自动检查更新（不自动安装）」开关在 `api/openapi.yaml` 里没有对应资源 → 按铁律 2 **先改 spec 再跑 `make gen`**，不得先写 handler。② `Event.type` 的 13 类封闭枚举里**没有一类对应「发现新版本」**，而 `release-and-update.md` §5 要求发一条 `app` 事件 —— 复用既有类型还是新增第 14 类需要人裁定，**登记进 `open-questions.md` 的 P1 表（下一个可用编号 Q33）后停下来**，裁定前不实现事件发布 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 假时钟推进 6 小时恰好触发一次 | 推进 6h 断言检查次数 == 1；推进 12h 断言 == 2 |
| R2 | 启动时立刻检查一次，不等 6 小时 | 断言启动后检查次数 == 1，且假时钟未推进 |
| R3 | 发现新版本发一条 `source: app` 的事件 | 断言 eventbus 收到 1 条，`source == "app"`，`type` 属于 `openapi.yaml` 的封闭枚举（取值以 Q33 的裁定为准） |
| R4 | **零下载**：调度器整个生命周期内无安装包请求 | mock server 记录全部请求路径，断言集合 ⊆ `{/latest.json}` |
| R5 | 开关关闭后完全静默 | 关开关 → 推进 24h → 断言检查次数 == 0 且事件数 == 0 |
| R6 | 同一版本不重复发事件 | 连续 4 次检查返回同一 `latest_version`，断言事件总数 == 1 |
| R7 | 进程退出时调度器停止，不泄漏 goroutine | `goleak` 断言无泄漏 |
| R8 | 开关默认值与设计稿一致（默认开启自动检查） | 断言首次启动时 `/v1/settings` 返回的该项为 `true` |

---

## S1.7 · `prepare`：更新前暂停所有工作并落检查点 ★

**阶段交付物**：`POST /v1/system/update/prepare` 可用，且在任何情况下都**不会让工作被静默丢弃**。

> `prepare` 是一次**跨聚合的领域操作，不是形式步骤**（`adr/0002` 决策 4）。
> 设计稿写死了那句「更新前会暂停所有工作并保存检查点，重启后从检查点恢复」。

### ○ U1.7.1 · `prepare` 简化版：无进行中工作时直接放行

| | |
|---|---|
| `goal` | 让 S1.1–S1.6 不必干等 M0：没有非终态 Work 时返回 `{"status":"ready","prepared":[],"blocked":[]}` |
| `allowed_changes` | `backend/internal/api/system/prepare.go` · `backend/internal/app/system/prepare.go` · `backend/internal/app/port/work.go`（只加只读查询方法）· `backend/tests/system/prepare_test.go` |
| `forbidden_changes` | **本单元不实现暂停与落检查点**；不 import `backend/internal/domain/model`（M0 U0.9.1 未合入时一律经 port）；不改 `api/openapi.yaml` 的 `UpdatePrepareResult` schema |
| `stop_conditions` | 存在非终态 Work 时**不得猜一个处理方式** —— 本单元一律返回 `blocked` 并附 `reason: work_state_machine_unavailable`，等 U1.7.2 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 无非终态 Work → `status == ready`，两个数组均为空 | 断言三个字段：`ready` / `len(prepared)==0` / `len(blocked)==0` |
| R2 | 存在非终态 Work → `status == blocked`，`reason == work_state_machine_unavailable` | 假 port 返回 1 个 `executing`，断言 `len(blocked)==1` 且 reason 一致 |
| R3 | **本单元不落任何检查点** | 断言 checkpoint port 的调用次数 == 0（spy 计数） |
| R4 | 响应通过 `UpdatePrepareResult` schema 校验 | `kin-openapi` |
| R5 | 连续调两次结果一致且无副作用 | 断言两次响应体逐字节相同，且 port 写方法调用次数 == 0 |
| R6 | 无 `Authorization` 一律 401 | 断言 401 |

> **失败安全**：简化版遇到非终态 Work 时**拦下**，不是放行。
> 放行等于在没有 checkpoint 能力的窗口期里让用户丢工作。

### ○ U1.7.2 · `prepare` 完整版：暂停 + 落检查点 + `blocked`

| | |
|---|---|
| `goal` | 对每个非终态 Work 走「两段式取消 → 采集证据与事件游标 → 落检查点 → `paused`」；任一 Work 卡住即整体 `blocked`，**且不落半截检查点** |
| `allowed_changes` | `backend/internal/app/system/prepare.go` · `backend/internal/domain/policy/update_prepare.go` · `backend/tests/system/prepare_test.go` · `backend/tests/fixtures/acp/prepare/**` |
| `forbidden_changes` | 不改 `Work` 状态机本身（M0 U0.9.1 的边界）；不改两段式取消的公开签名（M0 U0.6.1 的 `forbidden_changes`）；`domain` 里不做 IO；不改 `UpdatePrepareResult` schema |
| `stop_conditions` | ① M0 **U0.9.1**（`Work` 状态机）或 **U0.6.1**（两段式取消）未合入 → 本单元不得开工，留在 U1.7.1。② M0 里没有 Checkpoint 聚合（见全局停止条件第 7 条）→ 停，不要顺手发明一个。③ 撞上 `open-questions.md` **Q3**（`executing` 与 `waiting_user` 能否共存）—— 它直接决定 `waiting_user` 分支怎么写。④ 撞上 **Q8**（`contract_revision` 期间 Work 处于哪个状态）—— 状态穷举测试会少一格 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 六种 Work 状态各有明确处理，穷举 | 表驱动：`executing` / `reviewing_unit` / `waiting_user` / `paused` / `completed` / `failed` 各一个用例；新增一个状态而未处理时测试红 |
| R2 | `executing` 的处理顺序是「取消 → 采集游标 → 落检查点 → `paused`」 | spy 记录调用序列，断言序列逐项相等；顺序打乱即红 |
| R3 | `reviewing_unit` 的当前 Attempt 被标 `superseded` | 断言该 Attempt 状态 == `superseded` |
| R4 | `waiting_user` 直接落检查点并保留待决策项 | 断言检查点已落，且待决策项数量不变 |
| R5 | Runtime 不回 `stopReason` 时该 Work 进 `blocked`，`reason == cancel_timeout`，**且不落检查点** | Fake Runtime 用 `NeverStops`；断言 checkpoint 调用次数 == 0 |
| R6 | 任一 Work `blocked` → 顶层 `status == blocked` | 3 个 Work 中 1 个超时，断言顶层为 `blocked` 而不是 `ready` |
| R7 | 连续 `prepare` 两次只发一次协议 cancel | Fake 记录的 `session/cancel` 请求数 == 1（M0 U0.6.1 R1 在上层的复现） |
| R8 | pending 的 `session/request_permission` 全部收到 `cancelled` 应答 | Fake 发 2 个权限请求后 `prepare`，断言两个都收到 `cancelled` |
| R9 | 已落的检查点在 duetd 重启后可读 | 临时 SQLite + 临时 git 夹具仓库：落点 → 重启 duetd → 断言 `GET /v1/system/resume` 列出该 `work_id` 与 `checkpoint_id` |
| R10 | 测试全程不碰 `~/.acpflows` 与用户真实仓库 | 铁律 6 守卫断言 |

> R8 对应 `open-questions.md` 的 **Q4d**：规范硬要求，设计稿完全没提。
> **漏了会导致每次取消都超时、`prepare` 永远返回 `blocked`、自动更新彻底不可用。**

---

## S1.8 · 更新界面 · 一键更新并重启 · 恢复

**阶段交付物**：设计稿的「设置 → 应用更新」可用，走完 `available → prepare → 安装 → 重启 → resume`，
且 `blocked` 时**装不下去**。

### ○ U1.8.1 · 设置页「应用更新」卡片与自动检查开关

| | |
|---|---|
| `goal` | 按设计稿渲染当前版本 / 新版本 / 更新说明 / 体积 / 三个动作，并带「自动检查更新（不自动安装）」开关与那句说明文字 |
| `allowed_changes` | `frontend/src/features/settings/update/**` · `frontend/src/i18n/{zh-CN,en-US}.json` · `frontend/tests/features/settings/**` |
| `forbidden_changes` | 不硬编码 hex / 裸 px（铁律 3）；组件里不硬编码用户可见文本（一律 `t('key')`）；不翻译状态词与版本号；**本单元不实现下载与安装动作**（U1.8.2） |
| `stop_conditions` | 撞上 `open-questions.md` **Q14**（设置页 tab 选中态与设计规范第 08 节矛盾）、**Q17**（空/错误态无条目）、**Q19**（toast 无条目）→ 先补设计条目再实现（铁律 3） |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 卡片渲染五项：当前版本 / 新版本 / 更新说明 / 体积 / 发布时间 | 用固定的 `UpdateStatus` 夹具，逐项断言可见文本 |
| R2 | 三个动作的文案与设计稿一字不差 | 逐字断言 `一键更新并重启` · `完整更新日志` · `稍后` |
| R3 | 开关文案与说明文字一字不差 | 逐字断言 `自动检查更新（不自动安装）` 与 `更新前会暂停所有工作并保存检查点，重启后从检查点恢复。` |
| R4 | 零硬编码中文 | `grep -rnP '[\p{Han}]' frontend/src/features/settings/update --include='*.tsx'` 无输出 |
| R5 | 中英词条同进同退 | `make check-i18n` 退出码 0；只加 `zh-CN` 词条时退出码 `!= 0` |
| R6 | `state == idle` 时不显示更新卡片主体 | 断言「一键更新并重启」按钮不存在，且显示「已是最新」 |
| R7 | 版本号走 `--font-mono` | 断言该节点的 computed `font-family` 解析自 `var(--font-mono)` |
| R8 | 关闭开关后不再出现角标 | 断言角标节点不存在 |

### ○ U1.8.2 · 前端更新状态机与 `blocked` 阻断 ★

| | |
|---|---|
| `goal` | 实现 `idle → checking → available → preparing → downloading → installing → restarting → resuming` 与 `preparing → blocked`；`blocked` 时**不得进入 `downloading`** |
| `allowed_changes` | `frontend/src/features/settings/update/machine.ts` · `frontend/src/features/settings/update/**` · `frontend/tests/features/settings/update/**` |
| `forbidden_changes` | **`prepare` 返回前不得发起任何下载**；`blocked` 不得有任何通往 `downloading` 的迁移；不改 `api/openapi.yaml`；**不用真实 GitHub Release 做测试**（一律 MSW mock，`release-and-update.md` §9 末句） |
| `stop_conditions` | 「强制更新（丢弃 work-11 的 2:14 工作）」这个破坏性动作在设计规范里找不到条目 → 先补条目（铁律 3） |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 调用顺序是「先 `prepare` 后下载」 | MSW 记录请求顺序，断言 `POST /v1/system/update/prepare` 的时间戳严格早于任何安装包请求 |
| R2 | `prepare` 返回 `blocked` 时状态机停在 `blocked`，**零下载请求** | 断言 MSW 收到的安装包请求数 == 0 |
| R3 | `blocked` 没有通往 `downloading` 的迁移 | 穷举状态机转移表，断言 `transitions.blocked` 的目标集合 ⊆ `{idle, preparing}` |
| R4 | `blocked` 时把卡住的工作显示给用户，含 `work_id` 与原因 | 断言渲染出 `work-11`，以及 `cancel_timeout` 对应的中文词条 |
| R5 | 破坏性按钮写清后果，不是空动词 | 断言按钮文案匹配 `强制更新（丢弃 work-11 的 2:14 工作）` 的模式；出现「确定」「提交」即红 |
| R6 | `prepare` 网络失败时进 `blocked`，不进 `downloading` | mock 500，断言最终状态为 `blocked` |
| R7 | 八个状态全部可达，非法迁移被拒 | 表驱动穷举；新增一个状态而未处理时红 |
| R8 | 后端错误码经 i18n 翻译，界面不出现 `Problem.detail` 原文 | 断言渲染文本不含夹具里的英文 `detail` 字符串 |
| R9 | 未点按钮时不发生任何状态推进 | 停在 `available` 上推进假时钟 24h，断言状态不变、请求数不增 |

### ○ U1.8.3 · 重启后恢复：`/v1/system/resume` 与「启动时恢复」开关

| | |
|---|---|
| `goal` | App 重启后能列出可恢复的工作；「启动时恢复 · 从检查点恢复未完成的工作」开关决定自动恢复还是只提示 |
| `allowed_changes` | `backend/internal/api/system/resume.go` · `backend/internal/app/system/resume.go` · `frontend/src/app/bootstrap.ts` · `frontend/src/features/settings/update/**` · `api/openapi.yaml` 新增 `/system/resume/{workId}` 一节 · `backend/tests/system/**` |
| `forbidden_changes` | 恢复失败时**不得伪造「会话仍连续」**（M0 U0.5.2 R5）；不自动恢复处于 `blocked` 的工作；后端不返回中文文案 |
| `stop_conditions` | `api/openapi.yaml` 只有 `GET /system/resume`，而 `release-and-update.md` §8 还列了 `POST /v1/system/resume/{workId}` —— 契约缺一个端点，按铁律 2 **先补 spec 再实现**；M0 U0.5.2 未合入 → 停 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | `GET /v1/system/resume` 列出 U1.7.2 落下的检查点 | 集成测试：`prepare` → 重启 duetd → 断言 `resumable[0]` 的 `work_id` / `checkpoint_id` 与 `prepare` 响应一致 |
| R2 | 开关开启时启动后自动恢复 | 断言 `POST /v1/system/resume/{workId}` 被调用 1 次 |
| R3 | 开关关闭时只提示不恢复 | 断言调用次数 == 0，且界面渲染出可恢复工作的提示与手动恢复入口 |
| R4 | 恢复失败时显式标记为「新会话」 | 断言响应里的连续性标记为 false（字段名以 `api/openapi.yaml` 为准），且界面出现对应提示；**静默按「续接」渲染即红** |
| R5 | 无可恢复工作时 `resumable` 是 `[]` 不是 `null` | 断言 `jq -c .resumable` 输出 `[]` |
| R6 | 响应通过 schema 校验 | `kin-openapi` |
| R7 | `blocked` 的工作不在 `resumable` 里 | 造一个 `blocked` 的 Work，断言它不出现在列表中 |

---

## S1.9 · Runtime 更新（第二条线）

**阶段交付物**：`claude-agent-acp` / `codex-acp` 能从 npm 装进多版本目录，探针全过才允许切换，
出问题可一键退回。**不需要重启 App。**

> 这是与应用更新完全独立的第二条线（`release-and-update.md` §6）。
> 它只等 M0 的 `S0.7`，与 M1 其余部分零交集。

### ○ U1.9.1 · 从 npm 安装到多版本目录

| | |
|---|---|
| `goal` | duetd 能把指定版本装进 `~/.acpflows/runtimes/<name>/<version>/`，**不覆盖**已有版本 |
| `allowed_changes` | `backend/internal/acp/runtime/install.go` · `backend/internal/app/runtime/**` · `backend/internal/api/runtime/**` · `backend/tests/runtime/**` · `api/openapi.yaml` 新增 `/runtimes/{name}/install` 一节 |
| `forbidden_changes` | 不改 M0 U0.7.2 已合入的注册表文件；**不写 `~/.claude` / `~/.codex` 一个字节**；测试不访问真实 npm registry（一律本地 mock registry 或夹具 tarball）；上层不得按 runtime 名字分支 |
| `stop_conditions` | M0 **U0.7.2** 未合入 → 停；撞上 `open-questions.md` **Q13**（Runtime 是二值枚举还是注册表 —— 设置页出现了第三项 `acp-sidecar`） |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 多版本并存，装新版本不删旧版本 | 装 `0.63.0` 再装 `0.64.1`，断言 `<tempdir>/runtimes/<name>/` 下两个目录都存在 |
| R2 | 版本已存在时不重复下载 | mock registry 记录请求数，第二次安装断言新增请求数 == 0 |
| R3 | 安装过程不写 `~/.acpflows` 与用户真实目录 | 铁律 6 守卫断言 |
| R4 | 校验失败时不留半截目录 | 注入损坏 tarball，断言目标版本目录不存在（不是留一个空壳） |
| R5 | 接口返回 `Problem` 错误码而非中文 | 断言 `Problem.type` 匹配 `^[a-z0-9_]+$`，且响应体无中文字符 |
| R6 | 上层零品牌判断 | `grep -rn 'codex\|claude' backend/internal/{app,domain} backend/internal/api --exclude-dir=gen` 无输出 |
| R7 | 加第三个 runtime 只需加一个包 + 登记一行 | 加一个测试用 runtime，断言 `install.go` 与 `api/runtime/**` 无需改动 |

> R6 的 `--exclude-dir=gen` 是必要的：`api/openapi.yaml` 的 `RuntimeName` 是
> `enum: [claude, codex]`，生成物里必然出现这两个字面量。
> 若判定生成物也必须为空，那就是契约要改成开放字符串 —— **属于 `contract_revision`，停下来上报**，
> 不要在本单元里悄悄放宽 `scripts/check-naming.sh`。

### ○ U1.9.2 · 探针门禁与切换 / 退回

| | |
|---|---|
| `goal` | 新装版本必须 12 项探针全过才允许切为默认；有活跃会话时不打断执行中的单元；切换出问题可一键退回 |
| `allowed_changes` | `backend/internal/acp/runtime/activate.go` · `backend/internal/app/runtime/**` · `frontend/src/features/settings/runtime/**` · `backend/tests/runtime/**` · `api/openapi.yaml` 新增 `/runtimes/{name}/activate` 一节 |
| `forbidden_changes` | **探针不过时不得切换**，用户点了也不行；不删除旧版本目录；不按 runtime 名字分支；Runtime 更新**不得触发 App 重启** |
| `stop_conditions` | M0 **U0.7.1**（12 项能力探针与能力矩阵）未合入 → 停 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 探针 12/12 才允许切换 | Fake 声明 11/12，断言返回 `Problem.type == probe_failed` 且 `active_version` 未变 |
| R2 | 探针失败项被返回给前端 | 断言响应里列出失败探针的 `id` 与 `detail` |
| R3 | 有活跃会话时不立即切，会话结束后再切 | 起一个 Fake 会话 → `activate` → 断言 `active_version` 未变；结束会话 → 断言已切 |
| R4 | **不打断执行中的单元** | 断言 `activate` 期间 Fake 收到的 `session/cancel` 数 == 0 |
| R5 | 一键退回上一版本，旧目录仍在 | 切到 `0.64.1` 后退回 `0.63.0`，断言 `active_version == "0.63.0"` 且两个目录都在 |
| R6 | 设置页两种态都渲染正确 | 组件测试逐字断言 `claude-agent-acp 0.63.0 → 0.64.1` + `更新` 按钮，与 `codex-acp 1.1.7 已是最新` |
| R7 | 切换默认版本**不需要重启 App** | 集成测试断言 `activate` 前后 duetd 进程 pid 不变 |

---

## M1 验收

**全部单元 `✓` 之外，还要满足：**

| # | 标准 | 怎么验 |
|---|---|---|
| A1 | 从合一个 `feat:` PR 到 GitHub Release 上出现可下载的 `.dmg`，**全程只有一次人工点击**（合并 Release PR） | 记录整条链路的 run id；`gh release view v<X.Y.Z>` 有输出 |
| A2 | 一台**没装过 Duet 的 mac** 能装上并跑起来 | 下载 `.dmg` → 手动放行 Gatekeeper → 打开 → `GET /v1/system/version` 返回 200 |
| A3 | 已安装的旧版本能收到提示并一键更新成功 | 装 `vX.Y.Z` → 发 `vX.Y.Z+1` → 设置页出角标 → 点「一键更新并重启」→ 重启后 `/v1/system/version` 的 `version` 已变 |
| A4 | **全程零自动安装** | 未点按钮时，mock/抓包断言对安装包 URL 的请求数为 0；`grep -rn 'downloadAndInstall' frontend/src shell/src-tauri/src` 的结果全部在用户点击的回调路径上 |
| A5 | `prepare` 返回 `blocked` 时装不下去 | Fake Runtime 用 `NeverStops` 制造 `blocked`，Playwright 断言状态机走不到 `downloading` |
| A6 | 篡改安装包后客户端拒绝安装 | 重新上传一个改了 1 字节的 asset，断言 updater 报签名校验失败且不安装 |
| A7 | **minisign 私钥已离线备份并由人确认** | `open-questions.md` 的 **Q6** 已移入「已决」表，含结论、日期、保管人 |
| A8 | **issue #3 已关闭**，M1 全部 PR 的验收都在 CI 上取过证 | 逐个 PR 断言 `gh pr checks <n>` 里 `ci` 为 pass，且**没有一条依赖 `--admin`** |
| A9 | 不存在被删除重打的 tag | `git tag --list 'v*'` 与 `gh release list` 逐一对应，版本号无空洞 |
| A10 | P1 的两条开放项有结论 | `open-questions.md` 的「P1 · 挡住 M1」表为空 |
| A11 | 更新链路的全部检查本地都能跑 | `make check` 覆盖 `test-guards` / `check-version-source` / `test-make-updater-manifest` 三条（`ci.md` §6） |

**A3 + A5 是 M1 真正的验收标准。** 能发版只是过程；
**发出去的版本能被安全地装上、且不吃掉用户正在跑的工作**才是目的。

**A8 是取证纪律。** 一个没有在 CI 上跑过的验收，和没有验收是一回事。
