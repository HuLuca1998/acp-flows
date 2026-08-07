package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
	"github.com/HuLuca1998/acp-flows/backend/internal/store"
	"github.com/HuLuca1998/acp-flows/backend/tests/testutil"
)

// U2.1.1 · 项目落库（验收点 V4 的 R1）
//
// 用真 SQLite 临时文件，不用 :memory: —— 后者测不出 WAL 与并发行为，
// 而「重启后项目还在不在」问的正是磁盘上的东西。

func newProject(t *testing.T, id, path string) *model.Project {
	t.Helper()
	p, err := model.NewProject(id, path)
	if err != nil {
		t.Fatalf("构造项目失败: %v", err)
	}
	return p
}

// ★ R1：**重启 duetd 后项目仍在**。
//
// 这条不能用「存进去再从同一个连接读出来」糊弄——那证明的是缓存，不是持久化。
// 这里真的关掉 Store 再用同一个文件重新打开。
func TestProjectRepo_SurvivesRestart(t *testing.T) {
	paths := testutil.TempPaths(t)
	testutil.GuardPath(t, paths.DBPath())
	ctx := context.Background()

	first, err := store.Open(paths.DBPath(), testutil.FixedClock(testutil.T0))
	if err != nil {
		t.Fatalf("打开数据库: %v", err)
	}
	p := newProject(t, "proj-01", "/Users/me/work/my-app")
	p.SetGitInfo(true, "main")
	if err := first.Projects().SaveProject(ctx, p); err != nil {
		t.Fatalf("保存: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("关闭: %v", err)
	}

	// 同一个文件，新的连接——这才是"重启"
	second, err := store.Open(paths.DBPath(), testutil.FixedClock(testutil.T0))
	if err != nil {
		t.Fatalf("重新打开: %v", err)
	}
	defer func() { _ = second.Close() }()

	got, err := second.Projects().ListProjects(ctx)
	if err != nil {
		t.Fatalf("列出: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("重启后有 %d 个项目，想要 1 个", len(got))
	}
	if got[0].Path() != "/Users/me/work/my-app" {
		t.Errorf("Path = %q", got[0].Path())
	}
	// git 信息也得一起活下来，否则界面重启后会把一个 git 仓库显示成普通目录
	if !got[0].IsGitRepo() || got[0].DefaultBranch() != "main" {
		t.Errorf("git 信息丢了：IsGitRepo=%v DefaultBranch=%q",
			got[0].IsGitRepo(), got[0].DefaultBranch())
	}
}

func TestProjectRepo_FindByPath(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	repo := s.Projects()

	if err := repo.SaveProject(ctx, newProject(t, "proj-01", "/a/b")); err != nil {
		t.Fatal(err)
	}

	got, err := repo.FindProjectByPath(ctx, "/a/b")
	if err != nil {
		t.Fatalf("查已存在的路径: %v", err)
	}
	if got.ID() != "proj-01" {
		t.Errorf("ID = %q", got.ID())
	}

	// ★ 查不到必须是 ErrNotFound，不能是「零值 + nil error」。
	// 后者会让上层把「没查到」当成「查到了一个空项目」，
	// 于是重复添加检查形同虚设。
	if _, err := repo.FindProjectByPath(ctx, "/nope"); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("查不存在的路径 err = %v, 想要 ErrNotFound", err)
	}
}

// ★ path 唯一约束是**最后一道防线**。
//
// 应用层已经先查后写，但两个请求同时进来时那个检查挡不住——
// 只有数据库的唯一索引拦得住。
func TestProjectRepo_PathIsUnique(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	repo := s.Projects()

	if err := repo.SaveProject(ctx, newProject(t, "proj-01", "/a/b")); err != nil {
		t.Fatal(err)
	}

	// 不同 ID、同一个路径
	err := repo.SaveProject(ctx, newProject(t, "proj-02", "/a/b"))
	if err == nil {
		got, _ := repo.ListProjects(ctx)
		t.Fatalf("同一路径落了两条（现在有 %d 条）——唯一索引没生效", len(got))
	}
}

// 同一个 ID 再存一次是更新，不是插入第二条（用户改了显示名）。
func TestProjectRepo_SaveIsUpsert(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	repo := s.Projects()

	p := newProject(t, "proj-01", "/a/b")
	if err := repo.SaveProject(ctx, p); err != nil {
		t.Fatal(err)
	}

	if err := p.Rename("我的应用"); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveProject(ctx, p); err != nil {
		t.Fatalf("再次保存: %v", err)
	}

	got, err := repo.ListProjects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("有 %d 条记录，想要 1 条", len(got))
	}
	if got[0].Name() != "我的应用" {
		t.Errorf("Name = %q，改名没落库", got[0].Name())
	}
}

