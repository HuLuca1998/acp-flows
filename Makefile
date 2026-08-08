# Duet · 顶层任务入口
#
# 设计原则：子项目尚未脚手架时，相关 target 跳过而不是报错——
# 这样 `make check` 从第一天起就能跑通，不会因为"还没写代码"而失效。
# 一个跑不动的检查等于没有检查。

SHELL := /usr/bin/env bash
.DEFAULT_GOAL := help

LOG      ?= info

BACKEND  := backend
FRONTEND := frontend
SHELLDIR := shell
E2E      := e2e

has = $(shell [ -f $(1) ] && echo yes)

# ══ 帮助 ═══════════════════════════════════════════════════════
.PHONY: help
help: ## 显示所有可用命令
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

# ══ 总检查 ═════════════════════════════════════════════════════
.PHONY: check
check: check-ci-parity check-naming check-spec check-gen check-design-parity check-milestone-evidence check-license check-docs check-doc-commands check-doc-links check-doc-budget check-fanout check-milestones check-toolchain check-index check-icons check-commits check-wip check-merge lint test cover ## 提交前必跑：文档 + 索引 + 预算 + 提交信息 + lint + 全部测试

# ══ 文档完整性（根 AGENTS.md §4.1）═══════════════════════════════
.PHONY: check-ci-parity
check-ci-parity: ## 校验 make check 是 CI 的超集（本地绿了推上去不该红）
	@python3 scripts/check/lib/ci_parity.py .

.PHONY: check-naming
check-naming: ## 命名与文件组织规范（单文件行数、WaitDelay、品牌名……）
	@bash scripts/check/check-naming.sh

.PHONY: check-spec
check-spec: ## 校验 api/openapi.yaml 自身规范（CI 的 contract job 跑的就是它）
	@npx --yes @redocly/cli@1.34.5 lint api/openapi.yaml

.PHONY: check-design-parity
check-design-parity: ## 设计稿里的每个界面区块都在 design/PARITY.md 里表过态
	@python3 scripts/check/lib/design_parity.py .

.PHONY: check-milestone-evidence
check-milestone-evidence: ## 标了完成的里程碑，完成标志都留了实操证据
	@python3 scripts/check/lib/milestone_evidence.py .

.PHONY: check-license
check-license: ## 检查 LICENSE.md 没被裁过、版权人填实、README 说法一致
	@scripts/check/check-license.sh

.PHONY: check-docs
check-docs: ## 检查关键目录是否都有填实的 AGENTS.md + CLAUDE.md
	@bash scripts/check/check-agent-docs.sh

.PHONY: check-doc-commands
check-doc-commands: ## 文档里提到的 make 目标与脚本是否真实存在
	@bash scripts/check/check-doc-commands.sh

.PHONY: check-doc-links
check-doc-links: ## 文档里的相对链接指向的文件真实存在
	@bash scripts/check/check-doc-links.sh

.PHONY: check-doc-budget
check-doc-budget: ## 文档的上下文预算：L0 常驻 / L1 阶段 / L2 大文档读法块
	@bash scripts/check/check-doc-budget.sh

.PHONY: check-fanout
check-fanout: ## 目录扇出：平铺文件过多时逼着分包
	@bash scripts/check/check-dir-fanout.sh

.PHONY: check-milestones
check-milestones: ## 里程碑单元的四要素与验收标准断言是否齐备
	@bash scripts/check/check-milestones.sh

.PHONY: check-commits
check-commits: ## ★ 本分支相对 main 的提交信息格式（含「先红的测试」）
	@# 并进 make check 是因为真的漏跑过一次：commit 完直接 push，
	@# 到 CI 才发现 scope 用了取值表里没有的词，白跑一轮 CI。
	@bash scripts/check/check-commit-msg.sh

.PHONY: check-wip
check-wip: ## ★ 挡住半成品：只有测试没有实现的包
	@bash scripts/check/check-wip.sh

.PHONY: check-merge
check-merge: ## ★ 与 main 合并之后还能不能编译（本地绿 CI 红的常见来源）
	@bash scripts/check/check-merge-result.sh

.PHONY: check-toolchain
check-toolchain: ## ★ 工具链版本声明自洽（这类问题只在 CI 上出现）
	@bash scripts/check/check-toolchain.sh

.PHONY: docs-scaffold
docs-scaffold: ## 为目录生成文档骨架： make docs-scaffold DIR=backend/internal/store
	@bash scripts/gen/scaffold-agent-docs.sh "$(DIR)"

# ══ 索引一致性（挡住重复造轮子 / 重复写测试）══════════════════════
.PHONY: check-index
check-index: check-util-index check-test-index check-i18n ## 校验工具库索引、测试索引、i18n 词条

.PHONY: check-i18n
check-i18n: ## 中英词条 key 一致 + 无缺失/未使用（见 docs/rules/i18n.md）
	@bash scripts/check/check-i18n.sh

.PHONY: check-util-index
check-util-index: ## 工具库 INDEX.md 是否与实际导出函数一致
	@bash scripts/check/check-util-index.sh

