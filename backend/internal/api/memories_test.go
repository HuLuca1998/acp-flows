package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/api"
	"github.com/HuLuca1998/acp-flows/backend/internal/app/memory"
	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// M2 U2.3.1 · 记忆端点
//
// ★★ 这一族里最值钱的是 INV-MEM-2「绝不自动写入」：
// `candidate → active` 只有 review 这一条路，且必须带 actor。
// 错了的后果不是「多一条记忆」，而是 AI 把自己的一次臆断
// 变成了以后每一轮的前提，而用户从没看过那句话。

// memStub 是一个**真的**内存仓储（不是 mock）：它有真的状态，
// 状态机走的是真的领域模型。
type memStub struct {
	items map[string]*model.Memory
	err   error
}

func newMemStub(t *testing.T, ms ...*model.Memory) *memStub {
	t.Helper()
	s := &memStub{items: map[string]*model.Memory{}}
	for _, m := range ms {
		s.items[m.ID()] = m
	}
	return s
}

func (s *memStub) ListMemories(_ context.Context, f port.MemoryFilter) ([]*model.Memory, error) {
	if s.err != nil {
		return nil, s.err
	}
	out := make([]*model.Memory, 0, len(s.items))
	for _, m := range s.items {
		if f.Scope != "" && string(m.Scope()) != f.Scope {
			continue
		}
		if f.Status != "" && string(m.Status()) != f.Status {
			continue
		}
		out = append(out, m)
	}
	return out, nil
}

func (s *memStub) FindMemory(_ context.Context, id string) (*model.Memory, error) {
	m, ok := s.items[id]
	if !ok {
		return nil, model.ErrNotFound
	}
	return m, nil
}

func (s *memStub) SaveMemory(_ context.Context, m *model.Memory) error {
	s.items[m.ID()] = m
	return nil
}

