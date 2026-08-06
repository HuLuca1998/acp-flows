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

## ★ 四个反复踩的 shell 陷阱

写检查脚本时**每次都要对照这四条**。前三条我在同一个仓库里各踩过 2–3 次，
说明只写一句「注意」是不够的。

### 1. `set -e` + `[[ ]] && cmd`

```bash
set -euo pipefail
[[ -n $found ]] && report "$found"     # ✗ $found 为空时整个脚本退出
if [[ -n $found ]]; then report "$found"; fi   # ✓
```

AND 列表整体失败会触发 `set -e`。**检查脚本里「没发现问题」是最常见的路径**，
所以这个 bug 表现为「脚本永远静默退出 1」，最难排查。

### 2. `set -o pipefail` + 管道里的 `grep` / `find`

```bash
x=$(find . -name '*.go' | grep foo | sort)          # ✗ grep 无匹配返回 1 → 整脚本死
x=$(find . -name '*.go' | { grep foo || true; } | sort)   # ✓
x=$(... | sort || true)                                    # ✓ 收尾兜底
```

`grep` 无匹配、`find` 遇到无权限目录，都会返回非零。**在检查脚本里这两件事都是常态。**

### 3. `case` 的 `)` 在 `$(...)` 里让括号失衡

```bash
x=$(... | while read -r s; do
      case "$s" in *xxx*) continue ;; esac   # ✗ syntax error near `;;`
    done)

x=$(... | while read -r s; do
      [[ $s == *xxx* ]] && continue          # ✓
    done)
```

命令替换里**不要用 `case`**，用 `[[ ]]`。

### 4. `find` 剪枝要用 `-prune`

```bash
find . -not -path './node_modules/*'    # ✗ 只匹配顶层，漏掉 ./frontend/node_modules/
find . \( -name node_modules \) -prune -o -name '*.md' -print   # ✓
```

漏剪的后果很直观：几千个第三方 README 的散文被当成待检查内容。

### 验证方式

**改完必须手动造一个负例**，确认脚本真的会红。`bash -n` 只查语法，
查不出上面四条——它们都是运行时行为。

---

## 本域特有的坑

- **macOS 的 `bash` 是 3.2**，没有 `declare -A` 关联数组、没有 `${x@Q}`。
  要么用数组模拟，要么在脚本头部检测 bash 版本。
- **`grep` / `sed` 的 BSD 与 GNU 行为不同。** CI 跑 Linux，本地跑 macOS，
  两边都要过——优先用 POSIX 子集，或者用 `python3`（两边都有）。
- **`find` 的 `-prune` 要放对位置**，否则会扫进 `node_modules`，慢到不可用。
- **检查脚本自己要能被测。** 至少手动跑一次「应该失败」的场景，
  确认它真的会红——一个永远绿的检查比没有检查更糟。
