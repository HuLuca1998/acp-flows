package entity

import "time"

// Plan 是 plans 表的行结构。
//
// ★★ 这张表**没有 UPDATE 路径**（INV-PLAN-4）：计划改了就出新版本。
type Plan struct {
	WorkID  string `gorm:"column:work_id;size:64;primaryKey"`
	Version int    `gorm:"column:version;primaryKey"`
	Title   string `gorm:"column:title;not null;default:''"`
	// Dispositions 用**换行**连接（`id=disposition`）——标识里出现逗号
	// 不是不可能，而拆错的后果是一条处置凭空消失且没有任何报错。
	Dispositions string    `gorm:"column:dispositions;not null;default:''"`
	CreatedAt    time.Time `gorm:"column:created_at;not null"`
}

// TableName 显式指定表名，不依赖 GORM 的自动推导。
func (Plan) TableName() string { return "plans" }

// Subplan 是 subplans 表的行结构。
type Subplan struct {
	WorkID  string `gorm:"column:work_id;size:64;primaryKey"`
	Version int    `gorm:"column:version;primaryKey"`
	ID      string `gorm:"column:id;size:64;primaryKey"`
	Title   string `gorm:"column:title;not null;default:''"`
	// Ord 保住显示顺序——按 id 排的话 subplan-10 会排在 subplan-02 前面。
	Ord int `gorm:"column:ord;not null;default:0"`
}

// TableName 显式指定表名。
func (Subplan) TableName() string { return "subplans" }

// Unit 是 units 表的行结构。
type Unit struct {
	WorkID    string `gorm:"column:work_id;size:64;primaryKey"`
	Version   int    `gorm:"column:version;primaryKey"`
	SubplanID string `gorm:"column:subplan_id;size:64;not null"`
	ID        string `gorm:"column:id;size:64;primaryKey"`
	Title     string `gorm:"column:title;not null;default:''"`
	// RoleID 是**必填**（裁定三）：不写的话到执行时才发现没人认领。
	RoleID string `gorm:"column:role_id;size:64;not null"`
	// DependsOn 用换行连接，理由同 Plan.Dispositions。
	DependsOn      string `gorm:"column:depends_on;not null;default:''"`
	ContractFrozen bool   `gorm:"column:contract_frozen;not null;default:false"`
	Accepted       bool   `gorm:"column:accepted;not null;default:false"`
	Ord            int    `gorm:"column:ord;not null;default:0"`
}

// TableName 显式指定表名。
func (Unit) TableName() string { return "units" }