func newCandidate(t *testing.T, id string, scope model.MemoryScope) *model.Memory {
	t.Helper()
	m, err := model.ProposeCandidate(
		id, model.MemoryExperience, scope, []string{"ev-900"}, "memory_curator")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func callMemories(t *testing.T, cfg api.Config, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	cfg.Token = "t"
	h, err := api.NewRouter(cfg)
	if err != nil {
		t.Fatalf("建路由: %v", err)
	}
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	r.Header.Set("Authorization", "Bearer t")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

// ★★ 确认必须带 actor。
//
// 放行的话，一个后台任务就能把 AI 提的候选变成长期记忆——
// 而 AGENTS.md §9 把「自动把一次成功经验写成长期记忆」列为明令反例。
func TestReviewMemory_INVMEM2_RequiresAnActor(t *testing.T) {
	m := newCandidate(t, "mem-1", "p")
	cfg := api.Config{Memories: memory.New(newMemStub(t, m))}

	rec := callMemories(t, cfg, http.MethodPost, "/v1/memories/mem-1/review",
		`{"decision":"confirm"}`)
	if rec.Code == http.StatusOK {
		t.Fatalf("没带 actor 却确认成功了：%s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "memory_actor_required") {
		t.Errorf("没给出可查的错误码：%s", rec.Body.String())
	}
	// ★ 判据是**状态没变**，不只是「返回了错误」
	if m.Status() != model.MemoryCandidate {
		t.Errorf("被拒之后状态变成了 %q", m.Status())
	}
}

// 带了 actor 就能确认，且记下是谁。
func TestReviewMemory_ConfirmRecordsWho(t *testing.T) {
	m := newCandidate(t, "mem-1", "p")
	cfg := api.Config{Memories: memory.New(newMemStub(t, m))}

	rec := callMemories(t, cfg, http.MethodPost, "/v1/memories/mem-1/review",
		`{"decision":"confirm","actor":"luca"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("状态码 %d：%s", rec.Code, rec.Body.String())
	}

	var got memoryResp
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("解响应: %v", err)
	}
	if got.Status != "active" {
		t.Errorf("状态 = %q", got.Status)
	}
	if got.ConfirmedBy != "luca" {
		t.Errorf("confirmed_by = %q——半年后要能查「这条谁放行的」", got.ConfirmedBy)
	}
	if !got.Injectable {
		t.Error("确认之后仍不可注入")
	}
}

// ★ 候选态**不可注入**——列表里也要如实标出来。
func TestListMemories_CandidateIsNotInjectable(t *testing.T) {
	cfg := api.Config{Memories: memory.New(newMemStub(t, newCandidate(t, "mem-1", "p")))}

	rec := callMemories(t, cfg, http.MethodGet, "/v1/memories", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("状态码 %d：%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Memories []memoryResp `json:"memories"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Memories) != 1 {
		t.Fatalf("返回 %d 条", len(body.Memories))
	}
	if body.Memories[0].Injectable {
		t.Error("候选态标成了可注入——那就等于自动写入了")
	}
}

// ★★ INV-MEM-1：按 scope 筛，项目之间不串味。
func TestListMemories_INVMEM1_ScopeIsolation(t *testing.T) {
	cfg := api.Config{Memories: memory.New(newMemStub(t,
		newCandidate(t, "mem-a", "acp-engine"),
		newCandidate(t, "mem-b", "acp-sidecar"),
	))}

	rec := callMemories(t, cfg, http.MethodGet, "/v1/memories?scope=acp-engine", "")
	var body struct {
		Memories []memoryResp `json:"memories"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Memories) != 1 || body.Memories[0].ID != "mem-a" {
		t.Errorf("查 acp-engine 拿到 %d 条：%+v——两个项目的约定常常正好相反",
			len(body.Memories), body.Memories)
	}
}

// 对一条已经生效的记忆再点确认 → 409 而不是 500。
//
// ★ 那是用户操作的结果，不是我们坏了。回 500 会让他以为应用出故障。
func TestReviewMemory_AlreadyActiveIsConflict(t *testing.T) {
	m := newCandidate(t, "mem-1", "p")
	if err := m.Confirm("luca"); err != nil {
		t.Fatal(err)
	}
	cfg := api.Config{Memories: memory.New(newMemStub(t, m))}

	rec := callMemories(t, cfg, http.MethodPost, "/v1/memories/mem-1/review",
		`{"decision":"confirm","actor":"luca"}`)
	if rec.Code != http.StatusConflict {
		t.Errorf("状态码 %d，想要 409：%s", rec.Code, rec.Body.String())
	}
}

func TestReviewMemory_UnknownIdIs404(t *testing.T) {
	cfg := api.Config{Memories: memory.New(newMemStub(t))}
	rec := callMemories(t, cfg, http.MethodPost, "/v1/memories/mem-不存在/review",
		`{"decision":"confirm","actor":"luca"}`)
	if rec.Code != http.StatusNotFound {
		t.Errorf("状态码 %d，想要 404：%s", rec.Code, rec.Body.String())
	}
}

func TestReviewMemory_InvalidDecisionIsRejected(t *testing.T) {
	m := newCandidate(t, "mem-1", "p")
	cfg := api.Config{Memories: memory.New(newMemStub(t, m))}

	rec := callMemories(t, cfg, http.MethodPost, "/v1/memories/mem-1/review",
		`{"decision":"maybe","actor":"luca"}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("状态码 %d，想要 400", rec.Code)
	}
	if m.Status() != model.MemoryCandidate {
		t.Errorf("非法决定却改了状态：%q", m.Status())
	}
}

// ★ 空集合是 `[]` 不是 null。
func TestListMemories_EmptyIsArrayNotNull(t *testing.T) {
	cfg := api.Config{Memories: memory.New(newMemStub(t))}
	rec := callMemories(t, cfg, http.MethodGet, "/v1/memories", "")
	if strings.Contains(rec.Body.String(), `"memories":null`) {
		t.Errorf("空集合序列化成了 null：%s", rec.Body.String())
	}
}

// ★ 查不动要说出来，不装作「一条都没有」——
// 装作没有的话，用户以为 Duet 把记忆忘光了。
func TestListMemories_FailureIsReported(t *testing.T) {
	stub := newMemStub(t)
	stub.err = errBindingBroken
	cfg := api.Config{Memories: memory.New(stub)}

	rec := callMemories(t, cfg, http.MethodGet, "/v1/memories", "")
	if rec.Code == http.StatusOK {
		t.Fatalf("查询失败却回了 200：%s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "memory_list_failed") {
		t.Errorf("没给出可查的错误码：%s", rec.Body.String())
	}
}

func TestListMemories_UnconfiguredSaysSo(t *testing.T) {
	rec := callMemories(t, api.Config{}, http.MethodGet, "/v1/memories", "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("状态码 %d，想要 503", rec.Code)
	}
}

type memoryResp struct {
	ID          string   `json:"id"`
	Kind        string   `json:"kind"`
	Scope       string   `json:"scope"`
	Status      string   `json:"status"`
	SourceRefs  []string `json:"source_refs"`
	ConfirmedBy string   `json:"confirmed_by"`
	Injectable  bool     `json:"injectable"`
}
