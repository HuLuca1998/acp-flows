#!/usr/bin/env bash
# 校验 Markdown 里的相对链接指向的文件真实存在。
#
# 为什么需要：本仓库文档之间交叉引用极密（一份文档平均指向 5 份别的）。
# 链接断了**没有任何别的手段会报警**——只会表现为「AI 点不开就自己猜」，
# 而猜出来的东西会被当成事实往下写。
#
# 只查相对链接：http(s) 外链不查（慢且会误报），锚点只查文件部分。
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

fail=0
checked=0

# 收集所有 .md（排除依赖与工作区）。
# ★ 不用 mapfile —— macOS 自带 bash 是 3.2，没有这个内建。
docs=()
while IFS= read -r f; do
  docs+=("$f")
done < <(
  find . -name '*.md' \
    -not -path './node_modules/*' -not -path '*/node_modules/*' \
    -not -path './.git/*' -not -path './.worktree/*' \
    -not -path './target/*' -not -path '*/target/*' 2>/dev/null | sort
)

for src in "${docs[@]}"; do
  dir=$(dirname "$src")

  # 抓 [text](link) 里的 link。grep 无匹配返回 1，用 || true 挡住 pipefail
  links=$(
    { grep -oE '\]\([^)]+\)' "$src" || true; } \
    | sed -E 's/^\]\(//; s/\)$//'
  )

  while IFS= read -r raw; do
    if [[ -z $raw ]]; then
      continue
    fi
    # 跳过外链、mailto、纯锚点
    case "$raw" in
      http://*|https://*|mailto:*|'#'*) continue ;;
    esac

    # ★ 排除「看起来像链接但其实是代码」的假阳性。
    #   Go 泛型签名 `Chunk[T any](s []T, n int)` 在 Markdown 里长得和
    #   [text](link) 一模一样。判据：真链接不含空格，且必然有 `.` 或 `/`。
    if [[ $raw == *" "* || $raw != *[./]* ]]; then
      continue
    fi

    # 去掉锚点；URL 解码 %20 等（文档里有 "Duet Spec.dc.html"）
    target="${raw%%#*}"
    if [[ -z $target ]]; then
      continue
    fi
    target=$(printf '%b' "${target//%/\\x}")

    checked=$((checked + 1))
    if [[ ! -e "$dir/$target" ]]; then
      fail=1
      echo "✗ ${src#./} → $raw"
    fi
  done <<<"$links"
done

if [[ $fail -eq 1 ]]; then
  echo
  echo "上面的链接指向不存在的文件。"
  echo "常见原因：文档被移动/改名后引用没跟着改。"
  echo "改完再跑一次 bash scripts/check/check-doc-links.sh"
  exit 1
fi

echo "✓ ${#docs[@]} 份文档的 ${checked} 条相对链接全部有效"

# ── 源码注释里的文档引用 ─────────────────────────────────────
#
# ★ 这一类腐烂只查 .md 是看不到的：Go/TS/shell 的注释里到处写着
# 「规范见 docs/xxx.md §3」。文档一移动，这些注释就全成了死引用——
# 而 AI 会照着注释去读，读不到就开始猜。实测：一次 docs/ 重组打断了 30+ 处。
src_bad=$(
  { grep -rn --include='*.go' --include='*.ts' --include='*.tsx' \
             --include='*.rs' --include='*.sh' --include='*.yml' --include='*.yaml' \
             --include='Makefile' \
             -oE 'docs/[A-Za-z0-9_./-]+\.md' . 2>/dev/null || true; } \
  | grep -v node_modules \
  | while IFS= read -r hit; do
      loc="${hit%:*}"
      path="${hit##*:}"
      if [[ $path == *xxx* ]]; then      # 文档里的占位示例
        continue
      fi
      if [[ ! -f $path ]]; then
        echo "    $path  ← ${loc}"
      fi
    done | sort -u
)
if [[ -n $src_bad ]]; then
  echo
  echo "✗ 源码注释引用了不存在的文档："
  echo "$src_bad"
  echo
  echo "  注释里的文档路径也会腐烂，而且比 Markdown 链接更隐蔽。"
  echo "  移动文档时记得全仓库 grep 一遍，不只是改 .md。"
  exit 1
fi
echo "✓ 源码注释里的文档引用全部有效"
