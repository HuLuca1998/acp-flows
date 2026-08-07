#!/usr/bin/env bash
# 已退役 —— 起服务一律走 scripts/dev/services.sh。
#
# 原因：这个脚本用 trap + 前台阻塞的方式管进程，AI 用 Ctrl-C 或后台化调用时
# 经常留下孤儿进程占着端口。services.sh 改成 PID 文件 + 端口双重记账。
# 规范见 run-services skill。
set -euo pipefail
exec bash "$(dirname "$0")/services.sh" start all
