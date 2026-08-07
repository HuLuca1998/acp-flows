#!/usr/bin/env bash
# 干什么：把 design/icon/duet.svg 光栅化成 macOS 打包需要的全部图标产物，
#         写进 shell/src-tauri/icons/（32x32.png / 128x128.png / 128x128@2x.png /
#         icon.png / icon.icns / icon.ico），并逐个核对实际像素尺寸。
# 谁调用：改完 design/icon/duet.svg 之后手动跑 `./scripts/gen/gen-icons.sh`。
#         图标产物是提交进仓库的构建输入，不在 CI 里重新生成。
#
# 退出码：
#   0  全部产物生成成功且尺寸核对通过
#   1  源 SVG 缺失 / 损坏，或产物尺寸与预期不符
#   2  缺少必需的外部工具（信息里给出安装方式）

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SRC="$ROOT/design/icon/duet.svg"
# 输出目录可覆盖，供 check-icons.sh 生成到临时目录后与仓库内产物比对。
OUT="${ICON_OUT_DIR:-$ROOT/shell/src-tauri/icons}"
WORK="$(mktemp -d "${TMPDIR:-/tmp}/duet-icons.XXXXXX")"
trap 'rm -rf "$WORK"' EXIT

die() { printf '✗ %s\n' "$1" >&2; shift; for l in "$@"; do printf '  %s\n' "$l" >&2; done; exit 1; }
need() { printf '✗ %s\n' "$1" >&2; shift; for l in "$@"; do printf '  %s\n' "$l" >&2; done; exit 2; }

# ── 前置检查 ────────────────────────────────────────────────────────────────

[ -f "$SRC" ] || die "找不到源 SVG：${SRC#"$ROOT"/}" \
  "图标的唯一真源是这份 SVG。它不该被删除——从 git 恢复：" \
  "    git checkout -- design/icon/duet.svg"

# 光栅化后端：优先 macOS 自带的 sips，其次 rsvg-convert。
RENDERER=""
if command -v sips >/dev/null 2>&1; then
  RENDERER="sips"
elif command -v rsvg-convert >/dev/null 2>&1; then
  RENDERER="rsvg"
else
  need "没有可用的 SVG 光栅化工具（需要 sips 或 rsvg-convert）" \
    "macOS 上 sips 是系统自带的（/usr/bin/sips），找不到说明 PATH 被改坏了。" \
    "其他平台安装：  brew install librsvg   或   apt-get install librsvg2-bin"
fi

# .icns 只有 macOS 的 iconutil 能封装。
if ! command -v iconutil >/dev/null 2>&1; then
  need "找不到 iconutil，无法生成 icon.icns" \
    "iconutil 是 macOS 自带工具（/usr/bin/iconutil），只在 macOS 上可用。" \
    "本脚本必须在 macOS 上跑——.icns 是 macOS 打包的必需产物。"
fi

# 多尺寸 .ico 用 python3 打包（纯标准库，不装任何第三方包）。
if ! command -v python3 >/dev/null 2>&1; then
  need "找不到 python3，无法打包多尺寸 icon.ico" \
    "macOS：  xcode-select --install    （随命令行工具一起装上）" \
    "或：      brew install python3"
fi

# ── 光栅化 ──────────────────────────────────────────────────────────────────

# render <边长> <输出 png 路径>
render() {
  size="$1"; dest="$2"
  case "$RENDERER" in
    sips) sips -s format png -Z "$size" "$SRC" --out "$dest" >/dev/null 2>&1 \
            || die "sips 光栅化失败：${size}px" \
                 "多半是 SVG 语法错误。sips 走的是严格 XML 解析器，最常见的两个坑：" \
                 "  1. XML 注释里不能出现连续两个连字符——CSS 令牌名 --color-x 写进注释就会炸" \
                 "  2. 未闭合的标签 / 未转义的 & " \
                 "定位具体行号：  sips -s format png design/icon/duet.svg --out /dev/null" ;;
    rsvg) rsvg-convert -w "$size" -h "$size" -o "$dest" "$SRC" \
            || die "rsvg-convert 光栅化失败：${size}px" \
                 "先单独跑一次看报错：  rsvg-convert -w 64 -h 64 design/icon/duet.svg -o /dev/null" ;;
  esac
  [ -s "$dest" ] || die "光栅化产出了空文件：${dest#"$ROOT"/}（${size}px）" \
    "磁盘写满或 TMPDIR 不可写时会这样。检查：  df -h ."
}

mkdir -p "$OUT"

printf '光栅化 %s\n' "${SRC#"$ROOT"/}"

# Tauri 约定的四个 PNG。@2x 是逻辑尺寸的两倍像素。
render 32   "$OUT/32x32.png"
render 128  "$OUT/128x128.png"
render 256  "$OUT/128x128@2x.png"
render 1024 "$OUT/icon.png"

# ── icon.icns ───────────────────────────────────────────────────────────────
# iconutil 只吃目录名以 .iconset 结尾、文件名严格按 icon_<W>x<H>[@2x].png 的目录。

