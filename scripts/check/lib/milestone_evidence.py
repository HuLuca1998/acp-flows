#!/usr/bin/env python3
"""检查标成「完成」的里程碑，它的完成标志都留下了实操证据。

★ 为什么要有这条检查

2026-08-08：`M2` 被标成完成，而它的完成标志第 1 条是
「『创建项目』选一个本地代码文件夹 → 项目出现在左栏」——**用户根本做不到**：
左栏是骨架占位，「创建项目」是死按钮。

我验的是第 2、3 条（说需求、看时间线），而且是**自己用 curl 建好项目之后**
才走通的。那不是用户会做的事。用户一打开应用就发现了。

根因：**单元是我的切法，完成标志是用户的切法，两者不重合。**
把七八个单元都做完并不等于那三条用户能走通——中间可能整条缺一环，
而每个单元自己的测试都是绿的。

★ 判据

里程碑文件里标了「✓ 完成」的，它的「完成标志」每一条后面必须跟一行
`> 实操：<证据>`。证据要说清**在哪验的**（界面上点的 / curl 的 / 测试跑的），
因为「curl 验过了」对一条写着「点按钮」的标志来说**不算数**。

不检查证据的真假——那要人看。它只保证**每一条都被单独回答过**，
而不是笼统一句「M2 完成」。
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

DONE_RE = re.compile(r"^#\s*(M\d+)\b.*$", re.M)
# 「完成标志」那一节里的编号条目
STEP_RE = re.compile(r"^\s*(\d+)\.\s+(.+)$", re.M)
# 「> 实操：…」或「> 实操（2026-08-08）：…」——日期是可选的，
# 但**鼓励写**：半年后回头看，「什么时候验的」比「验过」有用得多。
EVIDENCE_RE = re.compile(r"^\s*>\s*实操\s*(?:（[^）]*）|\([^)]*\))?\s*[：:]\s*(.+)$", re.M)


def section(md: str, title: str) -> str:
    """取出某一节的正文（到下一个 ## 为止）。"""
    i = md.find(f"## {title}")
    if i < 0:
        return ""
    j = md.find("\n## ", i + 3)
    return md[i : j if j > 0 else len(md)]


def milestone_is_done(root: Path, name: str) -> bool:
    """里程碑在 roadmap 里被标成完成了吗。"""
    roadmap = root / "docs" / "plan" / "roadmap.md"
    if not roadmap.exists():
        return False
    md = roadmap.read_text(encoding="utf-8")
    # roadmap 里的写法有好几种：「★ M0–M3 全部完成」「`M2` | … | 已完成」
    # 「**M2 已完成**」。★ 范围式的「M0–M3」也要认——第一版只认单个编号，
    # 于是 M2 明明标了完成却查不出来，这条检查整个失效。
    n = int(name[1:])
    if re.search(re.escape(name) + r".{0,12}?(已完成|全部完成)", md):
        return True
    for lo, hi in re.findall(r"M(\d+)\s*[–—-]\s*M(\d+).{0,12}?(?:已完成|全部完成)", md):
        if int(lo) <= n <= int(hi):
            return True
    return False


def check(root: Path) -> list[str]:
    milestones = sorted((root / "docs" / "plan" / "milestones").glob("M*.md"))
    if not milestones:
        return ["找不到 docs/plan/milestones/M*.md"]

    problems: list[str] = []
    for path in milestones:
        md = path.read_text(encoding="utf-8")
        m = DONE_RE.search(md)
        if not m:
            continue
        name = m.group(1)
        if not milestone_is_done(root, name):
            continue  # 还没标完成，不要求证据

        body = section(md, "完成标志")
        if not body:
            problems.append(f"{name} 标了完成，但文件里没有「## 完成标志」一节")
            continue

        steps = STEP_RE.findall(body)
        evidence = EVIDENCE_RE.findall(body)
        if not steps:
            problems.append(f"{name} 的「完成标志」里没有编号条目")
            continue
        if len(evidence) < len(steps):
            problems.append(
                f"{name} 有 {len(steps)} 条完成标志，只留了 {len(evidence)} 条实操证据——\n"
                "    每一条都要单独回答「用户按这条做，成不成」。\n"
                "    格式：在那一条下面加一行 `> 实操：<在哪验的，看到了什么>`\n"
                "    ★ 「单元测试全绿」不是证据：单元是我的切法，"
                "完成标志是用户的切法，两者不重合"
            )
    return problems


def main() -> int:
    root = Path(sys.argv[1] if len(sys.argv) > 1 else ".").resolve()
    problems = check(root)
    if problems:
        print("✗ 里程碑的完成标志缺实操证据：")
        for p in problems:
            print(f"  · {p}")
        return 1
    print("✓ 已完成的里程碑，完成标志都留了实操证据")
    return 0


if __name__ == "__main__":
    sys.exit(main())
