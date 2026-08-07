# AGENTS.md · scripts

> **就近优先**：与根 [`AGENTS.md`](../AGENTS.md) 冲突时以本文件为准。

## 负责什么

把文档里的规则变成**可执行的检查**，以及本地开发/构建的入口。

> 本仓库没有人类逐行审阅。**没有检查手段的规则等于不存在。**
> 这个目录就是让规则真正生效的地方。

| 脚本 | 干什么 | 谁调 |
|---|---|---|
| `check-agent-docs.sh` | 关键目录是否都有填实的 `AGENTS.md` + `CLAUDE.md` | `make check-docs`、CI |
| `scaffold-agent-docs.sh` | 从模板生成两份文档骨架 | `make docs-scaffold` |
| `check-util-index.sh` | 工具库 `INDEX.md` 与实际导出函数是否一致 | `make check-util-index`、CI |
| `check-test-index.sh` | 测试 `INDEX.md` 与实际测试是否一致 | `make check-test-index`、CI |
| `check-naming.sh` | 垃圾桶文件名、文件行数、Get 前缀、TS enum、越层 import | CI |
| `check-commit-msg.sh` | Conventional Commits + 必填的「先红的测试」 | CI |
| `check-coverage.sh` | 分包覆盖率门槛 | `make cover`、CI |
| `worktree.sh` | 并行工作区管理 | `make wt` / `wt-clean` |
| `gen-api.sh` | 由 `api/openapi.yaml` 生成 Go 接口与 TS 客户端 | `make gen`、CI |
| `dev-web.sh` | 起 `duetd` + `vite` | `make dev-web` |
| `build.sh` | 编 `duetd` + 前端 `dist` | `make build`、CI |
| `setup-branch-protection.sh` | 一次性配置 `main` 分支保护 | 手动 |

## 不负责什么

- **不放业务逻辑。** 脚本只做编排与校验。
- **不放长期运行的服务。** 服务是 `duetd` 的事。

## 写脚本的规则

1. **`set -euo pipefail`** 开头，一个都不能少
2. **第一行注释说清「干什么」和「谁调用」**，第二段说清退出码含义
3. **失败信息要给出修正方式**，不只报错：
   ```
   ✗ 缺少文档的关键目录：backend/internal/store
   补齐方式： make docs-scaffold DIR=backend/internal/store
   ```
4. **子项目没脚手架时跳过而不是报错**——`make check` 必须从第一天起就能跑通
5. **不依赖未安装的工具**：用到 `jq` / `yq` 之类先检测并给出安装提示
6. **加了新检查 → 同步加到 `Makefile` 和 `.github/workflows/ci.yml`**，
   只写脚本不接进 CI 等于没写

## 检查命令

```bash
shellcheck scripts/*.sh          # 有装的话
bash -n scripts/xxx.sh           # 语法检查
```

## 改这里之前必读

- 你要强制的那条规则出自哪份文档 —— 检查逻辑必须和文档一字对应，
  文档说 400 行，脚本就不能写 500

## ★ 动手写之前：先过一遍 shell 陷阱清单

```bash
grep -n '^## Shell' -A2 docs/notes/pitfalls.md      # P-01 ~ P-06
```

**P-01（`set -e` + `[[ ]] && cmd` 静默退出）在本仓库已经被踩了 4 次。**
写检查脚本时它必然出现——因为「没发现问题」正是检查脚本最常见的路径。

细节不在这里，在 [`docs/notes/pitfalls.md`](../docs/notes/pitfalls.md)（一件事只写一处）：

| 编号 | 一句话 |
|---|---|
| P-01 | `set -e` 下 `[[ ]] && cmd`，条件为假时脚本静默终止 |
| P-02 | `pipefail` + `grep` 无匹配 / `find` 权限错 → 整条管道失败 |
| P-03 | `case` 的 `)` 在 `$( )` 里造成括号失配 |
| P-04 | `find -not -path './node_modules/*'` 只排除顶层 |
| P-05 | macOS 自带 bash 是 3.2，没有 `mapfile` / `declare -A` |
| P-06 | `nohup` 继承父进程 stdout 管道 → `make` 挂住 |

### 验证方式 —— 不能省

**改完必须手动造一个负例，确认脚本真的会红。** `bash -n` 只查语法，
上面这些都是运行时行为，语法检查一个都查不出来。

> 「配置了 ≠ 生效了」在本仓库已经发生过两次（见 pitfalls P-01、P-09）。
> **一个永远绿的检查比没有检查更糟**——它会让人以为这条规则有人守。

> ⚠️ 造负例时**不要用 `git reset --hard` 收尾**（pitfalls P-07，真的丢过东西）。
> 先提交你的改动，再造负例。

---

## 本域特有的坑

- **`grep` / `sed` 的 BSD 与 GNU 行为不同。** CI 跑 Linux，本地跑 macOS，
  两边都要过——优先用 POSIX 子集，或者用 `python3`（两边都有）。
- **`find` 的 `-prune` 要放对位置**，否则会扫进 `node_modules`，慢到不可用。
