package store_test

import (
	"context"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/constant"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
	"github.com/HuLuca1998/acp-flows/backend/internal/store"
	"github.com/HuLuca1998/acp-flows/backend/tests/testutil"
)

// U2.4.1 · SaveWork（验收点 V5 的 R6）
//
// port.WorkRepo 要的是 upsert 语义：新建与状态更新走同一个方法。
// 分成 Create/Update 两个的话，调用方每次都要先判断「这个工作存在吗」——
// 而那个判断在并发下必然出错。

// ★★ R6：**关掉再打开，工作还在**。
//
// 用真的关闭 Store 再用同一个文件重新打开，而不是「存进去再从同一个连接读」——
// 后者证明的是缓存，不是持久化。
func TestWorkRepo_SaveSurvivesRestart(t *testing.T) {
	paths := testutil.TempPaths(t)
	testutil.GuardPath(t, paths.DBPath())
	ctx := context.Background()

	first, err := store.Open(paths.DBPath(), testutil.FixedClock(testutil.T0))
	if err != nil {
		t.Fatalf("打开: %v", err)
	}
	w := model.NewWork("work-01")
	if err := first.Works().SaveWork(ctx, w); err != nil {
		t.Fatalf("保存: %v", err)
	}
	if err := w.Transition(constant.WorkStateClarifying); err != nil {
		t.Fatal(err)
	}
	if err := first.Works().SaveWork(ctx, w); err != nil {
		t.Fatalf("更新: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second, err := store.Open(paths.DBPath(), testutil.FixedClock(testutil.T0))
	if err != nil {
		t.Fatalf("重新打开: %v", err)
	}
	defer func() { _ = second.Close() }()

	got, err := second.Works().FindWork(ctx, "work-01")
	if err != nil {
		t.Fatalf("重启后查不到这个工作: %v", err)
	}
	// ★ 状态也要一起活下来——只存 ID 的话，重启后所有工作都退回初始状态，
	// 用户会看到一堆「正在初始化」的僵尸条目
	if got.State() != constant.WorkStateClarifying {
		t.Errorf("重启后 State = %q, 想要 clarifying", got.State())
	}
}

// SaveWork 是 upsert：同一个 ID 存两次是更新，不是插入第二条。
func TestWorkRepo_SaveIsUpsert(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	repo := s.Works()

	w := model.NewWork("work-01")
	if err := repo.SaveWork(ctx, w); err != nil {
		t.Fatal(err)
	}
	if err := w.Transition(constant.WorkStateClarifying); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveWork(ctx, w); err != nil {
		t.Fatal(err)
	}

	all, err := repo.ListWorks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("落了 %d 条，想要 1 条", len(all))
	}
	if all[0].State() != constant.WorkStateClarifying {
		t.Errorf("State = %q，更新没落库", all[0].State())
	}
}

// ★★ 序号要能**从库里回填**，否则重启后会覆盖已有的工作。
//
// `SaveWork` 是 upsert：一个已经有 work-01 的库重启后再发一次 work-01，
// 那条新工作会**直接覆盖掉旧的**，连 worktree 目录都是同一个
// （两个工作的 AI 在同一份代码上改）。
//
// ★ 这个坑在开发机上撞不到（数据库总是空的），只会在用户那儿炸：
// 他重开应用、新建一个工作，昨天那条就没了。
func TestWorkRepo_MaxWorkSeq(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()

	if n, err := db.Works().MaxWorkSeq(ctx); err != nil || n != 0 {
		t.Fatalf("空库的最大序号 = %d, err = %v，想要 0", n, err)
	}

	for _, id := range []string{"work-01", "work-07", "work-03"} {
		if err := db.Works().SaveWork(ctx, model.NewWork(id)); err != nil {
			t.Fatal(err)
		}
	}

	got, err := db.Works().MaxWorkSeq(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got != 7 {
		t.Errorf("最大序号 = %d，想要 7——回填成 3 的话下一个是 work-04，"+
			"而 work-07 会在几次之后被覆盖掉", got)
	}
}

// ★ 解析不出来的 ID **跳过**，不让 duetd 起不来。
//
// 宁可序号往后多跳几个，也不能因为一条手工改过的记录就打不开应用。
func TestWorkRepo_MaxWorkSeqSkipsWeirdIDs(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()

	for _, id := range []string{"work-02", "手工改的", "work-abc"} {
		if err := db.Works().SaveWork(ctx, model.NewWork(id)); err != nil {
			t.Fatal(err)
		}
	}

	got, err := db.Works().MaxWorkSeq(ctx)
	if err != nil {
		t.Fatalf("一条认不出的 ID 就让它报错了：%v——用户会打不开应用", err)
	}
	if got != 2 {
		t.Errorf("最大序号 = %d，想要 2", got)
	}
}
