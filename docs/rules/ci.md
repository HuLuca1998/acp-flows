# CI 设计规范

> 目标：**PR 上的反馈 < 5 分钟。**
>
> CI 慢会直接改变行为——AI 和人都会开始「先推上去看看」而不是本地跑 `make check`，
> 于是 CI 变成调试工具，队列越排越长，反馈越来越慢。这是个正反馈死循环。

---

## 1. 三条核心规则

### 规则 1 · 只跑受影响的部分

改了后端就不该扫前端。用 `dorny/paths-filter` 探测变更，各 job 用 `if` 守卫。

```yaml
changes:                         # ~10s，永远跑
  outputs:
    backend: ${{ steps.filter.outputs.backend }}

backend:
  needs: changes
  if: needs.changes.outputs.backend == 'true'
```

### 规则 2 · 分支保护只 required 汇总门禁 ★

**这是最容易踩的坑，踩了 PR 会永远卡住。**

如果把 `backend` 直接设为 required check，而这个 PR 只改了前端 →
`backend` 被跳过 → GitHub 永远等一个不会来的结果 → **PR 合不进去**。

正确做法：加一个 `if: always()` 的汇总 job，**分支保护只 required 它**：

```yaml
ci:
  if: always()
  needs: [changes, guard, contract, backend, frontend, shell]
  steps:
    - run: |
        for r in ${{ join(needs.*.result, ' ') }}; do
          case "$r" in
            success|skipped) ;;                    # 跳过算通过
            *) echo "✗ 有 job 未通过（$r）"; exit 1 ;;
          esac
        done
```

**`skipped` 算通过，`failure` / `cancelled` 不算。** 跳过不卡 PR，真失败也放不过去。

> 用 `on: pull_request: paths:` 过滤**整个 workflow** 会造成同样的死锁，
> 而且更隐蔽。**不要用它。** 过滤只在 job 层做。

### 规则 3 · 便宜的先跑

违规要尽早暴露。job 按成本排序：

| job | 时长预算 | 什么时候跑 |
|---|---|---|
| `changes` | ~10s | 永远 |
| `guard`（文档 / 索引 / 命名 / 提交信息） | ~30s | **永远**（便宜，且覆盖整个仓库） |
| `contract` | ~1min | `api/**` 变动 |
| `backend` | ~2min | `backend/**` 或契约变动 |
| `frontend` | ~2min | `frontend/**` 或契约变动 |
| `shell` | ~4min | `shell/**` 变动（**macOS runner 贵，能不跑就不跑**） |
| `ci` 汇总 | 秒级 | 永远 |
| `e2e` | ~6min | **只在 `main`**，不阻塞 PR |

并行执行，PR 的实际墙钟时间目标 **≈ 2.5 分钟**。

---

## 2. 变更过滤的定义

过滤规则要包含**跨目录的依赖**，漏了会导致该跑的没跑：

```yaml
backend:
  - 'backend/**'
  - 'api/openapi.yaml'        # 契约变了后端生成物要重生成
  - 'scripts/gen/gen-api.sh'
  - 'scripts/check/check-coverage.sh'
frontend:
  - 'frontend/**'
  - 'api/openapi.yaml'        # 同上
  - 'scripts/check/check-i18n.sh'
```

**新增跨目录依赖时必须同步更新过滤规则。**
判断方法：这个文件改了，会让哪个 job 的结果变化？

`guard` 不加过滤——它检查的是全仓库的文档、索引、命名，改哪都可能影响。

### 脚手架探测

`changes` 除了探测「改了什么」，还探测「有没有脚手架」：

```yaml
- id: scaffold
  run: |
    # backend/go.mod · frontend/package.json · shell/src-tauri/Cargo.toml
```

子项目尚未创建时（M0 之前），相关 job **直接跳过**，
而不是在 `actions/setup-go` 上硬失败（它找不到 `go.mod` 会直接报错，
`setup-node` 的 pnpm 缓存找不到 lockfile 也一样）。

**理由：CI 必须从第一天起就能跑通。** 一个跑不动的检查等于没有检查——
一旦 CI 长期是红的，所有人都会开始忽略它。

脚手架建好后这些守卫自动失效，不需要回来改。

---

## 3. 什么放 PR，什么放 main

