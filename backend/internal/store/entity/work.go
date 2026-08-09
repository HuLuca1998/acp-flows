// Package entity 是 GORM 行结构，store 包私有。
//
// ★ 这些类型**不是**领域模型，两者严格分离：
//   - 领域模型在 internal/domain/model，零 gorm 标签、零 gorm import
//   - 实体在这里，纯数据、无方法（除 TableName）
//   - 映射在 internal/store/mapper
//
// 给领域模型挂 gorm 标签是本项目最容易犯的错，理由见 docs/rules/database.md §1。
package entity

import "time"

// Work 是 works 表的行结构。
type Work struct {
	ID        string    `gorm:"column:id;primaryKey;size:64"`
	ProjectID string    `gorm:"column:project_id;size:64;not null"`
	State     string    `gorm:"column:state;size:32;not null"`
	Branch    string    `gorm:"column:branch;size:255;not null"`
	Worktree  string    `gorm:"column:worktree;size:1024;not null"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null"`
	// BaseCommit 是这个工作的基线。
	//
	// ★★ 右栏的「领先几个 commit」与验收 diff 都拿它当起点。
	// 不记的话，「AI 到底干了什么」只能靠猜，而猜出来的答案
	// 会随着仓库变化而漂移。
	BaseCommit string `gorm:"column:base_commit;size:64;not null;default:''"`
	// CurrentUnitID 是现在在做哪个单元——边界判定要靠它找到契约。
	// ★ 允许为空：澄清、规划阶段都还没开始做任何单元。
	CurrentUnitID string `gorm:"column:current_unit_id;size:64;not null;default:''"`
}

// TableName 显式指定表名，不依赖 GORM 的自动推导——
// 推导规则变化或字段改名时，表结构会静默漂移。
func (Work) TableName() string { return "works" }
