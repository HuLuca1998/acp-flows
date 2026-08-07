#!/usr/bin/env bash
# 检查目录扇出：一个目录下平铺的文件太多时报错，逼着分包。
# 谁调用：make check-fanout、make check、CI 的 docs job。
#
# 退出码：0 = 全部目录合规；1 = 有目录超过上限。
#
# 为什么要有这条检查：平铺到一定程度之后，目录本身就不再传达任何结构信息，
# 「这个文件该放哪」失去答案，于是新文件继续往根上堆。本仓库的 scripts/
# 堆到 27 个脚本时，它自己的 AGENTS.md 索引表只列了 13 个——**文档漂移
# 就是平铺过量的第一个症状**，而那时没人发现。
#
# 规则出处：docs/rules/coding-standards.md §1.5。

set -euo pipefail

# 一个目录下**直接子文件**的数量上限（不含子目录）。
MAX_FILES=15

cd "$(dirname "$0")/../.."

python3 - "$MAX_FILES" <<'PY'
import os
import sys

max_files = int(sys.argv[1])

# 不扫的目录：依赖、产物、版本控制内部。
SKIP_DIRS = {
    ".git", "node_modules", "dist", "build", "target", "coverage",
    ".next", ".venv", "__pycache__", ".worktree", ".idea", ".vscode",
}

# 显式豁免：每条**必须写理由**。
#
# 豁免是需要解释的决定，不是默认状态。加一条之前先问：
# 真的分不动，还是只是懒得改引用？
EXEMPT = {
    # 路径: 理由
    "backend/internal/api/gen": "生成物，由 api/openapi.yaml 决定，人改不了",
    "frontend/src/api/gen": "同上",
}

violations = []
for root, dirs, files in os.walk("."):
    dirs[:] = [d for d in dirs if d not in SKIP_DIRS]
    rel = os.path.relpath(root, ".")
    if rel == ".":
        rel = ""
    if rel in EXEMPT:
        continue
    # 隐藏文件不算：它们通常是配置，不构成「这个目录在讲什么」的一部分。
    visible = [f for f in files if not f.startswith(".")]
    if len(visible) > max_files:
        violations.append((len(visible), rel or "."))

if not violations:
    print(f"✓ 没有目录的平铺文件数超过 {max_files}")
    sys.exit(0)

violations.sort(reverse=True)
print(f"✗ 下列目录平铺的文件超过 {max_files} 个，考虑分包：")
print()
for count, path in violations:
    print(f"    {count:3d} 个  {path}/")
print()
print("怎么办（按优先级）：")
print("  1. 按职责分子目录 —— 子目录名要能回答「新文件该放哪」")
print("  2. 合并职责重复的文件 —— 平铺过多常常是同一件事写了好几份")
print("  3. 确实分不动 → 在 scripts/check/check-dir-fanout.sh 的 EXEMPT 里加一条，")
print("     **并写清楚理由**。豁免是需要解释的决定，不是默认状态。")
print()
print("规则见 docs/rules/coding-standards.md §1.5")
sys.exit(1)
PY
