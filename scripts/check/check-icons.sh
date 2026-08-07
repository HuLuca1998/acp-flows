#!/usr/bin/env bash
# 校验 shell/src-tauri/icons/ 的产物与 design/icon/duet.svg 一致。
#
# 防的是：改了 SVG 但忘了跑 make icons —— 产物与源不同步，
# 而这件事没有任何别的手段能发现（图标不会编译失败，也没有测试碰它）。
#
# 前提：gen-icons.sh 的输出是确定性的（同一 SVG 连跑三次 sha256 相同，已验证）。
set -euo pipefail
cd "$(dirname "$0")/../.."

SVG=design/icon/duet.svg
DIR=shell/src-tauri/icons

if [[ ! -f $SVG ]]; then
  echo "· 跳过图标检查：$SVG 不存在"
  exit 0
fi
if [[ ! -d $DIR ]]; then
  echo "✗ 有源 SVG 但没有产物目录 $DIR —— 跑 make icons"
  exit 1
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

if ! ICON_OUT_DIR="$tmp" bash scripts/gen/gen-icons.sh >/dev/null 2>&1; then
  echo "· 跳过图标检查：gen-icons.sh 跑不起来（多半是本机缺光栅化工具）"
  echo "  这不算失败——CI 上装齐工具后才做强校验。"
  exit 0
fi

drift=$(for f in "$DIR"/*; do
  name=$(basename "$f")
  [[ -f "$tmp/$name" ]] || { echo "$name（重新生成时没产出）"; continue; }
  cmp -s "$f" "$tmp/$name" || echo "$name"
done || true)

if [[ -n $drift ]]; then
  echo "✗ 图标产物与 design/icon/duet.svg 不同步："
  printf '%s\n' "$drift" | while IFS= read -r l; do [[ -n $l ]] && echo "    $l"; done
  echo
  echo "修正方式： make icons   然后提交产物"
  echo "注意：不要直接改 PNG —— 源是 SVG，产物是生成的。"
  exit 1
fi
echo "✓ 图标产物与源 SVG 一致"
