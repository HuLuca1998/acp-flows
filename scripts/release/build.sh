#!/usr/bin/env bash
# 编 duetd + 前端 dist。
#   scripts/release/build.sh [--target <rust-triple>]
#
# duetd 会作为 Tauri sidecar 使用，文件名必须带 target triple 后缀（Tauri 约定）。
set -euo pipefail
cd "$(dirname "$0")/../.."

TARGET=""
if [[ ${1:-} == "--target" ]]; then
  TARGET="${2:-}"
fi

# ★ 不给 --target 时用**宿主的 rust triple**，而不是产出一个不带后缀的文件。
# Tauri 的 externalBin 只认 `<name>-<triple>`；产出裸的 `duetd` 会让
# cargo build 报 `resource path binaries/duetd-<triple> doesn't exist`，
# 而那条报错离「build.sh 少写了个后缀」很远，很难联想到。
if [[ -z $TARGET ]]; then
  if command -v rustc >/dev/null 2>&1; then
    TARGET=$(rustc -vV | awk '/^host:/{print $2}')
  else
    # 没装 Rust 时按 Go 的 GOOS/GOARCH 推，够本地用
    case "$(go env GOOS)-$(go env GOARCH)" in
      darwin-arm64) TARGET=aarch64-apple-darwin ;;
      darwin-amd64) TARGET=x86_64-apple-darwin ;;
      linux-amd64)  TARGET=x86_64-unknown-linux-gnu ;;
      linux-arm64)  TARGET=aarch64-unknown-linux-gnu ;;
      *) echo "✗ 无法推断 rust triple，请显式传 --target" >&2; exit 1 ;;
    esac
  fi
fi

case "$TARGET" in
  aarch64-apple-darwin)      GOOS=darwin GOARCH=arm64 ;;
  x86_64-apple-darwin)       GOOS=darwin GOARCH=amd64 ;;
  x86_64-unknown-linux-gnu)  GOOS=linux  GOARCH=amd64 ;;
  aarch64-unknown-linux-gnu) GOOS=linux  GOARCH=arm64 ;;
  *) echo "✗ 未知 target: $TARGET" >&2; exit 1 ;;
esac

if [[ -f backend/go.mod ]]; then
  out="shell/src-tauri/binaries/duetd-$TARGET"
  mkdir -p "$(dirname "$out")"
  echo "→ duetd → $out"
  (cd backend && GOOS=$GOOS GOARCH=$GOARCH CGO_ENABLED=0 \
     go build -trimpath -ldflags="-s -w" -o "../$out" ./cmd/duetd)
else
  echo "· 跳过 duetd：backend/go.mod 尚未创建"
fi

if [[ -f frontend/package.json ]]; then
  echo "→ 前端 dist"
  (cd frontend && pnpm install --frozen-lockfile && pnpm build)
else
  echo "· 跳过前端：frontend/package.json 尚未创建"
fi