ICONSET="$WORK/duet.iconset"
mkdir -p "$ICONSET"
for spec in \
  "16:icon_16x16.png"      "32:icon_16x16@2x.png" \
  "32:icon_32x32.png"      "64:icon_32x32@2x.png" \
  "128:icon_128x128.png"   "256:icon_128x128@2x.png" \
  "256:icon_256x256.png"   "512:icon_256x256@2x.png" \
  "512:icon_512x512.png"   "1024:icon_512x512@2x.png"
do
  render "${spec%%:*}" "$ICONSET/${spec#*:}"
done

iconutil -c icns "$ICONSET" -o "$OUT/icon.icns" \
  || die "iconutil 封装 icns 失败" \
       "iconutil 对 iconset 目录里的文件名零容忍，必须是 icon_<W>x<H>[@2x].png。" \
       "手动复现：  iconutil -c icns $ICONSET -o /tmp/icon.icns"

# ── icon.ico ────────────────────────────────────────────────────────────────
# sips 写 ico 只能写单尺寸；Windows 任务栏 / 资源管理器要多尺寸，
# 所以自己按 ICO 规范把多张 PNG 打包（Vista 起支持 PNG 载荷）。

ICO_SIZES="16 24 32 48 64 128 256"
for s in $ICO_SIZES; do render "$s" "$WORK/ico_$s.png"; done

python3 - "$OUT/icon.ico" "$WORK" $ICO_SIZES <<'PY' \
  || die "打包 icon.ico 失败" "先确认上一步的 PNG 都在：  ls -la \"$WORK\"/ico_*.png"
import struct, sys

dest, work, sizes = sys.argv[1], sys.argv[2], [int(a) for a in sys.argv[3:]]
blobs = [open("%s/ico_%d.png" % (work, s), "rb").read() for s in sizes]

offset = 6 + 16 * len(sizes)
entries, payload = [], []
for size, blob in zip(sizes, blobs):
    side = 0 if size >= 256 else size          # ICO 用 0 表示 256
    entries.append(struct.pack("<BBBBHHII", side, side, 0, 0, 1, 32, len(blob), offset))
    payload.append(blob)
    offset += len(blob)

with open(dest, "wb") as fh:
    fh.write(struct.pack("<HHH", 0, 1, len(sizes)))
    for e in entries:
        fh.write(e)
    for p in payload:
        fh.write(p)
PY

# ── 核对 ────────────────────────────────────────────────────────────────────
# 生成了不等于生成对了。逐个读回真实像素尺寸，对不上就红。

printf '\n核对产物尺寸\n'
FAILED=0

check_png() {
  rel="$1"; want="$2"; path="$OUT/$rel"
  [ -f "$path" ] || { printf '  ✗ %-16s 文件不存在\n' "$rel"; FAILED=1; return; }
  w="$(sips -g pixelWidth  "$path" | awk '/pixelWidth/{print $2}')"
  h="$(sips -g pixelHeight "$path" | awk '/pixelHeight/{print $2}')"
  if [ "$w" = "$want" ] && [ "$h" = "$want" ]; then
    printf '  ✓ %-16s %sx%s\n' "$rel" "$w" "$h"
  else
    printf '  ✗ %-16s 实际 %sx%s，预期 %sx%s\n' "$rel" "$w" "$h" "$want" "$want"
    FAILED=1
  fi
}

check_png "32x32.png"      32
check_png "128x128.png"    128
check_png "128x128@2x.png" 256
check_png "icon.png"       1024

# icns：sips 读回来的是最大那张表示的尺寸
if [ -f "$OUT/icon.icns" ]; then
  w="$(sips -g pixelWidth "$OUT/icon.icns" | awk '/pixelWidth/{print $2}')"
  if [ "$w" = "1024" ]; then
    printf '  ✓ %-16s 最大表示 %sx%s（含 16/32/128/256/512 及各自 @2x）\n' "icon.icns" "$w" "$w"
  else
    printf '  ✗ %-16s 最大表示 %s，预期 1024\n' "icon.icns" "$w"; FAILED=1
  fi
else
  printf '  ✗ %-16s 文件不存在\n' "icon.icns"; FAILED=1
fi

# ico：读 ICONDIR 把每张的边长列出来
if ! python3 - "$OUT/icon.ico" $ICO_SIZES <<'PY'
import struct, sys
path, want = sys.argv[1], [int(a) for a in sys.argv[2:]]
raw = open(path, "rb").read()
_, kind, count = struct.unpack("<HHH", raw[:6])
got = []
for i in range(count):
    side = raw[6 + 16 * i]
    got.append(256 if side == 0 else side)
ok = kind == 1 and got == want
print("  %s %-16s %s" % ("✓" if ok else "✗", "icon.ico",
      "、".join("%dx%d" % (s, s) for s in got) if ok
      else "实际 %s，预期 %s" % (got, want)))
sys.exit(0 if ok else 1)
PY
then
  FAILED=1
fi

if [ "$FAILED" -ne 0 ]; then
  die "产物核对未通过" \
    "先看上面哪一行是 ✗，再重跑：  ./scripts/gen/gen-icons.sh" \
    "如果反复不过，多半是 design/icon/duet.svg 的 viewBox 不是 0 0 1024 1024。"
fi

printf '\n✓ 图标产物已写入 %s\n' "${OUT#"$ROOT"/}"
printf '  改图标请改 %s 后重跑本脚本，不要直接改 PNG。\n' "${SRC#"$ROOT"/}"
