#!/usr/bin/env bash
# 在本机打包并发布一个版本。
#
#   bash scripts/release/publish-local.sh 0.0.1              # 正式版，会成为一键更新的目标
#   bash scripts/release/publish-local.sh 0.0.1 --dry-run    # 只构建不发布
#
# ★ **为什么在本机而不在 GitHub Actions**（2026-08-08 改）：
# macOS runner 按 **10 倍**计费，而 universal 包要编两个架构的 Rust。
# 一次失败的发版就是十几分钟额度，前三次真发版里有两次挂在环境上
# （rust target 被 rust-cache 覆盖、门禁红）。本机编译不花额度、
# 出问题当场能看能改。release.yml 已 `gh workflow disable`，没有删除。
#
# 需要什么：
#   - Rust 工具链，且装了两个 target（脚本会自己补）
#   - Node + pnpm、Go
#   - minisign 私钥在 ~/.duet-updater/updater.key（0600）
#   - gh 已登录，且对本仓库有写权限
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

VERSION="${1:-}"
DRY_RUN=0
[[ "${2:-}" == "--dry-run" ]] && DRY_RUN=1

if [[ -z $VERSION ]]; then
  echo "用法: bash scripts/release/publish-local.sh <版本号> [--dry-run]"
  echo "  例: bash scripts/release/publish-local.sh 0.0.1"
  exit 1
