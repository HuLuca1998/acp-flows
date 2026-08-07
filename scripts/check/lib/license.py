#!/usr/bin/env python3
"""检查许可协议没有被悄悄改掉。

★ 为什么要有这条检查

协议文本是**法律文件**，不是文档。删掉一节、改一个词，协议就不再是
AGPL-3.0 了，而别人是靠协议名判断自己能做什么的。这类改动在 diff 里
毫不起眼，review 时最容易放过。

AGPL 尤其要盯住第 13 条（Remote Network Interaction）——它是 AGPL 相对
GPL 的**全部意义所在**：没有它，公司可以把改造版做成 SaaS 对外收费而
永远不算「分发」，因此不必开源。删掉它，这个仓库就等于降级成了 GPL。

★ 为什么不联网比对

联网检查会在没网的机器上红，那种红是噪音，久了大家就学会忽略它。
这里改成校验**结构完整性**：条款编号齐全、关键条款在、README 说法一致。
真要换协议，得同时改掉这条检查——那是显式动作，不是顺手一改。
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

# AGPL-3.0 正文的 17 条，官方文本里形如 "  1. Source Code."
REQUIRED_CLAUSES = 17

# 少了就不再是 AGPL 的关键条款。第 13 条是重中之重，见模块开头的说明。
REQUIRED_PHRASES = [
    ("GNU AFFERO GENERAL PUBLIC LICENSE", "协议标题"),
    ("Version 3, 19 November 2007", "版本与日期"),
    ("13. Remote Network Interaction", "★ 第 13 条 —— AGPL 区别于 GPL 的全部意义"),
    ("Notwithstanding any other provision of this License", "第 13 条的实质条款"),
    ("modified version", "衍生版本的义务"),
    ("15. Disclaimer of Warranty", "免责声明"),
    ("16. Limitation of Liability", "责任限制"),
]

LICENSE_URL = "LICENSE"


def check(root: Path) -> list[str]:
    problems: list[str] = []

    path = root / "LICENSE"
    if not path.exists():
        return ["缺少 LICENSE —— 没有协议的仓库，别人默认「保留所有权利」，谁都不敢用"]

    text = path.read_text(encoding="utf-8")

    for phrase, why in REQUIRED_PHRASES:
        if phrase not in text:
            problems.append(
                f"LICENSE 里找不到 {phrase!r}（{why}）。\n"
                "    协议被裁过就不再是 AGPL-3.0，而别人是靠协议名判断自己能做什么的。"
            )

    # 条款编号连续：1..17 一个都不能少
    numbered = {int(n) for n in re.findall(r"^ +(\d+)\. [A-Z]", text, re.MULTILINE)}
    missing = [n for n in range(1, REQUIRED_CLAUSES + 1) if n not in numbered]
    if missing:
        problems.append(
            "LICENSE 缺少条款：" + "、".join(f"第 {n} 条" for n in missing) + "\n"
            f"    AGPL-3.0 正文共 {REQUIRED_CLAUSES} 条，少一条就不是它了。"
        )

    readme = root / "README.md"
    if not readme.exists():
        return problems

    rt = readme.read_text(encoding="utf-8")
    if "AGPL-3.0" not in rt:
        problems.append(
            "README.md 的「许可」一节没写明 AGPL-3.0。\n"
            "    两处说法不一致时，用户会按 README 理解 —— 而有效的是 LICENSE。"
        )
    if not re.search(r"Copyright ©\s*\d{4}\s+\S", rt):
        problems.append(
            "README.md 里没有「Copyright © <年份> <版权人>」。\n"
            "    不写的话，想商谈商业授权的人不知道该找谁。"
        )

    return problems


def main() -> int:
    root = Path(sys.argv[1] if len(sys.argv) > 1 else ".").resolve()
    problems = check(root)
    if problems:
        print("✗ 许可协议检查未通过：")
        for p in problems:
            print(f"  · {p}")
        return 1
    print("✓ 许可协议完整（AGPL-3.0）")
    return 0


if __name__ == "__main__":
    sys.exit(main())
