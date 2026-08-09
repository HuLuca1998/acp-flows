package work_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/internal/app/work"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
	"github.com/HuLuca1998/acp-flows/backend/internal/gitx"
	"github.com/HuLuca1998/acp-flows/backend/tests/testutil"
)

// M8 U8.2.2 · 验收通过 → 提交 + 检查点
//
// ★★ **通过由用户点，不由 AI 判断**。这个方法只在他点了之后跑。

// realCommitter 用真 gitx——**不塞假的**。
//
// ★ 假实现返回什么都行，而这个单元测的正是「改动真的落进了那个分支」。
type realCommitter struct {
	mu    sync.Mutex
	calls []string
}

func (c *realCommitter) Commit(ctx context.Context, path, message string) (string, error) {
	c.mu.Lock()
	c.calls = append(c.calls, message)
	c.mu.Unlock()
	return gitx.Commit(ctx, path, message)
}

var _ port.Committer = (*realCommitter)(nil)

// acceptSetup 建一个工作、落契约与证据，返回可以验收的现场。
func acceptSetup(t *testing.T) (*work.Service, *recordingBus, *realCommitter, string, string) {
	t.Helper()
	project := testutil.NewGitRepo(t)
	bus := &recordingBus{}
	svc := newServiceWithRunner(t, &memWorks{}, bus, &fakeRunner{}).
		WithStatusProbe(realStatus{})
	contracts := newMemContracts()
	committer := &realCommitter{}
	svc.SetContracts(contracts)
	svc.SetEvidence(newMemEvidence())
	svc.SetCommitter(committer)
	ctx := context.Background()

	view, err := svc.Start(ctx, project, "做点事", "")
	if err != nil {
		t.Fatal(err)
	}

	c := model.NewUnitContract("unit-013", 1)
	if err := c.AddCriterion("ac-1", "取消必须幂等"); err != nil {
		t.Fatal(err)
	}
	if err := contracts.SaveContract(ctx, c); err != nil {
		t.Fatal(err)
	}
	return svc, bus, committer, view.ID, view.Worktree
}

// ★★ R2 R3 R4 · 提交在**工作分支**上、信息带单元与标准、落检查点绑 commit。
func TestAcceptUnit_CommitsAndCheckpoints(t *testing.T) {
	svc, bus, committer, workID, worktree := acceptSetup(t)
	ctx := context.Background()

	// 真的改点东西，再采一条证据
	if err := os.WriteFile(filepath.Join(worktree, "a.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CollectDiffEvidence(ctx, workID, "unit-013", []string{"ac-1"}); err != nil {
		t.Fatal(err)
	}

	sha, err := svc.AcceptUnit(ctx, workID, "unit-013")
	if err != nil {
		t.Fatalf("验收: %v", err)
	}
	if sha == "" {
		t.Error("没拿到 commit sha——检查点绑不上它，「恢复到哪」就没有答案")
	}

	// ★★ 判据：**改动真的落进了分支**（工作区干净了）
	dirty, err := gitx.IsDirty(ctx, worktree)
	if err != nil {
		t.Fatal(err)
	}
	if dirty {
		t.Error("验收之后工作区还是脏的——改动没落进提交")
	}

	// ★★ 提交信息里要看得出**是哪个单元、凭什么算通过**
	msg := committer.calls[0]
	for _, want := range []string{"unit-013", "ac-1", "取消必须幂等"} {
		if !strings.Contains(msg, want) {
			t.Errorf("提交信息里没有 %q——三个月后那条 commit 什么都说明不了：\n%s",
				want, msg)
		}
	}

	// ★ 检查点绑着那个 commit
	var found bool
	for _, e := range bus.snapshot() {
		if e.Type != "checkpoint" {
			continue
		}
		if e.Payload["commit"] == sha && e.Payload["unit"] == "unit-013" {
			found = true
		}
	}
	if !found {
		t.Error("检查点没绑上 commit——用户回头想接着干时，「恢复到哪」没有答案")
	}
}

// ★★ 一条标准都没有证据时**拒绝通过**。
//
// 允许的话，「验收」这个动作就没有内容了——用户点通过时以为自己核对过
// 什么，而实际上什么都没有。
func TestAcceptUnit_RefusesWithoutAnyEvidence(t *testing.T) {
	svc, _, committer, workID, worktree := acceptSetup(t)
	ctx := context.Background()
	if err := os.WriteFile(filepath.Join(worktree, "a.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := svc.AcceptUnit(ctx, workID, "unit-013")
	if !errors.Is(err, work.ErrNothingAccepted) {
		t.Fatalf("零证据却通过了：%v——用户点通过时以为自己核对过什么", err)
	}
	// ★ 判据：**一次提交都没发生**
	if len(committer.calls) != 0 {
		t.Errorf("被拒之后还是提交了 %d 次", len(committer.calls))
	}
}

// ★★ 没有改动时**不造空提交**（R5）。
//
// 一个「验收通过」却什么都没改的单元，说明该被质疑的是那次验收。
func TestAcceptUnit_RefusesToCommitNothing(t *testing.T) {
	svc, _, _, workID, worktree := acceptSetup(t)
	ctx := context.Background()

	// 先造改动、采证据，再把改动撤掉——模拟「证据是旧的，工作区已经干净了」
	if err := os.WriteFile(filepath.Join(worktree, "a.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CollectDiffEvidence(ctx, workID, "unit-013", []string{"ac-1"}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(worktree, "a.txt")); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.AcceptUnit(ctx, workID, "unit-013"); !errors.Is(err, gitx.ErrNothingToCommit) {
		t.Errorf("空提交却成功了：%v", err)
	}
}

// 没装配提交能力时**明确报错**，不是「通过了但什么都没提交」。
//
// 后者会让用户以为改动已经落进分支，而它其实还散在工作区里——
// 下一次 `git checkout` 就没了。
func TestAcceptUnit_NoCommitterSaysSo(t *testing.T) {
	svc := newServiceWithRunner(t, &memWorks{}, &recordingBus{}, &fakeRunner{})
	svc.SetContracts(newMemContracts())

	if _, err := svc.AcceptUnit(context.Background(), "work-01", "unit-013"); !errors.Is(err, work.ErrNoCommitter) {
		t.Errorf("err = %v，想要 ErrNoCommitter", err)
	}
}
