package store_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
	"github.com/HuLuca1998/acp-flows/backend/internal/store"
	"github.com/HuLuca1998/acp-flows/backend/tests/testutil"
)

// openTestStore 开一个真的 SQLite（临时文件，过隔离守卫）。
func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	paths := testutil.TempPaths(t)
	testutil.GuardPath(t, paths.DBPath())
	db, err := store.Open(paths.DBPath(), testutil.FixedClock(testutil.T0))
	if err != nil {
		t.Fatalf("打开数据库: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// tableColumns 直接问 SQLite 这张表有哪些列。
//
// ★ 问数据库而不是看 entity 结构体：结构体上加一个没有 gorm tag 的字段
// 不会建列，而**迁移脚本里加一列**才是真正的风险——那是人手写的 SQL。
func tableColumns(t *testing.T, db *store.Store, table string) []string {
	t.Helper()
	rows, err := db.DB().Raw("SELECT name FROM pragma_table_info(?)", table).Rows()
	if err != nil {
		t.Fatalf("查表结构: %v", err)
	}
	defer func() { _ = rows.Close() }()

	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("读列名: %v", err)
		}
		out = append(out, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("遍历列名: %v", err)
	}
	if len(out) == 0 {
		t.Fatalf("表 %s 没有任何列——迁移没跑？", table)
	}
	return out
}

// M2 U2.3.1 · 记忆落库
//
// ★ 用真的 SQLite（临时文件），不 mock ORM——「DB 里到底存了什么」
// 正是这一层唯一值得测的东西，而 sqlmock 测的是 SQL 字符串。

func candidateFor(t *testing.T, id string, scope model.MemoryScope) *model.Memory {
	t.Helper()
	m, err := model.ProposeCandidate(
		id, model.MemoryConstraint, scope,
		[]string{"ev-412", "unit-009"}, "memory_curator",
	)
	if err != nil {
		t.Fatalf("提候选: %v", err)
	}
	return m
}

// 存进去再取出来，状态与依据都不丢。
func TestMemoryRepo_SaveAndFindRoundTrip(t *testing.T) {
	db := openTestStore(t)
	repo := db.Memories()
	ctx := context.Background()

	m := candidateFor(t, "mem-203", "acp-engine")
	if err := m.Confirm("luca"); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveMemory(ctx, m); err != nil {
		t.Fatalf("保存: %v", err)
	}

	got, err := repo.FindMemory(ctx, "mem-203")
	if err != nil {
		t.Fatalf("取回: %v", err)
	}
	if got.Status() != model.MemoryActive {
		t.Errorf("状态 = %q", got.Status())
	}
	if got.Kind() != model.MemoryConstraint {
		t.Errorf("类型 = %q", got.Kind())
	}
	if !reflect.DeepEqual(got.SourceRefs(), []string{"ev-412", "unit-009"}) {
		t.Errorf("依据 = %v——source_refs 是溯源信息，丢了就查不到这条记忆凭什么成立", got.SourceRefs())
	}
	// ★ 谁确认的要存下来：半年后要能查「这条谁放行的」
	if got.ConfirmedBy() != "luca" {
		t.Errorf("confirmed_by = %q", got.ConfirmedBy())
	}
}

// ★★ INV-MEM-8：DB 里**没有正文那一列**。
//
// 两边各存一份的话它们迟早对不上，而到时候「哪一份是真的」没有答案。
func TestMemoryRepo_INVMEM8_TableHasNoContentColumn(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()
	if err := db.Memories().SaveMemory(ctx, candidateFor(t, "mem-1", "p")); err != nil {
		t.Fatal(err)
	}

	cols := tableColumns(t, db, "memories")
	forbidden := []string{"content", "body", "text", "markdown", "md"}
	for _, c := range cols {
		for _, bad := range forbidden {
			if strings.EqualFold(c, bad) {
				t.Errorf("memories 表有 %q 列——正文只存在于 md 文件里（INV-MEM-8）。"+
					"DB 也存一份的话，两边迟早对不上", c)
			}
		}
	}
}

// ★★ INV-MEM-6：仓储**没有删除方法**。
//
// 加一个毫不费力，而加完之后所有测试还是绿的——
// 直到有人想查「半年前那次运行用的是哪条记忆」。
func TestMemoryRepo_INVMEM6_HasNoDeleteMethod(t *testing.T) {
	rt := reflect.TypeOf(&store.MemoryRepo{})
	forbidden := []string{"delete", "remove", "purge", "drop", "erase", "truncate"}
	for i := range rt.NumMethod() {
		name := strings.ToLower(rt.Method(i).Name)
		for _, bad := range forbidden {
			if strings.Contains(name, bad) {
				t.Errorf("MemoryRepo 有方法 %q——失效不等于删除（INV-MEM-6）", rt.Method(i).Name)
			}
		}
	}
}

// ★★ INV-MEM-1：按 scope 筛，项目之间不串味。
func TestMemoryRepo_INVMEM1_ScopeIsolationInQueries(t *testing.T) {
	db := openTestStore(t)
	repo := db.Memories()
	ctx := context.Background()

	for id, scope := range map[string]model.MemoryScope{
		"mem-a1": "acp-engine",
		"mem-a2": "acp-engine",
		"mem-b1": "acp-sidecar",
		"mem-x1": model.CrossProjectScope,
	} {
		if err := repo.SaveMemory(ctx, candidateFor(t, id, scope)); err != nil {
			t.Fatal(err)
		}
	}

	got, err := repo.ListMemories(ctx, port.MemoryFilter{Scope: "acp-engine"})
	if err != nil {
		t.Fatalf("列出: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("acp-engine 下有 %d 条，想要 2 条", len(got))
	}
	for _, m := range got {
		if m.Scope() != "acp-engine" {
			t.Errorf("查 acp-engine 却拿到 %q 的记忆——两个项目的约定常常正好相反", m.Scope())
		}
	}

	// 跨项目单独查
	cross, err := repo.ListMemories(ctx, port.MemoryFilter{Scope: string(model.CrossProjectScope)})
	if err != nil {
		t.Fatal(err)
	}
	if len(cross) != 1 || cross[0].ID() != "mem-x1" {
		t.Errorf("跨项目记忆 = %d 条", len(cross))
	}
}

// 按状态筛：记忆页的「候选 / active / 已失效」三档靠它。
func TestMemoryRepo_ListByStatus(t *testing.T) {
	db := openTestStore(t)
	repo := db.Memories()
	ctx := context.Background()

	pending := candidateFor(t, "mem-pending", "p")
	if err := repo.SaveMemory(ctx, pending); err != nil {
		t.Fatal(err)
	}

	confirmed := candidateFor(t, "mem-active", "p")
	if err := confirmed.Confirm("luca"); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveMemory(ctx, confirmed); err != nil {
		t.Fatal(err)
	}

	got, err := repo.ListMemories(ctx, port.MemoryFilter{Status: string(model.MemoryCandidate)})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID() != "mem-pending" {
		t.Errorf("候选筛出 %d 条", len(got))
	}
}

// 状态变更要真的落盘。
//
// ★ GORM 的 `Updates` 传 struct 会静默丢零值——本项目一律用
// 显式列名的 upsert，这条测试守着它。
func TestMemoryRepo_StatusChangePersists(t *testing.T) {
	db := openTestStore(t)
	repo := db.Memories()
	ctx := context.Background()

	m := candidateFor(t, "mem-203", "p")
	if err := repo.SaveMemory(ctx, m); err != nil {
		t.Fatal(err)
	}

	if err := m.Confirm("luca"); err != nil {
		t.Fatal(err)
	}
	if err := m.MarkInvalid(); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveMemory(ctx, m); err != nil {
		t.Fatal(err)
	}

	got, err := repo.FindMemory(ctx, "mem-203")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status() != model.MemoryInvalid {
		t.Errorf("状态没落盘：%q", got.Status())
	}
	// ★ 变更历史条数只增不减（INV-MEM-10）
	if got.HistoryLen() != m.HistoryLen() {
		t.Errorf("历史条数 = %d，想要 %d", got.HistoryLen(), m.HistoryLen())
	}
}

// 查不到返回领域的 ErrNotFound，不泄漏 GORM 的错误。
func TestMemoryRepo_NotFoundIsDomainError(t *testing.T) {
	db := openTestStore(t)
	_, err := db.Memories().FindMemory(context.Background(), "mem-不存在")
	if err == nil {
		t.Fatal("查不到却没报错")
	}
	if !strings.Contains(err.Error(), model.ErrNotFound.Error()) {
		t.Errorf("错误 = %v，想要包含 model.ErrNotFound", err)
	}
	if strings.Contains(err.Error(), "gorm") {
		t.Errorf("GORM 的错误泄漏出去了：%v", err)
	}
}

// 空集合是空切片不是 nil，也不是错误。
func TestMemoryRepo_EmptyListIsNotAnError(t *testing.T) {
	db := openTestStore(t)
	got, err := db.Memories().ListMemories(context.Background(), port.MemoryFilter{})
	if err != nil {
		t.Fatalf("空库报了错：%v——一条记忆都没有是新用户的常态", err)
	}
	if got == nil {
		t.Error("返回了 nil 而不是空切片")
	}
	if len(got) != 0 {
		t.Errorf("空库返回 %d 条", len(got))
	}
}

// ★ 没有依据的记忆存不进来——但存量数据里的空 source_refs 要拆成空切片，
// 而不是 `[""]`。后者会让「有没有依据」这个判断变成假的。
func TestMemoryRepo_EmptyRefsRoundTripAsEmpty(t *testing.T) {
	db := openTestStore(t)
	repo := db.Memories()
	ctx := context.Background()

	m := model.RestoreMemory(
		"mem-legacy", model.MemoryFact, "p", model.MemoryActive,
		nil, "memory_curator", "luca", "", "", 3,
	)
	if err := repo.SaveMemory(ctx, m); err != nil {
		t.Fatal(err)
	}

	got, err := repo.FindMemory(ctx, "mem-legacy")
	if err != nil {
		t.Fatal(err)
	}
	if n := len(got.SourceRefs()); n != 0 {
		t.Errorf("空依据拆出了 %d 条：%v——一条没有依据的记忆看起来像是有一条空依据",
			n, got.SourceRefs())
	}
}
