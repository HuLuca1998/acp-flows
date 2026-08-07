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
| `TestRuntime_R1_ReplaysEverySessionUpdateKind` | `internal/acp/fake/runtime_test.go` | acp | M0 U0.4.1 R1：Fake 按脚本推送**全部 13 类** `sessionUpdate`，每条都带对的 `sessionId` 且能被 `protocol` 反序列化；样本数由 `AllSessionUpdateKinds()` 驱动，协议加变体时会红 |
| `TestRuntime_R2_StepDelayIsHonored` | `internal/acp/fake/runtime_test.go` | acp | M0 U0.4.1 R2：每条事件的延迟可配置，两条事件到达间隔 ≥ 配置值（实测过：删掉延迟实现会红） |
| `TestRuntime_R3_ReorderIsDeterministic` | `internal/acp/fake/runtime_test.go` | acp | M0 U0.4.1 R3：乱序**由 seed 驱动可复现**、确实改变了顺序、且不丢事件不造事件——复现不了的随机只会制造 flaky 测试 |
| `TestRuntime_R4_SilentAfterEndsTheStream` | `internal/acp/fake/runtime_test.go` | acp | M0 U0.4.1 R4：中途断流后消费方**感知到 EOF** 而不是永久阻塞（永久阻塞的症状是测试挂住，比失败更难查） |
| `TestRuntime_R5_NeverStopsLeavesPromptPending` | `internal/acp/fake/runtime_test.go` | acp | M0 U0.4.1 R5：★ `NeverStops` 下 `session/prompt` 永不 resolve，**但事件照常流出**——连接整个挂掉的话，S0.6 测出来的是「连接断了」而不是「Runtime 不收尾」 |
| `TestRuntime_R6_RecordsEveryRequestWithoutDeduping` | `internal/acp/fake/runtime_test.go` | acp | M0 U0.4.1 R6：★★ Fake **如实记录、绝不去重**：连发两次 `session/cancel` 就是 2 条；请求与通知可区分；原始 params 留存。Fake 若自己去重，U0.6.1 的幂等断言永远绿 |
| `TestRuntime_CompletesAScriptedTurn` | `internal/acp/fake/runtime_test.go` | acp | 正常路径：脚本指定的 `sessionId` 被采用、事件按序推送、`stopReason` 原样带回。这是 R1–R6 的前置 |
| `TestRuntime_ServeSpeaksTheSameProtocolAsTransport` | `internal/acp/fake/runtime_test.go` | acp | 子进程形态（`Serve`，给 e2e）与进程内形态（`Transport`，给单测）是同一份实现——不是的话「单测绿 + e2e 红」时你不知道该信谁 |
| `TestParseVersion_ComparesNumericallyNotLexically` | `internal/domain/model/version_test.go` | domain | ★ M1：`1.10.0` 必须比 `1.9.0` 新——按字符串比会判反，用户从此收不到更新且无任何报错 |
| `TestParseVersion_Ordering` | `internal/domain/model/version_test.go` | domain | 主/次/修订三级排序；**预发布版低于同号正式版**（判反会劝正式版用户降级到开发快照）；`v` 前缀等价 |
| `TestParseVersion_RejectsMalformed` | `internal/domain/model/version_test.go` | domain | ★ 9 种非法版本一律报错，**绝不静默当成 0.0.0**：latest.json 写错一个字符会让全部客户端永远收不到更新，且没有症状 |
| `TestVersion_StringRoundTrip` | `internal/domain/model/version_test.go` | domain | 还原成字符串；`v` 前缀被规整掉（界面上的 v 由前端加） |
| `TestVersion_IsSnapshot` | `internal/domain/model/version_test.go` | domain | 区分开发快照与正式版；`0.0.0`（占位版本）不算快照——两者界面文案不同 |
| `TestCheck_ReportsAvailableWithDetails` | `internal/app/system/update_test.go` | app | M1 S1.6：有新版本时报 `available`，并带上版本号/更新说明/体积/发布时间——那是用户「现在更不更」的判断依据 |
| `TestCheck_SameOrOlderIsIdle` | `internal/app/system/update_test.go` | app | ★ 相同或**更旧**一律 `idle`。回滚发布后仍提示更新的话，用户会被反复劝着装回旧版 |
| `TestCheck_WebBuildIsUnsupportedAndSkipsNetwork` | `internal/app/system/update_test.go` | app | ★ Web 形态报 `unsupported` 且**根本不查发布源**——浏览器里没 updater，「提示了更新却点不动」会把用户卡死 |
| `TestCheck_SourceFailurePropagates` | `internal/app/system/update_test.go` | app | ★ 发布源出错必须上报，**绝不降级成「已是最新版本」**：网络断了会伪装成没有更新，这类故障无症状 |
| `TestCheck_MalformedRemoteVersionIsAnError` | `internal/app/system/update_test.go` | app | 远端版本号非法要报错，可辨识为 `ErrInvalidVersion` |
| `TestPrepare_NoActiveWorkIsReady` | `internal/app/system/update_test.go` | app | M1 S1.7：没有进行中的工作时放行 |
| `TestPrepare_AnyActiveWorkBlocks` | `internal/app/system/update_test.go` | app | ★★ **失败安全**：6 种非终态各自都挡住更新，并给出机器可读的 `work_in_progress`。「先放行、以后再补暂停」会真实丢掉用户几十分钟的工作 |
| `TestPrepare_TerminalWorkDoesNotBlock` | `internal/app/system/update_test.go` | app | 已结束的工作（completed / failed / initializing_failed）不挡更新 |
| `TestPrepare_ListFailureBlocksInsteadOfAllowing` | `internal/app/system/update_test.go` | app | ★ 查不到工作列表时按 `blocked` 处理而不是放行——不知道有没有工作在跑时重启应用是最坏的选择 |
| `TestNewUpdateService_RejectsMissingDeps` | `internal/app/system/update_test.go` | app | 缺依赖时构造即失败，不留到运行时 panic |
| `TestLatest_ParsesTauriUpdaterManifest` | `internal/release/latest_test.go` | release | 解析 Tauri v2 的 latest.json（version / notes / pub_date）；查的是与 updater **同一个** URL |
| `TestLatest_NonOKIsAnError` | `internal/release/latest_test.go` | release | ★ 404/500/403 都报错且错误带状态码。**404 最常见**——还没发过 release 时就是它，静默的话第一版发出去前没人会发现链路是断的 |
| `TestLatest_MalformedJSONIsAnError` | `internal/release/latest_test.go` | release | 坏 JSON 报错，不静默返回零值 |
| `TestLatest_MissingVersionIsAnError` | `internal/release/latest_test.go` | release | 缺 `version` 字段报错——那样的 manifest 对更新检查毫无意义 |
| `TestLatest_RespectsContextCancellation` | `internal/release/latest_test.go` | release | ctx 取消被尊重（设置页关掉时这次检查就该停） |
| `TestLatest_RejectsOversizedBody` | `internal/release/latest_test.go` | release | ★ 响应体超 256KB 被拒——latest.json 正常几百字节，不设限则被劫持的端点能把 duetd 拖死 |
| `TestCheckUpdate_MatchesContract` | `internal/api/update_test.go` | api | `POST /v1/system/update/check` 响应形状与 openapi 的 `UpdateStatus` 一致 |
| `TestCheckUpdate_SourceFailureReturnsProblem` | `internal/api/update_test.go` | api | ★ 发布源失败回 `problem+json` 且 `type=update_check_failed`，不回 200「已是最新」 |
| `TestPrepareUpdate_ReadyWhenNoActiveWork` | `internal/api/update_test.go` | api | 放行时 `prepared`/`blocked` 必须是**空数组不是 null**——前端对 null 调 `.map()` 会白屏 |
| `TestPrepareUpdate_BlockedIsStillTwoHundred` | `internal/api/update_test.go` | api | ★ `blocked` 是业务结论不是错误，**仍回 200**：回 4xx 前端会当成请求出错，把「哪些工作在跑」的列表丢掉 |
| `TestUpdateEndpoints_WithoutServiceDoNotPanic` | `internal/api/update_test.go` | api | 未配置更新服务时返回 `update_not_configured` 而不是 panic（纯 Web 部署可以不接发布源） |
| `TestDetect` | `internal/acp/runtime/detect_test.go` | acp | 四态可区分：ready / not_installed / not_authenticated / probe_failed；未登录时给的是 `codex login` 这种能直接敲的命令 |
| `TestDetectorKeepsEveryField` | `internal/acp/runtime/detector_test.go` | acp | 适配层不丢字段——丢了 Remedy 用户就看不到该敲什么命令，而少写一行赋值编译器不会管 |
| `TestDetectorZeroValueUsesRegistry` | `internal/acp/runtime/detector_test.go` | acp | 零值 Detector 走内置注册表（duetd 就是这么构造的）；空 PATH 下每条都必须给出安装命令 |
| `TestDetectExtractsVersionNumber` | `internal/acp/runtime/detect_test.go` | acp | 从四种真实的 `--version` 输出里抽出版本号；抽不出时原样保留不弄丢信息 |
| `TestDetectNeverPrompts` | `internal/acp/runtime/detect_test.go` | acp | ★ 检测**零模型开销**——假 runtime 记下每次 argv，断言只出现声明过的两组参数，多一次调用就红 |
| `TestDetectAllIsolatesFailures` | `internal/acp/runtime/detect_test.go` | acp | ★ 一个 runtime 卡住不连累另一个，且必须并发——串行的话装 5 个就要等 5 倍超时 |
| `TestDetectIsIdempotent` | `internal/acp/runtime/detect_test.go` | acp | 连查两次结论一致（设置页会反复打开） |
| `TestListRuntimes_MapsEveryStatus` | `internal/api/runtimes_test.go` | api | 四种状态各自映射成什么逐条锁死；`installed`/`authenticated` 从 `status` **推导**而非各存一份（两份真源必然漂移） |
| `TestListRuntimes_EmptyIsArrayNotNull` | `internal/api/runtimes_test.go` | api | ★ `runtimes` 空时是 `[]` 不是 `null`——新用户第一次打开设置页正是这个状态，前端对 null 调 `.map()` 会白屏 |
| `TestListRuntimes_MissingDetectorDoesNotBreakOtherEndpoints` | `internal/api/runtimes_test.go` | api | ★ 没配检测器时不回 200 空列表（那会把「检测不了」显示成「一个都没装」），且不连累其他端点 |
| `TestListRuntimes_DetectsOncePerRequest` | `internal/api/runtimes_test.go` | api | 一次请求只探一轮——探测要拉子进程，重复探测会让设置页明显变慢 |
| `TestProbe` | `internal/gitx/probe_test.go` | gitx | 用真 git 仓库探测：仓库/普通目录/路径不存在/路径是文件四种情形；**非 git 仓库不算错误**（当成错误的话用户得先去命令行 git init） |
| `TestProbe_WritesNothing` | `internal/gitx/probe_test.go` | gitx | ★★ 探测**不往用户仓库写一个字节**——同时比 `git status` 与完整文件列表（只比 status 抓不到写进 .gitignore 或被忽略路径的情况） |
| `TestProbe_EmptyRepo` | `internal/gitx/probe_test.go` | gitx | 刚 `git init` 还没有 commit 时 HEAD 指向不存在的引用——这是「新建文件夹→git init→加进 Duet」的真实路径，不能崩 |
| `TestNewProject` | `internal/domain/model/project_test.go` | domain | 构造项目：**相对路径被拒**（相对路径在 duetd 的工作目录下解析，那是用户完全不知道的位置）；末尾斜杠被规整；根目录的显示名不能是空字符串 |
| `TestProject_PathIsNormalized` | `internal/domain/model/project_test.go` | domain | `/a/b`、`/a/b/`、`/a/./b`、`/a/c/../b` 规整成同一个 Path——否则用户从 Finder 拖两次会看到两条一样的记录 |
| `TestProject_RenameDoesNotTouchPath` | `internal/domain/model/project_test.go` | domain | ★ 改显示名**不动 path**——两者跟着一起变的话 Duet 会去操作一个不存在的目录 |
| `TestProject_RenameRejectsBlank` | `internal/domain/model/project_test.go` | domain | 空白名被拒且不落地（界面上会显示成一行空白，看起来像记录丢了） |
| `TestAdd_WritesNothingIntoUserProject` | `internal/app/project/service_test.go` | app | ★★ 添加项目往用户目录写**零个字节**——用真 git 仓库 + 真 gitx 实现验，假实现什么都不写、那条断言会永远绿 |
| `TestAdd` | `internal/app/project/service_test.go` | app | git 仓库记下默认分支；**普通目录也能加**并给出 `git init`（拒绝的话用户得先去命令行）；路径不存在报错且不落库；相对路径报错 |
| `TestAdd_IsIdempotent` | `internal/app/project/service_test.go` | app | 同一目录的不同写法只落一条——用户从 Finder 拖两次很常见，列表里冒出两条一样的会让人以为点错了 |
| `TestRemove_DoesNotDeleteUserFiles` | `internal/app/project/service_test.go` | app | ★ 移除只取消登记，用户目录原封不动——这是「移除」与「删除」的全部区别 |
| `TestProjectRepo_SurvivesRestart` | `internal/store/project_repo_test.go` | store | ★ R1：**真的关掉 Store 再用同一个文件重新打开**——「存进去再从同一连接读出来」证明的是缓存不是持久化；git 信息也要一起活下来 |
| `TestProjectRepo_FindByPath` | `internal/store/project_repo_test.go` | store | 查不到返回 `ErrNotFound` 而不是「零值 + nil error」（后者会让重复添加检查形同虚设） |
| `TestProjectRepo_PathIsUnique` | `internal/store/project_repo_test.go` | store | ★ path 唯一是最后一道防线——应用层先查后写挡不住并发添加 |
| `TestProjectRepo_SaveIsUpsert` | `internal/store/project_repo_test.go` | store | 同 ID 再存是更新（用户改显示名），不是插第二条 |
| `TestProjectRepo_Delete` | `internal/store/project_repo_test.go` | store | 删不存在的报 `ErrNotFound`——静默成功的话界面显示「已移除」而下次打开它还在 |
| `TestProjectRepo_ListIsStable` | `internal/store/project_repo_test.go` | store | 按添加顺序且两次一致；时间戳相同时靠 id 兜底，否则顺序由 SQLite 扫描顺序决定 |
| `TestProjectRepo_MaxSeqForPrime` | `internal/store/project_repo_test.go` | store | ★★ 重启后 ID 不能从头发——IDGen 序号在内存里，不回填的话重启后第一次添加就撞主键。这坑在开发机撞不到（库总是空的），只在用户那儿炸 |
| `TestProjectRepo_MaxSeqIgnoresUnparsableIDs` | `internal/store/project_repo_test.go` | store | 解析不了的 ID 跳过而不是报错——宁可序号多跳几个，也不能让 duetd 起不来 |
| `TestAddProject_ReturnsCreated` | `internal/api/projects_test.go` | api | 201 而不是 200；名字取目录名 |
| `TestAddProject_NonGitCarriesRemedy` | `internal/api/projects_test.go` | api | 非 git 目录带上 `git init` 这条能直接敲的命令 |
| `TestAddProject_DomainErrorBecomesProblem` | `internal/api/projects_test.go` | api | ★ 领域错误翻成机器可读错误码——直接吐 Go 的 error 文本会让英文漏进中文界面 |
| `TestAddProject_RejectsBadRequest` | `internal/api/projects_test.go` | api | 空 body / 空 path / 非法 JSON 都不当成成功 |
| `TestListProjects_EmptyIsArrayNotNull` | `internal/api/projects_test.go` | api | ★ 空列表是 `[]` 不是 `null`——「一个项目都没有」正是新用户第一次打开的状态 |
| `TestRemoveProject_ReturnsNoContent` | `internal/api/projects_test.go` | api | 204 且真的调到用例 |
| `TestProjects_WithoutServiceDoNotPanic` | `internal/api/projects_test.go` | api | 没配用例时不 panic 也不假装成功 |
| `TestProcess_ClearsNestedSessionEnv` | `internal/acp/runtime/process_test.go` | acp | R1：清掉 `CLAUDECODE` 等嵌套标记（不清的话 claude-agent-acp 会误判嵌套而拒绝服务），但只删该删的 |
| `TestProcess_CapturesStderr` | `internal/acp/runtime/process_test.go` | acp | R2：失败时把 stderr 附在错误里——只说「exit status 1」等于把唯一的线索丢了 |
| `TestProcess_EscalatesToKill` | `internal/acp/runtime/process_test.go` | acp | ★ R3：用**忽略 SIGTERM** 的真进程验「宽限期后升级到 SIGKILL」；负例（不升级）会让测试挂到超时 |
| `TestProcess_KillsGrandchildren` | `internal/acp/runtime/process_test.go` | acp | ★★ 杀的是**整个进程组**——ACP Runtime 是 node 启动器再 fork，只杀直接子进程的话孙进程会继续占着 worktree 改用户的文件；负例（去掉 Setpgid）立刻红 |
| `TestProcess_LeavesNoZombie` | `internal/acp/runtime/process_test.go` | acp | R4：Wait 回收进程，不留僵尸（用 `ps` 的状态位判，signal 0 对僵尸也返回 nil） |
| `TestProcess_StopIsIdempotent` | `internal/acp/runtime/process_test.go` | acp | 「停止」被连点两下不报错 |
| `TestProcess_StartFailureNamesTheBinary` | `internal/acp/runtime/process_test.go` | acp | 启动失败时错误信息指出是哪个可执行文件 |
| `TestConn_NotificationsAreProcessedInOrder` | `internal/acp/jsonrpc/conn_test.go` | acp | ★★ 通知**按到达顺序**处理——原来每条起一个 goroutine，实测 200 条里第一条到手的是 seq 3；对 ACP 来说这是语义错误：`agent_message_chunk` 的顺序就是用户看到的字序 |
| `TestOpen_ValidatesCwdBeforeAnyRequest` | `internal/acp/session/session_test.go` | acp | ★ R4：cwd 非法时**一个请求都不发**——先发再校验的话，Agent 那边已开了会话而我们报了错，它会挂着没人关 |
| `TestOpen_ReturnsSessionID` | `internal/acp/session/session_test.go` | acp | 会话建好能拿到 sessionID，后续取消与恢复都靠它 |
| `TestPrompt_StreamsFirstChunkLongBeforeTurnEnds` | `internal/acp/session/session_test.go` | acp | ★ R1：真流式——首块在 500ms 内到，而整轮 2 秒才结束；攒完再吐的话用户会盯着不动的界面等 |
| `TestPrompt_OnlyEndTurnIsSuccess` | `internal/acp/session/session_test.go` | acp | ★ R2：五种结束原因只有 `end_turn` 算正常；`max_tokens` 当成功的话，用户拿到截断的改动而界面显示「完成」 |
| `TestPrompt_EveryUpdateKindIsHandled` | `internal/acp/session/session_test.go` | acp | ★ R5：穷举 13 类事件都有去处；新增一类而没接住时会红，否则它静默消失、界面像是 AI 少说了点什么 |
| `TestPrompt_CarriesTextThrough` | `internal/acp/session/session_test.go` | acp | 文本原样带出，且思考摘要与正式消息**不混成一路**（界面上是两种显示） |
| `TestPublish_DoesNotFanOutWhenStoreFails` | `internal/eventbus/bus_test.go` | eventbus | ★★ R5：**落库失败就不扇出**——反过来会「前端收到了，重启后库里没有」，这种不一致比丢事件更糟；负例（颠倒顺序）立刻红 |
| `TestPublish_SeqIsMonotonic` | `internal/eventbus/bus_test.go` | eventbus | R1：序号单调递增、无洞 |
| `TestPublish_SeqContinuesAcrossRestart` | `internal/eventbus/bus_test.go` | eventbus | ★ R1 的另一半：序号**跨重启接续**——从 1 重来的话前端按 seq 去重会把新事件当旧的丢掉 |
| `TestSubscribe_ClosingReleasesSubscriber` | `internal/eventbus/bus_test.go` | eventbus | R3：订阅者被回收，**防泄漏** |
| `TestSubscribe_ContextCancelReleasesSubscriber` | `internal/eventbus/bus_test.go` | eventbus | ★ ctx 取消也回收——SSE 客户端断开走的正是这条路径，不会有人来调 Close |
| `TestPublish_SlowSubscriberDoesNotBlockOthers` | `internal/eventbus/bus_test.go` | eventbus | ★ R4：一个卡住的页面不该让 AI 的进度整个停下来 |
| `TestPublish_FansOutToAllSubscribers` | `internal/eventbus/bus_test.go` | eventbus | 扇出给所有订阅者，不是只给第一个 |
| `TestSubscription_CloseIsIdempotent` | `internal/eventbus/bus_test.go` | eventbus | 重复 Close 不 panic（SSE 清理路径会走两遍） |
| `TestEventRepo_AssignsSeq` | `internal/store/event_repo_test.go` | store | R1：序号由**自增主键**发放并写回，不由内存计数器发 |
| `TestEventRepo_SeqSurvivesRestart` | `internal/store/event_repo_test.go` | store | ★★ R1 的关键：真的关掉 Store 再打开同一个文件，序号接着往下走——从头发的话前端按 seq 去重会把新事件当旧的丢掉，表现是「重启后 AI 说的话不显示了」 |
| `TestEventRepo_MaxSeq` | `internal/store/event_repo_test.go` | store | 空库返回 0（`MAX()` 在空表上是 NULL 不是 0） |
| `TestEventRepo_EventsAfter` | `internal/store/event_repo_test.go` | store | ★ R2：断线重连只补没收到的——从头补会让整条时间线重放一遍，补少了则中间有洞而用户看不出来 |
| `TestEventRepo_EventsAfterRespectsLimit` | `internal/store/event_repo_test.go` | store | 补发有上限：断了一整天的客户端重连时不该被灌几万条 |
| `TestEventRepo_PayloadRoundTrips` | `internal/store/event_repo_test.go` | store | 载荷原样存取（含中文与嵌套）——它是界面上真正显示的内容 |
| `TestStreamEvents_SetsSSEHeaders` | `internal/api/events_test.go` | api | SSE 响应头对（`text/event-stream` + `no-cache`），否则浏览器不当它是事件流、中间层还可能缓存整条流 |
| `TestStreamEvents_DeliversLive` | `internal/api/events_test.go` | api | ★ 真流式：用**真 HTTP 服务器**而不是 ResponseRecorder（后者拿到的是「全部写完之后」，正好把边发边到测没了）；`id` 必须是 seq |
| `TestStreamEvents_ResumesFromLastEventID` | `internal/api/events_test.go` | api | ★★ R2：带 `Last-Event-ID` 重连只补它之后的；负例（忽略它从头补）立刻红 |
| `TestStreamEvents_FreshConnectionDoesNotReplayHistory` | `internal/api/events_test.go` | api | 全新连接不重放历史——它要的是「从现在开始」 |
| `TestStreamEvents_DisconnectReleasesSubscriber` | `internal/api/events_test.go` | api | ★ R3：客户端断开时订阅者被回收，防泄漏 |
| `TestStreamEvents_RequiresToken` | `internal/api/events_test.go` | api | 无 token 401——事件流能看到用户项目里发生的一切 |
| `TestStreamEvents_TolerantToBadLastEventID` | `internal/api/events_test.go` | api | 非法 `Last-Event-ID` 按全新连接处理而不是 500——为它回 500 会让用户彻底连不上 |
| `TestThoughtLevel_FoundByCategoryOnBothEnds` | `internal/acp/adapter/adapter_test.go` | acp | ★★ R1：**按 category 取而不是按 id**——两端 id 不同（effort / reasoning_effort）但 category 都是 thought_level；负例（退化成按 id）只有 codex 那端红，正是「差异没内化」的样子 |
| `TestConfigByCategory_ToleratesMissingAndEmpty` | `internal/acp/adapter/adapter_test.go` | acp | ★ category 缺失与空串是两回事，两种都不能 panic——claude 的 agent 选项是空串、codex 的 sandbox 根本没这字段，直接解引用会在其中一端崩 |
| `TestConfigByCategory_EmptyList` | `internal/acp/adapter/adapter_test.go` | acp | 空配置列表不是错误 |
| `TestCapabilities_DerivedFromProbes` | `internal/acp/adapter/adapter_test.go` | acp | R5：能力矩阵由探测结果算出，不写死——写死的话某个 Runtime 悄悄不支持时矩阵还显示通过 |
| `TestCapabilityMatrix_Supports` | `internal/acp/adapter/adapter_test.go` | acp | ★ 没探过的能力一律当不支持；负例（乐观返回 true）立刻红——乐观假设会让上层走进走不通的路 |
| `TestAddWorktree_LivesOutsideUserProject` | `internal/gitx/worktree_test.go` | gitx | ★★ R2：worktree 建在 `~/.acpflows/worktrees` **不在用户项目里**；负例（建进 `.duet-worktrees/`）两条断言各从不同角度抓到——路径前缀 + 用户仓库 git status 变了 |
| `TestAddWorktree_IsolatesWorks` | `internal/gitx/worktree_test.go` | gitx | ★ R1：两个工作互不干扰——共用目录的话两个 AI 会同时改同一份文件，用户看到一堆互相覆盖的改动而没有任何提示 |
| `TestAddWorktree_HasRepoContent` | `internal/gitx/worktree_test.go` | gitx | worktree 里有仓库内容，不是空目录 |
| `TestAddWorktree_IsIdempotent` | `internal/gitx/worktree_test.go` | gitx | 同一 WorkID 重复建返回已有的——重启后恢复工作走的正是这条路，报错的话工作就再也打不开了 |
| `TestRemoveWorktree_LeavesUserProjectAlone` | `internal/gitx/worktree_test.go` | gitx | ★ 移除不碰用户项目里的任何文件 |
| `TestRemoveWorktree_IsIdempotent` | `internal/gitx/worktree_test.go` | gitx | 移除不存在的不报错（清理路径会走两遍：正常结束 + 崩溃后重启） |
| `TestAddWorktree_RejectsNonRepo` | `internal/gitx/worktree_test.go` | gitx | 非 git 目录给明确错误，不是让 git 吐一句英文 |
| `TestStart_WritesNothingIntoUserProject` | `internal/app/work/service_test.go` | app | ★★ R2：建工作往用户项目写**零个字节**——worktree 用真 gitx 而不是假实现，假的什么都不写、那条断言会永远绿 |
| `TestStart_EachWorkGetsItsOwnWorktree` | `internal/app/work/service_test.go` | app | ★ R1：两个工作各有各的 worktree，文件改动互不可见 |
| `TestStart_TransitionsThroughInitializing` | `internal/app/work/service_test.go` | app | 从 initializing 起步再进 clarifying，且发 state_change 事件（界面靠它知道有新工作） |
| `TestStart_WorktreeFailureIsTerminal` | `internal/app/work/service_test.go` | app | ★★ 切失败进 initializing_failed 且**落库**；断言的是「最后一次落库时的状态」——内存仓储存指针的话，代码忘了 SaveWork 也测不出来（造负例两次没红才发现这个洞） |
| `TestStart_RejectsRelativePath` | `internal/app/work/service_test.go` | app | 相对路径在建工作前就拒——它会在 duetd 的工作目录下解析 |
| `TestStart_RejectsEmptyPrompt` | `internal/app/work/service_test.go` | app | 空需求被拒：没有需求的工作没有意义，而它会占着一个 worktree |
| `TestList_ReturnsAll` | `internal/app/work/service_test.go` | app | 列出全部工作 |