| | PR | main |
|---|---|---|
| lint / typecheck / 单测 / 集成测试 | ✅ | ✅ |
| 契约一致性 | ✅ | ✅ |
| 文档与索引 | ✅ | ✅ |
| **E2E** | ❌ | ✅ |
| **构建产物 / 打包** | ❌ | ✅（tag 时） |
| 跨平台矩阵 | ❌ | ✅（tag 时） |

**PR 上不做构建。** 构建慢且对"这个改动对不对"没有增量信息——
编译错误 lint 和 typecheck 已经抓到了。

---

## 4. 缓存

每个语言都必须配缓存，不配就是每次多花几分钟：

| 语言 | 缓存 |
|---|---|
| Go | `actions/setup-go` 的 `cache-dependency-path: backend/go.sum` |
| Node | `actions/setup-node` 的 `cache: pnpm` |
| Rust | `Swatinem/rust-cache`，`workspaces: shell/src-tauri` |
| Playwright 浏览器 | 只在 `main` 装，PR 不装 |

---

## 4.5 额度：macOS runner 是 10 倍计费 ★

每月的 Actions 额度有限，而 **macOS runner 按 10 倍扣**。
下面几条都是实测省下来的，加新 job 时照着做：

| 做法 | 省掉什么 |
|---|---|
| **每个 job 必须有 `timeout-minutes`** | GitHub 默认上限 **6 小时**——一个卡死的 macOS job 就能烧掉 60 小时额度。按实测耗时的 3–5 倍给 |
| `concurrency` + `cancel-in-progress` | 连续推送时自动取消上一轮 |
| 路径过滤（`changes` job）+ 各 job 的 `if` 守卫 | 只改文档时不跑 macOS |
| macOS job 里**不编 Go sidecar**，用空文件占位 | 每次 30–60s。`clippy` 只要求文件存在，不会执行它 |
| macOS job 里**不跑 `cargo test`** | `clippy --all-targets` 已经编过 test 目标；壳这层没有单元测试（行为只能靠真包验） |
| 打 universal 包时第二次 `build.sh` 带 `--skip-frontend` | 前端 dist 与架构无关，重复 `pnpm install + build` 白烧一分多钟 |
| 发布前置检查放 Linux job | 密钥没配时**在 1 倍计费的阶段**就失败，而不是在 macOS 上打包完才报 |

> **判断标准**：这一步的产物，macOS 上和 Linux 上是不是同一个？
> 是的话就挪去 Linux，或者直接省掉。

## 5. 其他必配项

```yaml
concurrency:
  group: ci-${{ github.ref }}
  cancel-in-progress: true      # 连续推送时取消旧的，省额度也省等待
permissions:
  contents: read                # 最小权限，需要写的 job 单独提权
```

---

## 6. 本地必须能跑同样的检查

**CI 里的每一条检查，本地都要有等价命令。**

```bash
make check      # = guard + lint + test
```

做不到这一点，AI 就会开始"推上去看看"，CI 沦为调试工具。

反过来也成立：**本地能跑的检查必须接进 CI**，
只写脚本不接 CI 等于没写（见 `scripts/AGENTS.md`）。

---

## 7. 加一个新检查的完整步骤

1. 写 `scripts/check-xxx.sh`，失败信息要给出修正方式
2. 加 `Makefile` target，并挂到 `make check` 下
3. 加进 `ci.yml` 的**合适的 job**（问自己：它受哪些路径影响？）
4. 如果新建了 job → **加进 `ci` 汇总门禁的 `needs`**，否则它的失败不会拦住 PR
5. 如果它引入了跨目录依赖 → 更新 §2 的过滤规则
6. **手动制造一次违规，确认它真的会红** —— 永远绿的检查比没有检查更糟

---

## 8. 禁止清单

- ✗ 用 `on: pull_request: paths:` 过滤整个 workflow（会让 required check 死锁）
- ✗ 把单个 job 设为 required check（同上）
- ✗ 新建 job 不加进 `ci` 汇总的 `needs`（它的失败不会拦住任何东西）
- ✗ 不配缓存
- ✗ 在 PR 上跑 E2E、构建、跨平台矩阵
- ✗ 无条件使用 macOS runner（比 Linux 贵约 10 倍）
- ✗ CI 里有本地跑不了的检查
- ✗ 加了检查脚本不接进 CI
- ✗ 为了让 CI 变绿而放宽检查 —— 这是最严重的违规，等同于伪造验收
