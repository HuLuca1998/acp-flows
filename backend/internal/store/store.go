// Package store 是 SQLite 持久化实现。
//
// 设计规范见 docs/rules/database.md，选型理由见 docs/adr/0005-persistence.md。
// 调试与改数据用 db-operate skill。
package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/HuLuca1998/acp-flows/backend/internal/store/migration"
)

// Clock 是 store 需要的时间源。
//
// 就地定义而不是 import app/port：接口定义在使用方，而且这样
// store 不必依赖 app 层。Go 是结构化类型，实现自动匹配。
type Clock interface{ Now() time.Time }

// Store 持有数据库连接与各个聚合的 repo。
type Store struct {
	db  *gorm.DB
	clk Clock
}

// Open 打开（必要时创建）数据库，执行迁移，返回可用的 Store。
//
// 连接串里的四个 pragma 缺一不可，尤其是 foreign_keys ——
// SQLite 默认关闭它，不显式打开等于外键白写。
func Open(path string, clk Clock) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	dsn := "file:" + path + "?" + strings.Join([]string{
		"_pragma=journal_mode(WAL)",   // 读写不互斥；桌面应用会并发跑多个 Work
		"_pragma=busy_timeout(5000)",  // 锁等待而不是立即 SQLITE_BUSY
		"_pragma=foreign_keys(ON)",    // ★ 默认是关的
		"_pragma=synchronous(NORMAL)", // WAL 下的推荐值
	}, "&")

	level := logger.Warn
	if os.Getenv("DUET_DB_TRACE") == "1" {
		level = logger.Info // 排查 N+1 时把全部 SQL 打出来
	}

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		PrepareStmt: true,
		Logger:      logger.Default.LogMode(level),
	})
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql.DB: %w", err)
	}
	// SQLite 同一时刻只允许一个写者。把并发写序列化在应用层，换取零 SQLITE_BUSY。
	// 改这个值前先跑并发测试。
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(0)

	if err := migration.Run(db, clk.Now); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &Store{db: db, clk: clk}, nil
}

// DB 暴露底层 *gorm.DB。
//
// 仅供本包内的 repo 与测试使用——它出现在 store 包外就是分层塌陷。
func (s *Store) DB() *gorm.DB { return s.db }

// Works 返回 Work 聚合的 repo。
func (s *Store) Works() *WorkRepo { return &WorkRepo{db: s.db, clk: s.clk} }

// Projects 返回项目仓储。
func (s *Store) Projects() *ProjectRepo { return &ProjectRepo{db: s.db, clk: s.clk} }

// Events 返回事件仓储。
func (s *Store) Events() *EventRepo { return &EventRepo{db: s.db} }

// Close 关闭数据库连接。
func (s *Store) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return fmt.Errorf("get sql.DB: %w", err)
	}
	return sqlDB.Close()
}
