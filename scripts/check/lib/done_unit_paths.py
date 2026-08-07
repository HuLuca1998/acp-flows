"""列出「已完成（✓）的里程碑单元」里引用了但并不存在的路径。

★ **为什么是 Python 而不是 awk。**

原来是一行 awk：

    awk '/^### ✓ U/{u=$0; keep=1} /^### [○◐⊘]/{keep=0} ...'

它在 CI（GNU awk）上工作，在开发机（macOS 自带的 BSD awk）上**一条都匹配不到**——
正则里的 `✓` `○` `◐` `⊘` 都是多字节字符，BSD awk 按字节处理，匹配不上。
于是本地永远绿、只有 CI 会红。

这已经是同一族问题的第三次了：
  ① C locale 下 grep 把多字节按字节拆（pitfalls P-10）
  ② 提交信息检查的 `[:：]` 字符类（scripts/check/lib/commit_msg.py）
  ③ 这一条

**本仓库的文档与标记大量使用中文和符号，凡是要匹配它们的判定一律交给 Python。**
一个本地放行、只有 CI 才红的检查，比没有检查更糟——它让人以为验证过了。
"""

import pathlib
import re
import sys

MILESTONES = pathlib.Path("docs/plan/milestones")

# 单元标题：### <标记> U<编号>
UNIT_RE = re.compile(r"^### ([○◐✓⊘]) (U[0-9][0-9A-Za-z.]*)", re.M)
# 反引号包起来的路径
PATH_RE = re.compile(r"`([a-z][a-zA-Z0-9_/.@-]+)`")
# 只检查这些后缀——「一句话说明」里也会出现反引号包的词
CHECKED_SUFFIXES = (".go", ".ts", ".tsx", ".sh", ".yaml", ".yml", ".json", ".md")

missing: dict[str, str] = {}  # 路径 → 是哪个单元引用的

for path in sorted(MILESTONES.glob("M*.md")):
    text = path.read_text(encoding="utf-8")

    # 按单元标题切块，只留标记为 ✓ 的那些
    parts = UNIT_RE.split(text)
    # parts = [前言, 标记1, 编号1, 正文1, 标记2, 编号2, 正文2, ...]
    for i in range(1, len(parts), 3):
        mark, unit, body = parts[i], parts[i + 1], parts[i + 2]
        if mark != "✓":
            continue  # 未完成单元的前向引用是正常的

        for line in body.splitlines():
            # 只认 allowed_changes 那一行本身。★ 说明文字里提到某个路径
            # （"platform/proc.go 没有建，理由是…"）不该被算成引用——
            # 否则解释为什么不建，本身就会让检查红。
            if not line.lstrip().startswith("| `allowed_changes`"):
                continue
            for candidate in PATH_RE.findall(line):
                if "*" in candidate:
                    continue  # 通配路径不检查
                if not candidate.endswith(CHECKED_SUFFIXES):
                    continue
                if not pathlib.Path(candidate).exists():
                    missing.setdefault(candidate, f"{path.name} 的 {unit}")

for p, where in sorted(missing.items()):
    print(f"{p}  ← {where}")

sys.exit(0)  # 输出即判定，退出码交给调用方
