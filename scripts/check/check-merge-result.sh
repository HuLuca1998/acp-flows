#!/usr/bin/env bash
# 检查「与 main 合并之后」还能不能编译。
#
# ★ 真踩过（2026-08-08，PR #14 三个 job 同时红而本地 make check 全绿）：
#
#   分支从 A 点分出，那时某些文件在 main 上；之后 main 用 revert 删掉了它们。
#   **从 git 的角度看分支并没有"改动"这些文件**——它只是继承了它们——
#   所以合并结果里它们跟着 revert 一起消失。
#   本地工作区一切正常，CI 报 `undefined: model.Project`。
#
# 本地检查看的是工作区，CI 看的是合并结果。这两者不一致时，
# 「本地全绿」就是一句空话。
#
# 做法：把 main 合进一个临时 worktree（不碰当前工作区），在那儿编译。
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

BASE="${1:-origin/main}"

# 没有远端 main（第一次 clone 之前、或离线）时跳过而不是报错——
# 一个跑不动的检查等于没有检查。
if ! git rev-parse -q --verify "$BASE" >/dev/null; then
  echo "· 跳过合并结果检查：找不到 ${BASE}"
  exit 0
fi

if [[ "$(git rev-parse HEAD)" == "$(git rev-parse "$BASE")" ]]; then
  echo "✓ 当前就在 ${BASE} 上，无需检查合并结果"
  exit 0
fi

# 已经包含 base 的全部提交（rebase 过 / 是它的后代）→ 合并结果就是 HEAD
if git merge-base --is-ancestor "$BASE" HEAD; then
  echo "✓ 已包含 ${BASE} 的全部提交，合并结果即当前状态"
  exit 0
fi

WORKTREE=$(mktemp -d)/merge-check
cleanup() { git worktree remove --force "$WORKTREE" >/dev/null 2>&1 || true; }
trap cleanup EXIT

git worktree add -q --detach "$WORKTREE" HEAD
(cd "$WORKTREE" && git merge --no-edit -q "$BASE" >/dev/null 2>&1) || {
  echo "✗ 与 ${BASE} 合并有冲突，先在本地 rebase 解决"
  exit 1
}

if [[ -f "$WORKTREE/backend/go.mod" ]]; then
  if ! (cd "$WORKTREE/backend" && go build ./... 2>&1); then
    echo
    echo "✗ **与 ${BASE} 合并之后编译不过**，而当前工作区是好的。"
    echo "  最常见的原因：你的分支从某个点分出，之后 main 删掉/改掉了"
    echo "  你依赖的文件——git 认为你没"改动"它们，于是合并时跟着删了。"
    echo "  修法：git rebase ${BASE}，然后把缺的文件取回来。"
    exit 1
  fi
fi

echo "✓ 与 ${BASE} 合并之后仍可编译"
