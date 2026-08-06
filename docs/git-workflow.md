# 分支、提交与协作规范

> 本仓库由 Claude 与 Codex 共同维护，**没有人类逐行审阅 diff**。
> 因此这套流程的重点不是"方便人协作"，而是：**让每一次合入都被机器和另一个 AI 检查过。**

---

## 1. 分支模型：trunk-based

只有一条长期分支。

```
main ──●──────●──────●──────●────▶   永远可发布，受保护
        ╲    ╱ ╲    ╱
         ●──●   ●──●                  短生命周期分支，squash 合入后删除
```

| | |
|---|---|
| 长期分支 | 只有 `main` |
| 特性分支寿命 | **≤ 2 天**。超过说明任务拆得太大，回去拆 |
| 合入方式 | **Squash merge**，合入后自动删分支 |
| 历史 | 线性，禁止 merge commit |
| 直接 push `main` | **禁止**（分支保护强制） |

### 分支命名

```
<type>/<scope>-<slug>
```

`<type>` 与 Conventional Commits 一致；`<scope>` 见 §2；`<slug>` 用 kebab-case 短语。

```
✓ feat/acp-session-cancel
✓ fix/store-migration-order
✓ chore/ci-add-coverage-gate
✗ dev            ✗ luca-work      ✗ feature/新功能      ✗ fix/bug
```

---

## 2. 提交信息：Conventional Commits

```
<type>(<scope>): <中文简述，祈使语气，不超过 50 字>

<正文：为什么这么改，不是改了什么——改了什么看 diff>

先红的测试: <测试名>
验证: <命令> → <退出码/关键输出>

Co-Authored-By: ...
```

### type

| type | 含义 | 影响版本号 |
|---|---|---|
| `feat` | 新功能 | **minor** |
| `fix` | 修缺陷 | **patch** |
| `perf` | 性能优化 | **patch** |
| `refactor` | 重构，外部行为不变 | 不发版 |
| `test` | 只动测试 | 不发版 |
| `docs` | 只动文档 | 不发版 |
| `build` `ci` `chore` | 构建 / CI / 杂项 | 不发版 |
| `feat!` 或正文含 `BREAKING CHANGE:` | 破坏性变更 | **major** |

### scope

固定取值，不许自创：

`acp` `domain` `app` `api` `store` `fsstore` `git` `gh` `event` `ui` `shell` `e2e` `ci` `docs` `deps`

### 「先红的测试」是必填项

铁律 1 要求测试先行。提交信息里必须写出**哪个测试先失败了**。

- `test` `docs` `chore` `ci` 类提交可写 `不适用`
- 其他 type 写 `不适用` 会被 CI 拦下（`scripts/check-commit-msg.sh`）

```
✓ 先红的测试: TestSessionCancelIsIdempotent
✓ 验证: cd backend && go test ./internal/acp/... -run TestSessionCancel → ok, 退出码 0
```

---

## 3. PR 流程

```
① 开分支  →  ② 干活（测试先行）  →  ③ 本地 make check 全绿
   →  ④ 开 PR  →  ⑤ CI 全绿  →  ⑥ 独立 AI 审查通过  →  ⑦ squash merge
```

### PR 必须包含

- 标题即 squash 后的 commit 标题，遵循 Conventional Commits
- 正文按 `.github/pull_request_template.md` 填，**不许删条目**
- 「先红的测试」与验证命令输出
- 改了 UI → 附截图或说明对应 `design/Duet Spec.dc.html` 的哪一条

### 谁能合并：实现方不得审查自己的 PR

这条直接对应产品自身的「实现审查员独立会话」机制。

| 实现方 | 审查方 |
|---|---|
| Claude | **Codex**（通过 `codex-collab` skill 发起只读审查） |
| Codex | **Claude** |
| 人 | 任一 AI |

审查方要判定四类结果之一，与产品的 `review_unit` 一致：

| 结果 | 处理 |
|---|---|
| `accepted` | 可以合并 |
| `implementation_fix` | 打回，实现方修，不改契约 |
| `contract_revision` | 需求/接口本身有问题，回去改 spec 或任务描述 |
| `global_replan` | 架构假设错误，停下来找人 |

**「CI 全绿」不等于「审查通过」。** 审查方必须实际读 diff，
并回答：这些测试真的验证了行为吗？边界有没有被悄悄扩大？

详见 [`ai-workflow.md`](ai-workflow.md)。

### 合并动作本身

合并到 `main` 会触发发版流水线，属于**对外可见的动作**。

- AI **可以**开 PR、跑 CI、做审查、把 PR 标记为 ready
- AI **合并前需要人类确认**，除非该 PR 是 `docs` / `chore` / `test` 且 CI 全绿
- `push` 到远端、发 Release、删除远端资源 → 对应产品里的 **D3**，**一律逐次授权**

