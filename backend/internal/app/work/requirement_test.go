package work_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/internal/app/work"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
	"github.com/HuLuca1998/acp-flows/backend/tests/testutil"
)

// M5 U5.2.1 · 需求快照跟着对话走
//
// ★★ 这一族守的是完成标志第 3、4 条：
// 追问清楚后产出 `requirement v1` 标「已冻结」；改一次变 `v2`，旧版留着。

// memRequirements 是内存版需求仓储，**与真 store 同一套规则**：
// 已冻结的版本改不动，未冻结的草稿原地覆盖。
//
// ★ 规则不一致的替身会撒谎——`memWorks.FindWork` 丢掉 worktree 那次
// 就是这么让 `Say` 的测试红在一个生产里不存在的情况上。
type memRequirements struct {
	mu    sync.Mutex
	items map[string][]*model.RequirementSnapshot
}

func newMemRequirements() *memRequirements {
	return &memRequirements{items: map[string][]*model.RequirementSnapshot{}}
}

func (r *memRequirements) SaveRequirement(_ context.Context, req *model.RequirementSnapshot) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	list := r.items[req.WorkID()]
	for i, existing := range list {
		if existing.Version() != req.Version() {
			continue
		}
		// 已冻结：只有一模一样才放行（与真 store 同规则）
		if existing.Frozen() {
			if sameItems(existing, req) && req.Frozen() {
				return nil
			}
			return model.ErrRequirementFrozen
		}
		list[i] = copyOf(req)
		return nil
	}
	r.items[req.WorkID()] = append(list, copyOf(req))
	return nil
}

func (r *memRequirements) LatestRequirement(
	_ context.Context, workID string,
) (*model.RequirementSnapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	list := r.items[workID]
	if len(list) == 0 {
		return nil, model.ErrNotFound
	}
	best := list[0]
	for _, req := range list {
		if req.Version() > best.Version() {
			best = req
		}
	}
	return copyOf(best), nil
}

func (r *memRequirements) RequirementVersions(
	_ context.Context, workID string,
) ([]*model.RequirementSnapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]*model.RequirementSnapshot, 0, len(r.items[workID]))
	for _, req := range r.items[workID] {
		out = append(out, copyOf(req))
	}
	return out, nil
}

// copyOf 重建一个快照。
//
// ★★ **交出去的不能是库里那个指针。** 真 store 每次都经
// `mapper.RequirementToModel` 从行重建，直接交指针的话这个替身比真实现
// 「更共享」：`FreezeRequirement` 拿到它、调 `Freeze()`，
// 库里那份就已经变成冻结的了——于是存回去时撞上「已冻结不能改」，
// 而真实路径上根本不会发生。（`memWorks.FindWork` 丢掉 worktree 是同一个坑。）
func copyOf(r *model.RequirementSnapshot) *model.RequirementSnapshot {
	return model.RestoreRequirement(r.WorkID(), r.Version(), r.Items(), r.OpenFacts(), r.Frozen())
}

func sameItems(a, b *model.RequirementSnapshot) bool {
	return slices.Equal(a.Items(), b.Items()) && slices.Equal(a.OpenFacts(), b.OpenFacts())
}

var _ port.Requirements = (*memRequirements)(nil)

func newServiceWithRequirements(
	t *testing.T, runner port.AgentRunner,
) (*work.Service, *memRequirements) {
	t.Helper()
	reqs := newMemRequirements()
	svc := newServiceWithRunner(t, &memWorks{}, &recordingBus{}, runner)
	svc.SetRequirements(reqs)
	return svc, reqs
}

// ★★ 用户提的第一句话就是需求快照 v1。
//
// 不记的话，「我当时到底要它做什么」没有答案——而计划与契约都要照着它做。
func TestRequirement_FirstSentenceBecomesV1(t *testing.T) {
	project := testutil.NewGitRepo(t)
	svc, _ := newServiceWithRequirements(t, &fakeRunner{})
	ctx := context.Background()

	const said = "用户能取消正在运行的 turn，取消后现场证据要保留"
	view, err := svc.Start(ctx, project, said, "")
	if err != nil {
		t.Fatal(err)
	}

	got, err := svc.RequirementOf(ctx, view.ID)
	if err != nil {
		t.Fatalf("读需求: %v", err)
	}
	if got.Version != 1 {
		t.Errorf("版本 = v%d，想要 v1", got.Version)
	}
	if len(got.Items) != 1 || got.Items[0] != said {
		t.Errorf("条目 = %v，想要用户的原话", got.Items)
	}
	// ★ 新建出来**不是冻结的**：需求分析师要先追问清楚
	if got.Frozen {
		t.Error("v1 一出来就冻上了——那用户就没机会再看一眼了")
	}
}

