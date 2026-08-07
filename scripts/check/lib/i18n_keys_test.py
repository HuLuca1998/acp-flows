#!/usr/bin/env python3
"""i18n_keys 的负例集 —— 证明这条检查真的在检查。

跑法：`python3 scripts/check/lib/i18n_keys_test.py`（`make check` 会带上）。

★ 每一条都是「过去漏掉过、或者很容易漏掉」的真实写法。
一条抓不到东西的检查比没有检查更糟：它让人以为这块已经被把关了。
"""

from __future__ import annotations

import sys

from i18n_keys import has_dynamic_key, mentioned_keys, required_keys

CASES: list[tuple[str, str, set[str]]] = [
    (
        "紧跟字面量 —— 原来的正则唯一认得的形式",
        "const s = t('nav.chat')",
        {"nav.chat"},
    ),
    (
        # ★★ 真机漏过的那个：t( 后面不是字面量，原正则整个跳过，
        # 于是 page.chat.title 这七个字直接显示在界面上
        "?? 兜底里的 key",
        "t(navPage?.titleKey ?? 'page.chat.title')",
        {"page.chat.title"},
    ),
    (
        "三元里的两个 key 都要算",
        "t(starting ? 'chat.starting' : 'chat.start')",
        {"chat.starting", "chat.start"},
    ),
    (
        "查表函数的返回值当参数时，表里的 key 由 mentioned 兜住；这里只要求不误报",
        "t(problemKey(code))",
        set(),
    ),
    (
        "嵌套括号不能把扫描截断",
        "t(keyOf(fallback('a.b')) ?? 'chat.error.unknown')",
        {"a.b", "chat.error.unknown"},
    ),
    (
        "字符串里的右括号不算配对",
        "t('a.b') + f(')')",
        {"a.b"},
    ),
    (
        "注释里的示例不算数 —— 否则写文档教人怎么做反而会让检查变红",
        "// 反例：t('nav.wrong')\nconst s = t('nav.right')",
        {"nav.right"},
    ),
    (
        "不带点的字面量不是 key（'ready' 这类普通字符串）",
        "t(status === 'ready' ? 'rail.ok' : 'rail.bad')",
        {"rail.ok", "rail.bad"},
    ),
]

DYNAMIC_CASES: list[tuple[str, str, bool]] = [
    ("模板拼接", "t(`rail.state.${status}`)", True),
    ("加号拼接", "t('rail.state.' + status)", True),
    ("显式映射不算动态", "t(STATE_KEY[status] ?? 'rail.unknown')", False),
    ("注释里的反例示例不算", "// 不许写成 t(`a.${b}`)\nt('a.b')", False),
]


def main() -> int:
    failures: list[str] = []

    for name, source, want in CASES:
        got = required_keys(source)
        if got != want:
            failures.append(f"  · {name}\n      源码: {source!r}\n      得到 {sorted(got)}，想要 {sorted(want)}")

    for name, source, want in DYNAMIC_CASES:
        got = has_dynamic_key(source)
        if got != want:
            failures.append(f"  · 动态检测「{name}」: 源码 {source!r} 得到 {got}，想要 {want}")

    # mentioned 的职责与 required 不同：它认所有点分字面量，
    # 包括注册表里的 titleKey —— 那是「有人用」的证据，不是「必须存在」的要求
    if "nav.report" not in mentioned_keys("{ id: 'report', titleKey: 'nav.report' }"):
        failures.append("  · mentioned_keys 认不出注册表里的 titleKey")

    if failures:
        print("✗ i18n_keys 负例未通过：")
        print("\n".join(failures))
        return 1
    print(f"✓ i18n_keys 负例全过（{len(CASES) + len(DYNAMIC_CASES) + 1} 条）")
    return 0


if __name__ == "__main__":
    sys.path.insert(0, str(__import__("pathlib").Path(__file__).parent))
    sys.exit(main())
