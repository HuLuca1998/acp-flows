package work_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/internal/app/work"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
	"github.com/HuLuca1998/acp-flows/backend/tests/testutil"
)

// M10 U10.2.1 · 从对话里冒出记忆候选
//
// ★★ 底线：**记忆由用户收下，不由 AI 自己写进去。** AI 自己写的话，
// 一条它误解的「经验」会一直影响后面每一轮，而用户从没同意过。

// 一条真实的经验——这个仓库真踩过的坑。
const memoryReply = "我把 events 表的 role 列加上了。\n\n```duet-memory\n" + `{
  "kind": "constraint",
  "scope": "project",
  "title": "这个仓库的迁移必须手写 SQL",
  "text": "gorm AutoMigrate 会把 events 表的 role 列改成 NOT NULL，而旧数据里那列是空的——启动直接失败。",
  "source_refs": ["unit-013"]
}
` + "```\n"

// memMemories 是记忆索引的替身。
//
// ★ 与真实现同规则：返回值一律重建，不交内部指针
// （替身比真实现少做一步，这个项目踩过四次）。
type memMemories struct {
	mu   sync.Mutex
	list []*model.Memory
}

func (m *memMemories) SaveMemory(_ context.Context, mem *model.Memory) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.list = append(m.list, mem)
	return nil
}

func (m *memMemories) FindMemory(_ context.Context, id string) (*model.Memory, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, mem := range m.list {
		if mem.ID() == id {
			return mem, nil
		}
	}
	return nil, nil
}

func (m *memMemories) ListMemories(_ context.Context, f port.MemoryFilter) ([]*model.Memory, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*model.Memory, 0, len(m.list))
	for _, mem := range m.list {
		if f.Status != "" && string(mem.Status()) != f.Status {
			continue
		}
		if f.Scope != "" && string(mem.Scope()) != f.Scope {
			continue
		}
		out = append(out, mem)
	}
	return out, nil
}

// memBodies 是正文存储的替身。
type memBodies struct {
	mu     sync.Mutex
	titles map[string]string
	texts  map[string]string
}

func newBodies() *memBodies {
	return &memBodies{titles: map[string]string{}, texts: map[string]string{}}
}

func (b *memBodies) WriteBody(id, title, text string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.titles[id] = title
	b.texts[id] = text
	return nil
}

func (b *memBodies) TitleOf(id string) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.titles[id]
}

// memorySetup 起一个装好记忆存储的工作。
//
// ★ 走的是**真实路径**：Start 的第一轮（开场白）本身就会调 OnReply，
// 于是记忆提取自然被触发——不是测试直接去调解析函数。
func memorySetup(
	t *testing.T, runner *fakeRunner, repo *memMemories, bodies *memBodies,
) (*work.Service, *recordingBus, string) {
	t.Helper()
	project := testutil.NewGitRepo(t)
	bus := &recordingBus{}
	svc := newServiceWithRunner(t, &memWorks{}, bus, runner)
	svc.SetRequirements(newMemRequirements())
	svc.SetMemories(repo, bodies)

	view, err := svc.Start(context.Background(), project, "让取消真的停下来", "")
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, "第一轮没跑起来", func() bool { return len(runner.snapshot()) == 1 })
	return svc, bus, view.ID
}

// candidateEvents 挑出候选事件的载荷。
func candidateEvents(bus *recordingBus) []map[string]any {
	var out []map[string]any
	for _, e := range bus.snapshot() {
		if e.Type == "memory_candidate" {
			out = append(out, e.Payload)
		}
	}
	return out
}