// ★★ 追问过程中接着说，**改的是同一版**，不升版本号。
//
// 每问一个问题就升一版的话，版本链记的就不再是「需求变过几次」
// 而是「问过几个问题」。
func TestRequirement_FollowUpsStayInTheSameDraft(t *testing.T) {
	project := testutil.NewGitRepo(t)
	svc, reqs := newServiceWithRequirements(t, &fakeRunner{})
	ctx := context.Background()

	view, err := svc.Start(ctx, project, "用户能取消正在运行的 turn", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, more := range []string{"取消后现场证据要保留", "先别写代码"} {
		if err := svc.Say(ctx, view.ID, more); err != nil {
			t.Fatal(err)
		}
	}

	got, err := svc.RequirementOf(ctx, view.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != 1 {
		t.Errorf("版本 = v%d，追问不该升版本号", got.Version)
	}
	if len(got.Items) != 3 {
		t.Errorf("条目 = %v，想要三条（每句话一条）", got.Items)
	}
	all, err := reqs.RequirementVersions(ctx, view.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Errorf("库里有 %d 版，追问不该造新版本", len(all))
	}
}

// ★★ 完成标志第 4 条：冻结之后再提要求 → **v2，而 v1 原样留着**。
func TestRequirement_SayingMoreAfterFreezeMakesV2(t *testing.T) {
	project := testutil.NewGitRepo(t)
	svc, reqs := newServiceWithRequirements(t, &fakeRunner{})
	ctx := context.Background()

	view, err := svc.Start(ctx, project, "用户能取消正在运行的 turn", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.FreezeRequirement(ctx, view.ID); err != nil {
		t.Fatalf("冻结: %v", err)
	}

	if err := svc.Say(ctx, view.ID, "另外，取消后要能接着干"); err != nil {
		t.Fatal(err)
	}

	got, err := svc.RequirementOf(ctx, view.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != 2 {
		t.Fatalf("版本 = v%d，冻结之后再提要求就是改需求", got.Version)
	}
	if got.Frozen {
		t.Error("v2 一出来就冻上了——那用户就没机会再看一眼了")
	}

	// ★★ 判据：v1 **一个字都没变**，而且还冻着
	all, err := reqs.RequirementVersions(ctx, view.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("库里有 %d 版，想要 2 版——旧版被覆盖的话，"+
			"「上周那版说的是什么」永远没有答案", len(all))
	}
	for _, req := range all {
		if req.Version() != 1 {
			continue
		}
		if !req.Frozen() {
			t.Error("v1 的冻结状态被改了")
		}
		if len(req.Items()) != 1 {
			t.Errorf("v1 的条目变了：%v", req.Items())
		}
	}
}

// ★★ 冻结**由用户点**，且落盘。
//
// 静静成功的话，用户点了「冻结」界面显示已冻结，而库里什么都没有——
// 下次打开又变回未冻结。
func TestRequirement_FreezePersists(t *testing.T) {
	project := testutil.NewGitRepo(t)
	svc, _ := newServiceWithRequirements(t, &fakeRunner{})
	ctx := context.Background()

	view, err := svc.Start(ctx, project, "做点事", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.FreezeRequirement(ctx, view.ID); err != nil {
		t.Fatal(err)
	}

	got, err := svc.RequirementOf(ctx, view.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Frozen {
		t.Error("冻结没落盘")
	}
}

// 没装配需求存储时**明确报错**，不静静成功。
func TestRequirement_UnconfiguredSaysSo(t *testing.T) {
	project := testutil.NewGitRepo(t)
	svc := newServiceWithRunner(t, &memWorks{}, &recordingBus{}, &fakeRunner{})
	ctx := context.Background()

	view, err := svc.Start(ctx, project, "做点事", "")
	if err != nil {
		// ★ 没有需求存储**不该让建工作失败**——它只是少了一层记录
		t.Fatalf("没装配需求存储却建不了工作：%v", err)
	}
	if err := svc.FreezeRequirement(ctx, view.ID); !errors.Is(err, work.ErrRequirementsUnavailable) {
		t.Errorf("err = %v，想要 ErrRequirementsUnavailable", err)
	}
}

// 还没有需求时返回 ErrNotFound——那是新工作的常态，界面据此不显示标签。
func TestRequirement_NoneYetIsNotFound(t *testing.T) {
	svc, _ := newServiceWithRequirements(t, &fakeRunner{})

	_, err := svc.RequirementOf(context.Background(), "work-nope")
	if !errors.Is(err, model.ErrNotFound) {
		t.Errorf("err = %v，想要能判定成 ErrNotFound", err)
	}
}

// ★★ 需求版本**盖在这一轮的事件上**，与角色同理。
//
// 界面另查一次的话，拿到的是「现在」的版本，而用户看的是一条历史消息——
// 他会以为当时就已经是 v3 了。
func TestRequirement_StampedOnTheTurn(t *testing.T) {
	project := testutil.NewGitRepo(t)
	runner := &fakeRunner{}
	svc, _ := newServiceWithRequirements(t, runner)
	ctx := context.Background()

	view, err := svc.Start(ctx, project, "做点事", "")
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, "第一轮没跑起来", func() bool { return len(runner.snapshot()) == 1 })

	first := runner.snapshot()[0]
	if first.RequirementVersion != 1 {
		t.Errorf("第一轮盖的版本 = %d，想要 1", first.RequirementVersion)
	}
	if first.RequirementFrozen {
		t.Error("第一轮就说需求冻结了")
	}

	// 冻结之后再说一句 → 那一轮盖的是 v2 且未冻结
	if err := svc.FreezeRequirement(ctx, view.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.Say(ctx, view.ID, "再加一条"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "第二轮没跑起来", func() bool { return len(runner.snapshot()) == 2 })

	second := runner.snapshot()[1]
	if second.RequirementVersion != 2 {
		t.Errorf("第二轮盖的版本 = %d，想要 2", second.RequirementVersion)
	}
}
