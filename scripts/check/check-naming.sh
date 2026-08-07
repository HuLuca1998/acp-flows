#!/usr/bin/env bash
# 命名与文件组织规范检查。规则见 docs/rules/coding-standards.md。
set -euo pipefail
cd "$(dirname "$0")/../.."

fail=0
MAX_LINES=400

# say <标题> <多行内容>
# 内容整体作为一个参数传入——不加引号会被按空格词分割，把一行拆成一堆碎片。
say() {
  fail=1
  echo "✗ $1"
  printf '%s\n' "$2" | while IFS= read -r line; do
    [[ -n $line ]] && echo "    $line"
  done
  echo
}

# 列出匹配的文件路径。
src() {
  find . -type d \( -name node_modules -o -name target -o -name dist -o -name .git \
    -o -name gen -o -name .worktree \) -prune -o -type f "$@" -print
}

# 在源文件里 grep，输出 file:line:content。
# ★ 不能用 src() 加 -exec：find 的 -print 会无视 -exec 的结果，把所有文件都打印出来。
grep_src() {
  local pattern=$1; shift
  src "$@" -print0 2>/dev/null | tr '\0' '\n' | while read -r f; do
    [[ -n $f ]] && grep -HnE "$pattern" "$f" 2>/dev/null || true
  done | sed 's|^\./||'
}

# ── 1. 垃圾桶文件名 ───────────────────────────────────────────
junk=$(src \( -name 'util.go' -o -name 'utils.go' -o -name 'helper.go' -o -name 'helpers.go' \
  -o -name 'common.go' -o -name 'misc.go' -o -name 'util.ts' -o -name 'helper.ts' \
  -o -name 'common.ts' -o -name 'misc.ts' \) | sed 's|^\./||' || true)
if [[ -n $junk ]]; then say "文件名不说明内容，等于垃圾桶（见 coding-standards §1.3）" "$junk"; fi

# ── 2. 单文件行数上限 ─────────────────────────────────────────
long=$(src \( -name '*.go' -o -name '*.ts' -o -name '*.tsx' -o -name '*.rs' \) \
  ! -name '*_test.go' ! -name '*.test.ts' ! -name '*.test.tsx' \
  | while read -r f; do
      n=$(wc -l < "$f" | tr -d ' ')
      [[ $n -gt $MAX_LINES ]] && echo "${f#./} ($n 行)"
    done || true)
if [[ -n $long ]]; then say "单文件超过 $MAX_LINES 行，职责不单一（见 coding-standards §2）" "$long"; fi

# ── 3. Go：禁止 Get 前缀的访问器 ──────────────────────────────
getters=$(grep_src '^func +\([^)]+\) +Get[A-Z]' -name '*.go' ! -name '*_test.go' || true)
if [[ -n $getters ]]; then say "Go 访问器禁止 Get 前缀，直接用字段名（见 coding-standards §3.3）" "$getters"; fi

# ── 4. TS：禁止 enum ─────────────────────────────────────────
enums=$(grep_src '^ *(export +)?enum +' \( -name '*.ts' -o -name '*.tsx' \) || true)
if [[ -n $enums ]]; then say "TS 禁用 enum，改用 as const 对象 + 联合类型（见 coding-standards §4.1）" "$enums"; fi

# ── 5. 前端越层 import Tauri ─────────────────────────────────
if [[ -d frontend/src ]]; then
  leak=$(grep -rn "@tauri-apps/" frontend/src --include='*.ts' --include='*.tsx' 2>/dev/null \
    | grep -v '^frontend/src/platform/' || true)
  if [[ -n $leak ]]; then say "只有 src/platform/ 可以 import @tauri-apps/*（见 architecture.md §5）" "$leak"; fi
fi

# ── 6. 生产代码不得 import 测试辅助包 ─────────────────────────
if [[ -d backend ]]; then
  # 只匹配真正的 import 路径（带引号），注释里提到 tests/testutil 不算
  leak=$(grep -rnE '"[^"]*/tests/testutil"' backend --include='*.go' 2>/dev/null \
    | grep -v '_test\.go:' || true)
  if [[ -n $leak ]]; then say "生产代码禁止 import tests/testutil（见 testing-strategy.md §2）" "$leak"; fi
fi

if [[ $fail -eq 1 ]]; then exit 1; fi
echo "✓ 命名与文件组织规范检查通过"
