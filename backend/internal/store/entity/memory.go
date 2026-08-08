package entity

import "time"

// Memory 是 memories 表的行结构。
//
// ★★ **没有正文字段**（INV-MEM-8）。正文只在 md 文件里——
// 两边各存一份的话它们迟早对不上，而到时候「哪一份是真的」没有答案。
// 有测试对着这个结构体守这条。
type Memory struct {
	ID     string `gorm:"column:id;primaryKey;size:64"`
	Kind   string `gorm:"column:kind;size:32;not null"`
	Scope  string `gorm:"column:scope;size:255;not null;index:idx_memories_scope_status"`
	Status string `gorm:"column:status;size:32;not null;index:idx_memories_scope_status"`
	// SourceRefs 用逗号连接存。
	//
	// ★ 不建关联表：`source_refs` 是**写一次就不再变**的溯源信息，
	// 没有任何按单个 ref 反查的需求。为它建一张表只会让「取一条记忆」
	// 从一次查询变成两次。
	SourceRefs  string    `gorm:"column:source_refs;size:2048;not null;default:''"`
	CreatedBy   string    `gorm:"column:created_by;size:64;not null;default:''"`
	ConfirmedBy string    `gorm:"column:confirmed_by;size:64;not null;default:''"`
	Reason      string    `gorm:"column:reason;size:1024;not null;default:''"`
	Supersedes  string    `gorm:"column:supersedes;size:64;not null;default:''"`
	HistoryLen  int       `gorm:"column:history_len;not null;default:1"`
	CreatedAt   time.Time `gorm:"column:created_at;not null"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null"`
}

// TableName 显式指定表名，不依赖 GORM 的自动推导。
func (Memory) TableName() string { return "memories" }
