# 测试索引 · backend

> **写任何新测试前，先在本表里按「行为」搜一遍。**
> AI 反复写重复测试的根因是不知道已经测过什么——本表就是用来挡这个的。
>
> 规则见 [`../../docs/testing-strategy.md`](../../docs/rules/testing-strategy.md) §8。
> `make check-test-index` 会逐项比对，不一致即红。

## 登记规则

- 每个**顶层 `func TestXxx`** 一行，覆盖 `backend/**` 下全部 `*_test.go`
- 表驱动的子用例**不单独登记**，在「覆盖的行为」列里概述
- 「覆盖的行为」写**行为**，不要抄函数名——抄函数名的索引没有检索价值
- 发现两个测试实质相同 → **合并**，不要并列

## 索引

| 测试 | 文件 | 层 | 覆盖的行为 / 验收标准 |
|---|---|---|---|
| `TestWorkState_R1_ExhaustiveAndMatchesGlossary` | `internal/domain/model/work_test.go` | domain | M0 U0.9.1 R1：**11 个**状态取值与 AGENTS.md §8 术语表一字不差（ADR 0006 Q1 从 9 加到 11）；未登记的取值一律非法；新增状态忘了登记时会红 |
| `TestWork_R2R3_Transition` | `internal/domain/model/work_test.go` | domain | M0 U0.9.1 R2/R3：29 条迁移的合法与非法路径；**三个终态**（含 `initializing_failed`，worktree 没切成不可恢复）；**completed 只能从 reviewing_unit 进入**；终态无出边；被拒时状态不变且错误含 from/to |
| `TestWork_R4_EveryStateIsReachableOrTerminal` | `internal/domain/model/work_test.go` | domain | M0 U0.9.1 R4：穷举——每个状态都有入边或出边，加了状态却没接进状态机时会红 |
| `TestNewWork_StartsInInitializing` | `internal/domain/model/work_test.go` | domain | ADR 0006 Q1：新建工作的初始状态是 `initializing` 而非 `clarifying`——worktree 还没切，对话还没开始 |
| `TestWork_DomainIsPure` | `internal/domain/model/work_test.go` | domain | 领域层是纯计算：构造与迁移都不需要 context |
| `TestGuard_R3_RejectsRealDataDir` | `tests/testutil/guard_test.go` | 夹具 | M0 U0.1.2 R3（**铁律 6**）：访问 `~/.acpflows` `~/.duet` `~/.claude` `~/.codex` 一律拦下，错误信息指向铁律 6 并含被拦路径 |
| `TestGuard_AllowsTempDir` | `tests/testutil/guard_test.go` | 夹具 | 守卫不误伤：临时目录下的同名路径（`<tmp>/.acpflows`）必须放行，否则所有测试都跑不了 |
| `TestTempPaths_IsolatedAndDistinct` | `tests/testutil/guard_test.go` | 夹具 | 六条路径都是绝对路径、互不重叠、都过守卫；**两次调用给出不同目录**（测试间不共享状态） |
| `TestDeterministicClockAndIDGen` | `tests/testutil/guard_test.go` | 夹具 | M0 U0.1.2 R2：FixedClock 可重复；SeqIDGen 按前缀递增；ULID 单调递增（事件表排序依赖它） |
| `TestOpen_R1_ForeignKeysPragmaIsOn` | `internal/store/store_test.go` | store | ★ `foreign_keys` pragma 必须是 1。SQLite 默认关闭，不开等于外键白写 |
| `TestOpen_R2_JournalModeIsWAL` | `internal/store/store_test.go` | store | WAL 模式：读写不互斥，桌面应用会并发跑多个 Work |
| `TestMigrate_R3_OnEmptyDatabase` | `internal/store/store_test.go` | store | 迁移在空库跑通、记入 `schema_migrations`、`works` 表建出来 |
| `TestMigrate_R4_Idempotent` | `internal/store/store_test.go` | store | 重复打开同一个库不出错、不重复记录迁移 |
| `TestWorkRepo_R5_SaveAndFindRoundTrip` | `internal/store/store_test.go` | store | 领域模型存进去再取出来，ID 与状态不丢（entity↔model 映射正确） |
| `TestWorkRepo_R6_NotFoundIsAnError` | `internal/store/store_test.go` | store | ★ 查不到返回 `model.ErrNotFound` 而非 nil；**`gorm.ErrRecordNotFound` 不泄漏出 store 包** |
| `TestWorkRepo_R7_UpdateStatePersists` | `internal/store/store_test.go` | store | ★ 状态更新真的落盘（GORM 的 `Updates` 传 struct 会静默丢零值，本项目一律用 map） |
| `TestWorkRepo_R8_ListEmptyIsNotAnError` | `internal/store/store_test.go` | store | 空集合返回空切片而非 nil，且不是错误 |
| `TestConn_R1_NewlineDelimited` | `internal/acp/jsonrpc/conn_test.go` | acp | M0 U0.2.1 R1：★ **ndjson 换行分帧**（不是 LSP 的 Content-Length）；内容里的换行被转义保留，不破坏分帧 |
| `TestConn_R2_OutOfOrderResponses` | `internal/acp/jsonrpc/conn_test.go` | acp | M0 U0.2.1 R2：★ 三个并发请求逆序回复，各自按 id 正确归位——agent 不保证按序回复，按顺序配对必错 |
| `TestConn_R3_NotifyHasNoID` | `internal/acp/jsonrpc/conn_test.go` | acp | M0 U0.2.1 R3：通知不带 id、不阻塞等响应（`session/cancel` 就是通知） |
| `TestConn_R4_HandlesIncomingRequest` | `internal/acp/jsonrpc/conn_test.go` | acp | M0 U0.2.1 R4：★ **反向请求**路由到 handler 并回值；ACP 的请求 id 从 0 开始 |
| `TestConn_UnhandledIncomingReturnsMethodNotFound` | `internal/acp/jsonrpc/conn_test.go` | acp | 没注册 handler 时反向请求回 -32601，不静默丢弃（丢弃会让 agent 整轮失败） |
| `TestConn_R5_ContextCancellation` | `internal/acp/jsonrpc/conn_test.go` | acp | M0 U0.2.1 R5：对方永不回复时超时返回 `DeadlineExceeded`，pending 表不泄漏 |
| `TestConn_R6_MalformedLineDoesNotKillConnection` | `internal/acp/jsonrpc/conn_test.go` | acp | M0 U0.2.1 R6：一行坏帧被跳过，后续消息仍正常处理 |
| `TestConn_PropagatesRemoteError` | `internal/acp/jsonrpc/conn_test.go` | acp | 远端 error 翻译成可辨识的 `*jsonrpc.Error`，保留 code（-32000 是 ACP 的认证错误）与 message |
| `TestAuth_R3_RejectsWithoutToken` | `internal/api/server_test.go` | api | M0 U0.10.1 R3：无 token / 空 token / 错 token / token 前缀，四种都必须 401；**401 响应里不泄漏正确 token 与版本信息** |
| `TestAuth_AcceptsValidToken` | `internal/api/server_test.go` | api | 带正确 token 时放行 |
| `TestSystemVersion_R5_MatchesContract` | `internal/api/server_test.go` | api | M0 U0.10.1 R5：响应 Content-Type 与 `openapi.yaml` 的 `VersionInfo` 必填字段一致 |
| `TestNotFound_ReturnsProblem` | `internal/api/server_test.go` | api | 未匹配路由返回 `application/problem+json`，`type` 是机器可读错误码而非中文文案 |
| `TestNewRouter_RejectsEmptyToken` | `internal/api/server_test.go` | api | 空 token 的配置被拒——那等于关掉鉴权 |
| `TestOpen_R9_DatabaseFileIsInTempDir` | `internal/store/store_test.go` | store | 数据库落在临时文件（不是 `:memory:`，那测不出 WAL 与并发），没碰用户真实数据 |
| `TestLevels_R1_FiveLevels` | `internal/platform/logging/logging_test.go` | platform | 五个级别齐备且顺序正确；`LevelTrace = -8`（低于 slog.LevelDebug） |
| `TestParseLevels_R2_PerComponent` | `internal/platform/logging/logging_test.go` | platform | ★ `DUET_LOG` 按域调级别：`info,acp=trace` 解析；大小写与空白容忍；未知级别名报错且列出可用取值 |
| `TestContext_R3_FieldsInherited` | `internal/platform/logging/logging_test.go` | platform | ★ 关联字段（work_id/unit_id）从 context 自动继承——手动传必然会漏，日志就串不起来 |
| `TestContext_EmptyIsSafe` | `internal/platform/logging/logging_test.go` | platform | 没有 context 字段时正常打日志，不 panic |
| `TestSink_R4_FailureDoesNotPropagate` | `internal/platform/logging/logging_test.go` | platform | ★ 落库失败只降级为「只写 stderr」，绝不向上抛——日志系统挂掉不该让产品挂掉 |
| `TestHandler_R5_ComponentLevelOverridesGlobal` | `internal/platform/logging/logging_test.go` | platform | 组件级别覆盖全局：全局 warn + `acp=trace` 时，acp 的 TRACE 落库、store 的 DEBUG 被挡 |
| `TestHandler_R6_TraceNotOnStderr` | `internal/platform/logging/logging_test.go` | platform | ★ TRACE 永不进 stderr（报文全文会把生命周期日志淹掉），但必须落库 |