// R1 ★ 回复里的 duet-memory 围栏 → 一条 candidate。
//
// R2 ★★ **AI 写不出 active**：状态由应用给。判据落在**库里那条记录**上，
// 不是解析函数的返回值——这个项目已经八次栽在「测试构造了真实路径
// 产生不了的输入」。
func TestService_MemoryCandidate_IsAlwaysCandidateNeverActive(t *testing.T) {
	repo, bodies := &memMemories{}, newBodies()
	// AI 在回复里**明写 active**，看它能不能得逞
	sneaky := strings.Replace(memoryReply,
		`"kind": "constraint",`, `"kind": "constraint", "status": "active",`, 1)
	_, _, _ = memorySetup(t, &fakeRunner{reply: sneaky}, repo, bodies)

	waitFor(t, "回复里有 duet-memory 围栏，但一条候选都没建", func() bool {
		got, _ := repo.ListMemories(context.Background(), port.MemoryFilter{})
		return len(got) == 1
	})

	saved, err := repo.ListMemories(context.Background(), port.MemoryFilter{})
	require.NoError(t, err)
	assert.Equal(t, model.MemoryCandidate, saved[0].Status(),
		"AI 在载荷里写了 active 就真让它 active 了——那是这一整步的底线")
	assert.Equal(t, "这个仓库的迁移必须手写 SQL", bodies.TitleOf(saved[0].ID()),
		"★★ 正文没落盘：索引里有这条而 md 不在，用户点开看到「文件不存在」")
}

// R5 ★ 候选事件带着「审核」要用的 id。
func TestService_MemoryCandidate_EventCarriesID(t *testing.T) {
	repo, bodies := &memMemories{}, newBodies()
	_, bus, _ := memorySetup(t, &fakeRunner{reply: memoryReply}, repo, bodies)

	waitFor(t, "候选建出来了但时间线上什么都没有——用户不知道有东西等他审",
		func() bool { return len(candidateEvents(bus)) == 1 })

	ev := candidateEvents(bus)[0]
	assert.NotEmpty(t, ev["memory_id"], "事件里没有 id，用户在时间线上看到候选却点不动")
	assert.Equal(t, "这个仓库的迁移必须手写 SQL", ev["title"])
	assert.Equal(t, "candidate", ev["status"])
}

// R3 ★★ 解析不出来时**不静默**，把原因给用户看。
//
// 咽下去的话，AI 想记的那条经验消失了而没有任何人知道。
func TestService_MemoryCandidate_BrokenFenceIsReported(t *testing.T) {
	broken := "记一条：\n\n```duet-memory\n{ 这不是 JSON \n```\n"
	_, bus, _ := memorySetup(t, &fakeRunner{reply: broken}, &memMemories{}, newBodies())

	waitFor(t, "围栏写坏了却什么都没说——那条经验就这么无声无息地没了",
		func() bool { return len(candidateEvents(bus)) == 1 })
	assert.NotEmpty(t, candidateEvents(bus)[0]["error"])
}

// ★ 没写围栏是**常态**，不该产生任何噪音。
//
// 绝大多数轮次本来就不该产出记忆。每轮都发一条的话，时间线会被废话淹掉。
func TestService_MemoryCandidate_SilentWhenNoFence(t *testing.T) {
	runner := &fakeRunner{reply: "我看了一遍 store 目录，没发现问题。"}
	_, bus, _ := memorySetup(t, runner, &memMemories{}, newBodies())

	assert.Empty(t, candidateEvents(bus),
		"这一轮没经验可记，却发了一条事件——时间线会被这种废话淹掉")
}

// R4 ★ 同一条正文重复冒出来时**不重复建**。
//
// AI 在一个单元里跑好几轮，常会把上一轮那条经验再讲一遍。
func TestService_MemoryCandidate_NoDuplicateForSameTitle(t *testing.T) {
	repo, bodies := &memMemories{}, newBodies()
	runner := &fakeRunner{reply: memoryReply}
	svc, _, workID := memorySetup(t, runner, repo, bodies)

	waitFor(t, "第一条候选没建出来", func() bool {
		got, _ := repo.ListMemories(context.Background(), port.MemoryFilter{})
		return len(got) == 1
	})
	// ★ 同一条经验再讲一遍——走 Say，也就是用户接着说的那条真实路径
	if err := svc.Say(context.Background(), workID, "继续"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "第二轮没跑起来", func() bool { return len(runner.snapshot()) == 2 })

	got, err := repo.ListMemories(context.Background(), port.MemoryFilter{})
	require.NoError(t, err)
	assert.Len(t, got, 1, "同一条经验被记了两遍，用户的待审列表里会出现一模一样的两条")
}

