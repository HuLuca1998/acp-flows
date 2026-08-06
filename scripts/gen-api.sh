#!/usr/bin/env bash
# 由 api/openapi.yaml 生成 Go 服务端接口与 TS 客户端。
# 铁律 2：改接口的顺序永远是 改 spec → 跑本脚本 → 改实现。
#
# 生成物（都不许手改）：
#   backend/internal/api/gen/    Go ServerInterface + 类型
#   frontend/src/api/gen/        TS client + 类型
#   frontend/tests/msw/          MSW mock handlers
set -euo pipefail
cd "$(dirname "$0")/.."

if [[ ! -f api/openapi.yaml ]]; then
  echo "· 跳过：api/openapi.yaml 尚未创建"
  exit 0
fi

echo "TODO(M0-0.8)：接入代码生成器"
echo "  Go  → oapi-codegen（生成 ServerInterface，用标准库 net/http）"
echo "  TS  → openapi-typescript + openapi-fetch"
echo "  MSW → openapi-msw"
echo
echo "生成物路径见本脚本头部注释。接入后删掉这段提示。"
