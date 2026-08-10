package work_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/internal/app/work"
	"github.com/HuLuca1998/acp-flows/backend/internal/constant"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
	"github.com/HuLuca1998/acp-flows/backend/tests/testutil"
)

// M9 U9.2.1 · 提问与作答
//
// ★★ AI 自己选一个往下走的话，用户是在几十个文件之后才发现
// 「它怎么这么做了」——而那时改回来的代价已经比一开始问一句大得多。

// memDecisions 是内存版决策仓储，**与真 store 同规则**：答过的不能改。
type memDecisions struct {
	mu    sync.Mutex
	items map[string]*model.Decision
}

func newMemDecisions() *memDecisions {
	return &memDecisions{items: map[string]*model.Decision{}}
}

func (m *memDecisions) SaveDecision(_ context.Context, d *model.Decision) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.items[d.ID()]; ok && existing.Answered() {
		if existing.AnsweredWith() == d.AnsweredWith() {
			return nil
		}
		return model.ErrDecisionAnswered
	}
	m.items[d.ID()] = copyDecision(d)
	return nil
}

func (m *memDecisions) FindDecision(_ context.Context, id string) (*model.Decision, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.items[id]
	if !ok {
		return nil, model.ErrNotFound
	}
	// ★★ 交副本不交指针——真 store 每次从行重建。
	// 交指针的话 `Answer()` 会直接改到库里那份，然后存回去撞「已答过」。
	return copyDecision(d), nil
}

func (m *memDecisions) PendingDecisions(
	_ context.Context, workID string,
) ([]*model.Decision, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*model.Decision
	for _, d := range m.items {
		if d.WorkID() == workID && !d.Answered() {
			out = append(out, copyDecision(d))
		}
	}
	return out, nil
}

func copyDecision(d *model.Decision) *model.Decision {
	return model.RestoreDecision(d.ID(), d.WorkID(), d.UnitID(), d.Level(),
		d.Question(), d.Options(), d.Recommended(), d.AnsweredWith())
}

var _ port.Decisions = (*memDecisions)(nil)

func decisionSetup(t *testing.T) (*work.Service, *memDecisions, *recordingBus, string) {
	t.Helper()
	project := testutil.NewGitRepo(t)
	bus := &recordingBus{}
	repo := &memWorks{}
	svc := newServiceWithRunner(t, repo, bus, &fakeRunner{})
	decisions := newMemDecisions()
	svc.SetDecisions(decisions)

	view, err := svc.Start(context.Background(), project, "做点事", "")
	if err != nil {
		t.Fatal(err)
	}

	// ★ 把工作推到 **executing**：D2 决策发生在执行中，而状态机不许
	// `clarifying → waiting_user`——那条限制是对的，绕过它等于让测试跑在
	// 一条真实路径产生不了的状态上。
	w := model.NewWorkAt(view.ID, constant.WorkStateExecuting)
	w.SetProject(project)
	w.SetWorktree(view.Worktree, view.Branch, view.BaseCommit)
	if err := repo.SaveWork(context.Background(), w); err != nil {
		t.Fatal(err)
	}
	return svc, decisions, bus, view.ID
}

