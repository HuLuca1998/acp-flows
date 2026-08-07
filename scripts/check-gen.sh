#!/usr/bin/env bash
# 校验生成物与 api/openapi.yaml 一致。由 make check-gen 调用（gen 先跑完）。
#
# 防的是铁律 2 被绕过：有人手改了实现却没改 spec，或改了 spec 没跑 make gen。
# 这两种都会让契约与实现悄悄脱钩——而前端是照着契约写的。
#
# 判据是两条，缺一不可：
#   ① 重新生成后**工作区与索引有差异** → spec 改了但没跑 make gen，或有人手改了生成物
#   ② 有**未跟踪**的生成物            → 从没被 add 过（第一次生成、或误删后重建）
#
# ★ 只用 `git diff` 会漏掉 ②：未跟踪文件不在 diff 里，检查会假装通过。
# ★ 只用 `git status --porcelain` 会误报「已 stage 待提交」（`A `）——
#   那恰恰是正确的工作流（跑完 gen 把结果 add 上），不该拦。
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

readonly PATHS=(
  backend/internal/api/gen
  frontend/src/api/gen
)

# 生成物必须存在。目录空 = spec 有端点但没生成出来，是真问题。
missing=()
for p in "${PATHS[@]}"; do
  parent="${p%%/*}"
  # 子项目尚未脚手架时跳过（M0 之前 make check 也要能跑通）
  if [[ $parent == backend && ! -f backend/go.mod ]]; then continue; fi
  if [[ $parent == frontend && ! -f frontend/package.json ]]; then continue; fi
  if [[ ! -d $p ]] || [[ -z "$(ls -A "$p" 2>/dev/null)" ]]; then
    missing+=("$p")
  fi
done
if [[ ${#missing[@]} -gt 0 ]]; then
  echo "✗ 生成物缺失（make gen 应该产出它们）："
  printf '    %s\n' "${missing[@]}"
  exit 1
fi

fail=0

# ① 重新生成的内容与索引/HEAD 不一致
if ! git diff --quiet -- "${PATHS[@]}"; then
  fail=1
  echo "✗ 生成物与 api/openapi.yaml 不一致："
  git --no-pager diff --stat -- "${PATHS[@]}"
  echo
  echo "  说明 spec 改了但没跑 make gen，或者有人手改了生成物。"
  echo "  正确顺序永远是：改 api/openapi.yaml → make gen → 改实现（铁律 2）。"
  echo
fi

# ② 存在未跟踪的生成物
untracked=$({ git ls-files --others --exclude-standard -- "${PATHS[@]}" || true; })
if [[ -n $untracked ]]; then
  fail=1
  echo "✗ 生成物未加入 git："
  printf '%s\n' "$untracked" | while IFS= read -r l; do echo "    $l"; done
  echo
  echo "  生成物必须进仓库——前端与 CI 都直接读它，不能要求每个人先跑一遍生成器。"
  echo
fi

if [[ $fail -eq 1 ]]; then
  exit 1
fi

echo "✓ 生成物与 api/openapi.yaml 一致"
