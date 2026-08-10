package work_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"os"
	"path/filepath"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
	"github.com/HuLuca1998/acp-flows/backend/internal/gitx"
	"github.com/HuLuca1998/acp-flows/backend/tests/testutil"
)

// M8 U8.1.2 · 应用直接采集证据
//
// ★★ 让 AI 报告自己改了什么，等于让被考核的人填自己的考勤表——
// 它不需要撒谎，只需要「记错了」一次，用户就再也不知道该信哪一条。

// memEvidence 是内存版证据仓储。★ 返回副本，与真 store 同规则。
type memEvidence struct {
	mu    sync.Mutex
	items map[string][]model.Evidence
}

func newMemEvidence() *memEvidence {
	return &memEvidence{items: map[string][]model.Evidence{}}
}

func (m *memEvidence) SaveEvidence(_ context.Context, workID string, e model.Evidence) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[workID] = append(m.items[workID], e)
	return nil
}

func (m *memEvidence) EvidenceOf(
	_ context.Context, workID, unitID string,
) ([]model.Evidence, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]model.Evidence, 0, len(m.items[workID]))
	for _, e := range m.items[workID] {
		if e.UnitID() == unitID {
			out = append(out, e)
		}
	}
	return out, nil
}

var _ port.Evidence = (*memEvidence)(nil)

// ★★ R1 R4 · diff 证据来自 **git**，且标着 `collected_by=app`。
func TestCollectDiffEvidence_R1R4_ComesFromGitAndIsMarkedAsApp(t *testing.T) {
	project := testutil.NewGitRepo(t)
	svc := newServiceWithRunner(t, &memWorks{}, &recordingBus{}, &fakeRunner{}).
		WithStatusProbe(realStatus{})
	svc.SetEvidence(newMemEvidence())
	ctx := context.Background()

	view, err := svc.Start(ctx, project, "做点事", "")
	if err != nil {
		t.Fatal(err)
	}

	// 在工作区里造两个改动——**真的写文件**，这样 git 才看得到
	writeInWorktree(t, view.Worktree, "a.txt", "hello\nworld\n")
	writeInWorktree(t, view.Worktree, "b.txt", "x\n")

	ev, err := svc.CollectDiffEvidence(ctx, view.ID, "unit-013", []string{"ac-1"})
	if err != nil {
		t.Fatalf("采集: %v", err)
	}

	// ★★ 判据：它是**应用采集的**，不是 AI 转述的
	if !ev.Trustworthy() {
		t.Error("采出来的证据标成了 AI 转述——用户判断「该不该信」全靠这一点")
	}
	if ev.Kind() != model.EvidenceDiff {
		t.Errorf("类别 = %q", ev.Kind())
	}
	// ★ 摘要里要看得出**改了几个文件**——两个新文件都该在
	if !strings.Contains(ev.Summary(), "2 个文件") {
		t.Errorf("摘要 = %q，想要 2 个文件——它是从真 git 读出来的", ev.Summary())
	}
	if !strings.Contains(ev.Body(), "a.txt") || !strings.Contains(ev.Body(), "b.txt") {
		t.Errorf("正文里没有改动的文件：\n%s", ev.Body())
	}
	if len(ev.Criteria()) != 1 || ev.Criteria()[0] != "ac-1" {
		t.Errorf("对应的标准 = %v", ev.Criteria())
	}
}

// ★★ R2 · 采集**只读**，不动工作区。
func TestCollectDiffEvidence_R2_DoesNotTouchTheWorktree(t *testing.T) {
	project := testutil.NewGitRepo(t)
	svc := newServiceWithRunner(t, &memWorks{}, &recordingBus{}, &fakeRunner{}).
		WithStatusProbe(realStatus{})
	svc.SetEvidence(newMemEvidence())
	ctx := context.Background()

	view, err := svc.Start(ctx, project, "做点事", "")
	if err != nil {
		t.Fatal(err)
	}
	writeInWorktree(t, view.Worktree, "a.txt", "hello\n")

	before := testutil.SnapshotDir(t, view.Worktree)
	if _, err := svc.CollectDiffEvidence(ctx, view.ID, "unit-013", nil); err != nil {
		t.Fatal(err)
	}
	testutil.AssertUnchanged(t, view.Worktree, before)
}

