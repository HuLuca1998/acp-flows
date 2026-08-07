#!/usr/bin/env bash
# 校验提交信息：Conventional Commits + 必填的「先红的测试」。
# 规则见 docs/rules/git-workflow.md §2。
#
#   scripts/check/check-commit-msg.sh <base-sha> <head-sha>
#   scripts/check/check-commit-msg.sh                        # 校验 HEAD 相对 origin/main
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

base="${1:-}"
head="${2:-HEAD}"
if [[ -z $base ]]; then
  base=$(git rev-parse --verify --quiet origin/main || git rev-parse HEAD~1)
fi

TYPES='feat|fix|perf|refactor|test|docs|build|ci|chore'
SCOPES='acp|domain|app|api|store|fsstore|git|gh|event|ui|shell|e2e|ci|docs|plan|skills|deps'
# 这些 type 的提交可以写「不适用」
NO_TEST_TYPES='test|docs|chore|ci|build'

fail=0

while read -r sha; do
  [[ -z $sha ]] && continue
  subject=$(git log -1 --format=%s "$sha")
  body=$(git log -1 --format=%b "$sha")
  short="${sha:0:7}"

  # release-please 生成的提交豁免
  [[ $subject =~ ^chore\(release\) ]] && continue

  # scope 允许逗号分隔的多个（feat(domain,store)）：
  # 一个真跨两个域的改动，强迫它选一个会选出误导性的那个。
  # 但每一段都必须来自固定取值表，不许自创。
  if [[ ! $subject =~ ^($TYPES)(\((($SCOPES)(,($SCOPES))*)\))?!?:\ .+ ]]; then
    fail=1
    echo "✗ $short 标题不符合 Conventional Commits:"
    echo "    $subject"
    echo "    格式: <type>(<scope>): <简述>"
    echo "    type:  $TYPES"
    echo "    scope: $SCOPES"
    echo
    continue
  fi

  type="${BASH_REMATCH[1]}"

  if [[ ! $type =~ ^($NO_TEST_TYPES)$ ]]; then
    if ! grep -qE '^先红的测试[:：]' <<<"$body"; then
      fail=1
      echo "✗ $short 缺少「先红的测试」行（铁律 1，见 git-workflow.md §2）:"
      echo "    $subject"
      echo "    请在正文加一行：先红的测试: TestXxx"
      echo
      continue
    fi
    if grep -qE '^先红的测试[:：] *(不适用|无|N/?A|none) *$' <<<"$body"; then
      fail=1
      echo "✗ $short 「先红的测试」写了「不适用」，但 type=$type 要求测试先行:"
      echo "    $subject"
      echo
    fi
  fi
done < <(git rev-list "$base..$head")

[[ $fail -eq 1 ]] && exit 1
echo "✓ 提交信息检查通过"
