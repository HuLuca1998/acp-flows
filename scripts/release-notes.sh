#!/usr/bin/env bash
# 从 conventional commits 生成 release notes。
#
# 放弃 release-please 后（adr/0007 修订 2），CHANGELOG.md 这个文件不再维护——
# GitHub Release 页面就是变更日志。本脚本产出那段正文。
#
#   scripts/release-notes.sh v0.2.0 > notes.md
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

tag="${1:-}"
[[ -z $tag ]] && { echo "用法: $0 <tag>" >&2; exit 2; }

# 上一个正式版 tag（跳过 snapshot）
prev=$(git tag -l 'v*' --sort=-v:refname | grep -v snapshot | head -1 || true)
range="${prev:+$prev..}HEAD"

section() {
  local title=$1 pattern=$2
  local body
  body=$(git log "$range" --no-merges --pretty='%s' 2>/dev/null \
    | { grep -E "$pattern" || true; } \
    | sed -E 's/^[a-z]+(\([^)]*\))?!?: //' \
    | sed 's/^/- /' || true)
  if [[ -n $body ]]; then
    printf '### %s\n\n%s\n\n' "$title" "$body"
  fi
}

printf '通道：%s · 提交 `%s`\n\n' \
  "$([[ $tag == *snapshot* ]] && echo 预发布快照 || echo 正式版)" \
  "$(git rev-parse --short HEAD)"

section '新功能'   '^feat(\([^)]*\))?!?: '
section '缺陷修复' '^fix(\([^)]*\))?!?: '
section '性能'     '^perf(\([^)]*\))?!?: '

# 破坏性变更单独提出来放最前面之外的显眼位置
breaking=$(git log "$range" --no-merges --pretty='%s%n%b' 2>/dev/null \
  | { grep -E '^BREAKING CHANGE:|^[a-z]+(\([^)]*\))?!:' || true; } || true)
if [[ -n $breaking ]]; then
  printf '### ⚠️ 破坏性变更\n\n%s\n\n' "$(sed 's/^/- /' <<<"$breaking")"
fi

printf -- '---\n\n应用内「设置 → 应用更新」可一键更新；更新包经 minisign 签名校验后才安装。\n'
[[ $tag == *snapshot* ]] && printf '\n> 这是预发布快照，**不会**推送给已安装用户。\n'
exit 0
