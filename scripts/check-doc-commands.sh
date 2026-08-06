#!/usr/bin/env bash
# 校验文档里提到的命令与脚本真的存在。
#
# 防的是一类特定的文档腐烂：文档说「跑 make xxx」，但那个目标早就改名或从没建过。
# AI 照着文档跑，撞一鼻子灰，然后开始不信任文档——这是最坏的结果。
#
# 两处例外，都是「按定义会引用尚不存在的东西」：
#   - docs/milestones/**  单元是**计划**不是**声称**。单元标记改成 ✓ 的那一刻起
#     它引用的路径就必须真的存在——本脚本对已完成的单元不放行（见文末那段检查）
#   - docs/open-questions.md  它是**已知缺口登记表**，登记「某某还不存在」正是它的职责
set -euo pipefail
cd "$(dirname "$0")/.."

fail=0

say() {
  fail=1
  echo "✗ $1"
  printf '%s\n' "$2" | while IFS= read -r line; do
    [[ -n $line ]] && echo "    $line"
  done
  echo
}

# 排除：正文里的占位示例（xxx）、里程碑的前向引用、Skill 自带的脚本
# 注意：每条 find 都收尾 || true。
# find 遇到无权限目录会返回非零，配合 set -o pipefail 会把整个脚本打死——
# 这个坑在本仓库的检查脚本里踩过三次，见 scripts/AGENTS.md。
docs_to_check() {
  # ★ 剪枝要用 -prune 而不是 -not -path './node_modules/*'：
  # 后者只匹配顶层，匹配不到 ./frontend/node_modules/...，
  # 结果会把几千个第三方 README 里的散文（"make an aggregate"）当成 make 目标。
  find . \
    \( -name node_modules -o -name .git -o -name .worktree -o -name dist \
       -o -name target -o -path './docs/milestones' \) -prune -o \
    -name '*.md' -not -name 'open-questions.md' -print \
    2>/dev/null || true
}

# ── make 目标 ─────────────────────────────────────────────────
targets=$(grep -oE '^\.PHONY: [a-zA-Z_-]+' Makefile | awk '{print $2}' | sort -u)

missing_make=$(docs_to_check | sort -u | while read -r f; do
  [[ -f $f ]] || continue
  grep -oE '\bmake (-s |-C [^ ]+ )?[a-z][a-z-]+' "$f" 2>/dev/null \
    | awk '{print $NF}' | while read -r t; do
      grep -qx "$t" <<<"$targets" || echo "make $t  ← $f"
    done
done | sort -u || true)
if [[ -n $missing_make ]]; then say "文档提到但 Makefile 里不存在的目标" "$missing_make"; fi

# ── scripts/*.sh ──────────────────────────────────────────────
# xxx.sh / check-xxx.sh 是文档里的占位示例，跳过
missing_script=$(docs_to_check | sort -u | while read -r f; do
  [[ -f $f ]] || continue
  grep -oE 'scripts/[a-z][a-z0-9_-]*\.sh' "$f" 2>/dev/null | while read -r s; do
    # 注意：这里不能用 case —— 它的 ) 会让 $(...) 的括号失衡，报 syntax error
    [[ $s == *xxx* ]] && continue           # 文档里的占位示例
    [[ -f $s ]] || echo "$s  ← $f"
  done
done | sort -u || true)
if [[ -n $missing_script ]]; then say "文档提到但不存在的脚本" "$missing_script"; fi

# ── 已完成的里程碑单元不许引用不存在的东西 ────────────────────
# 标记为 ✓ 的单元代表「已交付」，它 allowed_changes 里的路径必须真的存在。
done_missing=$(grep -rlE '^### ✓ U' docs/milestones/*.md 2>/dev/null | while read -r f; do
  awk '/^### ✓ U/{u=$0; keep=1} /^### [○◐⊘]/{keep=0} keep && /allowed_changes/{print FILENAME"\t"u"\t"$0}' "$f"
done | grep -oE '`[a-z][a-zA-Z0-9_/.@-]+`' | tr -d '`' | sort -u | while read -r p; do
  [[ $p == *"*"* ]] && continue                    # 通配路径不检查
  [[ $p =~ \.(go|ts|tsx|sh|yaml|yml|json|md)$ ]] || continue
  [[ -e $p ]] || echo "$p"
done || true)
if [[ -n $done_missing ]]; then say "已完成（✓）的里程碑单元引用了不存在的路径" "$done_missing"; fi

if [[ $fail -eq 1 ]]; then
  echo "修正方式：改文档指向真实存在的命令，或把命令建出来。"
  echo "注意：docs/milestones/ 里未完成（○/◐）单元的前向引用是正常的，本脚本已跳过。"
  exit 1
fi
echo "✓ 文档里的命令与脚本都真实存在"
