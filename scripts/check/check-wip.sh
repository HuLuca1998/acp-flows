#!/usr/bin/env bash
# 挡住「半成品被提交上去」。
#
# ★ 真踩过两次，同一天（2026-08-08）：
#   ① 把还没实现的 SettingsNav.test.tsx 推上 PR，CI 报了一轮红
#   ② 用 git add -A 提交一条 CI 紧急修复时，顺手把整个 U2.1.1 的半成品
#      带上了 main，发版门禁直接失败
#
# 两次都是 `git add -A` 不看改了什么就提交。靠记性不行，所以变成检查。
#
# 判据是**可编译性**，不是"看起来像不像写完了"：
#   - 有 *_test.go 却没有任何非测试 .go → 那个包根本编不过
#   - 有 *.test.tsx 却没有同名实现文件 → import 会直接失败
#
# 这两条都能在提交前几百毫秒内查出来，而在 CI 上要等好几分钟。
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

fail=0

# ── 1. Go：有测试文件的包必须有非测试源码 ─────────────────────
orphan_go=$(python3 - <<'PY' || true
import collections
import pathlib

pkgs = collections.defaultdict(lambda: {"test": [], "src": 0})
for p in pathlib.Path("backend").rglob("*.go"):
    if any(part in {"gen", "node_modules"} for part in p.parts):
        continue
    d = str(p.parent)
    if p.name.endswith("_test.go"):
        pkgs[d]["test"].append(p.name)
    else:
        pkgs[d]["src"] += 1

for d, info in sorted(pkgs.items()):
    if info["test"] and info["src"] == 0:
        print(f"{d}/  只有测试没有实现：{', '.join(sorted(info['test']))}")
PY
)
if [[ -n $orphan_go ]]; then
  fail=1
  echo "✗ 有 Go 包只有测试文件、没有任何实现——这个包编不过"
  printf '%s\n' "$orphan_go" | while IFS= read -r l; do [[ -n $l ]] && echo "    $l"; done
  echo "    要么把实现补上，要么先别提交这些测试（见 docs/rules/git-workflow.md）"
  echo
fi

# ── 2. 前端：组件测试必须有对应的实现文件 ─────────────────────
orphan_ts=$(python3 - <<'PY' || true
import pathlib

root = pathlib.Path("frontend/src")
if root.exists():
    for p in sorted(root.rglob("*.test.tsx")):
        stem = p.name[: -len(".test.tsx")]
        # 同目录下的 Xxx.tsx，或 index.tsx（整页测试常常测的是 index）
        if (p.parent / f"{stem}.tsx").exists() or (p.parent / "index.tsx").exists():
            continue
        print(f"{p}  找不到 {stem}.tsx")
PY
)
if [[ -n $orphan_ts ]]; then
  fail=1
  echo "✗ 有组件测试找不到被测组件——import 会直接失败"
  printf '%s\n' "$orphan_ts" | while IFS= read -r l; do [[ -n $l ]] && echo "    $l"; done
  echo
fi

if [[ $fail -eq 1 ]]; then exit 1; fi
echo "✓ 没有只有测试没有实现的半成品"
