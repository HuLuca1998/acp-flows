#!/usr/bin/env python3
"""从设计稿里抽出「界面上有哪些区块」。

★ 为什么要有这个

2026-08-08 用户打开应用第一句话是「为什么菜单没有项目列表和对话记录」。
查下来：左栏三块占位，我只接了 Runtime 那一块，另外两块的数据后端明明有。

**根因不是没读设计稿，是读它的方式是 grep。** 要做工具调用卡片就
grep「工具调用」，要做权限卡片就 grep「请求写入」——找到那一段照着做。

grep 只能找到**我已经知道要找的东西**。我不知道左栏该有项目列表，
所以不会去 grep「项目列表」。这是检索方式的结构性盲区，
而不是态度问题——换一个 AI 来做，同样会漏。

这个脚本把「设计稿上到底有哪些区块」从**我的记忆**变成**机器能列出的清单**。
有了清单，`design/PARITY.md` 才能逐条对照，`check-design-parity` 才能验证
「每一块都表过态」。

★ 抽取判据

设计稿里的区块标题有统一样式：`text-transform:uppercase` + `letter-spacing`。
那是设计系统里「小标题」的写法，一眼能和正文区分开。

不追求 100% 精确——漏抽一两个不致命（人还会看），
**误抽**才麻烦（会逼人为一个不存在的区块写理由）。所以判据取严不取宽。
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

# 区块标题：设计系统里 uppercase + letter-spacing 的那种小标题。
TITLE_RE = re.compile(r"text-transform:\s*uppercase[^>]*>([^<]{1,24})<")

# 这些不是「界面区块」，是数据示例或装饰，抽出来只会制造噪音。
#
# ★ 每条都要能说清**为什么它不是区块**——不能因为「懒得填」就加进来。
NOT_A_REGION = {
    # 文件树里的目录名，是示例数据不是区块
    "assets/", "references/", "scripts/",
    # 带具体编号的实例，同一个区块的不同数据（如「引用 · 3」与「引用 · 5」）
    # 由下面的 strip_count 归一化，这里不用列
}


def strip_count(title: str) -> str:
    """去掉标题里的计数与实例编号，只留区块本身。

    「引用 · 3」「GitHub 账号 · 2」「work-08 · plan v5」都是同一类——
    带的数字是**示例数据**，不是区块的一部分。不归一化的话，
    设计稿里换个数字就会被当成新区块。
    """
    # 「X · 数字」→ X
    title = re.sub(r"\s*·\s*\d+\s*$", "", title)
    # 「X · plan vN」这种实例标题，取前半
    title = re.sub(r"\s*·\s*(plan|attempt)\s*v?\d+\s*$", "", title, flags=re.I)
    return title.strip()


def regions(design_html: str) -> list[str]:
    """列出设计稿里的全部界面区块，去重并排序。"""
    out: set[str] = set()
    for raw in TITLE_RE.findall(design_html):
        title = strip_count(raw.strip())
        if not title or title in NOT_A_REGION:
            continue
        # 纯数字、纯符号不是区块名
        if not re.search(r"[一-鿿A-Za-z]", title):
            continue
        out.add(title)
    return sorted(out)


def main() -> int:
    root = Path(sys.argv[1] if len(sys.argv) > 1 else ".").resolve()
    design = root / "design" / "ACP Duet 1a.dc.html"
    if not design.exists():
        print(f"✗ 找不到设计稿：{design}")
        return 1

    found = regions(design.read_text(encoding="utf-8", errors="ignore"))
    for r in found:
        print(r)
    print(f"\n共 {len(found)} 个区块", file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
