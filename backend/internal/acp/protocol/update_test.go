package protocol_test

// protocol 线格式包 —— session/update 变体（原 U0.2.3，编号已废弃）
//
// 原单元编号 U0.2.3（已废弃）。能力归属见 docs/plan/roadmap.md 的「已经就绪的地基」。
//
// ★ 本文件的判别值全集来自 **官方 schema 源码**（A 级证据）：
//   @agentclientprotocol/sdk@1.3.0 的 dist/schema/types.gen.d.ts（PROTOCOL_VERSION = 1）
// 复核命令与裁定过程见 docs/notes/acp-field-notes.md §7.2。
//
// 为什么这些测试值得写：三份文档曾把变体数量写成 9 / 11 / 13 三个不同的数字。
// 「一共有几个」这种结论写在文档里必然漂移，只有穷举测试守得住。

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"slices"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/acp/protocol"
)

// goldenUpdates 是 v1 全部 sessionUpdate 变体的样本，字段名逐条照官方 schema 抄。
const goldenUpdates = "testdata/session_updates_v1.json"

// R1：判别值穷举 13 个，与官方 v1 schema 一字不差。
//
// 断言的是「全集」这个契约本身：漏登记一个，新变体上线时它会掉进未知分支
// 刷 warn 日志，把真正的未知变体淹掉（docs/spec/acp-integration.md §11.2）。
func TestSessionUpdateKind_R1_ExhaustiveMatchesSchemaV1(t *testing.T) {
	// 顺序照 types.gen.d.ts 里 SessionUpdate union 的声明顺序。
	want := []protocol.SessionUpdateKind{
		protocol.UpdateUserMessageChunk,
		protocol.UpdateAgentMessageChunk,
		protocol.UpdateAgentThoughtChunk,
		protocol.UpdateToolCall,
		protocol.UpdateToolCallUpdate,
		protocol.UpdatePlan,
		protocol.UpdatePlanUpdate,
		protocol.UpdatePlanRemoved,
		protocol.UpdateAvailableCommandsUpdate,
		protocol.UpdateCurrentModeUpdate,
		protocol.UpdateConfigOptionUpdate,
		protocol.UpdateSessionInfoUpdate,
		protocol.UpdateUsageUpdate,
	}

	got := protocol.AllSessionUpdateKinds()
	if !slices.Equal(want, got) {
		t.Fatalf("判别值全集与 v1 schema 不一致\n want=%v\n  got=%v", want, got)
	}
	if len(got) != 13 {
		t.Fatalf("v1 的 sessionUpdate 判别值应为 13 个，实际 %d 个", len(got))
	}

	// 字面量也要对：常量拼错的话上面的 slices.Equal 照样过（两边都是同一个错值）。
	wantLiterals := []string{
		"user_message_chunk", "agent_message_chunk", "agent_thought_chunk",
		"tool_call", "tool_call_update",
		"plan", "plan_update", "plan_removed",
		"available_commands_update", "current_mode_update",
		"config_option_update", "session_info_update", "usage_update",
	}
	for i, k := range got {
		if string(k) != wantLiterals[i] {
			t.Errorf("第 %d 个判别值：want %q, got %q", i, wantLiterals[i], k)
		}
		if !k.IsValid() {
			t.Errorf("%q 在全集里却 IsValid() == false", k)
		}
	}
}

// R2：未知判别值不报错，且判别值本身能被取回。
//
// ★ 这条最容易被写成假测试：只断言「没报错」是恒真式。
// 必须同时断言「取回了原判别值」——否则实现里 return nil 就能骗过去，
// 而那样上层根本不知道自己漏了什么变体。
func TestSessionUpdate_R2_UnknownKindIsPreservedNotAnError(t *testing.T) {
	// 一个 v2 才有的判别值：SDK 1.3.0 的 dist/v2/schema 里真实存在，
	// 拿它当「未来的未知变体」比编一个 "nope" 更接近真实升级场景。
	raw := []byte(`{"sessionUpdate":"terminal_output_chunk","terminalId":"term_01","chunk":"go: downloading\n"}`)

	var u protocol.SessionUpdate
	if err := json.Unmarshal(raw, &u); err != nil {
		t.Fatalf("未知判别值必须能被解析（协议可扩展，新增不算破坏性变更），got err = %v", err)
	}
	if got := u.Kind(); got != protocol.SessionUpdateKind("terminal_output_chunk") {
		t.Errorf("判别值应原样保留，want %q, got %q", "terminal_output_chunk", got)
	}
	if u.Kind().IsValid() {
		t.Error("v1 全集里没有 terminal_output_chunk，IsValid() 必须为 false")
	}

	// 载荷必须无损保留：上层要能把它原样记进日志排查，而不是只知道「有个东西没认出来」。
	var back map[string]any
	if err := json.Unmarshal(u.Payload(), &back); err != nil {
		t.Fatalf("未知变体的载荷应可读，got err = %v", err)
	}
	if back["chunk"] != "go: downloading\n" {
		t.Errorf("未知变体的载荷字段丢失：%+v", back)
	}
}

