#!/usr/bin/env bash
# 校验提交信息：Conventional Commits + 必填的「先红的测试」。
# 规则见 docs/rules/git-workflow.md §2。
#
#   scripts/check/check-commit-msg.sh <base-sha> <head-sha>
#   scripts/check/check-commit-msg.sh                        # 校验 HEAD 相对 origin/main
#
# ★ 判定逻辑在 lib/commit_msg.py，**不在这里**。
# 原来是 bash + grep，而它在开发机上会**放过错误格式**、只有 CI 才红——
# 本地验证因此失效。根因是 C locale 下 grep 按字节处理中文（与 pitfalls P-10 同源）。
# 详见那个 python 文件顶部的说明。
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

exec python3 "$(dirname "$0")/lib/commit_msg.py" "$@"