func TestProjectRepo_Delete(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	repo := s.Projects()

	if err := repo.SaveProject(ctx, newProject(t, "proj-01", "/a/b")); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteProject(ctx, "proj-01"); err != nil {
		t.Fatalf("删除: %v", err)
	}

	got, err := repo.ListProjects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("删除后还剩 %d 条", len(got))
	}

	// 删一个不存在的要报 ErrNotFound，不能静默成功——
	// 静默的话界面会显示「已移除」，而用户下次打开发现它还在
	if err := repo.DeleteProject(ctx, "proj-99"); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("删不存在的项目 err = %v, 想要 ErrNotFound", err)
	}
}

// 列表按添加顺序，且**两次列出必须一致**。
// 顺序每次不一样的话，用户会觉得列表在自己乱跳。
//
// ★ ID 按添加顺序递增（IDGen 就是这么发的），所以先加的 ID 更小——
// 测试数据得照这个来。第一版写成「proj-03 先加」，那在生产里不可能发生，
// 测出来的失败因此是假的。
func TestProjectRepo_ListIsStable(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	repo := s.Projects()

	for _, spec := range [][2]string{{"proj-01", "/a"}, {"proj-02", "/b"}, {"proj-03", "/c"}} {
		if err := repo.SaveProject(ctx, newProject(t, spec[0], spec[1])); err != nil {
			t.Fatal(err)
		}
	}

	first, err := repo.ListProjects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	second, err := repo.ListProjects(ctx)
	if err != nil {
		t.Fatal(err)
	}

	for i := range first {
		if first[i].ID() != second[i].ID() {
			t.Fatalf("两次列出的顺序不同：%q vs %q", first[i].ID(), second[i].ID())
		}
	}
	if first[0].Path() != "/a" {
		t.Errorf("第一条是 %q，想要最先添加的 /a", first[0].Path())
	}
	// ★ 时间戳相同时（同一毫秒内连续添加，测试里的 FixedClock 正是如此）
	// 靠 id 兜底定序。没有这个兜底，顺序就由 SQLite 的扫描顺序决定，
	// 表一变大可能就变了——那时才发现就晚了。
	if first[2].Path() != "/c" {
		t.Errorf("最后一条是 %q，想要最后添加的 /c", first[2].Path())
	}
}

// ★★ 重启后 ID 不能从头发。
//
// IDGen 的序号在内存里，重启就归零——不回填的话，一个已经有 proj-01/02/03
// 的库重启后会再发一次 proj-01，第一次添加项目就撞主键。
// 这个坑在开发机上几乎撞不到（数据库总是空的），要到用户用了一阵子、
// 重启一次应用才炸，那时现场离原因很远。
//
// PrimeSeq 从来没被调用过，因为在这之前没有任何用例用 IDGen——本单元是第一个。
func TestProjectRepo_MaxSeqForPrime(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	repo := s.Projects()

	// 空库返回 0：新用户第一次打开，下一个应当是 proj-01
	if n, err := repo.MaxProjectSeq(ctx); err != nil || n != 0 {
		t.Fatalf("空库 MaxProjectSeq = %d, err = %v，想要 0", n, err)
	}

	for _, spec := range [][2]string{{"proj-01", "/a"}, {"proj-02", "/b"}, {"proj-03", "/c"}} {
		if err := repo.SaveProject(ctx, newProject(t, spec[0], spec[1])); err != nil {
			t.Fatal(err)
		}
	}

	n, err := repo.MaxProjectSeq(ctx)
	if err != nil {
		t.Fatalf("MaxProjectSeq: %v", err)
	}
	if n != 3 {
		t.Errorf("MaxProjectSeq = %d, 想要 3——回填不到位的话重启后会再发一次 proj-01", n)
	}
}

// 手工改过 ID、或将来 ID 格式变了时不能崩。
// 解析不出来的一律跳过：宁可序号往后多跳几个，也不能让 duetd 起不来。
func TestProjectRepo_MaxSeqIgnoresUnparsableIDs(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	repo := s.Projects()

	for _, spec := range [][2]string{{"proj-02", "/a"}, {"proj-xyz", "/b"}, {"weird", "/c"}} {
		if err := repo.SaveProject(ctx, newProject(t, spec[0], spec[1])); err != nil {
			t.Fatal(err)
		}
	}

	n, err := repo.MaxProjectSeq(ctx)
	if err != nil {
		t.Fatalf("遇到解析不了的 ID 就报错了，duetd 会起不来: %v", err)
	}
	if n != 2 {
		t.Errorf("MaxProjectSeq = %d, 想要 2", n)
	}
}