// R5：plan_update / plan_removed 被认得，但标注为 unstable。
//
// 官方 schema 给这两个挂了 UNSTABLE / @experimental —— 不在正式 spec 内，随时可能改。
// 「认得」和「可依赖」是两件事：认得是为了不刷 warn，标注是为了不让上层建立映射。
func TestSessionUpdateKind_R5_ExperimentalVariantsAreFlagged(t *testing.T) {
	experimental := []protocol.SessionUpdateKind{
		protocol.UpdatePlanUpdate,
		protocol.UpdatePlanRemoved,
	}

	for _, k := range experimental {
		if !k.IsValid() {
			t.Errorf("%q 属于 v1 全集，IsValid() 必须为 true", k)
		}
		if !k.IsExperimental() {
			t.Errorf("%q 被官方标了 UNSTABLE，IsExperimental() 必须为 true", k)
		}
	}

	// 其余 11 个都不是实验特性 —— 标错了会让稳定变体也被上层跳过。
	for _, k := range protocol.AllSessionUpdateKinds() {
		if slices.Contains(experimental, k) {
			continue
		}
		if k.IsExperimental() {
			t.Errorf("%q 不是实验特性，IsExperimental() 必须为 false", k)
		}
	}

	// 未知判别值不是「实验特性」，是「不认识」。两者的处理路径不同，不能混。
	if protocol.SessionUpdateKind("terminal_update").IsExperimental() {
		t.Error("未知判别值不应被当成实验特性")
	}
}

// R4：golden 样本解析成结构体再序列化回去，一个字段都不能丢。
//
// ★ 为什么不能只做 struct round-trip：struct 里漏定义一个字段时，
// 解析会丢弃它、序列化也不会产出它，round-trip 照样相等——测试全绿而字段没了。
// 所以必须拿**原始 JSON 的键**做对照。这正是本仓库最怕的「测试制造虚假安全感」。
func TestSessionUpdate_R4_GoldenRoundTripLosesNothing(t *testing.T) {
	raw, err := os.ReadFile(goldenUpdates)
	if err != nil {
		t.Fatalf("读 golden 失败: %v", err)
	}

	var samples []json.RawMessage
	if err := json.Unmarshal(raw, &samples); err != nil {
		t.Fatalf("golden 不是合法 JSON 数组: %v", err)
	}

	seen := make(map[protocol.SessionUpdateKind]bool, len(samples))

	for _, sample := range samples {
		var u protocol.SessionUpdate
		if err := json.Unmarshal(sample, &u); err != nil {
			t.Errorf("解析样本失败: %v\n%s", err, sample)
			continue
		}
		kind := u.Kind()
		if !kind.IsValid() {
			t.Errorf("golden 里出现了不在全集里的判别值 %q", kind)
			continue
		}
		seen[kind] = true

		t.Run(string(kind), func(t *testing.T) {
			// 解成该变体的具体类型 —— 这一步才真正验证字段名对得上。
			typed := newVariant(t, kind)
			if err := json.Unmarshal(sample, typed); err != nil {
				t.Fatalf("解析成 %T 失败: %v", typed, err)
			}
			out, err := json.Marshal(typed)
			if err != nil {
				t.Fatalf("序列化 %T 失败: %v", typed, err)
			}

			var want, got any
			if err := json.Unmarshal(sample, &want); err != nil {
				t.Fatalf("规整原样本失败: %v", err)
			}
			if err := json.Unmarshal(out, &got); err != nil {
				t.Fatalf("规整输出失败: %v", err)
			}
			if !reflect.DeepEqual(want, got) {
				t.Errorf("round-trip 后与官方 shape 不一致\n want=%s\n  got=%s", sample, out)
			}
		})
	}

	// golden 必须覆盖全部 13 个变体：加了新判别值却没补样本时这里会红，
	// 逼着下一个人去核对官方 schema，而不是加个常量了事。
	for _, k := range protocol.AllSessionUpdateKinds() {
		if !seen[k] {
			t.Errorf("golden 缺少 %q 的样本（%s）", k, goldenUpdates)
		}
	}
}

