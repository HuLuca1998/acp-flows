"""校验提交信息：Conventional Commits + 必填的「先红的测试」。

规则见 docs/rules/git-workflow.md §2。用法：

    python3 commit_msg.py <base-sha> <head-sha>

★ **为什么是 Python 而不是 shell。**

原来是 bash + `grep -qE '^先红的测试[:：]'`。那一行在开发机上**放过了
错误格式**，只有 CI 会红——本地验证因此完全失效，而这正是这类检查最不该
出的问题。

根因有两条，叠在一起：

1. 脚本跑在 C locale 下（LANG 未设），grep 按**字节**而不是字符处理。
   于是 `[:：]` 里的「：」（EF BC 9A）散成三个字节进了集合。
   而「（」是 EF BC 88——**首字节相同**，于是
   「先红的测试（4 条）：」被判成合规。
2. `[` 后面紧跟 `:` 还会撞上 POSIX 字符类语法（`[[:alpha:]]` 那套），
   成了畸形表达式，BSD grep 与 GNU grep 处理方式不同。

这与 pitfalls P-10（`$var` 紧跟中文被吞）同源：**C locale 下的多字节按字节处理**。
本仓库的注释和文案里中文是常态，所以凡是要匹配中文的地方一律交给 Python，
不写进 shell 正则。
"""

import re
import subprocess
import sys

TYPES = ["feat", "fix", "perf", "refactor", "test", "docs", "build", "ci", "chore"]
# ★ 加 scope 时**两处一起改**：这张表和 docs/rules/git-workflow.md 的取值表。
# 2026-08-08 发现文档那份漏了 plan 与 skills——脚本先加、文档没跟上，
# 于是照文档写提交的人会被脚本拦下，而他查文档查不出原因。
SCOPES = [
    "acp", "domain", "app", "api", "store", "fsstore", "git", "gh",
    "event", "ui", "shell", "e2e", "ci", "docs", "design", "plan", "skills", "deps",
]
# 这些 type 的提交可以写「不适用」
NO_TEST_TYPES = {"test", "docs", "chore", "ci", "build"}

# scope 允许逗号分隔的多个（feat(domain,store)）：一个真跨两个域的改动，
# 强迫它选一个会选出误导性的那个。但每一段都必须来自固定取值表。
_scope = f"(?:{'|'.join(SCOPES)})"
SUBJECT_RE = re.compile(rf"^(?:{'|'.join(TYPES)})(?:\({_scope}(?:,{_scope})*\))?!?: .+")
TYPE_RE = re.compile(rf"^({'|'.join(TYPES)})")

# 半角或全角冒号都认；冒号必须**紧跟**在「先红的测试」后面。
# 写成「先红的测试（4 条）：」不算——那样正文里就没有一个稳定的锚点了。
RED_TEST_RE = re.compile(r"^先红的测试[:：]", re.M)
RED_TEST_EMPTY_RE = re.compile(r"^先红的测试[:：]\s*(不适用|无|N/?A|none)\s*$", re.M | re.I)

RELEASE_RE = re.compile(r"^chore\(release\)")


def git(*args: str) -> str:
    return subprocess.run(
        ["git", *args], capture_output=True, text=True, check=True
    ).stdout.rstrip("\n")


def main() -> int:
    base = sys.argv[1] if len(sys.argv) > 1 else ""
    head = sys.argv[2] if len(sys.argv) > 2 else "HEAD"
    if not base:
        try:
            base = git("rev-parse", "--verify", "--quiet", "origin/main")
        except subprocess.CalledProcessError:
            base = git("rev-parse", "HEAD~1")

    shas = [s for s in git("rev-list", f"{base}..{head}").splitlines() if s]
    failed = False

    for sha in shas:
        subject = git("log", "-1", "--format=%s", sha)
        body = git("log", "-1", "--format=%b", sha)
        short = sha[:7]

        if RELEASE_RE.match(subject):  # release-please 生成的提交豁免
            continue

        if not SUBJECT_RE.match(subject):
            failed = True
            print(f"✗ {short} 标题不符合 Conventional Commits:")
            print(f"    {subject}")
            print("    格式: <type>(<scope>): <简述>")
            print(f"    type:  {'|'.join(TYPES)}")
            print(f"    scope: {'|'.join(SCOPES)}")
            print()
            continue

        commit_type = TYPE_RE.match(subject).group(1)
        if commit_type in NO_TEST_TYPES:
            continue

        if not RED_TEST_RE.search(body):
            failed = True
            print(f"✗ {short} 缺少「先红的测试」行（铁律 1，见 git-workflow.md §2）:")
            print(f"    {subject}")
            print("    请在正文加一行：先红的测试: TestXxx")
            print("    ★ 冒号必须紧跟在「先红的测试」后面——"
                  "写成「先红的测试（3 条）：」不算，正文里就没有稳定锚点了")
            print()
            continue

        if RED_TEST_EMPTY_RE.search(body):
            failed = True
            print(f"✗ {short} 「先红的测试」写了「不适用」，"
                  f"但 type={commit_type} 要求测试先行:")
            print(f"    {subject}")
            print()

    if failed:
        return 1
    print(f"✓ 提交信息检查通过（{len(shas)} 条）")
    return 0


if __name__ == "__main__":
    sys.exit(main())
