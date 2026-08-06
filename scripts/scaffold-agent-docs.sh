#!/usr/bin/env bash
# 从模板为指定目录生成 AGENTS.md + CLAUDE.md 骨架。
# 用法： scripts/scaffold-agent-docs.sh backend/internal/store
#
# 已存在的文件不会被覆盖。
set -euo pipefail

cd "$(dirname "$0")/.."

dir="${1:-}"
if [[ -z $dir ]]; then
  echo "用法: $0 <目录>" >&2
  exit 2
fi
if [[ ! -d $dir ]]; then
  echo "目录不存在: $dir" >&2
  exit 2
fi

dir="${dir%/}"

for f in AGENTS.md CLAUDE.md; do
  target="$dir/$f"
  if [[ -f $target ]]; then
    echo "跳过（已存在）: $target"
    continue
  fi
  sed "s|{{DIR}}|$dir|g" "docs/templates/$f.tmpl" > "$target"
  echo "已生成: $target"
done

echo
echo "下一步：把 $dir/AGENTS.md 里的 TODO 填实，然后跑 make check-docs"
