#!/usr/bin/env bash
# 只编 duetd sidecar（不编前端）。给 cargo clippy / cargo test 用。
#
# 为什么单独拆出来：tauri.conf.json 的 externalBin 声明了
# `binaries/duetd-<rust-triple>`，文件不在时**任何** cargo 命令都会失败——
# 包括只想跑 clippy 的时候。而 scripts/release/build.sh 会连前端一起编，
# 为了跑一次 lint 去编一遍前端太浪费。
set -euo pipefail
cd "$(dirname "$0")/../.."

if [[ ! -f backend/go.mod ]]; then
  echo "· 跳过 sidecar：backend/go.mod 尚未创建"
  exit 0
fi

if command -v rustc >/dev/null 2>&1; then
  TARGET=$(rustc -vV | awk '/^host:/{print $2}')
else
  case "$(go env GOOS)-$(go env GOARCH)" in
    darwin-arm64) TARGET=aarch64-apple-darwin ;;
    darwin-amd64) TARGET=x86_64-apple-darwin ;;
    linux-amd64)  TARGET=x86_64-unknown-linux-gnu ;;
    linux-arm64)  TARGET=aarch64-unknown-linux-gnu ;;
    *) echo "✗ 无法推断 rust triple" >&2; exit 1 ;;
  esac
fi

out="shell/src-tauri/binaries/duetd-$TARGET"
mkdir -p "$(dirname "$out")"

# 已经比源码新就不重编 —— lint 是高频操作，每次重编 12MB 二进制太慢
if [[ -x $out ]] && [[ -z $(find backend -name '*.go' -newer "$out" -print -quit 2>/dev/null) ]]; then
  echo "· sidecar 已是最新：$out"
  exit 0
fi

echo "→ sidecar → $out"
(cd backend && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "../$out" ./cmd/duetd)
