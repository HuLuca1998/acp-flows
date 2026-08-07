#!/usr/bin/env bash
# 检查许可协议没有被悄悄改掉。
#
# ★ 判定逻辑在 lib/license.py，**不在这里**。
# 理由与 check-commit-msg.sh 同源：C locale 下 grep/awk 对中文与 © 这类
# 多字节字符按字节处理，写在 shell 里的匹配会静静放过该红的情况。踩过三次。
set -euo pipefail
exec python3 "$(dirname "$0")/lib/license.py" "$(dirname "$0")/../.."
