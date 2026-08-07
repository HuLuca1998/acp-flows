#!/usr/bin/env bash
# 删掉开发库并重建。开发期最省事的「回滚」——比写 down 迁移可靠得多。
#
# ★ 只碰开发数据目录。用户真实数据在 ~/.acpflows，本脚本一步都不会走到那里
#   （铁律 6）。生产环境的重置走 `duetd --reset`，那条路径有备份与二次确认。
set -euo pipefail
cd "$(dirname "$0")/../.."

readonly DEV_HOME="${DUET_DATA_DIR:-$HOME/.duet-dev}"
readonly DB_DIR="$DEV_HOME/.acpflows"          # DUET_DATA_DIR 是家目录替身，见 pitfalls P-16
readonly REAL_HOME="$HOME/.acpflows"

# 守卫：路径算错时宁可不删。这个脚本删的是目录，算错一次就是用户数据没了。
if [[ "$DB_DIR" == "$REAL_HOME" ]]; then
  echo "✗ 拒绝执行：解析出的路径是用户真实数据目录 $REAL_HOME" >&2
  echo "  DUET_DATA_DIR=$DEV_HOME —— 它应该指向开发目录，不是家目录" >&2
  exit 1
fi
case "$DB_DIR" in
  "$HOME"|"$HOME/"|/|"") echo "✗ 拒绝执行：路径 '$DB_DIR' 不安全" >&2; exit 1 ;;
esac

if [[ ! -d $DB_DIR ]]; then
  echo "· 开发库不存在（$DB_DIR），无需重置"
  exit 0
fi

# 有服务在跑时先停：删掉正在被写的库会留下 -wal/-shm 残骸，
# 下次启动可能拿到一个半死不活的库，症状极难排查。
if bash scripts/dev/services.sh status 2>/dev/null | grep -q '运行中'; then
  echo "· 检测到服务在跑，先停掉"
  bash scripts/dev/services.sh stop all
fi

rm -f "$DB_DIR"/duet.db "$DB_DIR"/duet.db-wal "$DB_DIR"/duet.db-shm
echo "✓ 已删除开发库：$DB_DIR/duet.db（含 -wal / -shm）"
echo "  下次 make dev 启动时会从 0 跑全部迁移并重新 seed。"