.PHONY: check-test-index
check-test-index: ## 测试 INDEX.md 是否与实际测试一致
	@bash scripts/check/check-test-index.sh

# ══ 契约代码生成 ════════════════════════════════════════════════
.PHONY: gen
gen: ## 由 api/openapi.yaml 生成 Go 服务端接口与 TS 客户端（改完 spec 必跑）
	@bash scripts/gen/gen-api.sh

.PHONY: check-gen
check-gen: gen ## 校验生成物与 spec 一致（CI 用；有 diff 即失败）
	@bash scripts/check/check-gen.sh

# ══ 测试 ═══════════════════════════════════════════════════════
.PHONY: test
test: test-backend test-frontend test-shell ## 跑后端 + 前端 + 外壳全部测试

.PHONY: test-backend
test-backend: ## Go 测试（含 -race）
ifeq ($(call has,$(BACKEND)/go.mod),yes)
	cd $(BACKEND) && go test ./... -race -count=1
else
	@echo "· 跳过 test-backend：$(BACKEND)/go.mod 尚未创建"
endif

.PHONY: cover
cover: ## Go 覆盖率 + 门槛校验（domain 包 >= 90%）
ifeq ($(call has,$(BACKEND)/go.mod),yes)
	@# ★ -coverpkg 不能省：没有它时「A 包的测试执行了 B 包的代码」不计入 B 的覆盖率。
	@# 本仓库的 entity / mapper / migration 全部由 store 的测试驱动，
	@# 不加 -coverpkg 会显示 0% —— 那是测量假象，会逼着人去写没有意义的测试。
	cd $(BACKEND) && go test ./... -covermode=atomic -coverpkg=./internal/... -coverprofile=coverage.raw
	@# 生成物由 openapi.yaml 决定，人改不了，不该进覆盖率门槛
	@grep -v '/internal/api/gen/' $(BACKEND)/coverage.raw > $(BACKEND)/coverage.out
	@rm -f $(BACKEND)/coverage.raw
	@bash scripts/check/check-coverage.sh
else
	@echo "· 跳过 cover：$(BACKEND)/go.mod 尚未创建"
endif

.PHONY: test-frontend
test-frontend: ## Vitest 单测
ifeq ($(call has,$(FRONTEND)/package.json),yes)
	cd $(FRONTEND) && pnpm test --run
else
	@echo "· 跳过 test-frontend：$(FRONTEND)/package.json 尚未创建"
endif

.PHONY: test-e2e
test-e2e: ## Playwright 端到端（对着真实 duetd + Fake Runtime）
ifeq ($(call has,$(E2E)/package.json),yes)
	cd $(E2E) && pnpm test
else
	@echo "· 跳过 test-e2e：$(E2E)/package.json 尚未创建"
endif

# ══ Lint ══════════════════════════════════════════════════════
.PHONY: lint
lint: lint-backend lint-frontend lint-shell ## 全部 lint

.PHONY: lint-backend
lint-backend: ## go vet + golangci-lint（含 depguard 分层约束）
ifeq ($(call has,$(BACKEND)/go.mod),yes)
	cd $(BACKEND) && go vet ./...
	@# golangci-lint 是可选工具：本机没装时给出安装方式而不是让 make check 整个失效。
	@# CI 上用 golangci/golangci-lint-action，一定会跑到。
	@if command -v golangci-lint >/dev/null 2>&1; then \
		cd $(BACKEND) && golangci-lint run; \
	else \
		echo "· 跳过 golangci-lint（本机未安装）"; \
		echo "  装它：brew install golangci-lint  或  go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		echo "  注意：depguard 的分层约束只有它能查，CI 上一定会跑。"; \
	fi
else
	@echo "· 跳过 lint-backend：$(BACKEND)/go.mod 尚未创建"
endif

.PHONY: lint-frontend
lint-frontend: ## ESLint + Stylelint（含设计系统合规规则）+ tsc
ifeq ($(call has,$(FRONTEND)/package.json),yes)
	cd $(FRONTEND) && pnpm lint && pnpm typecheck
else
	@echo "· 跳过 lint-frontend：$(FRONTEND)/package.json 尚未创建"
endif

# ★ shell 的 lint/test 在 CI 上是必跑的。本地不跑的话，
# 「make check 全绿但 CI 红」就是结构性必然 —— 而那是最消耗人的状态。
# Rust 工具链没装时跳过并说明，不让 make check 整个失效。
#
# ★ 都依赖 sidecar：tauri.conf.json 的 externalBin 声明了 binaries/duetd-<triple>，
# 文件不在时 cargo build 直接失败，报的是
# `resource path binaries/duetd-<triple> doesn't exist`。
.PHONY: sidecar
sidecar: ## 编出 Tauri 需要的 duetd sidecar（带 rust triple 后缀）
	@bash scripts/release/build-sidecar.sh

