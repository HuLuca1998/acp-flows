#!/usr/bin/env python3
"""从前端源码里提取 i18n key 的用法。

★ 为什么单独成一个模块

原来这段逻辑写在 `check-i18n.sh` 的内嵌 Python 里，用的正则是
`\\bt\\(\\s*['\"]([\\w.]+)['\"]` —— 只认 `t('key')` 这种**紧跟字面量**的形式。

于是 `t(navPage?.titleKey ?? 'page.chat.title')` 整个被漏掉，
而那个 key 在词条表里并不存在。检查绿着，界面上直接显示出
**`page.chat.title` 七个字**。真机走查才发现。

单独成模块是为了能对它造负例（见 `i18n_keys_test.py`）——
一条抓不到东西的检查，比没有检查更糟：它让人以为这块已经被把关了。
"""

from __future__ import annotations

import re

# 形如 a.b / a.b.c 的字面量才可能是 key。
# 不带点的（'ready' 'zh-CN'）排除掉：它们绝大多数是普通字符串。
KEYISH = re.compile(r"""['"]([a-zA-Z0-9_]+(?:\.[a-zA-Z0-9_]+)+)['"]""")

# t( 的调用点。\b 防止匹配到 format( / split( 之类的结尾。
T_CALL = re.compile(r"\bt\(")

# 动态拼 key：t('a.' + x) / t(`a.${x}`)
DYNAMIC = re.compile(r"""\bt\(\s*(['"][^'"]*['"]\s*\+|`[^`]*\$\{)""")


def strip_comments(source: str) -> str:
    """去掉注释行。

    注释里的反例示例（`t(\\`error.${code}\\`)` 这种教学用法）不是违规。
    不跳过的话，写文档教下一个人怎么做反而会让检查变红。
    """
    return "\n".join(
        ln
        for ln in source.splitlines()
        if not ln.lstrip().startswith(("//", "*", "/*"))
    )


def call_args(source: str, open_paren: int) -> str:
    """取 `t(` 的实参文本，从左括号扫到配对的右括号。

    引号里的括号不算数——`t('a (b)')` 的括号在字符串里。
    """
    depth = 0
    quote = ""
    i = open_paren
    while i < len(source):
        ch = source[i]
        if quote:
            if ch == "\\":
                i += 2
                continue
            if ch == quote:
                quote = ""
        elif ch in "'\"`":
            quote = ch
        elif ch == "(":
            depth += 1
        elif ch == ")":
            depth -= 1
            if depth == 0:
                return source[open_paren + 1 : i]
        i += 1
    # 括号没配上（多半是被上面的行过滤截断了）：把剩下的都算进来，
    # 宁可多要求几个 key，也不要静默漏掉
    return source[open_paren + 1 :]


def required_keys(source: str) -> set[str]:
    """`t(...)` 里出现的全部点分字面量 —— 这些**必须**存在于词条表。

    ★ 取的是整个实参，不是「紧跟在 t( 后面的那个字面量」：
    `t(cond ? 'a.b' : 'c.d')` 与 `t(x?.key ?? 'a.b')` 都要算上。
    漏掉的表现是 key 原样显示在界面上。
    """
    text = strip_comments(source)
    out: set[str] = set()
    for m in T_CALL.finditer(text):
        args = call_args(text, m.end() - 1)
        out |= set(KEYISH.findall(args))
    return out


def mentioned_keys(source: str) -> set[str]:
    """源码里出现过的全部点分字面量 —— 只用来判断「这个 key 有人用」。

    ★ 不能反过来要求它们必须存在于词条表：`'application/json'`、
    `'ph-caret-right'` 这类字符串会被误判成缺失的词条。
    """
    return set(KEYISH.findall(strip_comments(source)))


def has_dynamic_key(source: str) -> bool:
    """是否有动态拼接的 key。静态分析对它们无能为力，一律要求改成显式映射。"""
    return DYNAMIC.search(strip_comments(source)) is not None
