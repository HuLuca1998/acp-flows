#!/usr/bin/env bash
# 解除下载来的 Duet.app 的隔离标记。
#
# 没有 Apple 开发者证书时，产物是 ad-hoc 签名（adr/0007 修订 5）——
# Gatekeeper 会显示「未认证开发者」。这个脚本去掉 quarantine 属性，
# 让它能正常打开。
#
#   bash scripts/install-app.sh ~/Downloads/Duet.app
set -euo pipefail

app="${1:-/Applications/Duet.app}"
if [[ ! -d $app ]]; then
  echo "✗ 找不到 $app" >&2
  echo "  用法: $0 <Duet.app 的路径>" >&2
  exit 1
fi

echo "→ 解除隔离标记: $app"
xattr -dr com.apple.quarantine "$app"

echo "→ 校验签名"
codesign --verify --deep --strict "$app" 2>&1 | head -3 || true

cat <<'TIP'

✓ 完成。现在可以正常打开了。

这一步是必要的，因为当前版本没有 Apple 公证（需要 $99/年的开发者账号）。
更新包本身经过 minisign 签名校验，安全性不受影响——
Apple 公证解决的是「首次安装的信任提示」，不是更新链路的完整性。
TIP
