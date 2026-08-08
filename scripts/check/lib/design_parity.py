#!/usr/bin/env python3
"""检查设计稿里的每个界面区块都在 PARITY.md 里表过态。

★ 这条检查防的是什么

不是「实现得像不像」——那要人眼看。它防的是**根本没意识到设计稿里有这块**。

2026-08-08 用户打开应用第一句话是「为什么菜单没有项目列表和对话记录」。
左栏三块占位，只接了 Runtime 那一块，另外两块的数据后端明明有。

根因是读设计稿的方式是 grep：grep 只能找到你**已经知道要找的东西**。
不知道左栏该有项目列表，就不会去 grep 它。这条检查把「设计稿上有哪些区块」
从记忆变成机器核对——漏一块，红一次。

★ 它不检查什么

- 不检查像不像（人眼的活）
- 不检查做没做（`待实现` 是合法状态）
- **只检查有没有表态**，以及表态是否符合格式要求

「诚实地记下欠账」比「假装做完了」有用得多。
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent))
from design_regions import regions  # noqa: E402

# 状态取值，与 PARITY.md 的表头一致。
STATES = {"一致", "增强", "简化", "待实现", "偏离"}

# `待实现` 必须指向一个具体单元；`偏离` 必须写明谁批准。
UNIT_RE = re.compile(r"U\d+\.\d+\.\d+")
APPROVED_RE = re.compile(r"批准|approved|裁决")


def parse_table(md: str) -> dict[str, tuple[str, str]]:
    """解析对照表，返回 {区块名: (状态, 理由)}。"""
    out: dict[str, tuple[str, str]] = {}
    for line in md.splitlines():
        if not line.startswith("| `"):
            continue
        cells = [c.strip() for c in line.strip().strip("|").split("|")]
        if len(cells) < 4:
            continue
        # 第一列可能是「`A` / `B` / `C`」——一行覆盖几个同类区块
        names = re.findall(r"`([^`]+)`", cells[0])
        state = next((s for s in STATES if s in cells[2]), "")
        # ★ 状态列与理由列**一起**看：单元号常常写在状态旁边
        # （`**待实现** \`U5.1.1\``），只搜理由列会全军覆没——
        # 第一版就是这么写的，44 个区块报了 40 条假违规。
        reason = cells[2] + " " + cells[3]
        for n in names:
            out[n] = (state, reason)
    return out


def check(root: Path) -> list[str]:
    design = root / "design" / "ACP Duet 1a.dc.html"
    parity = root / "design" / "PARITY.md"

    if not design.exists():
        return [f"找不到设计稿：{design}"]
    if not parity.exists():
        return [
            "缺少 design/PARITY.md ——\n"
            "    设计稿上有哪些区块必须有一张机器能核对的表，"
            "不能只存在于某一次会话的记忆里"
        ]

    want = regions(design.read_text(encoding="utf-8", errors="ignore"))
    got = parse_table(parity.read_text(encoding="utf-8"))

    problems: list[str] = []

    missing = [r for r in want if r not in got]
    if missing:
        problems.append(
            "设计稿里有、PARITY.md 里没表态的区块：\n"
            + "\n".join(f"      · {m}" for m in missing)
            + "\n    每一块都要说清楚：照做了 / 简化了（理由）/ 还欠着（哪个单元）"
        )

    stale = [r for r in got if r not in want]
    if stale:
        problems.append(
            "PARITY.md 里有、设计稿里已经没有的区块：\n"
            + "\n".join(f"      · {s}" for s in stale)
            + "\n    设计稿改过了？把这几行删掉或更新"
        )

    for name, (state, reason) in sorted(got.items()):
        if state == "":
            problems.append(f"「{name}」没有合法状态（应为 {' / '.join(sorted(STATES))} 之一）")
            continue
        if state == "待实现" and not UNIT_RE.search(reason):
            problems.append(
                f"「{name}」标了待实现却没指向单元——\n"
                "    没有单元号的欠账等于没记：下一个人不知道它归谁做"
            )
        if state == "偏离" and not APPROVED_RE.search(reason):
            problems.append(
                f"「{name}」标了偏离却没写谁批准的——\n"
                "    偏离设计稿要有人拍板。没批准的话按设计稿做，"
                "把疑问记进 docs/plan/open-questions.md"
            )
        if state in ("简化", "偏离") and len(reason) < 12:
            problems.append(
                f"「{name}」的理由太短（{len(reason)} 字）——\n"
                "    要说清用户损失了什么，不是「先这样」"
            )

    return problems


def main() -> int:
    root = Path(sys.argv[1] if len(sys.argv) > 1 else ".").resolve()
    problems = check(root)
    if problems:
        print("✗ 设计稿对照表有问题：")
        for p in problems:
            print(f"  · {p}")
        print()
        print("  这条检查防的不是「实现得像不像」，是**根本没意识到设计稿里有这块**。")
        print("  规则见 design/PARITY.md 开头。")
        return 1

    md = (root / "design" / "PARITY.md").read_text(encoding="utf-8")
    got = parse_table(md)
    done = sum(1 for s, _ in got.values() if s in ("一致", "增强"))
    print(f"✓ 设计稿 {len(got)} 个区块都表过态（已实现 {done}，欠着 {len(got) - done}）")
    return 0


if __name__ == "__main__":
    sys.exit(main())
