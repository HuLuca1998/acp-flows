#!/usr/bin/env bash
# 校验测试索引与实际测试一致。
#   · 有测试但没登记 → 失败
#   · 登记了但测试已删除 → 失败
#
# 目的：挡住"不同轮次的 AI 各写一个功能相同的测试"。
# 规则见 docs/testing-strategy.md §8。
set -euo pipefail

cd "$(dirname "$0")/.."

fail=0

report() {
  local scope=$1 kind=$2; shift 2
  if [[ $# -gt 0 ]]; then
    fail=1
    echo "✗ [$scope] $kind:"
    printf '    %s\n' "$@"
    echo
  fi
}

# 从索引表第一列抓登记项： | `Name` | ...
index_entries() {
  local idx=$1
  [[ -f $idx ]] || return 0
  # 只取表格数据行，忽略 HTML 注释里的示例。
  # grep 无匹配时返回 1，pipefail 会把它放大成整脚本退出，故收尾 || true
  sed '/^<!--/,/-->/d' "$idx" \
    | { grep -oE '^\| *`[^`]+` *\|' || true; } \
    | sed 's/^| *`//; s/` *|$//' \
    | sort -u
}

# ── 后端 Go ───────────────────────────────────────────────────
go_idx="backend/tests/INDEX.md"
if [[ -d backend ]] && find backend -name '*_test.go' -print -quit | grep -q .; then
  if [[ ! -f $go_idx ]]; then
    echo "✗ 缺少测试索引: $go_idx"; fail=1
  else
    actual=$(find backend -name '*_test.go' -not -path '*/node_modules/*' -exec \
      grep -hoE '^func +Test[A-Za-z0-9_]*' {} + 2>/dev/null \
      | awk '{print $2}' | sort -u || true)
    listed=$(index_entries "$go_idx")
    report backend "有测试但未登记进 INDEX.md" $(comm -23 <(echo "$actual") <(echo "$listed"))
    report backend "INDEX.md 里登记了但测试已不存在" $(comm -13 <(echo "$actual") <(echo "$listed"))
  fi
fi

# ── 前端 Vitest（按文件登记）──────────────────────────────────
ts_idx="frontend/tests/INDEX.md"
if [[ -d frontend/src ]] && find frontend/src -name '*.test.ts*' -print -quit | grep -q .; then
  if [[ ! -f $ts_idx ]]; then
    echo "✗ 缺少测试索引: $ts_idx"; fail=1
  else
    actual=$(find frontend/src -name '*.test.ts' -o -name '*.test.tsx' \
      | xargs -n1 basename | sort -u)
    listed=$(index_entries "$ts_idx")
    report frontend "有测试文件但未登记" $(comm -23 <(echo "$actual") <(echo "$listed"))
    report frontend "登记了但测试文件已不存在" $(comm -13 <(echo "$actual") <(echo "$listed"))
  fi
fi

# ── E2E（按 spec 文件登记）────────────────────────────────────
e2e_idx="e2e/INDEX.md"
if [[ -d e2e ]] && find e2e -name '*.spec.ts' -print -quit | grep -q .; then
  if [[ ! -f $e2e_idx ]]; then
    echo "✗ 缺少测试索引: $e2e_idx"; fail=1
  else
    actual=$(find e2e -name '*.spec.ts' | xargs -n1 basename | sort -u)
    listed=$(index_entries "$e2e_idx")
    report e2e "有 spec 但未登记" $(comm -23 <(echo "$actual") <(echo "$listed"))
    report e2e "登记了但 spec 已不存在" $(comm -13 <(echo "$actual") <(echo "$listed"))
  fi
fi

if [[ $fail -eq 1 ]]; then
  echo "修正方式：更新对应的 INDEX.md（见 docs/testing-strategy.md §8）"
  echo "注意：如果新测试与已登记的测试实质重复，应该合并而不是补登记。"
  exit 1
fi

echo "✓ 测试索引与实际测试一致"