fi
if [[ ! $VERSION =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
  echo "✗ 版本号要形如 0.0.1 或 0.1.0-beta.1，收到: ${VERSION}"
  exit 1
fi

TAG="v${VERSION}"
KEY_PATH="${DUET_SIGNING_KEY:-$HOME/.duet-updater/updater.key}"
CONF=shell/src-tauri/tauri.conf.json
BUNDLE=shell/src-tauri/target/universal-apple-darwin/release/bundle

say() { echo; echo "── $1 ────────────────────────────────"; }

# ── 0. 前置检查：全在编译之前做完 ────────────────────────────
# 编译要好几分钟。任何一条前置不满足都应当**立刻**报出来，
# 而不是等编完了才发现签不了名。
say "前置检查"

for cmd in git gh jq cargo rustup pnpm go; do
  command -v "$cmd" >/dev/null 2>&1 || { echo "✗ 缺少 ${cmd}"; exit 1; }
done

if [[ ! -f $KEY_PATH ]]; then
  echo "✗ 找不到签名私钥: ${KEY_PATH}"
  echo "  没有它签不出 .sig，已安装的客户端会**拒绝**这个更新。"
  echo "  生成方式见 docs/spec/release-and-update.md。"
  exit 1
fi

if [[ $DRY_RUN -eq 0 ]] && git rev-parse -q --verify "refs/tags/${TAG}" >/dev/null; then
  echo "✗ tag ${TAG} 已存在——**不要重打已发布的 tag**（M1 的 forbidden_changes）。"
  echo "  已经装了旧包的用户会收到一个内容变了但版本号没变的更新。"
  exit 1
fi

if [[ -n $(git status --porcelain) ]]; then
  echo "✗ 工作区不干净。发版必须从一个确定的提交出发，"
  echo "  否则事后没人说得清这个包到底是哪份代码编出来的："
  git status --short | head -10
  exit 1
fi

echo "✓ 版本 ${VERSION} · tag ${TAG} · 私钥 ${KEY_PATH}"

# ── 1. 版本号只在发布时注入 ──────────────────────────────────
# 仓库里的 version 始终是 0.0.0，避免每次发版产生一次 bump 提交
# （adr/0007 修订 2）。所以这里改完**一定要还原**，包括失败时。
say "注入版本号"
cp "$CONF" "${CONF}.bak"
restore_conf() {
  if [[ -f "${CONF}.bak" ]]; then
    mv "${CONF}.bak" "$CONF"
    echo "· 已还原 ${CONF} 的 version"
  fi
}
trap restore_conf EXIT

tmp=$(mktemp)
jq --arg v "$VERSION" '.version = $v' "$CONF" > "$tmp"
mv "$tmp" "$CONF"
echo "✓ tauri.conf.json version = $(jq -r '.version' "$CONF")"

# ── 2. 两个架构的 sidecar ────────────────────────────────────
# universal 包是两个真实架构的合并，两个 duetd 都得在。
say "编译 duetd 与前端"
rustup target add aarch64-apple-darwin x86_64-apple-darwin >/dev/null
bash scripts/release/build.sh --target aarch64-apple-darwin
bash scripts/release/build.sh --target x86_64-apple-darwin --skip-frontend

# ── 3. 打包 + 签名 ───────────────────────────────────────────
# ad-hoc 签名（identity "-"）：免费、不需要 Apple 账号。
# Apple Silicon 要求 arm64 可执行文件必须有签名，否则 Gatekeeper 直接
# 判「已损坏」而不是给出「未认证开发者」的可绕过提示（adr/0007 修订 5）。
say "打包（ad-hoc 签名，未公证）"
(
  cd shell
  TAURI_SIGNING_PRIVATE_KEY="$(cat "$KEY_PATH")" \
  TAURI_SIGNING_PRIVATE_KEY_PASSWORD="${DUET_SIGNING_PASSWORD:-}" \
  APPLE_SIGNING_IDENTITY='-' \
    pnpm tauri build --target universal-apple-darwin
)

# ── 4. 产物齐不齐，在这里断言 ────────────────────────────────
# 少一类就是「用户装得上但更新不了」，而那要到下一次发版才会暴露。
say "校验产物"
DMG=$(find "$BUNDLE/dmg" -name '*.dmg' -maxdepth 1 | head -1)
TARBALL=$(find "$BUNDLE/macos" -name '*.app.tar.gz' -maxdepth 1 | head -1)
SIG=$(find "$BUNDLE/macos" -name '*.app.tar.gz.sig' -maxdepth 1 | head -1)

for pair in "dmg:$DMG" "tar.gz:$TARBALL" "sig:$SIG"; do
  name="${pair%%:*}"; path="${pair#*:}"
  [[ -n $path && -f $path ]] || { echo "✗ 缺少产物: ${name}"; exit 1; }
  echo "✓ ${name}: $(basename "$path")"
done

# ── 5. latest.json —— 一键更新读的就是它 ─────────────────────
# ★ 这个文件是更新链路的全部。字段拼错、签名放错位置，表现都是
# 「用户点了更新，什么也没发生」——而客户端不会告诉你为什么。
say "生成 latest.json"
PUB_URL="https://github.com/HuLuca1998/acp-flows/releases/download/${TAG}/$(basename "$TARBALL")"
LATEST=$(mktemp -d)/latest.json
jq -n \
  --arg version "$VERSION" \
  --arg pub_date "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --arg sig "$(cat "$SIG")" \
  --arg url "$PUB_URL" \
  '{
     version: $version,
     notes: "见 Release 页面",
     pub_date: $pub_date,
     platforms: { "darwin-universal": { signature: $sig, url: $url } }
   }' > "$LATEST"

# 版本号必须与 tag 一致：不一致的话客户端会反复提示同一个更新
[[ "$(jq -r '.version' "$LATEST")" == "$VERSION" ]] || { echo "✗ latest.json 版本号不对"; exit 1; }
echo "✓ latest.json（darwin-universal）"

if [[ $DRY_RUN -eq 1 ]]; then
  say "dry run 结束"
  echo "产物在 ${BUNDLE}"
  echo "latest.json 在 ${LATEST}"
  echo "没有创建 tag，没有发布。"
  exit 0
fi

# ── 6. 发布 ──────────────────────────────────────────────────
say "发布到 GitHub"
git tag -a "$TAG" -m "Duet ${VERSION}"
git push origin "$TAG"

notes=$(bash scripts/release/release-notes.sh "$TAG" 2>/dev/null || echo "Duet ${VERSION}")
gh release create "$TAG" \
  --title "Duet ${VERSION}" \
  --notes "$notes" \
  "$DMG" "$TARBALL" "$SIG" "$LATEST"

say "完成"
gh release view "$TAG" --json assets --jq '.assets[].name'
echo
echo "装它： bash scripts/release/install-app.sh <解压出来的 Duet.app>"
