package store_test

// 数据层的地基验证。规格见 docs/rules/database.md。
//
// 这些测试先于实现写就。重点是 R1——foreign_keys pragma 是 SQLite 最著名的坑：
// 默认关闭，外键写了但不生效，删父行子行还在。不显式打开等于外键白写。

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/constant"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
	"github.com/HuLuca1998/acp-flows/backend/internal/store"
	"github.com/HuLuca1998/acp-flows/backend/tests/testutil"
	"gorm.io/gorm"
)

func newStore(t *testing.T) *store.Store {
	t.Helper()
	paths := testutil.TempPaths(t)
	testutil.GuardPath(t, paths.DBPath()) // 自证没碰用户真实数据（铁律 6）

	s, err := store.Open(paths.DBPath(), testutil.FixedClock(testutil.T0))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// R1 ★ foreign_keys pragma 必须是开的。
//
// 这条不测，外键约束会静默失效——然后所有「删了父行子行还在」的怪现象
// 都会被误判成业务逻辑 bug。
func TestOpen_R1_ForeignKeysPragmaIsOn(t *testing.T) {
	s := newStore(t)

	var on int
	if err := s.DB().Raw("PRAGMA foreign_keys").Scan(&on).Error; err != nil {
		t.Fatalf("query pragma: %v", err)
	}
	if on != 1 {
		t.Fatalf("foreign_keys = %d, want 1 —— 外键约束没生效，见 docs/rules/database.md §6", on)
	}
}

// R2 · WAL 模式：读写不互斥，桌面应用会并发跑多个 Work。
func TestOpen_R2_JournalModeIsWAL(t *testing.T) {
	s := newStore(t)

	var mode string
	if err := s.DB().Raw("PRAGMA journal_mode").Scan(&mode).Error; err != nil {
		t.Fatalf("query pragma: %v", err)
	}
	if mode != "wal" {
		t.Errorf("journal_mode = %q, want %q", mode, "wal")
	}
}

// R3 · 迁移在空库上跑通，且记录进 schema_migrations。
func TestMigrate_R3_OnEmptyDatabase(t *testing.T) {
	s := newStore(t)

	var count int64
	if err := s.DB().Table("schema_migrations").Count(&count).Error; err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if count == 0 {
		t.Fatal("schema_migrations 为空——迁移没跑或没记录")
	}

	// works 表必须存在且可写
	if !s.DB().Migrator().HasTable("works") {
		t.Error("works 表不存在")
	}
}

// R4 · 迁移幂等：重复打开同一个库不出错，也不重复记录。
func TestMigrate_R4_Idempotent(t *testing.T) {
	paths := testutil.TempPaths(t)
	clk := testutil.FixedClock(testutil.T0)

	s1, err := store.Open(paths.DBPath(), clk)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	var first int64
	s1.DB().Table("schema_migrations").Count(&first)
	_ = s1.Close()

	s2, err := store.Open(paths.DBPath(), clk)
	if err != nil {
		t.Fatalf("second open: %v —— 迁移不幂等", err)
	}
	t.Cleanup(func() { _ = s2.Close() })

	var second int64
	s2.DB().Table("schema_migrations").Count(&second)
	if first != second {
		t.Errorf("重复打开后迁移记录变了: %d → %d", first, second)
	}
}

// R5 · Work 往返：存进去再取出来，领域模型的状态不丢。
func TestWorkRepo_R5_SaveAndFindRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	repo := s.Works()

	want := model.NewWorkAt("work-01", constant.WorkStateExecuting)
	if err := repo.CreateWork(ctx, want); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.FindWork(ctx, "work-01")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.ID() != want.ID() {
		t.Errorf("ID = %q, want %q", got.ID(), want.ID())
	}
	if got.State() != want.State() {
		t.Errorf("State = %q, want %q", got.State(), want.State())
	}
}

// R6 ★ 记录不存在时必须返回 model.ErrNotFound，不是 nil。
//
// GORM 的 Find 在记录不存在时 err 是 nil —— 用错方法会让「查不到」
// 静默变成「查到了一个零值」。查单条一律 First。
func TestWorkRepo_R6_NotFoundIsAnError(t *testing.T) {
	ctx := context.Background()
	repo := newStore(t).Works()

	_, err := repo.FindWork(ctx, "work-does-not-exist")
	if err == nil {
		t.Fatal("查不存在的记录返回了 nil error")
	}
	if !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("错误类型不符: got %v, want model.ErrNotFound", err)
	}
	// GORM 的错误类型不许泄漏到包外
	if errors.Is(err, errGormRecordNotFound()) {
		t.Error("gorm.ErrRecordNotFound 泄漏出了 store 包")
	}
}

// R7 ★ 状态更新必须真的写进去。
//
// GORM 的 Updates 传 struct 时会忽略零值字段——状态机把状态改成
// 看起来像零值的值时会静默丢更新。本项目一律用 map 形式。
func TestWorkRepo_R7_UpdateStatePersists(t *testing.T) {
	ctx := context.Background()
	repo := newStore(t).Works()

	w := model.NewWorkAt("work-01", constant.WorkStateReady)
	if err := repo.CreateWork(ctx, w); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := w.Transition(constant.WorkStateExecuting); err != nil {
		t.Fatalf("transition: %v", err)
	}
	if err := repo.UpdateWork(ctx, w); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := repo.FindWork(ctx, "work-01")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.State() != constant.WorkStateExecuting {
		t.Errorf("更新没落盘: got %q, want %q", got.State(), constant.WorkStateExecuting)
	}
}

// R8 · List 返回空集合时是空切片，不是 nil，且不是错误。
func TestWorkRepo_R8_ListEmptyIsNotAnError(t *testing.T) {
	ctx := context.Background()
	repo := newStore(t).Works()

	got, err := repo.ListWorks(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if got == nil {
		t.Error("空集合返回了 nil，应该是空切片")
	}
	if len(got) != 0 {
		t.Errorf("空库返回了 %d 条", len(got))
	}
}

// R9 · 数据库文件真的落在临时目录里，没碰用户数据。
func TestOpen_R9_DatabaseFileIsInTempDir(t *testing.T) {
	paths := testutil.TempPaths(t)
	s, err := store.Open(paths.DBPath(), testutil.FixedClock(testutil.T0))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	if _, err := os.Stat(paths.DBPath()); err != nil {
		t.Fatalf("数据库文件不存在: %v", err)
	}
	// 用临时文件而不是 :memory:——:memory: 测不出 WAL 与并发行为
	if filepath.Ext(paths.DBPath()) != ".db" {
		t.Errorf("数据库路径不像文件: %s", paths.DBPath())
	}
}

// errGormRecordNotFound 返回 GORM 的哨兵错误。
//
// 测试专用：R6 要断言这个错误**不会**泄漏到 store 包外，所以这里必须能引用到它。
// 生产代码里 store 之外的任何包 import gorm 都会被 depguard 拦下。
func errGormRecordNotFound() error { return gorm.ErrRecordNotFound }
