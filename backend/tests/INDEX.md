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
| `TestCallInto_FailsWhenPeerDies` | `internal/acp/jsonrpc/conn_test.go` | acp | ★ **对方进程退出时，等着的调用立刻失败**（`ErrPeerGone`）而不是挂到 ctx 超时——Agent 起不来时用户看到的是界面停在「正在初始化」，没有转圈没有报错，只能杀应用 |
| `TestCallInto_FailsAfterServeStopped` | `internal/acp/jsonrpc/conn_test.go` | acp | 连接断掉**之后**新发起的调用也立刻失败，不等 ctx 超时 |
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
| `TestWorkRepo_SaveSurvivesRestart` | `internal/store/work_repo_test.go` | store | ★★ R6：真的关掉 Store 再打开同一个文件，工作与**它的状态**都还在——只存 ID 的话重启后全退回初始状态，用户会看到一堆「正在初始化」的僵尸条目 |
| `TestWorkRepo_SaveIsUpsert` | `internal/store/work_repo_test.go` | store | 同 ID 存两次是更新不是插入；upsert 而非 Create/Update 分开——分开的话调用方每次要先判断「存在吗」，那个判断在并发下必然出错 |
| `TestStartWork_ReturnsCreated` | `internal/api/works_test.go` | api | 201 + 带出 worktree 路径（「在哪干活」是用户会问的第一个问题） |
| `TestStartWork_NonRepoBecomesActionableProblem` | `internal/api/works_test.go` | api | ★ 非 git 仓库要有**能让用户自己解决**的错误码——落到通用错误的话界面只有一句「操作失败」，而他其实只需要 `git init` |
| `TestStartWork_RejectsBadRequest` | `internal/api/works_test.go` | api | 缺 project / 缺 prompt / 空值 / 非法 JSON 都不当成功 |
| `TestListWorks_EmptyIsArrayNotNull` | `internal/api/works_test.go` | api | 空列表是 `[]` 不是 `null` |
| `TestWorks_WithoutServiceDoNotPanic` | `internal/api/works_test.go` | api | 没配用例时不 panic 也不假装成功 |
| `TestWorks_RequiresToken` | `internal/api/works_test.go` | api | 无 token 401——工作能看到用户项目里正在发生的一切 |
| `TestClose_ReturnsWhenPeerStaysAlive` | `internal/acp/session/session_test.go` | acp | ★ Agent 一轮结束后**不退出**（它等下一条需求）时 `Close` 仍在 2s 内返回——死等的话两边互相等，表现是应用退不出去、Agent 进程越攒越多 |
| `TestRun_TranslatesUpdateKinds` | `internal/acp/agent/agent_test.go` | acp | M2 U2.4.1 R3：★ ACP 的判别值翻成**契约里的**事件类型（`agent_message_chunk`→`message_chunk`、`tool_call`/`tool_call_update`→`tool_call`、三种计划变化→`plan_version`）；靠字符串相等碰运气的话前端全落到兜底渲染器 |
| `TestTimelineType_CoversEveryKind` | `internal/acp/agent/agent_test.go` | acp | ★★ 本包的守卫：翻译表必须盖住 `protocol.AllSessionUpdateKinds()` 全集——Agent 新增一类事件时逼人做一次决定（翻成哪类，还是明确「认识但不上时间线」），否则它静静消失 |
| `TestRun_TagsEveryEventWithWorkID` | `internal/acp/agent/agent_test.go` | acp | 每条事件带 `work_id` 与 `source=acp`——前端靠 work_id 过滤，不带会串台 |
| `TestRun_EmitsTurnEndWithReason` | `internal/acp/agent/agent_test.go` | acp | 轮次结束必发 `turn_end` 且带**真实** stopReason（`end_turn`/`max_tokens`/`refusal`）——不发的话界面停在那儿，用户不知道是说完了还是卡住了 |
| `TestRun_CarriesText` | `internal/acp/agent/agent_test.go` | acp | 文本块的内容带进 payload，那是用户真正读的东西 |
| `TestRun_EmitsWhileRunning` | `internal/acp/agent/agent_test.go` | acp | ★ 事件**边说边发**不攒批（第一条 <500ms，整轮 1.5s）——攒的话用户盯着不动的界面等 |
| `TestRun_SkipsSessionMetadata` | `internal/acp/agent/agent_test.go` | acp | 会话元数据（用量、可用命令）不上时间线，只留 `turn_end`——塞进去会淹掉真正的进展 |
| `TestRun_PrependsSystemPromptOnce` | `internal/acp/agent/agent_test.go` | acp | M2 U2.2.2 R3 欠账：系统提示词只在**第一轮**拼上去，每轮都发的话几十轮后上下文全是重复的同一段话 |
| `TestRun_InvalidCwdEmitsNothing` | `internal/acp/agent/agent_test.go` | acp | cwd 非法时一个事件都不发——那意味着这一轮压根没开始 |
| `TestRun_DoesNotClobberAgentFields` | `internal/acp/agent/agent_test.go` | acp | ★★ 我们加的元信息一律 `acp_` 前缀，**不覆盖 Agent 自己的字段**——真机撞到：翻译层的 `kind` 顶掉了 ACP tool_call 自带的 `kind`（工具种类），前端拿到的值时对时不对，表现是四条工具调用卡片长得一模一样 |
| `TestProcessRunner_NoRuntimeReady` | `internal/acp/agent/runner_test.go` | acp | ★ 一个 Runtime 都没就绪时错误里带**补救办法**（`npm i -g ...`）——这句话会出现在时间线的失败事件里，是用户唯一能看到的线索 |
| `TestProcessRunner_ReportsAgentStderr` | `internal/acp/agent/runner_test.go` | acp | ★ Agent 起来又立刻退出时把它的 stderr 带回来（「not authenticated」）——不带的话真正的原因躺在一个没人读的管道里 |
| `TestProcessRunner_PublishesToBus` | `internal/acp/agent/runner_test.go` | acp | 事件发到**总线**（那是它去到界面的唯一通路）；对手方是 `t.TempDir()` 里按 ACP 规范说话的 shell 脚本，不是 Fake——要验的正是进程怎么拉起来 |
| `TestProcessRunner_LeavesNoOrphan` | `internal/acp/agent/runner_test.go` | acp | ★★ 一轮跑完 Agent 进程**不能还活着**（假 Agent 里放 `sleep 300`，跑完 `ps` 查 pid）——留着的话用户每提一个需求就多一个常驻进程，关掉应用后它们还在 |
| `TestProcessRunner_BusFailureDoesNotFailTurn` | `internal/acp/agent/runner_test.go` | acp | 总线发不出去不让整轮失败——AI 那边的活已经干了，报「失败」而磁盘上躺着改好的文件比不通知更糟 |
| `TestStart_RunsTurnInWorktree` | `internal/app/work/service_test.go` | app | ★★ M2 U2.4.1：工作建好后**真的把需求送给 AI**，且 cwd 是工作自己的 worktree 而非用户项目目录——后者等于让 AI 直接在他的分支上改文件 |
| `TestStart_TurnSurvivesRequestCancel` | `internal/app/work/service_test.go` | app | ★★ 这一轮脱开请求的 ctx（`WithoutCancel`）——挂在上面的话 HTTP 一返回 AI 就被砍掉，用户看到时间线停在半截且没有报错 |
| `TestStart_ReportsTurnFailure` | `internal/app/work/service_test.go` | app | AI 那一轮跑挂了要发 `state_change → failed` **并带原因**——静默的话用户对着「正在澄清需求」永远等 |
| `TestStart_NoTurnWhenWorktreeFails` | `internal/app/work/service_test.go` | app | worktree 没切成就不跑 AI——没有现场可以让它干活 |
| `TestStart_NilRunnerDoesNotPanic` | `internal/app/work/service_test.go` | app | 没配 runner 时（只跑 API 冒烟）不崩 |
| `TestAskPermission_R1_BlocksUntilAnswered` | `internal/acp/fake/permission_test.go` | acp | M3 U3.1.1 R1：★★ Fake 主动发 `session/request_permission` 并**阻塞等应答**——不阻塞的话上层裁决逻辑没有真实对手方，测试会以为「问过了」而 Agent 根本没等答案 |
| `TestAskPermission_R2_CancelledEndsTurnAsCancelled` | `internal/acp/fake/permission_test.go` | acp | M3 U3.1.1 R2：应答 `cancelled` 时这一轮以 **cancelled** 收尾而非脚本里的 `end_turn`——用户拒了却显示「完成」比不问更糟 |
| `TestAskPermission_R5_NeverTimesOut` | `internal/acp/fake/permission_test.go` | acp | M3 U3.1.1 R5：★★ 权限请求**没有超时**（干等 2s 这一轮仍未结束）——真 Agent 会一直等用户，Fake 自作主张超时的话上层「等用户裁决」的逻辑测不出来 |
| `TestAskPermission_R4_OptionIDRoundTripsVerbatim` | `internal/acp/fake/permission_test.go` | acp | M3 U3.1.1 R4：★★ `optionId` **原样回传**。脚本用一组 id 与 kind 语义相反的选项（`opt-allow` 的 kind 是 `reject_once`），按类别猜 id 的实现在这里翻车；同时断言 Fake 没有「顺手纠正」它们 |
| `TestNewSession_R3_UndeclaredCapabilityIsAbsentOnTheWire` | `internal/acp/fake/permission_test.go` | acp | M3 U3.1.1 R3：声明的能力**表现为真实协议行为**——不声明 modes 时 `session/new` 响应里真的没有那个字段，否则测的是我们自己的探针代码 |
| `TestAskPermission_CarriesToolCall` | `internal/acp/fake/permission_test.go` | acp | 权限请求带 `toolCallId` 与 `sessionId`——不带的话界面只能问「要不要允许？」而说不出允许什么 |
| `TestAskPermission_TwoAsksInOneTurn` | `internal/acp/fake/permission_test.go` | acp | 一轮里问两次，各自阻塞各自等应答（真 Agent 一轮问三五次是常态）；只处理第一次的实现会在这里挂住 |
| `TestDecide_R1_AutoAllowReadonlyOnlyCoversReadKinds` | `internal/acp/session/permission_test.go` | acp | M3 U3.1.2 R1：★★ 「自动允许只读」**只对读类工具生效**，穷举 `ToolKind` 全集。只读是**白名单**（read/search/think）——反过来列危险的话，Agent 新增一类工具会默认落进「自动允许」，用户以为开的是「让它随便看」而实际是「让它随便改」 |
| `TestDecide_R2_NeverGuessesAnOptionID` | `internal/acp/session/permission_test.go` | acp | M3 U3.1.2 R2：★★ 选项集合不认识时走保守分支，**绝不猜 optionId**（想允许却没有 allow 类 / 一个选项都没有 / 类别不认识）。猜的话可能替用户点了「永久允许」 |
| `TestDecide_AskAlwaysDefers` | `internal/acp/session/permission_test.go` | acp | 「每次都问」就是每次都问，不因为是只读就放过 |
| `TestDecide_RejectAllCoversEveryKind` | `internal/acp/session/permission_test.go` | acp | 「一律拒绝」对所有类别都拒绝，包括只读——用户明确表达的「我先看着，什么都别动」 |
| `TestDecide_PrefersOnceOverAlways` | `internal/acp/session/permission_test.go` | acp | ★★ 优先 `once` 而非 `always`（脚本故意把 always 排前面）——选 always 的话，一次自动裁决会永久改变后续所有请求，而用户不知道发生过 |
| `TestDecide_R4_ReasonIsAMachineCode` | `internal/acp/session/permission_test.go` | acp | M3 U3.1.2 R4：理由码是**机器可读枚举**（断言全 ASCII）——界面按它查词条，塞人话会把日志过滤规则改坏且没法翻译 |
| `TestDecide_UnknownPolicyDefersToUser` | `internal/acp/session/permission_test.go` | acp | 认不出的策略回落到最保守的「交给用户」——配置被手改坏或旧版本遗留一个已删除的策略名时，默认放行是灾难 |
| `TestAllPolicies_CoversEveryDecideBranch` | `internal/acp/session/permission_test.go` | acp | 策略全集与 `Decide` 的分支必须同步——加策略时漏一处，界面上选了它会变成「交给用户」 |
| `TestSession_AutoDecidesWithoutAskingUser` | `internal/acp/session/permission_test.go` | acp | 策略能自动裁决时这一轮不停下来等人——否则「自动允许只读」这个开关等于没有 |
| `TestSession_UserChoiceGoesBackVerbatim` | `internal/acp/session/permission_test.go` | acp | ★★ 用户选的 `optionId` **原样回给 Agent**。脚本用 id 与 kind 语义相反的选项——按类别重新匹配的话，用户点「拒绝」而 Agent 收到「允许」 |
| `TestSession_AskUserFailureAnswersCancelled` | `internal/acp/session/permission_test.go` | acp | ★ 用户那边出错（界面关了、超时）时回 `cancelled`——随便挑一个的话，用户关掉窗口就等于默认同意了 |
| `TestSession_NoAskUserAnswersCancelled` | `internal/acp/session/permission_test.go` | acp | ★★ 没配 `AskUser` 时回 `cancelled` 而非自动允许——装配漏一根线的表现必须是「什么都干不了」，不能是「什么都放行」 |
| `TestSession_R4_EmitsDecisionWithReasonCode` | `internal/acp/session/permission_test.go` | acp | M3 U3.1.2 R4：裁决发一条带理由码的事件——用户回头问「它为什么没问我就改了文件」，答案得在时间线上找得到 |
| `TestSession_R3_BlockingIsPerSession` | `internal/acp/session/permission_test.go` | acp | M3 U3.1.2 R3：★★ 阻塞**只是这一轮**——A 卡在权限请求上时 B 照常跑完。★ B 也必须发权限请求，否则加一把全局锁这条测试照样绿（造负例时发现的） |
| `TestAsk_R1_PublishesEventWithOptions` | `internal/app/permission/broker_test.go` | app | M3 U3.1.4 R1：★★ 权限请求**作为事件发出去**且载荷带齐 `ask_id`/`tool_call_id`/`runtime`/`options`——只放内存的话界面永远不知道有人在等它 |
| `TestAsk_R2_AnswerReturnsTheChosenOptionVerbatim` | `internal/app/permission/broker_test.go` | app | M3 U3.1.4 R2：用户选的 `optionId` 原样交回给等着的那一方 |
| `TestAnswer_R4_SecondAnswerIsRejected` | `internal/app/permission/broker_test.go` | app | M3 U3.1.4 R4：★★ 同一条请求只能应答一次，第二次报 `ErrNotPending` **且不覆盖第一次的值**——Agent 只在等一个响应，多发的会被静静丢弃 |
| `TestAnswer_UnknownAskIsRejected` | `internal/app/permission/broker_test.go` | app | 应答不存在的请求要报错——静静成功的话界面以为处理完了，而 AI 还在等 |
| `TestAnswer_WrongWorkIsRejected` | `internal/app/permission/broker_test.go` | app | ★ 校验 workID：两个工作同时开着时，一次误点不该应答到另一个头上 |
| `TestCancelWork_R5_ReleasesEveryPendingAsk` | `internal/app/permission/broker_test.go` | app | M3 U3.1.4 R5：★★ 取消一个工作时它 pending 的请求**全部**被叫醒——漏一条那条会话永远挂着，进程退不出去 |
| `TestCancelWork_DoesNotTouchOtherWorks` | `internal/app/permission/broker_test.go` | app | 取消一个工作不影响别的工作——用户取消一个，另一个不该跟着废 |
| `TestAsk_ContextCancelReleases` | `internal/app/permission/broker_test.go` | app | ctx 结束时解除等待，不永久挂着 |
| `TestAsk_BusFailureDoesNotHang` | `internal/app/permission/broker_test.go` | app | ★★ 事件发不出去时**立刻失败不挂起**——挂起的话界面根本不知道有这条请求，而 AI 一直等，用户没有任何提示 |
| `TestPending_ListsOpenAsksForWork` | `internal/app/permission/broker_test.go` | app | Pending 按工作隔离，应答后摘掉——不摘的话界面刷新会重复画出卡片 |
| `TestAnswerPermission_PassesOptionIDVerbatim` | `internal/api/permission_test.go` | api | M3 U3.1.4 R2：★★ 端点**不做任何加工**，`option_id` 原样往下传——这一层不知道 Agent 定义了什么 |
| `TestAnswerPermission_R4_DuplicateIs409` | `internal/api/permission_test.go` | api | M3 U3.1.4 R4：★★ 重复应答翻成 **409** 而非 500——500 会让界面提示「再试一次」，而用户一试就又发一条，无限循环 |
| `TestAnswerPermission_RejectsMissingFields` | `internal/api/permission_test.go` | api | ★ 缺 `ask_id` / 空 `option_id` 一律 400 且**不转调**——空 id 送到 Agent，这一轮会以没人预料的方式收场 |
| `TestAnswerPermission_UnconfiguredIs503` | `internal/api/permission_test.go` | api | 没配 Broker 时回 503 而非 404——404 会让人以为是路径写错了，而真正的原因是装配漏了一根线 |
| `TestAnswerPermission_RequiresToken` | `internal/api/permission_test.go` | api | 没带 token 一律 401 且不转调——回环上任何本机进程都能替用户点「允许」 |
| `TestRun_PassesPermissionConfigToSession` | `internal/acp/agent/agent_test.go` | acp | 权限配置真的传给会话——不传的话会话拿零值，「自动允许只读」在真跑时完全不生效 |
| `TestProcessRunner_BindsWorkIDIntoAskUser` | `internal/acp/agent/agent_test.go` | acp | ★ 交给用户时把 workID 绑进去——不绑的话事件发不到对的时间线，用户会在 A 工作里看到 B 工作的权限卡片 |
| `TestProcessRunner_NoAskUserLeavesItNil` | `internal/acp/agent/agent_test.go` | acp | 没配 AskUser 时留 nil，不塞一个假装有人在接的空函数 |
| `TestLogSink_WriteAfterCloseDoesNotPanic` | `internal/store/log_sink_test.go` | store | ★★ Close 之后写日志**不许 panic**。真机撞到：进程启动失败 → main 用 `slog.Error` 记原因 → sink 已关 → 「send on closed channel」，而**真正的失败原因（端口被占）被 panic 栈完全盖住** |
| `TestLogSink_DoubleCloseIsSafe` | `internal/store/log_sink_test.go` | store | 重复 Close 不炸——优雅退出路径上很容易调两次 |
| `TestSession_PermissionAskCarriesPath` | `internal/acp/session/permission_test.go` | acp | ★★ 权限请求带上**动的是哪个文件**：优先 `locations`（Agent 明确标出的位置），退到 `rawInput.file_path`/`path`/`filePath`，都没有时留空。真机撞到：卡片上只写「AI 请求写入」，用户没法判断该不该允许 |
| `TestProcessRunner_ShortensPathToProjectRelative` | `internal/acp/agent/agent_test.go` | acp | ★★ 交给用户的是**项目内相对路径**（`README.md`）而非 worktree 绝对路径（`/Users/…/worktrees/work-01/README.md`）——后者把卡片撑成两行，还把「worktree 放哪」摊给用户。★ worktree **之外**的保持原样：AI 要动 `/etc/hosts` 时完整路径才是有信息量的 |
| `TestCancel_R1_IsIdempotent` | `internal/acp/session/cancel_test.go` | acp | M3 U3.2.1 R1：★★ **同时**点三下只发一条 `session/cancel`。★ 串行调用测不到——第一次之后这一轮就结束了，后两次走「没在跑，空操作」那条路，把幂等判断整个删掉照样绿（造负例时发现的） |
| `TestCancel_R3_AnswersEveryPendingPermission` | `internal/acp/session/cancel_test.go` | acp | M3 U3.2.1 R3：★★ 取消时用 `cancelled` 应答**所有** pending 的权限请求。ACP 硬要求且设计稿完全没提——漏了每次取消都超时，而超时直接连着 M1 的一键更新（`update/prepare` 永远返回 blocked） |
| `TestCancel_R4_TimeoutIsDiagnosable` | `internal/acp/session/cancel_test.go` | acp | M3 U3.2.1 R4：超时错误里带**会话标识与等了多久**——只说「timeout」的话，用户提了工单也查不出是哪条会话、卡了多久 |
| `TestCancel_R5_TimeoutDemandsKill` | `internal/acp/session/cancel_test.go` | acp | M3 U3.2.1 R5：★★ 超时返回的错误让 `MustKill` 为真，**且协议取消照样发了**（能停就停，停不下来再杀）。这是「界面说已取消、后台还在烧钱改文件」的唯一防线 |
| `TestCancel_NormalCancelDoesNotDemandKill` | `internal/acp/session/cancel_test.go` | acp | 正常取消不要求杀进程——那样每次取消都要重拉一个 Agent |
| `TestCancel_R2_CursorSurvivesCancel` | `internal/acp/session/cancel_test.go` | acp | M3 U3.2.1 R2：取消之后会话标识与已收事件都还在——用户点了停，第一件想知道的是「它停在哪一步」 |
| `TestCancel_WhenIdleIsNoop` | `internal/acp/session/cancel_test.go` | acp | 空闲时取消不报错也不发协议取消。★ 「在跑」的判据是独立的 `inTurn`，**不能拿 `onEvent != nil`**——调用方可以不关心事件而传 nil，那时用户点了停会毫无反应而 AI 照跑 |
| `TestCancel_FakeRecordsEveryNotificationVerbatim` | `internal/acp/fake/permission_test.go` | acp | ★★ Fake **如实记录**每一条 `session/cancel`，绝不去重。去重是被测代码的职责——Fake 替它做了的话 R1 会永远绿 |
| `TestCanCancel_CoversEveryState` | `internal/domain/model/cancellable_test.go` | domain | M3 U3.2.3 R3：★★ **每个** `WorkState` 都明确表态能不能取消，对着 `constant.AllWorkStates()` 逐个核。新增状态忘了表态就红——漏掉的默认行为无论倒向哪边都会咬人 |
| `TestCanCancel_RejectsReviewing` | `internal/domain/model/cancellable_test.go` | domain | ★★ 审查中拒绝取消——半路掐掉的话那个单元既没通过也没被驳回，用户回头看「它到底做完没有」答案是「不知道」 |
| `TestCanCancel_UnknownStateIsRefused` | `internal/domain/model/cancellable_test.go` | domain | 认不出的状态一律不许取消（最保守）——数据库里存了个不认识的值时，默认允许会把一个完全不了解的状态强行推走 |
| `TestCancel_R1_RefusesWhileReviewing` | `internal/app/work/service_test.go` | app | M3 U3.2.3 R1：★★ 审查中拒绝取消，**并且一次协议取消都不发**——先问规则再动手，反过来的话已经掐掉了才发现不该掐 |
| `TestCancel_R2_RefusalCarriesACode` | `internal/app/work/service_test.go` | app | M3 U3.2.3 R2：拒绝的理由是机器可读的码（`work_cancel_not_allowed`），界面按它查词条 |
| `TestCancel_R4_PausesAndCheckpoints` | `internal/app/work/service_test.go` | app | M3 U3.2.3 R4：取消成功后进 `paused` **并落检查点事件**——不落的话，用户回头想接着干，没有任何东西告诉他上次停在哪 |
| `TestCancel_R5_TimeoutKillsAndFails` | `internal/app/work/service_test.go` | app | M3 U3.2.3 R5：★★ 停不下来就**杀进程 + 推到 failed + 带原因码**。只报错不杀的话，界面说「取消失败」而那个 Agent 还在后台改文件 |
| `TestCancel_UnknownWorkIsRejected` | `internal/app/work/service_test.go` | app | 取消不存在的工作要报错，不是静静成功 |
| `TestCancel_NoCancellerIsAnError` | `internal/app/work/service_test.go` | app | 没人能执行取消时明确报错——静静成功的话界面显示「已停止」而 AI 照跑，账单继续涨 |
| `TestCancelWork_HappyPathIs204` | `internal/api/works_test.go` | api | 取消端点正常路径 204 并转调一次 |
| `TestCancelWork_NotAllowedIs409` | `internal/api/works_test.go` | api | ★★ 「现在不能停」翻成 **409** 而非 500——500 会让界面提示「再试一次」，而用户一试还是同样结果 |
| `TestCancelWork_UnknownIs404` | `internal/api/works_test.go` | api | 工作不存在时 404 而非 500 |
| `TestCancelWork_RequiresToken` | `internal/api/works_test.go` | api | 没带 token 401 且不转调——回环上任何本机进程都能停掉用户的工作 |
| `TestProcessRunner_CancelIdleWorkIsNoop` | `internal/acp/agent/runner_test.go` | acp | 没在跑的工作取消不报错——报错的话 app 层会把一个已经成功结束的工作推到 failed |
| `TestProcessRunner_ForgetsFinishedTurns` | `internal/acp/agent/runner_test.go` | acp | ★★ 跑完就摘掉记录——留着的话，取消一个早就结束的工作会去动一条已经关掉的会话 |
| `TestProcessRunner_CancelRunningTurnDemandsKillWhenDead` | `internal/acp/agent/runner_test.go` | acp | ★★ 用真进程验 `KillAgent` 能把装死的 Agent 收掉。★ **先 track 进程再握手**——反过来的话，连 initialize 都不回的 Agent 会让 `session.Open` 挂住，而那时 KillAgent 找不到它（造这条测试时发现的） |
| `TestCanCancel_MatchesTransitionTable` | `internal/domain/model/cancellable_test.go` | domain | ★★ 「能取消」与「迁得到 paused」必须一致。真机撞到：`CanCancel(clarifying)` 是 true 而迁移表里没这条出边——用户点了停，规则放行、状态机拒绝，他看到一句「invalid work state transition」。★ 只要求单向：`paused` 的入边不止「用户点停」（更新前的暂停也走它） |
| `TestCancel_UserCancelIsNotAFailure` | `internal/app/work/service_test.go` | app | ★★ **用户主动停 ≠ AI 跑挂了**。真机撞到：点停之后那一轮因 `stopReason=cancelled` 返回错误，被「跑挂了」那条路径推到 failed，抢在 paused 之前——用户明明是自己点的停，界面说「失败」。★ 取消标记由 `runTurn` 的 goroutine 自己清（只有它知道什么时候真跑完），在 Cancel 里清的话序列会是 `[… paused failed]` |
| `TestPrompt_R2_SameSessionAcrossTurns` | `internal/acp/session/resume_test.go` | acp | M4 U4.1.1 R2：★★ 两轮落在**同一个 sessionId** 上。这是前一个项目最严重的一条错误——标识在某一层丢了，第 2 轮对 Agent 是全新对话，而界面看着完全正常，用户只觉得「AI 忽然变笨了」 |
| `TestResume_R1_ContinuesExistingSession` | `internal/acp/session/resume_test.go` | acp | M4 U4.1.1 R1：走 `session/load` 接上旧会话（不是 `session/new`），恢复后 `ID()` 是那一条、`IsFresh()` 为假 |
| `TestResume_NewTurnStaysOnResumedSession` | `internal/acp/session/resume_test.go` | acp | 恢复之后新一轮打在被恢复的那条会话上——恢复了却打在别的会话上等于白恢复 |
| `TestResume_R3_FallsBackToFreshSessionExplicitly` | `internal/acp/session/resume_test.go` | acp | M4 U4.1.1 R3：★★ Agent 不支持 `session/load`（回 -32601）时**显式降级**：`IsFresh()` 为真、换新 ID、`ResumeError()` 有值。假装成功的话，用户接着上文提问而 AI 一无所知，双方都不知道发生了什么 |
| `TestResume_ValidatesCwdBeforeTalking` | `internal/acp/session/resume_test.go` | acp | 先校验 cwd 再说话——反过来的话 Agent 已经加载了会话而我们报了错，它挂在那儿占资源 |
| `TestResume_EmptySessionIDOpensFresh` | `internal/acp/session/resume_test.go` | acp | 没有旧 ID 时直接开新的，不发一个注定失败的 load |
| `TestResume_ErrorMessageIsReadable` | `internal/acp/session/resume_test.go` | acp | 降级原因里带上是哪条会话——排查时得看得出「哪条没恢复上」 |
| `TestListResumable_R1_OnlyPausedWorks` | `internal/app/checkpoint/service_test.go` | app | M4 U4.1.2 R1：★ **只列 paused**（穷举全部状态核对）——把跑完的、失败的也列出来的话，用户对着一串条目不知道该点哪个 |
| `TestListResumable_EmptyIsASlice` | `internal/app/checkpoint/service_test.go` | app | 空结果返回空切片而非 nil——api 层要序列化成 `[]`，前端对 null 调 `.map()` 会白屏 |
| `TestListResumable_R1_CarriesPausedAt` | `internal/app/checkpoint/service_test.go` | app | 带上暂停时间——开着三四个工作时，用户靠它认出哪个是刚才那个 |
| `TestResume_R3_DirtyWorktreeIsReported` | `internal/app/checkpoint/service_test.go` | app | M4 U4.1.2 R3：★★ 工作区被手工改过时**先告知、且一个字节都不改**（状态仍是 paused）——先斩后奏的话，用户点「取消」也来不及了。★ 用**真 `gitx.IsDirty`**，假实现只会回一个我们自己设的布尔值 |
| `TestResume_UntrackedFileCountsAsDirty` | `internal/app/checkpoint/service_test.go` | app | ★★ **未跟踪的新文件也算脏**。★ 与上一条分开写：上一条改的是已跟踪文件，把 `status --porcelain` 换成 `diff --name-only` 它照样绿（造负例时发现的） |
| `TestResume_ForcedProceedsAndKeepsChanges` | `internal/app/checkpoint/service_test.go` | app | 确认后恢复，**不覆盖用户的改动**（本单元 forbidden_changes 明写） |
| `TestResume_R2_RestoresRunnableState` | `internal/app/checkpoint/service_test.go` | app | M4 U4.1.2 R2：恢复后状态回到能接着跑的那一个，且与落库一致 |
| `TestResume_NonPausedIsRejected` | `internal/app/checkpoint/service_test.go` | app | ★ 断言**具体的** `ErrNotResumable`，不是「有错就行」——状态检查删掉之后后面的 Transition 也会失败，「有错就行」照样绿（造负例时发现的） |
| `TestResume_UnknownWorkIsRejected` | `internal/app/checkpoint/service_test.go` | app | 恢复不存在的工作要报错，不是静静成功 |
| `TestResume_MissingWorktreeIsAnError` | `internal/app/checkpoint/service_test.go` | app | ★ 断言**上游的真实原因**传上来了：不校验的话路径变空串、脏检查在空路径上失败，照样返回错误，而「工作区没了」被「查工作区状态失败」盖住（造负例时发现的） |
| `TestListResumable_ReturnsItems` | `internal/api/resume_test.go` | api | `/v1/system/resume` 返回条目与暂停时间 |
| `TestListResumable_EmptyIsJSONArray` | `internal/api/resume_test.go` | api | ★★ 空时序列化成 `[]` 而非 `null`——「一个可恢复的都没有」正是绝大多数用户每次打开应用时的状态 |
| `TestListResumable_UnconfiguredIs503` | `internal/api/resume_test.go` | api | 没配服务回 503 而非 404 |
| `TestListResumable_FailureIsNotAnEmptyList` | `internal/api/resume_test.go` | api | ★★ 查询失败**绝不降级成空列表**——那会让用户以为「没有可恢复的」，而实际是查不了，他会以为自己的工作丢了 |
| `TestListResumable_RequiresToken` | `internal/api/resume_test.go` | api | 没带 token 401 |
| `TestPlanVersion_R1_HasNoMutators` | `internal/domain/model/plan_test.go` | domain | M4 U4.2.1 R1：★★ 反射断言 `PlanVersion` **没有任何指针接收者的导出方法**（计划只能新增版本不能改写，INV-PLAN-4）。★ 必须对**指针类型**取方法集——值类型的方法集不含指针接收者的方法，加一个 setter 上去照样绿（造负例时发现的） |
| `TestUnitContract_R1_FrozenIsImmutable` | `internal/domain/model/plan_test.go` | domain | ★★ 对着冻结的契约把每个导出方法都调一遍，核对内容没变。★ 判据不能是「有没有指针接收者」——`UnitID()` 这类读方法用指针接收者完全正常 |
| `TestUnitContract_CriteriaIsACopy` | `internal/domain/model/plan_test.go` | domain | ★★ `Criteria()` 返回副本，改它不影响契约——返回内部切片的话，任何拿到它的人都能改掉一份冻结的契约，「冻结」两个字名存实亡（造负例时发现「修订后旧契约几条标准」那条盖不住它） |
| `TestPlanVersion_R2_RequiresDispositionForEveryAccepted` | `internal/domain/model/plan_test.go` | domain | M4 U4.2.1 R2：★★ v ≥ 2 必须对**每一项**已验收工作给出处置；漏一项、多给一个没验收过的、取值不认识——各有各的错误码 |
| `TestPlanVersion_FirstVersionNeedsNoDisposition` | `internal/domain/model/plan_test.go` | domain | v1 不需要处置——那时还没有任何已验收的工作 |
| `TestDisposition_AllValuesAreValid` | `internal/domain/model/plan_test.go` | domain | 处置是封闭枚举（仍有效/需补充/需回滚/已废弃），标识全 ASCII（界面按它查词条） |
| `TestPlanVersion_R4_VersionIncrementsByOne` | `internal/domain/model/plan_test.go` | domain | M4 U4.2.1 R4：版本号严格递增不跳号——跳号的话用户看到 v1 之后是 v3，会以为自己漏看了几版 |
| `TestUnitContract_R3_FrozenRejectsChanges` | `internal/domain/model/plan_test.go` | domain | M4 U4.2.1 R3：冻结后加标准报 `ErrContractFrozen`——之后还能改的话，AI 可以一边做一边把标准改成自己刚好达到的样子 |
| `TestUnitContract_FreezeIsIdempotent` | `internal/domain/model/plan_test.go` | domain | 重复冻结不报错——用户点两下「冻结」是常态 |
| `TestUnitContract_EmptyCannotFreeze` | `internal/domain/model/plan_test.go` | domain | ★ 空契约不许冻结——「做完了」没有任何判据，AI 说做完了就是做完了 |
| `TestUnitContract_R4_VersionIncrementsByOne` | `internal/domain/model/plan_test.go` | domain | 契约版本号严格递增不跳号 |
| `TestUnitContract_RevisedIsUnfrozen` | `internal/domain/model/plan_test.go` | domain | 修订出来的新契约没冻结（能继续加标准），而**旧的那份一个字不变**——那是「修订」与「改写」的全部区别 |
| `TestPlanModel_StaysPure` | `internal/domain/model/plan_test.go` | domain | domain 是纯计算：构造与校验都不需要 context、不取当前时间 |
| `TestPresetRoles_R1_EightRolesMatchDesign` | `internal/domain/model/role_test.go` | domain | M2 U2.1.1 R1：★ 八个预置角色的 id / 显示名 / 承担操作逐条对上 `INVENTORY.md` §八，**顺序就是设计稿行序**（界面照它渲染）。数量单独断言——第一版清单只抽到 5 个，漏了滚动区外那三行 |
| `TestPresetRoles_R1b_EveryOperationHasAnOwner` | `internal/domain/model/role_test.go` | domain | M2 U2.1.1 R1b（INV-ROLE-6）：★★ 11 个 AI 操作每个都有且只有一个角色认领；漏派的后果是**跑到那一步才发现没人干**，而那时计划已经排好了 |
| `TestRoleByID_R4_UnknownRoleErrsNoFallback` | `internal/domain/model/role_test.go` | domain | M2 U2.1.1 R4：认不出的角色名返回 `ErrUnknownRole` 且不返回任何角色。★ 静默回落的后果是「本该审查员做的事被实现工程师做了」，而实现方审查自己正是 INV-ATT-8 禁止的 |
| `TestRoleByID_R4b_EveryPresetIsRetrievable` | `internal/domain/model/role_test.go` | domain | 八个预置角色都按 id 取得出来，取出来的显示名对得上 |
| `TestRole_INVROLE2_HasNoModelOrEffortField` | `internal/domain/model/role_test.go` | domain | INV-ROLE-2：★ 反射断言 `Role` 没有 model / reasoning_effort 一类字段——ACP 不提供这两个设置，它们是观测结果不是配置项（§16.5）。加上去毫不费力且加完测试照绿，所以用反射守 |
| `TestPresetRoles_R5_UserFacingRolesAreReadOnly` | `internal/domain/model/role_test.go` | domain | M2 U2.1.1 R5（**Q42**）：需求分析师 / 计划架构师 / 单元设计师 / 实现审查员 / 记忆管理员五个角色的会话档位必须是只读。★ 判据在档位上而不在权限裁决上——实测证明客户端全拒拦不住沙箱内的写 |
| `TestPresetRoles_R5b_OnlyImplementerMayWrite` | `internal/domain/model/role_test.go` | domain | 反向穷举：除实现工程师外任何角色拿到写权限就红（上一条只查名单里的五个，漏掉新角色时测不出）；「放开」档一个都不该用上 |
| `TestPermissionPolicy_R6_ClosedEnumOfTwo` | `internal/domain/model/role_test.go` | domain | M2 U2.1.1 R6：权限裁决只有「逐条询问 / 自动允许读」——设计稿的下拉里**没有**「一律拒绝」，别把旧 `session.Policy` 那三种搬过来 |
| `TestRole_R6b_WithPermissionPolicyRejectsInvalid` | `internal/domain/model/role_test.go` | domain | `WithPermissionPolicy` 拒非法值、返回新实例、原实例不变 |
| `TestPresetRoles_R7_ReturnsCopies` | `internal/domain/model/role_test.go` | domain | ★ `PresetRoles()` / `Operations()` 返回副本。不返回副本的话调用方一个 `sort.Slice` 就把预置表顺序永久改了，而界面照那个顺序渲染 |
| `TestAllOperations_R7b_ReturnsCopy` | `internal/domain/model/role_test.go` | domain | 同上，操作全集也是副本 |
| `TestPresetRoles_FourElementsAllFilled` | `internal/domain/model/role_test.go` | domain | §16.2 的四要素（职责 / 性格 / 边界 / 产出）都填了——边界是可测的约束不是文案，空着的话角色卡是一片空白 |
| `TestModeNameOn_R3_SameIntentDiffersPerRuntime` | `internal/acp/runtime/mode_test.go` | acp | M2 U2.1.1 R3：★★ 同一个「只读」claude 叫 `plan`、codex 叫 `read-only`，**一个字都不一样**。硬编码成某一端的取值 → 换 Runtime 时发出对方不认识的档名 → 收权失败，而失败的表现是「沙箱照旧放行」 |
| `TestModeNameOn_R3b_CodexHasNoAutoMode` | `internal/acp/runtime/mode_test.go` | acp | ★ 设计稿角色表给 codex 写的 `auto` 是 **0.16.0 的旧档名**，1.1.7 已改；照设计稿抄会静默收权失败（`acp-field-notes.md` §7 裁定 2） |
| `TestModeNameOn_R3c_UnknownRuntimeErrsNoGuess` | `internal/acp/runtime/mode_test.go` | acp | 没登记映射的 Runtime 返回 `ErrUnknownRuntime` 且不返回档名——猜一个出来等于在不知道权限多大的情况下开工 |
| `TestModeNameOn_R3d_MapHasNoHoles` | `internal/acp/runtime/mode_test.go` | acp | 三个语义档 × 两个 Runtime 全部翻译得出；漏填一格要到真开会话时才发现 |
| `TestRecommendedRuntimeFor_R2_EightPresetBindings` | `internal/acp/runtime/mode_test.go` | acp | M2 U2.1.1 R2：八个角色的推荐 Runtime 对上设计稿角色表。★ 「推荐」是认真的——设计稿有「恢复推荐绑定」按钮，说明用户改得动 |
| `TestRecommendedRuntimeFor_R2b_UnlistedRoleErrs` | `internal/acp/runtime/mode_test.go` | acp | 自定义角色没有推荐绑定时报错，不给「反正 claude 能干」的默认值 |
| `TestPresetRoles_R2c_BindingAndModeFitTogether` | `internal/acp/runtime/mode_test.go` | acp | ★★ 把两张表**连起来**验：单看都对，合起来可能是「某角色推荐 codex，而 codex 上没有它要的那一档」——那种错只在真开会话时暴露 |
| `TestApplyMode_R1_PrefersSetConfigOption` | `internal/acp/session/mode_test.go` | acp | M2 U2.1.2 R1：两个方法都可用时走 `set_config_option`，`set_mode` 调用次数为 0。★ 后者官方已挂废弃告示，先试旧的会让代码一直走在废弃路径上直到它被移除 |
| `TestApplyMode_R2_ParamIsConfigIdNotOptionId` | `internal/acp/session/mode_test.go` | acp | M2 U2.1.2 R2：★★ 参数名是 `configId`。判据是**档位真的变了**（Fake 那侧回读），不是「请求发出去了」——写成 `optionId` 时 Agent 什么都不设而响应仍然成功。另外直查线上帧的键名 |
| `TestApplyMode_R3_LooksUpByCategoryNotID` | `internal/acp/session/mode_test.go` | acp | M2 U2.1.2 R3：按 `category` 取配置项。三个不同的 id 都要能取到，且前面塞了别的 category 的项以排除「取第一项蒙对」 |
| `TestApplyMode_R4_FallsBackToSetMode` | `internal/acp/session/mode_test.go` | acp | M2 U2.1.2 R4：只声明 `modes` 没有 `configOptions` 时降级到 `set_mode`，且档位真的设进去了。「不支持」的判据是 Agent 自己声明的能力 |
| `TestApplyMode_R5_RefusesWhenNeitherSupported` | `internal/acp/session/mode_test.go` | acp | M2 U2.1.2 R5：★★ 两个都不支持 → `ErrCannotRestrictMode`，不返回可用会话，**且一句 prompt 都不发**。收不了权还继续跑等于让 AI 在不受限档位上动用户代码 |
| `TestApplyMode_R6_VerifiesByReadingBack` | `internal/acp/session/mode_test.go` | acp | M2 U2.1.2 R6：要设的档位不在可选值里 → 发之前就拒绝（`ErrModeNotAvailable`） |
| `TestApplyMode_R6b_ErrsWhenReadBackDiffers` | `internal/acp/session/mode_test.go` | acp | ★ Agent 收下请求、回成功、但值没变（`IgnoreConfigWrites`）→ 回读发现不一致，报 `ErrModeNotApplied`。与 R6 的区别：那条是发之前查出来，这条是发之后才发现——只有前者的话，Agent 悄悄忽略一个合法请求时我们照样一路绿灯 |
| `TestApplyMode_EmptyModeSkipsRestriction` | `internal/acp/session/mode_test.go` | acp | `RequiredModeID` 留空时不发任何收权请求——不是所有会话都需要限制 |
| `TestApplyMode_LegacyRejectsUnavailableMode` | `internal/acp/session/mode_test.go` | acp | 降级路径上档位不在 `availableModes` 里也要拒绝，且不发 `set_mode` |
| `TestValidateSkill_R2_ExplainsWhyItFailed` | `internal/domain/model/skill_test.go` | domain | M2 U2.2.1 R2（INV-SKL-2）：★ 校验不过要说清**为什么**（缺 name / description / SKILL.md / 版本号形态），两个字段都缺时一次说全。静默拒绝的话用户看到 draft 却不知道改什么，只能删了重建——而重建出来还是 draft |
| `TestValidateSkill_R2b_MissingFileTakesPrecedence` | `internal/domain/model/skill_test.go` | domain | ★ 缺 `SKILL.md` 时不能报成「缺 description」——顺序错了会把人引向改一个不存在的文件 |
| `TestSkillVersion_R4_Compare` | `internal/domain/model/skill_test.go` | domain | M2 U2.2.1 R4：版本号按段比较（`2.10 > 2.9`，字符串比法下是反的）；`v` 前缀可选 |
| `TestParseSkillVersion_RejectsMalformed` | `internal/domain/model/skill_test.go` | domain | 拒绝空 / 一段 / **三段**（那是应用版本的形态）/ 非数字 / **前导零**（`01` 与 `1` 会排成两个版本而界面上长得几乎一样）/ 负数 |
| `TestSkillVersion_StringRoundTrip` | `internal/domain/model/skill_test.go` | domain | 解析再转回字符串不变形 |
| `TestSkillStatus_ClosedEnum` | `internal/domain/model/skill_test.go` | domain | 三态封闭枚举（draft / active / deprecated）；`published` 与空串一律非法 |
| `TestAllSkillStatuses_ReturnsCopy` | `internal/domain/model/skill_test.go` | domain | 状态全集返回副本 |
| `TestScan_R1_ParsesFrontmatter` | `internal/fsstore/skill/scan_test.go` | fsstore | M2 U2.2.1 R1：真目录真文件扫出 name / version / description / compatibility（值里带 `>=` 和空格，不能在第一个冒号之后再切）；★ 扫出来的一律是 `draft`（INV-SKL-1）——扫盘就直接 active 的话，用户往目录里丢个文件就等于让它进了注入清单 |
| `TestScan_R2_MissingDescriptionExplained` | `internal/fsstore/skill/scan_test.go` | fsstore | M2 U2.2.1 R2：缺 description 的条目状态是 draft、原因点名 description，且名字仍认得出来（用户才知道去改哪一个） |
| `TestScan_R3_OneBrokenDoesNotHideOthers` | `internal/fsstore/skill/scan_test.go` | fsstore | M2 U2.2.1 R3：★★ 一条 frontmatter 坏的不让整个库列不出来——整批失败的话用户连修它的入口都找不到 |
| `TestScan_R5_DoesNotTouchUserFiles` | `internal/fsstore/skill/scan_test.go` | fsstore | M2 U2.2.1 R5（**红线 3** / INV-SKL-6）：★★ 判据是**全目录内容哈希 + 文件清单**，不是「没写 O_WRONLY」——前者才管得住「顺手补个默认 frontmatter」这种好意 |
| `TestScan_R4_DoesNotFollowSymlinks` | `internal/fsstore/skill/scan_test.go` | fsstore | 符号链接不跟出去。★ 两道防线各自独立有效（显式判 ModeSymlink + ReadDir 的 Lstat 语义），造负例分别验过 |
| `TestScan_R6_MissingDirIsEmptyNotError` | `internal/fsstore/skill/scan_test.go` | fsstore | M2 U2.2.1 R6：目录不存在 = 空列表不是错误。绝大多数项目没有 `.claude/skills`，当错误的话创建项目的预演会因一个正常状态而失败 |
| `TestScan_R6b_EmptyDirIsEmpty` | `internal/fsstore/skill/scan_test.go` | fsstore | 空目录返回空列表 |
| `TestScan_IgnoresLooseFiles` | `internal/fsstore/skill/scan_test.go` | fsstore | 散装文件不算 skill——skill 是目录不是文件 |
| `TestScan_HandlesCRLF` | `internal/fsstore/skill/scan_test.go` | fsstore | ★ Windows 换行的 SKILL.md 也读得出来。读不出的症状是「缺 name、description」，会把人引向一个根本没写错的文件 |
| `TestScan_SortedByDir` | `internal/fsstore/skill/scan_test.go` | fsstore | 结果按目录名排序，不受文件系统返回顺序影响 |
| `TestMemoryKind_R1_ClosedEnum` | `internal/domain/model/memory_test.go` | domain | M2 U2.3.1 R1：类型封闭枚举（constraint / experience / fact）；`note` 与空串一律非法 |
| `TestMemoryStatus_R2_FiveStatesFourTransitions` | `internal/domain/model/memory_test.go` | domain | M2 U2.3.1 R2（INV-MEM-4）：**五态**（设计稿筛选器那三档是界面分组，「已失效」同时装 invalid 与 obsolete）；迁移只有四条；★ **没有任何一条边指回 candidate**——有回边的话，一条被人确认过、注入了几十轮的记忆能重新变成待审 |
| `TestProposeCandidate_INVMEM2_OnlyCreatesCandidates` | `internal/domain/model/memory_test.go` | domain | ★★ INV-MEM-2 **绝不自动写入**：唯一的构造入口只造得出 candidate，刚建出来不可注入、没有 confirmedBy |
| `TestMemory_INVMEM2_ConfirmNeedsAnActor` | `internal/domain/model/memory_test.go` | domain | ★★ INV-MEM-2：`candidate → active` 必须带一个**人**的动作。空 actor / 全空白都拒；确认后记下是谁放行的。允许空 actor 的话一句 `Confirm("")` 就绕过整条规矩而代码读起来完全正常 |
| `TestProposeCandidate_INVMEM3_RequiresSourceRefs` | `internal/domain/model/memory_test.go` | domain | ★★ INV-MEM-3：没有 `source_refs` 就不能成为记忆——空着的话 AI 的一句臆断就能变成以后每一轮的前提 |
| `TestProposeCandidate_RejectsBadInput` | `internal/domain/model/memory_test.go` | domain | 空 id / 不认识的类型 / 空 scope 一律拒，错误信息点名是哪一项 |
| `TestMemory_R4_OnlyActiveIsInjectable` | `internal/domain/model/memory_test.go` | domain | M2 U2.3.1 R4（INV-MEM-5）：只有 active 进**新**注入清单；candidate 进了就等于自动写入，invalid 进了等于失效没生效 |
| `TestMemory_R4b_DiscardedAndObsoleteNotInjectable` | `internal/domain/model/memory_test.go` | domain | 被否决的、已废弃的都不可注入 |
| `TestMemory_INVMEM1_ScopeIsolation` | `internal/domain/model/memory_test.go` | domain | ★★ INV-MEM-1：P1 的记忆永不出现在 P2 的清单里，也不出现在跨项目列表里。串项目 = 把 A 的约束当成 B 的前提，而两个项目的约定常常正好相反 |
| `TestMemory_R3_CrossProjectVisibleEverywhere` | `internal/domain/model/memory_test.go` | domain | M2 U2.3.1 R3：L3 跨项目记忆对所有项目可见 |
| `TestMemory_INVMEM4_RejectsIllegalTransitions` | `internal/domain/model/memory_test.go` | domain | 非法迁移一律拒且状态不变；三个终态（discarded/invalid/obsolete）没有出边 |
| `TestMemory_INVMEM7_DeprecateNeedsReason` | `internal/domain/model/memory_test.go` | domain | INV-MEM-7：废弃必须给理由，`supersedes` 不能指向自身；被拒时状态不变 |
| `TestMemory_INVMEM6_HasNoDeleteMethod` | `internal/domain/model/memory_test.go` | domain | ★★ INV-MEM-6 反射断言**没有 Delete**（失效 ≠ 删除）。★ 对**指针类型**取方法集——值类型的方法集不含指针接收者的方法（PlanVersion 那条负例的教训） |
| `TestMemory_INVMEM8_DoesNotHoldContent` | `internal/domain/model/memory_test.go` | domain | ★★ INV-MEM-8 反射断言模型**不存正文**：正文只在 md 文件里。存两份的话迟早对不上，而「哪一份是真的」没有答案 |
| `TestMemory_INVMEM10_HistoryGrowsOnEveryChange` | `internal/domain/model/memory_test.go` | domain | INV-MEM-10：每次状态变更追加一条历史；★ **被拒的迁移不留历史**——它什么都没发生 |
| `TestMemory_SourceRefsReturnsCopy` | `internal/domain/model/memory_test.go` | domain | `SourceRefs()` 返回副本 |
| `TestAllMemoryStatuses_ReturnsCopy` | `internal/domain/model/memory_test.go` | domain | 状态全集返回副本 |
| `TestAllMemoryKinds_ReturnsCopy` | `internal/domain/model/memory_test.go` | domain | 类型全集返回副本 |
| `TestRunTurn_SendsSetConfigOptionOnTheWire` | `internal/acp/agent/role_test.go` | acp | M2 U2.1.2 **最后一公里**：★★ 判据是 Agent 那侧**收到的帧**，不是「我们调了那个函数」。接上之前 `applyMode` 六条测试全绿而没有任何调用方传 `RequiredModeID`——那段代码一次都没跑过 |
| `TestRunTurn_RestrictsBeforeAnyPrompt` | `internal/acp/agent/role_test.go` | acp | ★★ 收权帧在 prompt 帧**之前**。顺序反了的话，中间那个窗口里 codex 跑在 workspace-write 沙箱，写操作连审批都不触发 |
| `TestRunTurn_ModeFollowsTheRole` | `internal/acp/agent/role_test.go` | acp | ★★ 把「角色 → 语义档 → 那一端的档名」整条链路验穿：实现工程师发 `default`、审查员与需求分析师发 `plan`、留空按实现工程师。少了它的话「所有角色都发同一个档」测不出来 |
| `TestRunTurn_RefusesWhenAgentCannotRestrict` | `internal/acp/agent/role_test.go` | acp | Agent 不声明任何 mode 能力 → 拒绝开工，**线上一句 prompt 都没有** |
| `TestRunTurn_UnknownRoleIsRefusedNotDefaulted` | `internal/acp/agent/role_test.go` | acp | 认不出的角色报错且错误里点名是哪个，**不回落到默认角色**——回落的后果是「本该审查员做的事被实现工程师做了」，而这种错没有症状：审查照常「通过」 |
| `TestListRoles_ReturnsEightPresets` | `internal/api/roles_test.go` | api | M2 U2.4.1：八个角色都出来、四要素不空、顺序是设计稿行序。★ 用**真的** app 服务 + 真的 adapter，mock 掉任一头就测不出「domain 的角色 + adapter 的绑定拼得对不对」 |
| `TestListRoles_ExposesSemanticModeAndTranslatedName` | `internal/api/roles_test.go` | api | ★★ 同时给出**语义档**与**翻译好的档名**：实现工程师 `guarded_write`→`agent`（codex）、审查员 `read_only`→`plan`（claude）、测试执行者 `read_only`→`read-only`（codex）。直接返回档名的话前端就得认识品牌相关的取值 |
| `TestListRoles_UnconfiguredIsNotAnEmptyList` | `internal/api/roles_test.go` | api | ★★ 没装配回 503 而**不是 200 空列表**——八个预置角色是内置的，空表只会让用户以为应用坏了 |
| `TestListRoles_BrokenBindingStillListsTheRole` | `internal/api/roles_test.go` | api | ★ 绑定坏掉的角色照样列出来并带原因；跳过的话用户看到七个角色而不知少了哪个 |
| `TestListSkills_CarriesValidationReasonToTheUI` | `internal/api/skills_test.go` | api | M2 U2.4.1：真目录真文件 → 版本/描述/兼容性原样到界面；**校验没过的带原因**；扫出来一律 `draft`；来源必须标出 |
| `TestListSkills_EmptyIsArrayNotNull` | `internal/api/skills_test.go` | api | ★ 空集合序列化成 `[]` 不是 `null`——null 会让前端崩在 `.map` 上，而「一个都没有」正是新用户的常态 |
| `TestListSkills_ProjectScopeSaysNotReady` | `internal/api/skills_test.go` | api | ★ 项目级 Skill 还没做（要等创建项目）→ 回 501 **明说没有**，而不是回空列表让用户以为自己的 skill 没被认出来 |
| `TestListSkills_UnconfiguredIsNotAnEmptyList` | `internal/api/skills_test.go` | api | 没装配回 503 并给出可查的错误码 |
| `TestListSkills_ScanFailureIsReported` | `internal/api/skills_test.go` | api | ★ 扫不动要说出来，不装作「一个都没有」——装作没有的话用户以为自己的 skill 丢了 |
| `TestMemoryRepo_SaveAndFindRoundTrip` | `internal/store/memory_repo_test.go` | store | M2 U2.3.1：真 SQLite 存取往返，状态/类型/依据/确认人都不丢。`source_refs` 是溯源信息，丢了就查不到这条记忆凭什么成立 |
| `TestMemoryRepo_INVMEM8_TableHasNoContentColumn` | `internal/store/memory_repo_test.go` | store | ★★ INV-MEM-8：**问 SQLite 要表结构**（`pragma_table_info`），断言没有 content/body/text 一类列。★ 问数据库而不是看结构体——迁移脚本是人手写的 SQL，那才是真风险 |
| `TestMemoryRepo_INVMEM6_HasNoDeleteMethod` | `internal/store/memory_repo_test.go` | store | ★★ INV-MEM-6 反射断言仓储没有删除类方法。失效 ≠ 删除——删了的话半年前那次运行「当时用的是哪条记忆」永远查不到 |
| `TestMemoryRepo_INVMEM1_ScopeIsolationInQueries` | `internal/store/memory_repo_test.go` | store | ★★ INV-MEM-1：按 scope 查不串项目；跨项目（`*`）单独可查 |
| `TestMemoryRepo_ListByStatus` | `internal/store/memory_repo_test.go` | store | 按状态筛——记忆页那四档筛选靠它 |
| `TestMemoryRepo_StatusChangePersists` | `internal/store/memory_repo_test.go` | store | 状态变更真的落盘（GORM 的 `Updates` 传 struct 会静默丢零值，本项目一律用显式列名 upsert）；变更历史条数一并持久化 |
| `TestMemoryRepo_NotFoundIsDomainError` | `internal/store/memory_repo_test.go` | store | 查不到返回 `model.ErrNotFound`，**GORM 的错误不泄漏出 store 包** |
| `TestMemoryRepo_EmptyListIsNotAnError` | `internal/store/memory_repo_test.go` | store | 空库返回空切片而非 nil，且不是错误——一条记忆都没有是新用户的常态 |
| `TestMemoryRepo_EmptyRefsRoundTripAsEmpty` | `internal/store/memory_repo_test.go` | store | ★ 空 `source_refs` 拆成空切片而不是 `[""]`——后者会让「有没有依据」这个判断变成假的 |
| `TestReviewMemory_INVMEM2_RequiresAnActor` | `internal/api/memories_test.go` | api | ★★ INV-MEM-2：不带 actor 一律拒，且**状态没变**。放行的话一个后台任务就能把 AI 提的候选变成长期记忆——AGENTS.md §9 把这列为明令反例 |
| `TestReviewMemory_ConfirmRecordsWho` | `internal/api/memories_test.go` | api | 带 actor 才能确认，且记下是谁——半年后要能查「这条谁放行的」；确认后可注入 |
| `TestListMemories_CandidateIsNotInjectable` | `internal/api/memories_test.go` | api | ★ 候选态 `injectable=false`——标成可注入就等于自动写入了 |
| `TestListMemories_INVMEM1_ScopeIsolation` | `internal/api/memories_test.go` | api | ★★ 按 scope 查不串项目 |
| `TestReviewMemory_AlreadyActiveIsConflict` | `internal/api/memories_test.go` | api | ★ 对已生效的记忆再点确认 → **409 而不是 500**：那是用户操作的结果，不是我们坏了 |
| `TestReviewMemory_UnknownIdIs404` | `internal/api/memories_test.go` | api | 不存在的记忆返回 404 |
| `TestReviewMemory_InvalidDecisionIsRejected` | `internal/api/memories_test.go` | api | 非法 decision 回 400 且状态不变 |
| `TestListMemories_EmptyIsArrayNotNull` | `internal/api/memories_test.go` | api | 空集合序列化成 `[]` 不是 null |
| `TestListMemories_FailureIsReported` | `internal/api/memories_test.go` | api | ★ 查不动要说出来，不装作「一条都没有」——装作没有的话用户以为 Duet 把记忆忘光了 |
| `TestListMemories_UnconfiguredSaysSo` | `internal/api/memories_test.go` | api | 没装配回 503 |
| `TestMakePlan_R1_WritesNothing` | `internal/fsstore/project/init_test.go` | fsstore | M3 U3.1.1 R1：★★ 预演**一个字节都不写**，判据是**全目录指纹**（路径+内容+结构）而不是「我们没调 WriteFile」；且每一步都要写得出 `Reason`——不然用户凭什么点确认 |
| `TestApply_R2_DoesExactlyWhatThePlanSaid` | `internal/fsstore/project/init_test.go` | fsstore | M3 U3.1.1 R2：★★ 同一份 Plan 分别取「说要动的路径」与「执行后真的出现的路径」，两集合必须相等。分别写两套的话必然漂移，而漂移方向永远是「预演里没说的那件事被做了」 |
| `TestApply_R3_AppendsToGitignoreKeepingEveryLine` | `internal/fsstore/project/init_test.go` | fsstore | M3 U3.1.1 R3：`.gitignore` 是**追加**不是覆盖——覆盖等于删掉用户自己的规则，而他不会立刻发现 |
| `TestApply_R3b_IsIdempotent` | `internal/fsstore/project/init_test.go` | fsstore | 跑三次只追加一次，否则 `.gitignore` 会越长越长 |
| `TestApply_R3c_HandlesMissingTrailingNewline` | `internal/fsstore/project/init_test.go` | fsstore | ★ 用户的 `.gitignore` 末行常常没换行符，直接追加会和他的最后一条规则**粘成一行**——那条规则就此失效而 git 不报错 |
| `TestApply_R3d_CommentedRuleDoesNotCount` | `internal/fsstore/project/init_test.go` | fsstore | ★ 逐行精确比对而非 `strings.Contains`：`# .acpflows/runs/` 会让含糊匹配以为规则已生效，而它被注释着 |
| `TestMakePlan_R4_NonRepoIsReportedNotInitialized` | `internal/fsstore/project/init_test.go` | fsstore | M3 U3.1.1 R4：★★ 非 git 仓库如实报告，**且不擅自 `git init`**（在别人的目录里建仓库是不可逆的）；也不凭空造 `.gitignore`；但 `.acpflows` 照建 |
| `TestApply_R5_RollsBackOnFailure` | `internal/fsstore/project/init_test.go` | fsstore | M3 U3.1.1 R5：★★ 判据是「**我们自己建的东西**回到原样」。★ 发现真 bug：`MkdirAll` 对已存在目录成功返回，记成「这次创建的」再 `RemoveAll` 会**连同用户已有的内容一起删掉**——计划里的 `AlreadyThere` 挡不住，它是算计划那一刻的快照 |
| `TestApply_R5b_RollbackKeepsUserFiles` | `internal/fsstore/project/init_test.go` | fsstore | ★ 回滚**不删用户自己的文件**：一次失败的初始化不该顺手清掉他的 `.gitignore` |
| `TestMakePlan_MarksExistingItems` | `internal/fsstore/project/init_test.go` | fsstore | 已初始化过的目录，条目标成「已经在了」而不是消失——用户要看的是「最终长什么样」 |
| `TestMakePlan_RejectsBadRoots` | `internal/fsstore/project/init_test.go` | fsstore | 相对路径 / 不存在的目录 / 指向文件一律拒（相对路径的失败信息很难懂，在最外层就拦掉） |
| `TestMakePlan_GitFileCountsAsRepo` | `internal/fsstore/project/init_test.go` | fsstore | ★ worktree / submodule 里 `.git` 是**文件**不是目录，报成非仓库的话那些项目都拿不到忽略规则 |
| `TestDiscover_R1_FindsEverySkillsDirAndTagsSource` | `internal/fsstore/skill/discover_test.go` | fsstore | M3 U3.1.2 R1：★ 按设计稿扫 `**/skills`（不是只看两个固定目录）；每条标**项目内的相对路径**当来源——不标的话用户不知道 Duet 翻了他哪些目录，报绝对路径的话界面上一长串前缀全是噪声 |
| `TestDiscover_R2_ReusesTheSameValidator` | `internal/fsstore/skill/discover_test.go` | fsstore | M3 U3.1.2 R2：★★ 同一个坏 frontmatter，`Discover` 与 `Scan` 给出**同样的原因文本**。另起一套的话用户会看到两种说法而不知道该信哪个 |
| `TestDiscover_R3_DoesNotTouchUserFiles` | `internal/fsstore/skill/discover_test.go` | fsstore | R3：全目录指纹守只读（红线 3） |
| `TestDiscover_R4_DoesNotFollowSymlinksOutOfProject` | `internal/fsstore/skill/discover_test.go` | fsstore | R4：★ 项目里一个指向 `~` 的链接就能让扫描漫游整个家目录 |
| `TestDiscover_SkipsHeavyDirs` | `internal/fsstore/skill/discover_test.go` | fsstore | ★ 跳过 node_modules / target / dist / .git——不跳的话一个装了依赖的前端项目要走十几万个目录，而创建项目的弹层是**用户点了在等**的界面 |
| `TestDiscover_DoesNotDescendIntoSkillDirs` | `internal/fsstore/skill/discover_test.go` | fsstore | ★ 找到 `skills/` 就停：继续下探的话每个 skill 自己的 `scripts/`、`references/` 都会被当成候选 |
| `TestDiscover_R5_NoSkillsIsEmptyNotError` | `internal/fsstore/skill/discover_test.go` | fsstore | R5：没有 skill 目录是**空**不是错——绝大多数项目就是没有 |
| `TestDiscover_EmptyRootIsEmpty` | `internal/fsstore/skill/discover_test.go` | fsstore | 空路径返回空 |
| `TestDiscover_SortedBySourceThenDir` | `internal/fsstore/skill/discover_test.go` | fsstore | 结果按来源+目录名排序，不受文件系统顺序影响 |
| `TestParseRemoteURL_R1_BothFormsGiveTheSameSlug` | `internal/gitx/remote_test.go` | gitx | M3 U3.1.3 R1：https / scp / `ssh://` / 带端口 / 结尾斜杠 五种真实写法都得到同一个 `owner/repo`；URL 原文始终保留 |
| `TestParseRemoteURL_R4_NonGitHubIsKept` | `internal/gitx/remote_test.go` | gitx | R4：★ GitLab 与自建 remote **照常显示**——丢掉的话用 GitLab 的用户会看到「没有 remote」而他明明配了一个。★ 多级 group 取**最后两段**，否则 `group/sub` 会被当成 owner/repo |
| `TestParseRemoteURL_StripsCredentials` | `internal/gitx/remote_test.go` | gitx | ★★ `https://user:token@github.com/...` 这种写法很常见，而这个字段会显示在界面上、写进日志——凭据必须摘掉 |
| `TestParseRemoteURL_KeepsUrlWhenUnparseable` | `internal/gitx/remote_test.go` | gitx | 解析不出 owner/repo 时保留 URL 原文，不编造 slug |
| `TestProbeRemote_R2_NoRemoteIsEmptyNotError` | `internal/gitx/remote_test.go` | gitx | R2：没有 origin **不是错误**（本地仓库、还没推过的项目都很常见），当成错误的话创建项目的预演会因一个正常状态而失败 |
| `TestProbeRemote_ReadsConfiguredOrigin` | `internal/gitx/remote_test.go` | gitx | 真 git 仓库里配了 origin 就读得出来 |
| `TestProbeRemote_NonRepoIsEmpty` | `internal/gitx/remote_test.go` | gitx | 非 git 目录返回空、不报错 |
| `TestProbeRemote_R3_DoesNotModifyRepo` | `internal/gitx/remote_test.go` | gitx | R3：读 remote 不改仓库（判据是仓库全目录内容指纹，含 `.git` 里的配置） |
| `TestDetect_ReadyReadsVersionAndAccount` | `internal/ghx/detect_test.go` | ghx | M3 U3.1.3 R5（**Q41**）：装了且登录了 → 读出版本与账号，且**不给修复命令**（没什么要修的） |
| `TestDetect_NotAuthenticatedGivesLoginCommand` | `internal/ghx/detect_test.go` | ghx | 装了没登录 → `gh auth login`。★ `gh auth status` 没登录时以非 0 退出，**那是正常结论不是故障** |
| `TestDetect_UnknownAuthFailureIsProbeFailedNotUnauthenticated` | `internal/ghx/detect_test.go` | ghx | ★★ 认不出的失败当成 probe_failed：给一句「请运行 gh auth login」而实际问题是别的，会让用户照着做一遍然后发现没用 |
| `TestDetect_BrokenBinaryIsNotReportedAsMissing` | `internal/ghx/detect_test.go` | ghx | ★ 文件在但跑不起来（权限/架构/装坏了）≠ 没装——报成没装的话用户会去 brew install 一个已经在的东西 |
| `TestDetect_UnparseableAccountIsEmptyNotGuessed` | `internal/ghx/detect_test.go` | ghx | ★ 账号名取不到就留空：显示一个错的账号名比不显示糟得多，用户会以为自己登在另一个账号上 |
| `TestDetect_NeverLeaksToken` | `internal/ghx/detect_test.go` | ghx | ★★ **Q41 的核心**：Result 的每个字段都不含 `gho_`。`gh auth status` 输出里就有一行 Token，整段塞进 Detail/Account 的话令牌会进日志、进界面 |
| `TestDetect_UnparseableVersionStillWorks` | `internal/ghx/detect_test.go` | ghx | 版本号读不出不影响「装了且登录了」这个结论 |
| `TestDetect_NotInstalledGivesInstallCommand` | `internal/ghx/detect_test.go` | ghx | 没装 → `brew install gh`（本机装了 gh 时跳过） |
| `TestPreviewProject_ReturnsAllFourBlocks` | `internal/api/project_preview_test.go` | api | M3 U3.2.1：预演返回四块（将做什么 / 已有 Skill / remote / gh），**每一步都带 reason**——不说为什么的话用户凭什么点确认；skill 要标来源 |
| `TestPreviewProject_DoesNotInitialize` | `internal/api/project_preview_test.go` | api | ★★ 预演**不动手**：判据是初始化器一次都没被调用。先看后做是这一步的全部意义 |
| `TestAddProject_DoesNotInitializeByDefault` | `internal/api/project_preview_test.go` | api | ★★ 加项目**默认不初始化**——静默往用户的仓库里写东西是最快失去信任的方式 |
| `TestAddProject_InitializesWhenAsked` | `internal/api/project_preview_test.go` | api | 显式传 `initialize:true` 才照计划执行 |
| `TestAddProject_InitFailureIsReportedNotSilent` | `internal/api/project_preview_test.go` | api | ★ 初始化失败给出 `project_init_failed`，且**登记不回滚**：连登记一起撤的话用户点了「创建」却什么都没发生，而错误一闪而过 |
| `TestPreviewProject_RejectsBadInput` | `internal/api/project_preview_test.go` | api | 空路径 / 全空白 / 坏 JSON 一律 400 |
| `TestPreviewProject_UnconfiguredSaysSo` | `internal/api/project_preview_test.go` | api | 没装配回 503 |
| `TestPreviewProject_EmptyCollectionsAreArrays` | `internal/api/project_preview_test.go` | api | 空集合序列化成 `[]` 不是 null |
