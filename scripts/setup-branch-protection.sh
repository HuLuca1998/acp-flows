#!/usr/bin/env bash
# 一次性配置 main 分支保护。规则见 docs/git-workflow.md §5。
#
# ★ required check 只认 CI 的汇总门禁 "ci"，不认单个 job——
#   单个 job 被路径过滤跳过时会让 PR 永远 pending。见 docs/ci.md 规则 2。
set -euo pipefail

REPO="${1:-HuLuca1998/acp-flows}"
echo "配置 $REPO 的 main 分支保护…"

gh api -X PUT "repos/$REPO/branches/main/protection" \
  --input - <<'JSON'
{
  "required_status_checks": { "strict": true, "contexts": ["ci"] },
  "enforce_admins": false,
  "required_pull_request_reviews": {
    "required_approving_review_count": 0,
    "dismiss_stale_reviews": true
  },
  "restrictions": null,
  "required_linear_history": true,
  "allow_force_pushes": false,
  "allow_deletions": false,
  "required_conversation_resolution": true
}
JSON

gh api -X PATCH "repos/$REPO" \
  -f allow_squash_merge=true \
  -F allow_merge_commit=false \
  -F allow_rebase_merge=false \
  -F delete_branch_on_merge=true \
  --silent

echo "✓ 完成：仅 squash · 线性历史 · required check = ci · 合并后删分支"
