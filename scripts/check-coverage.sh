#!/usr/bin/env bash
# 分包覆盖率门槛。门槛见 docs/testing-strategy.md §2。
#
# 覆盖率是下限不是目标——它只用来发现"完全没测的地方"。
set -euo pipefail
cd "$(dirname "$0")/.."

profile="backend/coverage.out"
if [[ ! -f $profile ]]; then
  echo "· 跳过覆盖率检查：$profile 不存在（先跑 make cover）"
  exit 0
fi

# 包路径前缀 → 最低覆盖率
declare -a GATES=(
  "internal/domain/model:90"
  "internal/domain/policy:90"
  "internal/acp:80"
  "internal/app:75"
  "internal/store:70"
  "internal/fsstore:70"
)

fail=0
report=$(cd backend && go tool cover -func=coverage.out)

for gate in "${GATES[@]}"; do
  pkg="${gate%%:*}"
  want="${gate##*:}"

  lines=$(grep "/$pkg/" <<<"$report" | grep -v '^total:' || true)
  if [[ -z $lines ]]; then
    echo "· $pkg —— 尚无代码，跳过"
    continue
  fi

  got=$(awk '{gsub("%","",$NF); s+=$NF; n++} END{if(n>0) printf "%.1f", s/n; else print "0"}' <<<"$lines")

  if awk "BEGIN{exit !($got < $want)}"; then
    fail=1
    printf '✗ %-26s %5s%%  <  门槛 %s%%\n' "$pkg" "$got" "$want"
  else
    printf '✓ %-26s %5s%%  >= 门槛 %s%%\n' "$pkg" "$got" "$want"
  fi
done

if [[ $fail -eq 1 ]]; then
  echo
  echo "覆盖率不达标。注意：补测试不等于补断言——"
  echo "对着 docs/testing-strategy.md §3「假测试图鉴」自查后再提交。"
  exit 1
fi