// ★★ R3 · 没装配 git 探针时**说出来**，不留一条空证据。
//
// 空证据会让「验收证据 1 条」这个数变成假的——用户以为有东西可看，
// 点开是空的，而他不会再信这个数。
func TestCollectDiffEvidence_R3_NoProbeSaysSo(t *testing.T) {
	project := testutil.NewGitRepo(t)
	svc := newServiceWithRunner(t, &memWorks{}, &recordingBus{}, &fakeRunner{})
	svc.SetEvidence(newMemEvidence())
	ctx := context.Background()

	view, err := svc.Start(ctx, project, "做点事", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CollectDiffEvidence(ctx, view.ID, "unit-013", nil); err == nil {
		t.Fatal("没有 git 探针却采出了证据——那条证据里什么都没有")
	}
}

// ★★ 标准与证据对上：有证据的标 `✓ ev-x`，没证据的**留在表里且为空**。
//
// 把没证据当成通过的话，一个什么都没做的单元也能「全部通过」。
func TestAcceptanceOf_MarksCriteriaWithoutEvidence(t *testing.T) {
	project := testutil.NewGitRepo(t)
	svc := newServiceWithRunner(t, &memWorks{}, &recordingBus{}, &fakeRunner{})
	svc = svc.WithStatusProbe(realStatus{})
	contracts := newMemContracts()
	svc.SetContracts(contracts)
	svc.SetEvidence(newMemEvidence())
	ctx := context.Background()

	view, err := svc.Start(ctx, project, "做点事", "")
	if err != nil {
		t.Fatal(err)
	}

	c := model.NewUnitContract("unit-013", 1)
	for _, crit := range []model.Criterion{
		{ID: "ac-1", Text: "取消必须幂等"},
		{ID: "ac-2", Text: "现场证据可读"},
	} {
		if err := c.AddCriterion(crit.ID, crit.Text); err != nil {
			t.Fatal(err)
		}
	}
	if err := contracts.SaveContract(ctx, c); err != nil {
		t.Fatal(err)
	}

	writeInWorktree(t, view.Worktree, "a.txt", "hello\n")
	if _, err := svc.CollectDiffEvidence(ctx, view.ID, "unit-013", []string{"ac-1"}); err != nil {
		t.Fatal(err)
	}

	got, err := svc.AcceptanceOf(ctx, view.ID, "unit-013")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Criteria) != 2 {
		t.Fatalf("标准 = %d 条，想要 2 条", len(got.Criteria))
	}
	// ★ 顺序照**契约**，不照 map——用户是照着契约那张表一条条核对的
	if got.Criteria[0].ID != "ac-1" || got.Criteria[1].ID != "ac-2" {
		t.Errorf("顺序变了：%s %s", got.Criteria[0].ID, got.Criteria[1].ID)
	}
	if len(got.Criteria[0].EvidenceIDs) != 1 {
		t.Errorf("ac-1 的证据 = %v", got.Criteria[0].EvidenceIDs)
	}
	// ★★ 没证据的那条**留在表里且为空**，不是「通过」
	if len(got.Criteria[1].EvidenceIDs) != 0 {
		t.Errorf("ac-2 凭空有了证据：%v", got.Criteria[1].EvidenceIDs)
	}
	if len(got.Evidence) != 1 || !got.Evidence[0].Trustworthy {
		t.Errorf("证据列表 = %+v", got.Evidence)
	}
}

// writeInWorktree 在工作区里**真的写一个文件**——这样 git 才看得到。
//
// ★ 用假的文件系统的话，「证据来自 git」这条断言会永远绿，
// 而它正是这个单元的全部意义。
func writeInWorktree(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// realStatus 用真 gitx 探测——**不塞假的**。
//
// ★ 假实现返回什么都行，而这个单元测的正是「读出来的与仓库里的一致」。
type realStatus struct{}

func (realStatus) ProbeRepoStatus(ctx context.Context, path string) (port.RepoStatus, error) {
	st, err := gitx.ProbeStatus(ctx, path)
	if err != nil {
		return port.RepoStatus{}, err
	}
	return port.RepoStatus{
		CurrentBranch: st.CurrentBranch, Branches: st.Branches,
		HeadCommit: st.HeadCommit, TrackedDirty: st.TrackedDirty, Untracked: st.Untracked,
	}, nil
}

func (realStatus) ProbeWorktreeState(
	ctx context.Context, path, base string,
) (port.WorktreeState, error) {
	st, err := gitx.ProbeWorktree(ctx, path, base)
	if err != nil {
		return port.WorktreeState{}, err
	}
	out := port.WorktreeState{
		Branch: st.Branch, BaseCommit: st.BaseCommit, Ahead: st.Ahead,
		Changes: make([]port.FileChange, 0, len(st.Changes)),
	}
	for _, c := range st.Changes {
		out.Changes = append(out.Changes, port.FileChange{
			Path: c.Path, Added: c.Added, Removed: c.Removed,
		})
	}
	return out, nil
}
