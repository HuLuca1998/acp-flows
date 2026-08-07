#!/usr/bin/env bash
# 校验文档的上下文预算。规则见 docs/ai-playbook.md §7。
#
# 为什么需要这个检查：全部文档加起来 216k token，超过 200k 上下文窗口。
# AI 读不完就等于没写——而**读不完这件事没有任何别的手段会报警**，
# 只会表现为「AI 好像没按文档做」。
#
# 三条：
#   L0（常驻）不超预算            —— 超了每次对话都在浪费上下文
#   L1（skill / 目录 AGENTS.md）  —— 超了说明该拆或该下沉
#   L2 大文档必须有「读法」块      —— 否则 AI 会整篇读，一次吃掉 10% 上下文
set -euo pipefail
cd "$(dirname "$0")/.."

# 中文约 2.5 字符/token（英文约 4）。取 2.5 是保守估计。
readonly CHARS_PER_TOK=2.5

readonly L0_BUDGET=6000      # 根 AGENTS.md + CLAUDE.md
readonly L1_BUDGET=2000      # 单个 skill / 单份目录 AGENTS.md
readonly READING_GUIDE=8000  # 超过这个体量必须有「读法」块

fail=0

toks() { python3 -c "import sys,pathlib;print(int(len(pathlib.Path(sys.argv[1]).read_text(errors='ignore'))/$CHARS_PER_TOK))" "$1"; }

say() {
  fail=1
  echo "✗ $1"
  printf '%s\n' "$2" | while IFS= read -r l; do [[ -n $l ]] && echo "    $l"; done
  echo
}

# ── L0：常驻上下文 ────────────────────────────────────────────
l0=0
for f in AGENTS.md CLAUDE.md; do
  [[ -f $f ]] && l0=$((l0 + $(toks "$f")))
done
if [[ $l0 -gt $L0_BUDGET ]]; then
  say "L0 常驻文档超预算：${l0} tok > ${L0_BUDGET}" \
"根 AGENTS.md + CLAUDE.md 每次对话都在上下文里，超了是持续浪费。
把细节下沉到 docs/ 的专题文档，根 AGENTS.md 只留铁律与路由表。"
else
  echo "✓ L0 常驻 ${l0} tok（预算 ${L0_BUDGET}）"
fi

# ── L1：skill 与目录 AGENTS.md ───────────────────────────────
#
# ★ 循环体末尾一律用 `if ... fi`，不要用 `[[ ]] && echo`：
#   条件为假时 AND-list 返回 1，set -e 会把整个脚本静默干掉，
#   后面两段检查根本不会跑（scripts/AGENTS.md 陷阱 #1，这里已经踩过一次）。
over_l1=$(
  {
    find .skills -name SKILL.md 2>/dev/null || true
    find . -name AGENTS.md \
      -not -path './node_modules/*' -not -path '*/node_modules/*' \
      -not -path './.git/*' -not -path './.worktree/*' 2>/dev/null || true
  } | while read -r f; do
    if [[ -z $f || $f == ./AGENTS.md ]]; then   # 根 AGENTS.md 按 L0 算
      continue
    fi
    t=$(toks "$f")
    if [[ $t -gt $L1_BUDGET ]]; then
      echo "$f  ${t} tok"
    fi
  done
)
if [[ -n $over_l1 ]]; then
  say "L1 文档超预算（单份上限 ${L1_BUDGET} tok）" \
"$over_l1

这些是「开工时加载一份」的，太长会挤掉干活的余量。
拆子目录，或把详细规格挪进 docs/ 的专题文档并在这里只留链接。"
else
  echo "✓ L1 全部在 ${L1_BUDGET} tok 以内"
fi

# ── L2：大文档必须有「读法」块 ───────────────────────────────
missing_guide=$(
  { find docs -name '*.md' 2>/dev/null || true; } | while read -r f; do
    t=$(toks "$f")
    if [[ $t -le $READING_GUIDE ]]; then
      continue
    fi
    if ! grep -q '读法' "$f"; then
      echo "$f  ${t} tok"
    fi
  done
)
if [[ -n $missing_guide ]]; then
  say "大文档缺「读法」块（超过 ${READING_GUIDE} tok 就要有）" \
"$missing_guide

没有读法块，AI 会整篇读——一次吃掉 10% 以上的上下文。
在文档顶部加一个「读法」块：说明不要整篇读，给出章节索引与 grep 定位方式。
形如：

> **读法**：本文 ~27k token，**不要整篇读**。按需 grep 定位：
> | 你要找 | grep |
> |---|---|
> | 两段式取消 | \`grep -n '两段式取消' docs/xxx.md\` |"
else
  echo "✓ 大文档都有「读法」块"
fi

# ── 总量提示（不失败，只是让人心里有数）──────────────────────
total=$(find . -name '*.md' -not -path './node_modules/*' -not -path '*/node_modules/*' \
  -not -path './.git/*' -not -path './.worktree/*' 2>/dev/null \
  | while read -r f; do toks "$f"; done | paste -sd+ - | bc)
echo "· 文档总量 ${total} tok（上下文窗口 200k —— 所以 L2 必须 grep 定位，不能整篇读）"

[[ $fail -eq 1 ]] && { echo "规则见 docs/ai-playbook.md §7"; exit 1; }
exit 0
