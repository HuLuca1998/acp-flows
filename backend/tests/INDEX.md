# 测试索引 · backend

> **写任何新测试前，先在本表里按「行为」搜一遍。**
> AI 反复写重复测试的根因是不知道已经测过什么——本表就是用来挡这个的。
>
> 规则见 [`../../docs/rules/testing-strategy.md`](../../docs/rules/testing-strategy.md) §8。
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
| `TestLogSink_R1_PersistsAllFields` | `internal/store/log_sink_test.go` | store | ★ 关联字段（work_id/unit_id/attempt_id/trace_id）与 attrs JSON 都落库——丢了的话日志还在但「这个 Work 的全过程」查不出来 |
| `TestLogSink_R2_CloseFlushesRemainder` | `internal/store/log_sink_test.go` | store | ★ Close 冲刷不满一批的剩余日志。不冲刷的话，**崩溃前最有价值的那几十条**全没 |
| `TestLogSink_R3_WriteNeverBlocks` | `internal/store/log_sink_test.go` | store | ★ 灌 5 万条超出缓冲，Write 必须不阻塞（带 10s 超时——阻塞会表现为挂住而不是失败） |
| `TestLogSink_R4_RetentionKeepsErrorsLonger` | `internal/store/log_sink_test.go` | store | ★ 保留策略：普通日志 7 天、ERROR 30 天。四个年龄×级别组合逐个断言留还是删 |
| `TestLogSink_R5_RowCapDropsOldest` | `internal/store/log_sink_test.go` | store | ★ 超过条数上限时从**最旧**的删；断言留下的是最新那批（删错方向会把新日志删光） |
| `TestLogSink_R6_UnmarshalableAttrsDoNotDropEntry` | `internal/store/log_sink_test.go` | store | attrs 序列化失败时保留整条并标 `__marshal_error`——message 往往正是排查要的 |
| `TestLogSink_R7_InsertFailureIsSwallowed` | `internal/store/log_sink_test.go` | store | ★ 表被删后写入必然失败，仍不 panic、不阻塞、Close 不返回错误 |
| `TestLogSink_R8_CloseIsIdempotent` | `internal/store/log_sink_test.go` | store | Close 重复调用不 panic（关闭路径上有多处兜底调用） |
| `TestConn_R7_IncomingNotificationRoutedAndNotAnswered` | `internal/acp/jsonrpc/conn_test.go` | acp | ★ 反向通知路由到 handler 且**不回响应**。ACP 的 `session/update` 全是通知——这条断了症状是「界面什么都不显示」而不是报错 |
| `TestConn_R8_NotificationHandlerErrorDoesNotKillConn` | `internal/acp/jsonrpc/conn_test.go` | acp | 通知处理出错不打断连接：紧接着的反向请求仍能正常回 |
| `TestConn_R9_NotificationWithoutHandlerIsDropped` | `internal/acp/jsonrpc/conn_test.go` | acp | 没注册 handler 时通知静默丢弃，不产生任何回帧 |
| `TestConn_R10_HandlerRPCErrorKeepsCode` | `internal/acp/jsonrpc/conn_test.go` | acp | ★ handler 返回 `*jsonrpc.Error` 时 code 原样传出。被包装成 -32603 的话，对方分不清「拒绝」与「我们崩了」 |
| `TestConn_R11_HandlerPlainErrorBecomesInternalError` | `internal/acp/jsonrpc/conn_test.go` | acp | 普通 error 包装成 -32603 |
| `TestConn_R12_UnknownResponseIDIsIgnored` | `internal/acp/jsonrpc/conn_test.go` | acp | 收到没发过的 id 的响应只告警不崩；真实 Runtime 出过这种时序错乱 |
| `TestSessionUpdateKind_R1_ExhaustiveMatchesSchemaV1` | `internal/acp/protocol/update_test.go` | acp | M0 U0.2.3 R1：★ v1 的 `sessionUpdate` 判别值**穷举 13 个**，全集与字面量都对照官方 schema；三份文档曾把这个数字写成 9/11/13，只有这条测试守得住 |
| `TestSessionUpdate_R2_UnknownKindIsPreservedNotAnError` | `internal/acp/protocol/update_test.go` | acp | M0 U0.2.3 R2：★ 未知判别值**不报错**且判别值与载荷都原样取回——遇到没见过的变体就断开，等于每次 Runtime 升级都炸一次 |
| `TestSessionUpdateKind_R5_ExperimentalVariantsAreFlagged` | `internal/acp/protocol/update_test.go` | acp | M0 U0.2.3 R5：`plan_update` / `plan_removed` 官方标了 UNSTABLE——认得（不刷 warn）但标注为实验（不建映射）；未知判别值 ≠ 实验特性 |
| `TestSessionUpdate_R4_GoldenRoundTripLosesNothing` | `internal/acp/protocol/update_test.go` | acp | M0 U0.2.3 R4：★ 13 个变体的 golden 逐个 round-trip，拿**原始 JSON 的键**做对照——只做 struct round-trip 的话，struct 漏定义字段时测试照样绿；同时守住 golden 必须覆盖全部判别值 |
| `TestSessionNotification_CarriesSessionIDAndUpdate` | `internal/acp/protocol/update_test.go` | acp | `sessionId` 端到端穿透（前一个项目 H-1 就是它在某一层丢了，表现为「第 2 轮不记得第 1 轮」） |
| `TestProtocolEnums_R3_ExhaustiveAndRejectUnknown` | `internal/acp/protocol/enum_test.go` | acp | M0 U0.2.3 R3：`StopReason`/`ToolKind`/`ToolCallStatus`/`PermissionOptionKind` 四个封闭枚举穷举，拒绝表外取值与空值；表外取值挑的是最容易被编出来的那些（`timeout`/`write`/`allow`） |
| `TestProtocolEnums_R3_AllSlicesMatchSchema` | `internal/acp/protocol/enum_test.go` | acp | 四个 `All*()` 返回的全集与官方 schema 逐字一致（上一条只验证了字面量合法，这条验证不多不少） |
| `TestStopReason_OnlyEndTurnIsSuccess` | `internal/acp/protocol/enum_test.go` | acp | ★ 只有 `end_turn` 算正常收尾。前一个项目 H-5：把所有 stopReason 当成功，`max_tokens` 截断的半成品被当作已完成验收 |
| `TestRequestPermissionOutcome_DistinguishesCancelledFromSelected` | `internal/acp/protocol/enum_test.go` | acp | ★ 权限应答的 `cancelled` 与 `selected` 可判别且可发出。分不出来的话每次取消都超时、M1 的 `prepare` 永远返回 `blocked` |
| `TestTextBlock_MatchesOfficialShape` | `internal/acp/protocol/content_test.go` | acp | `TextBlock` 构造的 JSON 与官方 shape 逐字一致（Fake 的 `.Say()` 直接用它） |
| `TestDiffContent_MatchesOfficialShape` | `internal/acp/protocol/content_test.go` | acp | `DiffContent` 同上；挡住 `oldText` 写成 `old_text` 这类不报错但 agent 静默少显示的 bug |
| `TestContentBlock_UnhandledTypesSurviveRoundTrip` | `internal/acp/protocol/content_test.go` | acp | 没有强类型访问器的内容块（image / resource_link）原样转发不丢；非 text 块的 `Text()` 明确返回「没有」而不是空串 |
| `TestNewSessionUpdate_TakesKindFromPayload` | `internal/acp/protocol/update_test.go` | acp | 构造方向：判别值从载荷读回而非另传一遍（传两遍会出现「结构体说 tool_call、参数说 plan」的不一致）；Fake 发事件走这条路 |
| `TestNewSessionUpdate_MissingDiscriminatorIsAnError` | `internal/acp/protocol/update_test.go` | acp | 缺 `sessionUpdate` 字段是**报文不合协议**，返回 `ErrMissingDiscriminator`——与「判别值不认识」区分开，否则畸形报文会被当成协议演进静静丢掉 |
| `TestSessionUpdate_ZeroValueRefusesToMarshal` | `internal/acp/protocol/update_test.go` | acp | 零值 `SessionUpdate` 拒绝序列化，不写成 `null`（Fake 脚本漏填 emit 时要立刻暴露） |
| `TestContentBlocks_ZeroValueRefusesToMarshal` | `internal/acp/protocol/content_test.go` | acp | 同上，作用于 `ContentBlock` / `ToolCallContent` |
| `TestConfigOption_CategoryOrEmpty_HandlesMissingAndBlank` | `internal/acp/protocol/enum_test.go` | acp | ★ claude 的 `agent` 配置项 **category 是空字符串**（实测 N2）：缺失与空串都不能 panic，且线上要能区分（回写时不给本无 category 的选项凭空加一个）。差异内化整套方案建立在「按 category 取」之上 |
