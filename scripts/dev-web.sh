#!/usr/bin/env bash
# 默认开发形态：duetd + vite，浏览器打开 http://localhost:5173
# 不需要 Rust 工具链。这是 AI 自测的主要通道。
set -euo pipefail
cd "$(dirname "$0")/.."

export DUET_DEV_TOKEN="${DUET_DEV_TOKEN:-dev-local-token}"
export DUET_PORT="${DUET_PORT:-7777}"

if [[ ! -f backend/go.mod ]]; then
  echo "· backend/go.mod 尚未创建，无法启动 duetd（见 docs/roadmap.md M0-0.8）"
  exit 1
fi

cleanup() { [[ -n ${DUETD_PID:-} ]] && kill "$DUETD_PID" 2>/dev/null || true; }
trap cleanup EXIT

echo "→ duetd  http://127.0.0.1:$DUET_PORT"
(cd backend && go run ./cmd/duetd serve --port "$DUET_PORT" --dev) &
DUETD_PID=$!

if [[ -f frontend/package.json ]]; then
  echo "→ vite   http://localhost:5173"
  (cd frontend && pnpm dev)
else
  echo "· frontend 尚未脚手架，只跑 duetd"
  wait "$DUETD_PID"
fi
