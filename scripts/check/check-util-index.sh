#!/usr/bin/env bash
# 校验工具库索引与实际导出的函数一致。
#   · 导出了但没登记 → 失败
#   · 登记了但代码里已不存在 → 失败
#
# 索引是被校验的清单，不是文档。规则见 docs/rules/coding-standards.md §1.4。
set -euo pipefail

cd "$(dirname "$0")/../.."

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

# 从 INDEX.md 第一列抓出登记的函数名： | `Name` | ...
# 跳过 HTML 注释块——里面是格式示例，不是真实登记项。
index_names() {
  local idx=$1
  [[ -f $idx ]] || return 0
  # grep 无匹配时返回 1，pipefail 会把它放大成整脚本退出，故收尾 || true
  sed '/^<!--/,/-->/d' "$idx" \
    | { grep -oE '^\| *`[A-Za-z_][A-Za-z0-9_]*` *\|' || true; } \
    | tr -d '|` ' | sort -u
}

# ── Go ────────────────────────────────────────────────────────
go_dir="backend/internal/util"
go_idx="$go_dir/INDEX.md"
if [[ -d $go_dir ]]; then
  if [[ ! -f $go_idx ]]; then
    echo "✗ 缺少工具库索引: $go_idx"; fail=1
  else
    exported=$(find "$go_dir" -maxdepth 1 -name '*.go' ! -name '*_test.go' -exec \
      grep -hoE '^func +\(?[^)]*\)? *([A-Z][A-Za-z0-9_]*)[[(]' {} + 2>/dev/null \
      | grep -oE '[A-Z][A-Za-z0-9_]*[[(]$' | sed 's/[[(]$//' | sort -u || true)
    listed=$(index_names "$go_idx")
    report go "导出了但未登记进 INDEX.md" $(comm -23 <(echo "$exported") <(echo "$listed"))
    report go "INDEX.md 里登记了但代码中不存在" $(comm -13 <(echo "$exported") <(echo "$listed"))
  fi
fi

# ── TypeScript ────────────────────────────────────────────────
ts_dir="frontend/src/utils"
ts_idx="$ts_dir/INDEX.md"
if [[ -d $ts_dir ]]; then
  if [[ ! -f $ts_idx ]]; then
    echo "✗ 缺少工具库索引: $ts_idx"; fail=1
  else
    exported=$(find "$ts_dir" -maxdepth 1 -name '*.ts' ! -name '*.test.ts' -exec \
      grep -hoE '^export +(async +)?(function|const) +[a-zA-Z_][a-zA-Z0-9_]*' {} + 2>/dev/null \
      | awk '{print $NF}' | sort -u || true)
    listed=$(index_names "$ts_idx")
    report ts "导出了但未登记进 INDEX.md" $(comm -23 <(echo "$exported") <(echo "$listed"))
    report ts "INDEX.md 里登记了但代码中不存在" $(comm -13 <(echo "$exported") <(echo "$listed"))
  fi
fi

if [[ $fail -eq 1 ]]; then
  echo "修正方式：更新对应的 INDEX.md（见 docs/rules/coding-standards.md §1.4）"
  exit 1
fi

echo "✓ 工具库索引与代码一致"
