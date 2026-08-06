#!/usr/bin/env bash
# 并行工作区管理。规范见 docs/git-workflow.md §4。
#
#   scripts/worktree.sh add feat/acp-session-cancel
#   scripts/worktree.sh clean
#
# 工作区一律建在仓库根的 .worktree/ 下（已 gitignore），
# 目录名 = 分支名把 / 换成 -。
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

ROOT=".worktree"

usage() {
  cat >&2 <<'EOF'
用法:
  worktree.sh add <分支名>    建工作区 + 分支 + 装依赖
  worktree.sh clean           清理已合并分支的工作区
  worktree.sh list            列出当前工作区
EOF
  exit 2
}

slug() { echo "${1//\//-}"; }

cmd_add() {
  local branch="${1:-}"
  [[ -z $branch ]] && usage

  if [[ ! $branch =~ ^(feat|fix|perf|refactor|test|docs|build|ci|chore)/[a-z0-9-]+$ ]]; then
    echo "✗ 分支名不合规: $branch" >&2
    echo "  格式: <type>/<scope>-<slug>，例如 feat/acp-session-cancel" >&2
    echo "  见 docs/git-workflow.md §1" >&2
    exit 1
  fi

  local dir="$ROOT/$(slug "$branch")"
  if [[ -e $dir ]]; then
    echo "✗ 已存在: $dir" >&2
    exit 1
  fi

  git fetch origin main --quiet 2>/dev/null || true
  local base
  base=$(git rev-parse --verify --quiet origin/main || git rev-parse main)

  git worktree add "$dir" -b "$branch" "$base"
  echo "✓ 工作区: $dir  分支: $branch  基线: ${base:0:7}"

  # node 依赖不跨 worktree 共享，需各自安装
  for p in frontend e2e shell; do
    if [[ -f "$dir/$p/package.json" ]]; then
      echo "· 安装 $p 依赖…"
      (cd "$dir/$p" && pnpm install --silent)
    fi
  done

  cat <<EOF

下一步:
  cd $dir
  make check        # 先确认基线是绿的，再动手
EOF
}

cmd_clean() {
  [[ -d $ROOT ]] || { echo "没有工作区"; return; }

  git fetch origin main --quiet 2>/dev/null || true
  local removed=0

  while read -r dir; do
    [[ -z $dir ]] && continue
    local branch
    branch=$(git -C "$dir" rev-parse --abbrev-ref HEAD 2>/dev/null || echo "")
    [[ -z $branch || $branch == "main" ]] && continue

    # 分支已合并进 main（或远端已删）→ 可清理
    if git merge-base --is-ancestor "$branch" origin/main 2>/dev/null; then
      echo "· 清理已合并: $dir ($branch)"
      git worktree remove "$dir" --force
      git branch -d "$branch" 2>/dev/null || true
      removed=$((removed + 1))
    else
      echo "· 保留未合并: $dir ($branch)"
    fi
  done < <(find "$ROOT" -mindepth 1 -maxdepth 1 -type d 2>/dev/null)

  git worktree prune
  echo "✓ 清理了 $removed 个"
}

case "${1:-}" in
  add)   shift; cmd_add "$@" ;;
  clean) cmd_clean ;;
  list)  git worktree list ;;
  *)     usage ;;
esac
