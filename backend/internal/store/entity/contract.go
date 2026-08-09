package entity

import "time"

// Contract 是 contracts 表的行结构。
//
// ★★ 这张表**没有 UPDATE 路径**，除了冻结（INV-UC-2）。
type Contract struct {
	UnitID    string    `gorm:"column:unit_id;size:64;primaryKey"`
	Version   int       `gorm:"column:version;primaryKey"`
	Frozen    bool      `gorm:"column:frozen;not null;default:false"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null"`
}

// TableName 显式指定表名。
func (Contract) TableName() string { return "contracts" }

// ContractCriterion 是 contract_criteria 表的行结构。
type ContractCriterion struct {
	UnitID  string `gorm:"column:unit_id;size:64;primaryKey"`
	Version int    `gorm:"column:version;primaryKey"`
	ID      string `gorm:"column:id;size:64;primaryKey"`
	Text    string `gorm:"column:text;not null;default:''"`
	Ord     int    `gorm:"column:ord;not null;default:0"`
}

// TableName 显式指定表名。
func (ContractCriterion) TableName() string { return "contract_criteria" }

// ContractBoundary 是 contract_boundaries 表的行结构。
//
// ★ 一行一条前缀，**不拼在一列里**：前缀里出现分隔符不是不可能，
// 而拆错的后果是「允许改 internal/」变成两条谁都不认识的规则。
type ContractBoundary struct {
	UnitID  string `gorm:"column:unit_id;size:64;primaryKey"`
	Version int    `gorm:"column:version;primaryKey"`
	// Kind 取 allowed | forbidden。
	Kind   string `gorm:"column:kind;size:16;primaryKey"`
	Prefix string `gorm:"column:prefix;primaryKey"`
	Ord    int    `gorm:"column:ord;not null;default:0"`
}

// TableName 显式指定表名。
func (ContractBoundary) TableName() string { return "contract_boundaries" }
