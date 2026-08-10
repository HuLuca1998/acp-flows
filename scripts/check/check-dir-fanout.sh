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
    # 设计系统原语库：frontend-guide.md §7 的组件清单写明**一文件一组件 +
    # 同名 module.css 平铺在 src/ui/**，全表二十多个原语，做完必然超 15。
    # 「新文件放哪」有明确答案（照 §7 的表），分子目录反而要发明
    # Tag 算「标签类」还是「文本类」这种没有对错的分组。
    "frontend/src/ui": "设计系统原语，§7 规定一文件一组件平铺；清单本身就有 20+ 个",
    # app/work 的每个文件都是 *Service 的方法集——Go 不允许方法跨包定义，
    # 「分子目录」对它结构上不可行；按 400 行上限合并又会造出巨型文件。
    # 「新文件放哪」有明确答案：按用例名（say / plan / unit / cancel / accept…）。
    "backend/internal/app/work": "全是 *Service 方法，无法分包；一用例一文件，新文件按用例名放",
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
    # ★ HTTP 边界层：一个文件 = openapi 的一个资源族（works / projects /
    # memories / skills / roles / runtimes / events / permission / resume /
    # update），「新端点放哪」有唯一答案——看它属于哪个资源。
    #
    # 分子目录要先把 writeProblem / writeJSON / Config / NewRouter 提成一个
    # 新包，而那个包除了「被所有人 import」之外没有身份——它不回答
    # 「这是什么」，只回答「谁需要它」。那不是分包，那是把耦合改个名字。
    #
    # **什么时候该重新考虑**：某个资源族自己长到三四个文件（那时它自成一包，
    # 比如 api/work/），或总数超过 22。到那时分法也会自然清楚。
    # ★ 领域模型：一个文件 = 一个聚合或值对象（work / plan / subplan /
    # requirement / unit_contract / evidence / memory / skill / role / project…），
    # 「新聚合放哪」有唯一答案——新建一个同名文件。
    #
    # **分不动的原因是它们互相引用**：`Unit` 校验角色要 `RoleByID`，
    # `CriteriaCoverage` 要 `Criterion`，`PlanVersion` 装 `Subplan`。
    # 拆成「一个包一个聚合」会立刻循环依赖；拆成「聚合包 + 共享包」的话，
    # 那个共享包会装下所有互相引用的类型——也就是大部分，
    # 而剩下的几个孤岛不值得一个新包。
    #
    # **什么时候该重新考虑**：出现一族**互不引用**的类型（比如将来的
    # 报表统计值对象），或总数超过 25。
    # ★ 前端模型：一个文件 = openapi 的一个 schema 族，内容是三五行的
    # 重导出（`export type Work = components['schemas']['Work']`）。
    # 「新类型放哪」有唯一答案——新建一个与 schema 同名的文件。
    #
    # **合并成一个 `models/index.ts` 可能更好**（16 个三行文件 → 一个 60 行
    # 文件），但那会让 `from '@/models/work'` 变成 `from '@/models'`，
    # 而「这个类型属于哪个资源」就从 import 语句里消失了。
    # 改动面也是全仓库的 import。
    #
    # **什么时候该重新考虑**：超过 22 个，或出现一个文件装多个 schema 的情况
    # （那说明「一个文件一个 schema」这条规矩已经名存实亡）。
    # ★ 迁移文件天然是一条**递增序列**：`0001_init.sql` → `0015_add_decisions.sql`。
    # 「新迁移放哪」有唯一答案——下一个编号。
    #
    # 分子目录（按年份？按模块？）只会让「下一个编号是几」变成要先翻两层
    # 目录才答得出的问题，而编号连续正是这套机制的全部依据。
    #
    # **什么时候该重新考虑**：从来不需要——这个目录只会一直增长，
    # 而它的组织方式是编号，不是层级。这条豁免是永久的。
    "backend/internal/store/migration": "迁移是一条递增序列，「新的放哪」= 下一个编号；"
                                        "分层只会让「下一个编号是几」变得难答。永久豁免",
    "frontend/src/models": "一个文件 = openapi 的一个 schema 族（三五行重导出），"
                           "「新类型放哪」有唯一答案；合并会让「属于哪个资源」从 import 里消失。"
                           "超过 22 个时重新考虑",
    "backend/internal/domain/model": "一个文件 = 一个聚合或值对象，「新聚合放哪」有唯一答案；"
                                     "它们互相引用（Unit→RoleByID、Coverage→Criterion），"
                                     "分包会立刻循环依赖。出现互不引用的一族、或超过 25 个时重新考虑",
    "backend/internal/api": "一个文件 = openapi 的一个资源族，「新端点放哪」有唯一答案；"
                            "分子目录要先把 Problem/JSON/Config 提成一个没有身份的公共包。"
                            "某个资源族长到三四个文件、或总数超过 22 时重新考虑",
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
