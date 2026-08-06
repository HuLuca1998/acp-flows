#!/usr/bin/env bash
# 编 duetd + 前端 dist。
#   scripts/build.sh [--target <rust-triple>]
#
# duetd 会作为 Tauri sidecar 使用，文件名必须带 target triple 后缀（Tauri 约定）。
set -euo pipefail
cd "$(dirname "$0")/.."

TARGET=""
[[ ${1:-} == "--target" ]] && TARGET="${2:-}"

case "$TARGET" in
  aarch64-apple-darwin) GOOS=darwin GOARCH=arm64 ;;
  x86_64-apple-darwin)  GOOS=darwin GOARCH=amd64 ;;
  "")                   GOOS=$(go env GOOS) GOARCH=$(go env GOARCH) ;;
  *) echo "✗ 未知 target: $TARGET" >&2; exit 1 ;;
esac

if [[ -f backend/go.mod ]]; then
  out="shell/src-tauri/binaries/duetd${TARGET:+-$TARGET}"
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
