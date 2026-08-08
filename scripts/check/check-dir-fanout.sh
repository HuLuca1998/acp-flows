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
    # ★ 这一条是给检查自己开的豁免，所以理由要写得比别处更硬。
    #
    # scripts/check/ 下每个 check-*.sh 都是**一个独立的检查入口**，
    # 与 Makefile 的一个 target 一一对应。分子目录要回答「新检查放哪」，
    # 而这个问题没有好答案：check-i18n 算代码类还是文档类？
    # check-commit-msg 算 git 类还是规范类？分错了比不分更难找。
    #
    # 辅助脚本已经分出去了（lib/），那才是这个目录里真正需要分的东西。
    #
    # **什么时候该重新考虑**：超过 25 个，或者出现了明显自成一族的一批
    # （比如将来有五六个 check-perf-*）。到那时再分，分法也会自然清楚。
    "scripts/check": "每个文件是一个独立检查入口，与 Makefile target 一一对应；"
                     "分子目录会让「新检查放哪」变成没有好答案的问题。超过 25 个时重新考虑",
}

# 测试文件与源文件的配对后缀，按语言。
TEST_SUFFIXES = [
    (".go", "_test.go"),
    (".ts", ".test.ts"),
    (".tsx", ".test.tsx"),
    (".py", "_test.py"),
]


def paired_test(name, siblings):
    """这个文件是不是「某个同目录源文件的测试」。"""
    for ext, test_suffix in TEST_SUFFIXES:
        if not name.endswith(test_suffix):
            continue
        source = name[: -len(test_suffix)] + ext
        return source in siblings
    return False


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

    # ★ **与源文件配对的测试文件不单独计数。**
    #
    # 这条检查防的是「职责不单一」，而 foo_test.go 讲的正是 foo.go 那件事——
    # 把它算成第二个职责的话，任何「一个文件一个测试」的目录到 8 个源文件
    # 就会红，而阈值写的是 15。那会逼人要么少写测试、要么胡乱分包，
    # 两个都比平铺糟。
    #
    # **孤立的测试文件照样算**：没有对应源文件的测试是独立的一坨，
    # 它确实在讲一件单独的事。
    counted = [f for f in visible if not paired_test(f, visible)]

    if len(counted) > max_files:
        violations.append((len(counted), rel or "."))

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