// newVariant 按判别值返回对应变体的空实例指针。
//
// ★ 这张表是穷举的第二道防线：加了判别值而没加载荷类型时，default 分支会让测试红。
func newVariant(t *testing.T, k protocol.SessionUpdateKind) any {
	t.Helper()
	switch k {
	case protocol.UpdateUserMessageChunk, protocol.UpdateAgentMessageChunk,
		protocol.UpdateAgentThoughtChunk:
		// 三者字段完全相同，只有判别值不同 —— 共用一个载荷类型。
		return &protocol.ContentChunkUpdate{}
	case protocol.UpdateToolCall:
		return &protocol.ToolCallStartUpdate{}
	case protocol.UpdateToolCallUpdate:
		return &protocol.ToolCallDeltaUpdate{}
	case protocol.UpdatePlan:
		return &protocol.PlanSnapshotUpdate{}
	case protocol.UpdatePlanUpdate:
		return &protocol.PlanContentUpdate{}
	case protocol.UpdatePlanRemoved:
		return &protocol.PlanRemovedUpdate{}
	case protocol.UpdateAvailableCommandsUpdate:
		return &protocol.AvailableCommandsUpdate{}
	case protocol.UpdateCurrentModeUpdate:
		return &protocol.CurrentModeUpdate{}
	case protocol.UpdateConfigOptionUpdate:
		return &protocol.ConfigOptionUpdate{}
	case protocol.UpdateSessionInfoUpdate:
		return &protocol.SessionInfoUpdate{}
	case protocol.UpdateUsageUpdate:
		return &protocol.UsageUpdate{}
	default:
		t.Fatalf("判别值 %q 没有对应的载荷类型——加了常量却没加类型？", k)
		return nil
	}
}

// Fake Runtime 要能把一个变体载荷发出去，所以构造方向也是契约。
//
// 判别值从序列化结果读回，而不是让调用方另传一遍 ——
// 传两遍就会出现「结构体说自己是 tool_call、参数说是 plan」的不一致。
func TestNewSessionUpdate_TakesKindFromPayload(t *testing.T) {
	u, err := protocol.NewSessionUpdate(protocol.ToolCallStartUpdate{
		SessionUpdate: protocol.UpdateToolCall,
		ToolCall: protocol.ToolCall{
			ToolCallID: "call_001",
			Title:      "编辑 backend/internal/acp/fake/script.go",
			Kind:       protocol.ToolKindEdit,
			Status:     protocol.ToolCallStatusPending,
		},
	})
	if err != nil {
		t.Fatalf("构造失败: %v", err)
	}
	if u.Kind() != protocol.UpdateToolCall {
		t.Errorf("判别值: want %q, got %q", protocol.UpdateToolCall, u.Kind())
	}

	out, err := json.Marshal(u)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	const want = `{"sessionUpdate":"tool_call","toolCallId":"call_001",` +
		`"title":"编辑 backend/internal/acp/fake/script.go","kind":"edit","status":"pending"}`
	if string(out) != want {
		t.Errorf("want %s\n got %s", want, out)
	}
}

// 缺判别字段是**报文不合协议**，与「判别值不认识」是两回事，必须区分。
//
// 混为一谈的话，一条畸形报文会被当成正常的协议演进静静丢掉。
func TestNewSessionUpdate_MissingDiscriminatorIsAnError(t *testing.T) {
	_, err := protocol.NewSessionUpdate(map[string]string{"toolCallId": "call_001"})
	if !errors.Is(err, protocol.ErrMissingDiscriminator) {
		t.Errorf("want ErrMissingDiscriminator, got %v", err)
	}
}

// 零值 SessionUpdate 不能被静静写成 "null" 发出去。
//
// Fake 的脚本里漏填一个 emit 时，我们要在测试里立刻看到错误，
// 而不是让对端收到一条 null 然后报一个语焉不详的解析失败。
func TestSessionUpdate_ZeroValueRefusesToMarshal(t *testing.T) {
	var u protocol.SessionUpdate
	if _, err := json.Marshal(u); err == nil {
		t.Fatal("未初始化的 SessionUpdate 必须拒绝序列化")
	}
}

// SessionNotification 是 session/update 通知的 params，会话 id 必须能穿透。
//
// 前一个项目的 H-1（最严重的一条）就是 sessionId 在某一层丢了，
// 表现为「第 2 轮不记得第 1 轮」。这一层丢了的话，上层根本找不到是哪个 Work。
func TestSessionNotification_CarriesSessionIDAndUpdate(t *testing.T) {
	raw := []byte(`{"sessionId":"sess_fake_0001","update":{"sessionUpdate":"current_mode_update","currentModeId":"read-only"}}`)

	var n protocol.SessionNotification
	if err := json.Unmarshal(raw, &n); err != nil {
		t.Fatalf("解析 session/update 通知失败: %v", err)
	}
	if n.SessionID != "sess_fake_0001" {
		t.Errorf("sessionId 丢了: want %q, got %q", "sess_fake_0001", n.SessionID)
	}
	if n.Update.Kind() != protocol.UpdateCurrentModeUpdate {
		t.Errorf("判别值不对: want %q, got %q", protocol.UpdateCurrentModeUpdate, n.Update.Kind())
	}

	var mode protocol.CurrentModeUpdate
	if err := json.Unmarshal(n.Update.Payload(), &mode); err != nil {
		t.Fatalf("解析 current_mode_update 载荷失败: %v", err)
	}
	if mode.CurrentModeID != "read-only" {
		t.Errorf("currentModeId: want %q, got %q", "read-only", mode.CurrentModeID)
	}
}
