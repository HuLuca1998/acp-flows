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

# ── 7. Shell：变量引用后紧跟非 ASCII 必须用 ${var} ────────────
#
# ★ 真踩过（2026-08-07，services.sh 起不来）：
#   echo "端口 $port）"     在 LC_CTYPE=C 下报 `port?: unbound variable`
# C locale 里 bash 用 isalnum() 判断变量名边界，全角括号的首字节 \xef
# 会被吞进变量名。开发机通常是 UTF-8 locale 所以看不出来，**CI 与
# 非交互 shell 是 C locale**——于是"本地好好的，一上 CI 就崩"。
#
# 修法是显式界定：${port}）。中文注释与文案是本仓库的常态，这个组合会反复出现。
naked=$(python3 - <<'PY' || true
import pathlib, re
pat = re.compile(rb'\$[a-zA-Z_][a-zA-Z0-9_]*(?=[\x80-\xff])')
for p in sorted(pathlib.Path('scripts').rglob('*.sh')):
    for n, raw in enumerate(p.read_bytes().split(b'\n'), start=1):
        # 注释不会被 bash 求值，不可能触发这个 bug——
        # 跳过它才能在注释里放反例示例教下一个人。
        if raw.lstrip().startswith(b'#'):
            continue
        for m in pat.finditer(raw):
            print(f"{p}:{n}: {m.group().decode()}")
PY
)
if [[ -n $naked ]]; then
  say "变量后紧跟中文时必须写 \${var}——C locale 下 bash 会把多字节首字节吞进变量名" "$naked"
fi

# ── 8. Go：exec.CommandContext 必须设 WaitDelay ───────────────
#
# ★ 真踩过（2026-08-07，U1.3.1 的检测超时形同虚设）：
#   ctx 到期时 CommandContext 只 kill **直接**子进程。子进程若 fork 过
#   （shell 包装脚本、node 启动器——真实 CLI 几乎都是这样），孙子进程
#   继承着 stdout 管道活下去，而 Output()/Wait() 要等管道关闭才返回。
#   实测：`/bin/sleep 30` 配 1.5 秒超时，跑满了 30 秒。
#
# 这个 bug 不会让编译或 vet 有任何反应，只表现为"偶尔卡很久"。
# WaitDelay 是标准库给的解法，设了就行——所以直接检查有没有设。
nodelay=$(python3 "$(dirname "$0")/lib/waitdelay.py" || true)
if [[ -n $nodelay ]]; then
  say "exec.CommandContext 之后必须设 cmd.WaitDelay——否则 fork 过的子进程会架空超时" "$nodelay"
fi

# ── 9. Shell：语法必须过 bash -n ──────────────────────────────
#
# ★ 真踩过（2026-08-08）：check-merge-result.sh 里写了一个**全角左括号**
#   （cd ... )
# 中文注释写多了顺手带出来的。它只在执行到那一行时才炸，而那一行前面有个
# 提前 exit 0 的分支——于是脚本"跑通了"，实际上从没走到出错的地方。
#
# bash -n 只做语法解析、不执行任何命令，几十毫秒扫完全部脚本。
badsyntax=""
while IFS= read -r f; do
  [[ -z $f ]] && continue
  if ! err=$(bash -n "$f" 2>&1); then
    badsyntax+="${f}: ${err}"$'\n'
  fi
done < <(find scripts -name '*.sh')
if [[ -n $badsyntax ]]; then
  say "shell 脚本语法错误（bash -n）——中文注释里的全角括号是常见来源" "$badsyntax"
fi

# ── 10. 上层代码不许出现 Runtime 品牌名 ──────────────────────
#
# U2.2.3 的 R2 明写这条要接进 CI。上层一旦写 `if name == "codex"`，
# 加第三个 Runtime 就要在几十个地方改 if，而每漏一处都是一个
# 只在那个 Runtime 下出现的 bug。
#
# 差异要么在 adapter 里填平，要么升级成能力查询——
# 两条路都不该让上层知道品牌。
brands=$(python3 "$(dirname "$0")/lib/no_brand_in_upper.py" || true)
if [[ -n $brands ]]; then
  say "app / domain / api 里出现了 Runtime 品牌名——差异应在 adapter 内填平或升级为能力查询" "$brands"
fi

if [[ $fail -eq 1 ]]; then exit 1; fi
echo "✓ 命名与文件组织规范检查通过"
