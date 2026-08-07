#!/usr/bin/env bash
# 由 api/openapi.yaml 生成 Go 服务端接口与 TS 类型。
# 铁律 2：改接口的顺序永远是 改 spec → 跑本脚本 → 改实现。
#
# 生成物（★ 都不许手改，改了下次 make gen 会覆盖，且 CI 的 check-gen 会红）：
#   backend/internal/api/gen/api.gen.go   Go ServerInterface + 类型 + 内嵌 spec
#   frontend/src/api/gen/schema.d.ts      TS paths/operations/components 类型
#
# 版本写死在这里，不用 @latest：
# 生成器版本一变，生成物就变，CI 的 check-gen 会在一个与本次改动无关的 PR 上炸。
set -euo pipefail
cd "$(dirname "$0")/../.."

readonly OAPI_CODEGEN_VERSION=v2.4.1
readonly SPEC=api/openapi.yaml

if [[ ! -f $SPEC ]]; then
  echo "· 跳过：$SPEC 尚未创建"
  exit 0
fi

fail=0

# ── Go：oapi-codegen ──────────────────────────────────────────
if [[ -f backend/go.mod ]]; then
  echo "→ Go   backend/internal/api/gen/api.gen.go"
  mkdir -p backend/internal/api/gen
  (
    cd backend
    # 3.1 的警告是已知的：oapi-codegen 尚未完整支持 3.1，但我们用到的
    # 构造（object/string/enum/${ref}）在 3.0 与 3.1 里语义相同，实测生成正确。
    # 不要因为这条警告把 spec 降级到 3.0 —— 见 api/AGENTS.md。
    go run "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@${OAPI_CODEGEN_VERSION}" \
      --config ../api/oapi-codegen.yaml ../api/openapi.yaml 2>&1 \
      | grep -v 'WARNING: You are using an OpenAPI 3.1' || true
  ) || fail=1
  # 生成物必须能编译——生成成功但编译不过等于没生成
  (cd backend && go build ./internal/api/gen/) || {
    echo "✗ 生成的 Go 代码编译不过。多半是 spec 用了 oapi-codegen 不支持的构造。" >&2
    fail=1
  }
else
  echo "· 跳过 Go：backend/go.mod 尚未创建"
fi

# ── TS：openapi-typescript ────────────────────────────────────
if [[ -f frontend/package.json ]]; then
  echo "→ TS   frontend/src/api/gen/schema.d.ts"
  mkdir -p frontend/src/api/gen
  (
    cd frontend
    pnpm exec openapi-typescript ../api/openapi.yaml -o src/api/gen/schema.d.ts
  ) || fail=1
else
  echo "· 跳过 TS：frontend/package.json 尚未创建"
fi

if [[ $fail -eq 1 ]]; then
  echo
  echo "✗ 代码生成失败。不要手写生成物绕过它——那会让契约与实现永久脱钩。" >&2
  exit 1
fi

echo "✓ 生成完成。生成物不许手改；要改接口就改 $SPEC 再跑一次。"