// ★★ **接线检查**：真正发给 Agent 的提示词里必须带着「怎么提记忆」。
//
// 不带的话 AI 永远不会输出围栏，上面那一整套解析代码就是死的——
// 单测全绿，而真实路径上一条候选都不会出现。断的是 `AgentTurn.SystemPrompt`
// 这个**真的传出去了的值**，不是某个函数的返回值。
func TestAgentTurn_SystemPromptTellsAgentHowToProposeMemory(t *testing.T) {
	runner := &fakeRunner{}
	_, _, _ = memorySetup(t, runner, &memMemories{}, newBodies())

	prompt := runner.snapshot()[0].SystemPrompt
	require.NotEmpty(t, prompt)
	assert.Contains(t, prompt, "duet-memory",
		"提示词里没教它用哪个围栏，解析器就永远等不到输入")
	assert.Contains(t, prompt, "没有就不要输出",
		"不明说的话它每轮硬凑一条，用户的记忆库会被废话塞满")
}

// ── U10.3.1 · 注入清单与命中计数 ──────────────────────────────

// activeMemory 造一条**用户已经收下的**记忆。
func activeMemory(t *testing.T, id, scope string, bodies *memBodies) *model.Memory {
	t.Helper()
	m, err := model.ProposeCandidate(id, model.MemoryConstraint,
		model.MemoryScope(scope), []string{"unit-013"}, "agent")
	require.NoError(t, err)
	// ★ 走真实的迁移，不手搓状态：candidate → active 必须有用户确认动作
	require.NoError(t, m.Confirm("user"))
	require.NoError(t, bodies.WriteBody(id, "这个仓库的迁移必须手写 SQL", "正文"))
	return m
}

// hitCounter 记命中计数的替身。
type hitCounter struct {
	mu   sync.Mutex
	hits map[string]int
}

func newHits() *hitCounter { return &hitCounter{hits: map[string]int{}} }

func (h *hitCounter) BumpHits(_ context.Context, ids []string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, id := range ids {
		h.hits[id]++
	}
	return nil
}

func (h *hitCounter) countOf(id string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.hits[id]
}

// R1 ★★ 开场白里带着 active 记忆。
//
// 判据落在**真的发给 Agent 的那段 prompt** 上，不是某个函数的返回值。
func TestService_Injection_ActiveMemoryReachesThePrompt(t *testing.T) {
	repo, bodies := &memMemories{}, newBodies()
	runner := &fakeRunner{}
	project := testutil.NewGitRepo(t)
	bus := &recordingBus{}
	svc := newServiceWithRunner(t, &memWorks{}, bus, runner)
	svc.SetRequirements(newMemRequirements())
	svc.SetMemories(repo, bodies)
	require.NoError(t, repo.SaveMemory(context.Background(),
		activeMemory(t, "mem-01", project, bodies)))

	_, err := svc.Start(context.Background(), project, "让取消真的停下来", "")
	require.NoError(t, err)
	waitFor(t, "第一轮没跑起来", func() bool { return len(runner.snapshot()) == 1 })

	assert.Contains(t, runner.snapshot()[0].Prompt, "这个仓库的迁移必须手写 SQL",
		"★★ 记忆没进 prompt——它还是每次从零开始，那这一整步就白做了")
}

