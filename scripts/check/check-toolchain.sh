#!/usr/bin/env bash
# 校验工具链版本声明自洽，且本地与 CI 用的是同一套。
#
# 为什么需要：本仓库的 CI 曾经**从来没绿过**，三个失败全是工具链版本问题，
# 而 `make check` 在本地一路全绿——因为这类问题在本地物理上看不见：
#
#   ① pnpm/action-setup 找不到版本声明（仓库根没有 package.json）
#   ② golangci-lint 用 go1.24 构建，读不了 go.mod 里的 go 1.26
#   ③ tauri.conf.json 用了 v1 的 bundle target
#
# 「本地绿 CI 红」是最消耗人的一种状态：AI 会以为是环境抽风而反复重跑，
# 或者更糟——开始怀疑自己的改动，把好代码改坏。
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

fail=0
say() { fail=1; echo "✗ $1"; printf '%s\n' "$2" | while IFS= read -r l; do [[ -n $l ]] && echo "    $l"; done; echo; }

# ── ① pnpm 版本：各子项目声明一致，且 CI 指得到 ──────────────
pkgs=$({ find . -maxdepth 2 -name package.json -not -path './node_modules/*' -not -path '*/node_modules/*' 2>/dev/null || true; } | sort)
missing_pm=""
versions=""
while IFS= read -r f; do
  [[ -z $f ]] && continue
  pm=$(python3 -c "import json,sys;print(json.load(open(sys.argv[1])).get('packageManager',''))" "$f")
  if [[ -z $pm ]]; then
    missing_pm+="$f"$'\n'
  else
    versions+="$pm"$'\n'
  fi
done <<<"$pkgs"

if [[ -n $missing_pm ]]; then
  say "有 package.json 没声明 packageManager" \
"$missing_pm
pnpm/action-setup 靠这个字段确定版本。缺了它 CI 会报
「Error: No pnpm version is specified」——而本地因为已经装好 pnpm，完全看不出问题。"
fi

uniq_versions=$(printf '%s' "$versions" | sort -u | grep -v '^$' || true)
if [[ $(printf '%s\n' "$uniq_versions" | grep -c . || true) -gt 1 ]]; then
  say "各子项目声明的 pnpm 版本不一致" \
"$uniq_versions
不同版本会产出不兼容的 lockfile，--frozen-lockfile 在 CI 上直接失败。"
fi

# CI 里每个 pnpm/action-setup 都必须指定 package_json_file 或 version
bad_setup=$(
  { grep -rn -A1 'uses: pnpm/action-setup' .github/workflows/*.yml 2>/dev/null || true; } \
  | grep -B0 'uses: pnpm/action-setup' | while IFS= read -r hit; do
      file="${hit%%:*}"; rest="${hit#*:}"; line="${rest%%:*}"
      # 看整个 step 块，不只是下一行 —— 中间可能夹着 `if:` 之类的键
      block=$(sed -n "$((line + 1)),$((line + 4))p" "$file" | sed -n '/^      - /q;p')
      if [[ $block != *package_json_file* && $block != *version* ]]; then
        echo "    $file:$line"
      fi
    done
)
if [[ -n $bad_setup ]]; then
  say "CI 里的 pnpm/action-setup 没指定版本来源" \
"$bad_setup
仓库根没有 package.json，action 找不到 packageManager 就会失败。
加一行： with: { package_json_file: <子项目>/package.json }"
fi

# ── ② golangci-lint：action 版本 + 工具版本必须写死且够新 ────
if [[ -f backend/go.mod ]]; then
  go_ver=$(awk '/^go /{print $2}' backend/go.mod)
  action_ver=$({ grep -oE 'golangci/golangci-lint-action@v[0-9]+' .github/workflows/ci.yml || true; } | head -1)
  tool_ver=$({ grep -A4 'golangci-lint-action' .github/workflows/ci.yml | grep -oE 'version: v[0-9.]+' || true; } | head -1)
  cfg_ver=$({ grep -oE '^version: "[0-9]+"' backend/.golangci.yml || true; } | head -1)

  if [[ -z $tool_ver ]]; then
    say "CI 没写死 golangci-lint 的版本" \
"不写死就用 action 自带的默认版本，而它可能是用比 go.mod（go ${go_ver}）更旧的 Go 构建的，
报「the Go language version used to build golangci-lint is lower than the targeted Go version」。
这条只在 CI 上出现，本地装的版本通常是新的，完全看不见。"
  fi
  # .golangci.yml 是 version: "2" 时，action 必须 ≥ v7
  if [[ $cfg_ver == 'version: "2"' ]]; then
    num="${action_ver##*@v}"
    if [[ -n $num && $num -lt 7 ]]; then
      say "golangci-lint-action 版本太旧" \
"backend/.golangci.yml 是 version: \"2\"（golangci-lint v2 的配置格式），
但 CI 用的是 $action_ver —— v6 及以下不认这个格式。升到 @v8。"
    fi
  fi
fi

# ── ③ Tauri：bundle target 必须是 v2 的取值 ──────────────────
readonly TAURI_CONF=shell/src-tauri/tauri.conf.json
if [[ -f $TAURI_CONF ]]; then
  bad_target=$(python3 - "$TAURI_CONF" <<'PY'
import json, sys
# Tauri v2 的合法 bundle target。"updater" 是 **v1** 的写法，
# v2 改成了 bundle.createUpdaterArtifacts —— 用错了只在 cargo build 时才炸，
# 报的还是一句看不懂的 "data did not match any variant of untagged enum BundleTargetInner"。
VALID = {"all", "default", "app", "dmg", "deb", "rpm", "appimage",
         "msi", "nsis", "pacman"}
d = json.load(open(sys.argv[1]))
targets = d.get("bundle", {}).get("targets", [])
if isinstance(targets, str):
    targets = [targets]
for t in targets:
    if t not in VALID:
        print(t)
PY
)
  if [[ -n $bad_target ]]; then
    say "tauri.conf.json 的 bundle.targets 有非法取值" \
"$bad_target
Tauri v2 的合法取值：all default app dmg deb rpm appimage msi nsis pacman。
特别注意：\"updater\" 是 **v1** 的写法，v2 要用 bundle.createUpdaterArtifacts: true。
用错了只在 cargo build 时炸，报的是看不懂的
「data did not match any variant of untagged enum BundleTargetInner」。"
  fi
fi

if [[ $fail -eq 1 ]]; then
  echo "这类问题**只在 CI 上出现**，本地 make check 结构上看不见。"
  echo "「本地绿 CI 红」最消耗人：会让人以为是环境抽风而反复重跑，或者开始怀疑自己的改动。"
  exit 1
fi

echo "✓ 工具链版本声明自洽（pnpm · golangci-lint · Tauri bundle target）"
