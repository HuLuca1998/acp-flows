#!/usr/bin/env python3
"""检查 `make check` 是 CI 的超集。

★ 为什么要有这条检查

2026-08-08 撞到的：本地 `make check` 全绿，推上去 CI 的 guard 红了——
`check-naming.sh` 只在 CI 里跑，`make check` 的依赖链里没有它。

这类漏洞的代价不是「多跑一次」：它让 `make check` 这句承诺失效。
下一个 AI 看到本地全绿就开 PR，然后在 CI 上撞墙、回来改、再推——
每一轮都是几分钟，而且它会开始怀疑本地检查到底管不管用。

所以规则是：**CI 里跑的每一条检查，`make check` 里都要有。**
反过来不要求——本地可以比 CI 更严（比如跑得慢的真机验证）。

★ 判定方式

从 `.github/workflows/ci.yml` 里提取所有 `scripts/check/*.sh` 与 `make <目标>`，
再从 Makefile 里展开 `check` 目标的**传递依赖**，比对。

不解析 shell 语义，只做文本提取——够用，且不会因为一个复杂的 if 判断失灵。
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

# CI 里这些不是「检查」，不要求 make check 覆盖。
#
# build / gen 是产物构建；test-e2e 要起真服务，本地按需跑。
EXEMPT_TARGETS = {
    "build",
    "build-app",
    "gen",
    "test-e2e",
    "test-e",  # 提取时被 `test-e2e` 的正则截断过，一并豁免
}

SCRIPT_RE = re.compile(r"scripts/check/([a-z0-9-]+)\.sh")
MAKE_RE = re.compile(r"\bmake\s+([a-z][a-z0-9-]*)")

# ★ CI 里**直接跑的外部工具**（不经 make、不经 scripts/check）。
#
# 这一类原来整个被漏掉：contract job 直接 `npx redocly lint`，
# 而 make check 里没有对应的目标——本地全绿，推上去 contract 红。
# 撞过一次（把两个 schema 放进了 components/responses）。
#
# 键是识别用的关键词，值是 make check 里必须存在的目标。
DIRECT_TOOLS = {
    "redocly": "check-spec",
}


def ci_checks(ci_yml: Path) -> tuple[set[str], set[str]]:
    """返回 CI 里跑的 (检查脚本名, make 目标名)。

    直接跑的外部工具按 DIRECT_TOOLS 折算成它应该对应的 make 目标。
    """
    text = ci_yml.read_text(encoding="utf-8")
    targets = set(MAKE_RE.findall(text)) - EXEMPT_TARGETS
    for keyword, target in DIRECT_TOOLS.items():
        if keyword in text:
            targets.add(target)
    return set(SCRIPT_RE.findall(text)), targets


def make_targets(makefile: Path) -> dict[str, list[str]]:
    """解析 Makefile 的目标 → 依赖列表。"""
    out: dict[str, list[str]] = {}
    for line in makefile.read_text(encoding="utf-8").splitlines():
        m = re.match(r"^([a-z][a-z0-9-]*):([^=]*)$", line)
        if not m:
            continue
        deps = m.group(2).split("##")[0].split()
        out[m.group(1)] = deps
    return out


def expand(targets: dict[str, list[str]], root: str) -> set[str]:
    """展开一个目标的传递依赖，含它自己。"""
    seen: set[str] = set()
    stack = [root]
    while stack:
        cur = stack.pop()
        if cur in seen:
            continue
        seen.add(cur)
        stack.extend(targets.get(cur, []))
    return seen


def scripts_of(targets: dict[str, list[str]], names: set[str], makefile: Path) -> set[str]:
    """找出这些 make 目标的配方里调用了哪些 scripts/check/*.sh。"""
    text = makefile.read_text(encoding="utf-8")
    found: set[str] = set()
    current: str | None = None
    for line in text.splitlines():
        m = re.match(r"^([a-z][a-z0-9-]*):", line)
        if m:
            current = m.group(1)
            continue
        if current in names:
            found |= set(SCRIPT_RE.findall(line))
    return found


def main() -> int:
    root = Path(sys.argv[1] if len(sys.argv) > 1 else ".").resolve()
    makefile = root / "Makefile"
    ci_yml = root / ".github" / "workflows" / "ci.yml"

    if not makefile.exists() or not ci_yml.exists():
        print("✗ 找不到 Makefile 或 .github/workflows/ci.yml")
        return 1

    targets = make_targets(makefile)
    if "check" not in targets:
        print("✗ Makefile 里没有 check 目标")
        return 1

    covered_targets = expand(targets, "check")
    covered_scripts = scripts_of(targets, covered_targets, makefile)

    want_scripts, want_targets = ci_checks(ci_yml)

    missing_scripts = sorted(want_scripts - covered_scripts)
    missing_targets = sorted(want_targets - covered_targets)

    if not missing_scripts and not missing_targets:
        print(f"✓ make check 覆盖了 CI 的全部检查（{len(want_scripts)} 个脚本 + {len(want_targets)} 个目标）")
        return 0

    print("✗ CI 里跑了、而 `make check` 没跑的检查：")
    for name in missing_scripts:
        print(f"    scripts/check/{name}.sh")
    for name in missing_targets:
        print(f"    make {name}")
    print()
    print("  `make check` 必须是 CI 的超集 —— 否则本地全绿推上去照样红，")
    print("  而下一个人会开始怀疑本地检查到底管不管用。")
    print("  修法：把它加进 Makefile 的 check 依赖链；")
    print("  确实不该本地跑的（要起真服务之类），加进本文件的 EXEMPT_TARGETS 并写明理由。")
    return 1


if __name__ == "__main__":
    sys.exit(main())