---

## 4. worktree 使用规范

> 注意区分两个 worktree，别搞混：
>
> | | 谁在用 | 位置 |
> |---|---|---|
> | **开发本仓库**的并行工作区 | Claude / Codex / 你 | `.worktree/<分支名>/`（本节） |
> | **产品功能**：每个 Work 一个隔离工作区 | Duet 运行时 | `~/.duet/worktrees/<项目>/<work>/`（见 [`domain-model.md`](domain-model.md)） |

### 位置与命名

工作区一律建在仓库根下的 `.worktree/`，目录名 = 分支名去掉 `/` 后的 slug：

```bash
git worktree add .worktree/feat-acp-session-cancel -b feat/acp-session-cancel
```

**`.worktree/` 已写进 `.gitignore`，绝不入库。**
不要建在 `/tmp` 或家目录——那样 AI 下一轮就找不到它了。

### 什么时候用 worktree

| 场景 | 用不用 |
|---|---|
| 两个 AI 并行做**互不重叠**的任务 | ✅ 必须用，否则互相踩文件 |
| 一个大改动想保留主工作区随时能跑 | ✅ 用 |
| 跑长时间的构建/E2E，同时想继续改代码 | ✅ 用 |
| 单个小改动 | ❌ 直接开分支，别引入额外目录 |
| 只是想看另一个分支的代码 | ❌ `git show` / `git diff` 就够了 |

### 规则

1. **一个 worktree 一个分支一个任务。** 不要在一个 worktree 里串着做几件事。
2. **禁止跨 worktree 改文件。** 在 `.worktree/a/` 里工作时不许去改 `.worktree/b/` 或主工作区的文件——
   这是并行安全的唯一保证，破坏它就等于没隔离。
3. **每个 worktree 独立跑 `make check`。** 别假设主工作区绿了这里就绿。
4. **依赖各自安装。** Go module cache 是共享的（安全）；`node_modules` 不共享，
   新 worktree 要跑一次 `pnpm install`。
5. **合并后立刻清理**，不要留着：
   ```bash
   git worktree remove .worktree/feat-acp-session-cancel
   git worktree prune
   ```
6. **定期检查有没有僵尸工作区**：
   ```bash
   git worktree list
   ```

### 快捷命令

```bash
make wt NAME=feat/acp-session-cancel     # 建 worktree + 分支 + 装依赖
make wt-clean                            # 清理所有已合并分支的 worktree
git worktree list                        # 看当前有哪些
```

---

## 5. main 分支保护

用 `gh` 一次性配好（见 `scripts/setup-branch-protection.sh`）：

| 规则 | 值 |
|---|---|
| 要求 PR 才能合入 | 是 |
| 要求状态检查通过 | `ci / backend` `ci / frontend` `ci / docs` `ci / contract` |
| 要求分支为最新 | 是 |
| 要求线性历史 | 是 |
| 允许的合并方式 | 仅 squash |
| 合并后自动删分支 | 是 |
| 允许强推 | 否 |
| 允许删除 | 否 |

---

## 6. 版本号与发版

语义化版本 `vMAJOR.MINOR.PATCH`，**由提交信息自动推导**，任何人不得手改版本号。

```
PR squash 合入 main
      ▼
release-please 读取 main 上的 conventional commits
      ▼
自动开 / 更新一个 "chore(release): vX.Y.Z" 的 Release PR（含 CHANGELOG）
      ▼
人类合并这个 Release PR   ← 唯一需要人拍板的地方
      ▼
打 tag vX.Y.Z  →  触发构建、签名、发布 GitHub Release、生成 latest.json
      ▼
客户端检测到新版本
```

**发版这一下由人点。** 理由：发版对外可见且不可撤回，属于 D3。
其余全部自动，AI 不需要也不应该碰版本号。

完整发布与客户端更新链路见 [`release-and-update.md`](release-and-update.md)。

---

## 7. 常用命令

```bash
# 开分支
git switch -c feat/acp-session-cancel

# 提交前
make check

# 开 PR（Claude 用 gh-pr skill；也可直接 gh）
gh pr create --fill --base main

# 看 CI
gh pr checks --watch

# 合并（squash，删分支）
gh pr merge --squash --delete-branch
```

---

## 8. 禁止清单

- ✗ 直接 push `main`
- ✗ merge commit / rebase 出非线性历史
- ✗ 一个 PR 里塞多个不相关的改动（拆）
- ✗ 提交信息里写没有命令输出支撑的结论
- ✗ 自己审查自己的 PR
- ✗ 手改版本号或 CHANGELOG（由 release-please 生成）
- ✗ 为了让 CI 变绿而删测试、加 `skip`、放宽断言
      —— **这是最严重的违规**，等同于伪造验收
- ✗ 把凭据、令牌、`.env` 提交进仓库
