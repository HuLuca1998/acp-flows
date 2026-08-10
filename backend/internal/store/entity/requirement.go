package entity

import "time"

// Requirement 是 requirements 表的行结构。
//
// ★★ 这张表**没有 UPDATE 路径**（INV-REQ-2）：改需求就插一条新版本。
// 复合主键 (work_id, version) 保证同一版只有一条。
type Requirement struct {
	WorkID  string `gorm:"column:work_id;primaryKey;size:64"`
	Version int    `gorm:"column:version;primaryKey"`
	// Items 与 OpenFacts 用**换行**连接。
	//
	// ★ 不用逗号：需求条目里出现逗号是很正常的事
	// （「取消必须幂等，且现场要保留」）。
	Items     string    `gorm:"column:items;not null;default:''"`
	OpenFacts string    `gorm:"column:open_facts;not null;default:''"`
	Frozen    bool      `gorm:"column:frozen;not null;default:false"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null"`
}

// TableName 显式指定表名，不依赖 GORM 的自动推导。
func (Requirement) TableName() string { return "requirements" }
