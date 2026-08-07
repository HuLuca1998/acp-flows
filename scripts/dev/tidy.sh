#!/usr/bin/env bash
# 清理已合并的分支、worktree、以及远端已删的引用。
#
# 合并 PR 之后必跑。不清的后果是累积性的：
#   · `git branch` 越列越长，下一轮 AI 分不清哪个还在用
#   · 残留的 worktree 占磁盘，且 `git worktree list` 里全是死链接
#   · 远端已删的 origin/* 引用会让 `git branch -a` 和补全变成噪音
#
#   scripts/dev/tidy.sh          清理
#   scripts/dev/tidy.sh --check  只报告不动手（CI 用）
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

CHECK_ONLY=false
[[ ${1:-} == "--check" ]] && CHECK_ONLY=true

BASE=main
found=0

say() { echo "  $*"; }

# ── 1. 同步远端状态 ───────────────────────────────────────────
if [[ $CHECK_ONLY == false ]]; then
  git fetch --prune --quiet origin 2>/dev/null || true
fi

# ── 2. 已合并分支的 worktree ─────────────────────────────────
# 顺序很重要：worktree 占着分支时删不掉分支，必须先摘 worktree。
echo "worktree:"
wt_removed=0
while read -r dir; do
  [[ -z $dir || $dir == "$(pwd)" ]] && continue
  branch=$(git -C "$dir" rev-parse --abbrev-ref HEAD 2>/dev/null || echo "")
  [[ -z $branch || $branch == "$BASE" ]] && continue

  if git merge-base --is-ancestor "$branch" "origin/$BASE" 2>/dev/null; then
    found=1
    if [[ $CHECK_ONLY == true ]]; then
      say "· 可清理（已合并）: $dir [$branch]"
    else
      say "· 清理: $dir [$branch]"
      git worktree remove "$dir" --force
      wt_removed=$((wt_removed + 1))
    fi
  else
    say "· 保留（未合并）: $dir [$branch]"
  fi
done < <(git worktree list --porcelain | awk '/^worktree /{print $2}')
[[ $wt_removed -eq 0 && $found -eq 0 ]] && say "· 无"
[[ $CHECK_ONLY == false ]] && git worktree prune

# ── 3. 已合并的本地分支 ──────────────────────────────────────
echo "本地分支:"
merged=$(git branch --merged "origin/$BASE" --format='%(refname:short)' 2>/dev/null \
  | { grep -vxE "$BASE|\*.*" || true; } || true)
if [[ -z $merged ]]; then
  say "· 无"
else
  found=1
  while IFS= read -r b; do
    [[ -z $b ]] && continue
    # 当前所在的分支删不掉，跳过并提示
    if [[ $b == "$(git rev-parse --abbrev-ref HEAD)" ]]; then
      say "· 跳过（正在使用）: $b —— 切到 $BASE 后再跑一次"
      continue
    fi
    if [[ $CHECK_ONLY == true ]]; then
      say "· 可删除（已合并）: $b"
    else
      say "· 删除: $b"
      git branch -d "$b"
    fi
  done <<<"$merged"
fi

# ── 4. 远端已删的引用 ────────────────────────────────────────
echo "远端残留引用:"
stale=$(git remote prune origin --dry-run 2>/dev/null | { grep -oE 'origin/\S+' || true; } || true)
if [[ -z $stale ]]; then
  say "· 无"
else
  found=1
  printf '%s\n' "$stale" | while IFS= read -r r; do
    [[ -n $r ]] && say "$([[ $CHECK_ONLY == true ]] && echo '· 可清理' || echo '· 清理'): $r"
  done
  [[ $CHECK_ONLY == false ]] && git remote prune origin >/dev/null
fi

echo
if [[ $CHECK_ONLY == true && $found -eq 1 ]]; then
  echo "有可清理项 —— 跑 make tidy"
  exit 1
fi
echo "✓ 清理完成"