.PHONY: lint-shell
lint-shell: ## cargo clippy（-D warnings）
ifeq ($(call has,$(SHELLDIR)/src-tauri/Cargo.toml),yes)
	@if command -v cargo >/dev/null 2>&1; then \
		bash scripts/release/build-sidecar.sh; \
		cd $(SHELLDIR)/src-tauri && cargo clippy --all-targets -- -D warnings; \
	else \
		echo "· 跳过 lint-shell（本机没有 Rust 工具链）"; \
		echo "  装它：https://rustup.rs  ——  CI 上一定会跑，本地不跑等于把问题推给 CI"; \
	fi
else
	@echo "· 跳过 lint-shell：$(SHELLDIR)/src-tauri/Cargo.toml 尚未创建"
endif

.PHONY: test-shell
test-shell: ## cargo test
ifeq ($(call has,$(SHELLDIR)/src-tauri/Cargo.toml),yes)
	@if command -v cargo >/dev/null 2>&1; then \
		bash scripts/release/build-sidecar.sh; \
		cd $(SHELLDIR)/src-tauri && cargo test; \
	else \
		echo "· 跳过 test-shell（本机没有 Rust 工具链）"; \
	fi
else
	@echo "· 跳过 test-shell：$(SHELLDIR)/src-tauri/Cargo.toml 尚未创建"
endif

# ══ 开发 ═══════════════════════════════════════════════════════
.PHONY: icons
icons: ## 由 design/icon/duet.svg 重新生成全部图标尺寸
	@bash scripts/gen/gen-icons.sh

.PHONY: check-icons
check-icons: ## 图标产物是否与源 SVG 同步
	@bash scripts/check/check-icons.sh

.PHONY: probe
probe: ## ★ 真机探针：零模型开销地核对 ACP Runtime 的真实行为
	cd $(BACKEND) && go run ./cmd/acpprobe --out=tests/fixtures/probe/codex.json  codex
	cd $(BACKEND) && go run ./cmd/acpprobe --out=tests/fixtures/probe/claude.json claude
	@echo "报告已更新。对照 docs/notes/acp-field-notes.md §7.1 核对差异。"

# ══ 本地服务（规范见 run-services skill）════════════════════════
# 端口写死：duetd 7777 · vite 5173。幂等启动、干净关闭。
# ★ 不要裸跑 go run / pnpm dev —— 那会绕过 PID 记账，导致进程越积越多。
.PHONY: dev
dev: ## ★ 起前后端（幂等）。调级别： make dev LOG=acp=trace
	@DUET_LOG="$(LOG)" bash scripts/dev/services.sh start all

.PHONY: dev-stop
dev-stop: ## ★ 停掉前后端。**用完必须停**
	@bash scripts/dev/services.sh stop all

.PHONY: dev-status
dev-status: ## 看谁在跑
	@bash scripts/dev/services.sh status

.PHONY: dev-logs
dev-logs: ## 跟踪后端日志
	@bash scripts/dev/services.sh logs backend

.PHONY: dev-restart
dev-restart: ## 重启（后端改代码后必须 —— go run 不会自动重载）
	@DUET_LOG="$(LOG)" bash scripts/dev/services.sh restart all

.PHONY: logs-db
logs-db: ## 查落库的日志（最近 30 条）。完整查询见 debug skill
	@sqlite3 -header -box "$${DUET_DATA_DIR:-$$HOME/.duet-dev}/.acpflows/duet.db" \
	  "SELECT seq, ts, CASE level WHEN -8 THEN 'TRACE' WHEN -4 THEN 'DEBUG' WHEN 0 THEN 'INFO' WHEN 4 THEN 'WARN' ELSE 'ERROR' END AS lv, component, msg FROM logs ORDER BY seq DESC LIMIT 30;"

.PHONY: db-reset
db-reset: ## 删掉开发库并重建（开发期最省事的"回滚"；不碰 ~/.acpflows）
	@bash scripts/dev/db-reset.sh

.PHONY: dev-web
dev-web: dev ## dev 的别名（历史文档里用过这个名字）

.PHONY: dev-app
dev-app: ## Tauri 壳联调（需要 Rust 工具链）
	cd $(SHELLDIR) && pnpm tauri dev

# ══ worktree（规范见 docs/rules/git-workflow.md §4）════════════════════
.PHONY: wt
wt: ## 建并行工作区： make wt NAME=feat/acp-session-cancel
	@bash scripts/dev/worktree.sh add "$(NAME)"

.PHONY: tidy
tidy: ## ★ 合并 PR 后必跑：清理已合并的分支、worktree、远端残留引用
	@bash scripts/dev/tidy.sh

.PHONY: wt-list
wt-list: ## 列出当前所有 worktree
	@git worktree list

# ══ 构建 ═══════════════════════════════════════════════════════
.PHONY: build
build: ## 构建 duetd + 前端 dist
	@bash scripts/release/build.sh

.PHONY: build-app
build-app: build ## 构建 Duet.app（含 minisign 签名，需 TAURI_SIGNING_PRIVATE_KEY）
	cd $(SHELLDIR) && pnpm tauri build