// R4 ★★ 失效的记忆**不注入**。
//
// 注入进去等于让 AI 照着一条用户否决过的前提干活，
// 而他很难想到问题出在一条老记忆上。
func TestService_Injection_SkipsNonActiveMemories(t *testing.T) {
	repo, bodies := &memMemories{}, newBodies()
	runner := &fakeRunner{}
	project := testutil.NewGitRepo(t)
	svc := newServiceWithRunner(t, &memWorks{}, &recordingBus{}, runner)
	svc.SetRequirements(newMemRequirements())
	svc.SetMemories(repo, bodies)

	// 一条还没被收下的候选
	cand, err := model.ProposeCandidate("mem-02", model.MemoryExperience,
		model.MemoryScope(project), []string{"unit-013"}, "agent")
	require.NoError(t, err)
	require.NoError(t, bodies.WriteBody("mem-02", "这条用户还没收下", "正文"))
	require.NoError(t, repo.SaveMemory(context.Background(), cand))

	_, err = svc.Start(context.Background(), project, "让取消真的停下来", "")
	require.NoError(t, err)
	waitFor(t, "第一轮没跑起来", func() bool { return len(runner.snapshot()) == 1 })

	assert.NotContains(t, runner.snapshot()[0].Prompt, "这条用户还没收下",
		"候选被当成生效的注入了——用户从没同意过这条")
}

// R3 ★ 命中计数每注入一次加一，且**由应用数**，不问 AI。
func TestService_Injection_CountsHitsPerTurn(t *testing.T) {
	repo, bodies, hits := &memMemories{}, newBodies(), newHits()
	runner := &fakeRunner{}
	project := testutil.NewGitRepo(t)
	svc := newServiceWithRunner(t, &memWorks{}, &recordingBus{}, runner)
	svc.SetRequirements(newMemRequirements())
	svc.SetMemories(repo, bodies)
	svc.SetMemoryHits(hits)
	require.NoError(t, repo.SaveMemory(context.Background(),
		activeMemory(t, "mem-01", project, bodies)))

	view, err := svc.Start(context.Background(), project, "让取消真的停下来", "")
	require.NoError(t, err)
	waitFor(t, "第一轮没跑起来", func() bool { return len(runner.snapshot()) == 1 })
	require.NoError(t, svc.Say(context.Background(), view.ID, "接着说"))
	waitFor(t, "第二轮没跑起来", func() bool { return len(runner.snapshot()) == 2 })

	assert.Equal(t, 2, hits.countOf("mem-01"), "跑了两轮，命中计数不是 2")
}

// R5 ★ 一条记忆都没有时**不发空清单**。
//
// 发一条「注入 0 条」的话，时间线上多出一行永远为空的噪音。
func TestService_Injection_NoEventWhenNothingToInject(t *testing.T) {
	runner := &fakeRunner{}
	_, bus, _ := memorySetup(t, runner, &memMemories{}, newBodies())

	for _, e := range bus.snapshot() {
		assert.NotEqual(t, "injection", e.Type,
			"没有可注入的记忆却发了清单——那一行什么也没告诉用户")
	}
}

// R2 ★★ 注入清单**由应用记**，不解析 AI 的自由文本。
//
// 它说「我参考了那条记忆」时可能根本没收到那条——
// 而用户正是靠这份清单判断「它是不是带着我的规矩在干活」。
func TestService_Injection_ListComesFromWhatWeActuallySent(t *testing.T) {
	repo, bodies := &memMemories{}, newBodies()
	// AI 在回复里胡说自己用了另一条
	runner := &fakeRunner{reply: "我参考了 mem-99 那条经验。"}
	project := testutil.NewGitRepo(t)
	bus := &recordingBus{}
	svc := newServiceWithRunner(t, &memWorks{}, bus, runner)
	svc.SetRequirements(newMemRequirements())
	svc.SetMemories(repo, bodies)
	require.NoError(t, repo.SaveMemory(context.Background(),
		activeMemory(t, "mem-01", project, bodies)))

	_, err := svc.Start(context.Background(), project, "让取消真的停下来", "")
	require.NoError(t, err)
	waitFor(t, "注入清单没发出来", func() bool {
		for _, e := range bus.snapshot() {
			if e.Type == "injection" {
				return true
			}
		}
		return false
	})

	for _, e := range bus.snapshot() {
		if e.Type != "injection" {
			continue
		}
		assert.Equal(t, []string{"mem-01"}, e.Payload["memory_ids"],
			"清单跟着 AI 的说法走了——它说什么就记什么，那这份清单没有任何价值")
	}
}
