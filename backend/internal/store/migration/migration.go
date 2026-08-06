// Package migration 执行版本化 SQL 迁移。
//
// 不用 GORM 的 AutoMigrate：它不删列、不改类型、不重命名，没有 down 路径，
// 行为随 GORM 版本变化。用户机器上的数据库是用户的数据，不能让一个隐式推导去改它。
// 理由见 docs/adr/0005-persistence.md。
package migration

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

//go:embed *.sql
var files embed.FS

// record 是 schema_migrations 表的一行。
type record struct {
	Version   int       `gorm:"column:version;primaryKey"`
	Name      string    `gorm:"column:name;not null"`
	Checksum  string    `gorm:"column:checksum;not null"`
	AppliedAt time.Time `gorm:"column:applied_at;not null"`
}

func (record) TableName() string { return "schema_migrations" }

// migration 是一个待执行的迁移文件。
type migration struct {
	version  int
	name     string
	sql      string
	checksum string
}

// Run 按版本顺序执行尚未应用的迁移。
//
// 全部在一个事务里：中途失败则整体回滚，不会留下半个 schema。
//
// 已应用迁移的 checksum 变了会直接报错退出——这不是可以静默继续的情况，
// 意味着有人改了已经跑过的迁移文件。要改结构就加新文件。
func Run(db *gorm.DB, now func() time.Time) error {
	all, err := load()
	if err != nil {
		return err
	}

	if err := db.AutoMigrate(&record{}); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	var applied []record
	if err := db.Order("version").Find(&applied).Error; err != nil {
		return fmt.Errorf("read schema_migrations: %w", err)
	}
	appliedBy := make(map[int]record, len(applied))
	for _, r := range applied {
		appliedBy[r.Version] = r
	}

	return db.Transaction(func(tx *gorm.DB) error {
		for _, m := range all {
			if prev, ok := appliedBy[m.version]; ok {
				if prev.Checksum != m.checksum {
					return fmt.Errorf(
						"迁移 %04d_%s 的内容与已应用的版本不一致（checksum %s → %s）；"+
							"已应用的迁移不许修改，请新增一个迁移文件",
						m.version, m.name, prev.Checksum[:8], m.checksum[:8])
				}
				continue // 已应用且未被改动
			}
			if err := tx.Exec(m.sql).Error; err != nil {
				return fmt.Errorf("apply migration %04d_%s: %w", m.version, m.name, err)
			}
			r := record{Version: m.version, Name: m.name, Checksum: m.checksum, AppliedAt: now()}
			if err := tx.Create(&r).Error; err != nil {
				return fmt.Errorf("record migration %04d_%s: %w", m.version, m.name, err)
			}
		}
		return nil
	})
}

// load 读取并解析全部嵌入的迁移文件，按版本升序返回。
func load() ([]migration, error) {
	entries, err := fs.Glob(files, "*.sql")
	if err != nil {
		return nil, fmt.Errorf("glob migrations: %w", err)
	}

	out := make([]migration, 0, len(entries))
	seen := map[int]string{}

	for _, name := range entries {
		version, label, err := parseName(name)
		if err != nil {
			return nil, err
		}
		if prev, dup := seen[version]; dup {
			return nil, fmt.Errorf("迁移版本号重复: %04d 同时被 %s 与 %s 使用；编号只增不复用", version, prev, name)
		}
		seen[version] = name

		body, err := files.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		sum := sha256.Sum256(body)
		out = append(out, migration{
			version:  version,
			name:     label,
			sql:      string(body),
			checksum: hex.EncodeToString(sum[:]),
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	return out, nil
}

// parseName 解析 NNNN_<动词>_<对象>.sql 形式的文件名。
func parseName(name string) (int, string, error) {
	base := strings.TrimSuffix(name, ".sql")
	idx := strings.Index(base, "_")
	if idx <= 0 {
		return 0, "", fmt.Errorf("迁移文件名不合规: %s（应为 NNNN_<动词>_<对象>.sql）", name)
	}
	version, err := strconv.Atoi(base[:idx])
	if err != nil {
		return 0, "", fmt.Errorf("迁移文件名的版本号不是数字: %s", name)
	}
	return version, base[idx+1:], nil
}
