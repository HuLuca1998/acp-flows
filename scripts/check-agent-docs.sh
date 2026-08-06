#!/usr/bin/env bash
# 检查所有「关键目录」是否都有成对的 AGENTS.md + CLAUDE.md，且内容已填实。
# 关键目录的定义见根 AGENTS.md §4.1。
#
# 退出码：0 = 全部齐备；1 = 有缺失或未填实。
set -euo pipefail

cd "$(dirname "$0")/.."

missing=()
unfilled=()

# ── 收集关键目录 ─────────────────────────────────────────────
collect_key_dirs() {
  # 规则 1：顶层目录 + 显式登记的关键目录
  for d in backend frontend shell api e2e design docs scripts \
           backend/cmd backend/tests frontend/tests; do
    [[ -d $d ]] && echo "$d"
  done

  # 规则 2：backend/internal/ 下的一级子包
  [[ -d backend/internal ]] && find backend/internal -mindepth 1 -maxdepth 1 -type d

  # 规则 3：frontend/src/ 与 frontend/src/features/ 下的一级子目录
  [[ -d frontend/src ]] && find frontend/src -mindepth 1 -maxdepth 1 -type d
  [[ -d frontend/src/features ]] && find frontend/src/features -mindepth 1 -maxdepth 1 -type d

  # 规则 4：任何直接包含 >= 3 个源文件的目录
  find . \
    -type d \
    \( -name node_modules -o -name target -o -name dist -o -name .git -o -name gen \) -prune -o \
    -type d -print | while read -r d; do
    n=$(find "$d" -maxdepth 1 -type f \
      \( -name '*.go' -o -name '*.ts' -o -name '*.tsx' -o -name '*.rs' \) | wc -l | tr -d ' ')
    if [[ $n -ge 3 ]]; then
      echo "${d#./}"
    fi
  done
}

# ── 校验 ─────────────────────────────────────────────────────
while read -r dir; do
  [[ -z $dir ]] && continue
  dir="${dir#./}"
  [[ $dir == "." ]] && continue

  for f in AGENTS.md CLAUDE.md; do
    if [[ ! -f "$dir/$f" ]]; then
      missing+=("$dir/$f")
    # 只认模板里的占位形态（行首 TODO / 表格单元 | TODO | / 列表项 - TODO），
    # 避免把正文里的「Agent 的 TODO 清单」这类词误判成未填实。
    elif grep -qE '(^|\| *|- +)TODO' "$dir/$f"; then
      unfilled+=("$dir/$f")
    fi
  done
done < <(collect_key_dirs | sort -u)

# ── 报告 ─────────────────────────────────────────────────────
fail=0

if [[ ${#missing[@]} -gt 0 ]]; then
  fail=1
  echo "✗ 缺少文档的关键目录（见根 AGENTS.md §4.1）："
  printf '    %s\n' "${missing[@]}"
  echo
fi

if [[ ${#unfilled[@]} -gt 0 ]]; then
  fail=1
  echo "✗ 文档里仍有未填实的 TODO 占位符："
  printf '    %s\n' "${unfilled[@]}"
  echo
fi

if [[ $fail -eq 1 ]]; then
  echo "补齐方式： make docs-scaffold DIR=<目录>   然后把 TODO 填实"
  exit 1
fi

echo "✓ 所有关键目录都有填实的 AGENTS.md + CLAUDE.md"