func askD2(t *testing.T, workID string) *model.Decision {
	t.Helper()
	d, err := model.NewDecision("dec-01", workID, "unit-013", model.DecisionD2,
		"取消之后要不要回滚已写入的文件？",
		[]model.DecisionOption{
			{ID: "a", Text: "不回滚，仅停止", Impact: "已写入的文件留着"},
			{ID: "b", Text: "回退到检查点", Impact: "这一轮的三个文件改动会没有"},
		}, "a")
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// ★★ D2 提问时工作进 `waiting_user`——**AI 停在这里等他**。
//
// 不停的话，它会带着一个自己选的答案往下做几十个文件。
func TestAskDecision_D2StopsAndWaits(t *testing.T) {
	svc, _, bus, workID := decisionSetup(t)
	ctx := context.Background()

	if err := svc.AskDecision(ctx, askD2(t, workID)); err != nil {
		t.Fatal(err)
	}

	var waiting bool
	for _, e := range bus.snapshot() {
		if e.Type == "state_change" && e.Payload["to"] == string(constant.WorkStateWaitingUser) {
			waiting = true
		}
	}
	if !waiting {
		t.Error("D2 提问却没停下来等——AI 会带着自己选的答案往下做几十个文件")
	}
}

// ★★ 事件载荷里**每个选项都带影响**，且推荐的**只是标记不是预选**。
//
// 预选中的话，用户会顺手点确定——而那正好绕过了「让他自己决定」这件事。
func TestAskDecision_OptionsCarryImpactAndRecommendation(t *testing.T) {
	svc, _, bus, workID := decisionSetup(t)
	ctx := context.Background()

	if err := svc.AskDecision(ctx, askD2(t, workID)); err != nil {
		t.Fatal(err)
	}

	for _, e := range bus.snapshot() {
		if e.Type != "decision" {
			continue
		}
		opts, _ := e.Payload["options"].([]map[string]any)
		if len(opts) != 2 {
			t.Fatalf("选项 = %d 个", len(opts))
		}
		for _, o := range opts {
			if impact, _ := o["impact"].(string); impact == "" {
				t.Errorf("选项 %v 没有影响说明——用户会在盲选", o["id"])
			}
		}
		if opts[0]["recommended"] != true || opts[1]["recommended"] != false {
			t.Errorf("推荐标记不对：%v", opts)
		}
		return
	}
	t.Fatal("没发 decision 事件——用户不会知道有件事在等他")
}

// ★★ D0/D1 **不停下来等**——全停的话用户会被一堆
// 「用哪个变量名」的问题烦死。
func TestAskDecision_D1DoesNotStop(t *testing.T) {
	svc, _, bus, workID := decisionSetup(t)
	ctx := context.Background()

	d, err := model.NewDecision("dec-02", workID, "", model.DecisionD1,
		"用哪个包名？",
		[]model.DecisionOption{
			{ID: "a", Text: "gitx", Impact: "与现有包名一致"},
			{ID: "b", Text: "vcs", Impact: "更通用但与现有不一致"},
		}, "a")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.AskDecision(ctx, d); err != nil {
		t.Fatal(err)
	}

	for _, e := range bus.snapshot() {
		if e.Type == "state_change" && e.Payload["to"] == string(constant.WorkStateWaitingUser) {
			t.Fatal("D1 也停下来等了——用户会被一堆「用哪个变量名」的问题烦死")
		}
	}
}

// ★★ 答完之后工作**回到执行态**——不回的话它一直停在那儿，
// 而用户以为自己已经放行了。
func TestAnswerDecision_ResumesTheWork(t *testing.T) {
	svc, _, bus, workID := decisionSetup(t)
	ctx := context.Background()
	if err := svc.AskDecision(ctx, askD2(t, workID)); err != nil {
		t.Fatal(err)
	}

	if err := svc.AnswerDecision(ctx, workID, "dec-01", "b"); err != nil {
		t.Fatalf("作答: %v", err)
	}

	var resumed bool
	for _, e := range bus.snapshot() {
		if e.Type == "state_change" && e.Payload["to"] == string(constant.WorkStateExecuting) {
			resumed = true
		}
	}
	if !resumed {
		t.Error("答完却还停在 waiting_user——用户以为自己已经放行了")
	}
}

// ★★ 还有别的没答完时**继续等**——一次答一条。
func TestAnswerDecision_KeepsWaitingWhileOthersPend(t *testing.T) {
	svc, decisions, _, workID := decisionSetup(t)
	ctx := context.Background()
	if err := svc.AskDecision(ctx, askD2(t, workID)); err != nil {
		t.Fatal(err)
	}
	second, err := model.NewDecision("dec-02", workID, "", model.DecisionD2, "另一件事",
		[]model.DecisionOption{
			{ID: "x", Text: "x", Impact: "会怎样"},
			{ID: "y", Text: "y", Impact: "会怎样"},
		}, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.AskDecision(ctx, second); err != nil {
		t.Fatal(err)
	}

	if err := svc.AnswerDecision(ctx, workID, "dec-01", "a"); err != nil {
		t.Fatal(err)
	}

	pending, err := decisions.PendingDecisions(ctx, workID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 {
		t.Errorf("待决策 = %d 条，想要 1 条——答了一条就全放行的话，"+
			"另一条会被静静跳过", len(pending))
	}
}

// ★★ 答过的**不能再答**。
func TestAnswerDecision_OnlyOnce(t *testing.T) {
	svc, _, _, workID := decisionSetup(t)
	ctx := context.Background()
	if err := svc.AskDecision(ctx, askD2(t, workID)); err != nil {
		t.Fatal(err)
	}
	if err := svc.AnswerDecision(ctx, workID, "dec-01", "a"); err != nil {
		t.Fatal(err)
	}

	err := svc.AnswerDecision(ctx, workID, "dec-01", "b")
	if !errors.Is(err, model.ErrDecisionAnswered) {
		t.Errorf("答了第二次：%v——「他当时选了什么」就没有答案了", err)
	}
}

// 没装配决策存储时明确报错，不是「AI 自己选一个往下走」。
func TestDecision_UnconfiguredSaysSo(t *testing.T) {
	svc := newServiceWithRunner(t, &memWorks{}, &recordingBus{}, &fakeRunner{})

	err := svc.AskDecision(context.Background(), askD2(t, "work-01"))
	if !errors.Is(err, work.ErrDecisionsUnavailable) {
		t.Errorf("err = %v，想要 ErrDecisionsUnavailable", err)
	}
}
