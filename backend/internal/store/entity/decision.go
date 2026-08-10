package entity

import "time"

// Decision 是 decisions 表的行结构。
//
// ★★ 答过的**一个字都不能改**：改了的话「他当时选了什么」就没有答案。
type Decision struct {
	ID       string `gorm:"column:id;size:64;primaryKey"`
	WorkID   string `gorm:"column:work_id;size:64;not null"`
	UnitID   string `gorm:"column:unit_id;size:64;not null;default:''"`
	Level    string `gorm:"column:level;size:8;not null"`
	Question string `gorm:"column:question;not null"`
	// Recommended 空表示 AI 也拿不准。
	Recommended string `gorm:"column:recommended;size:64;not null;default:''"`
	// AnsweredWith 空表示**还没答**——左栏那个亮点靠它。
	AnsweredWith string    `gorm:"column:answered_with;size:64;not null;default:''"`
	CreatedAt    time.Time `gorm:"column:created_at;not null"`
}

// TableName 显式指定表名。
func (Decision) TableName() string { return "decisions" }

// DecisionOption 是 decision_options 表的行结构。
type DecisionOption struct {
	DecisionID string `gorm:"column:decision_id;size:64;primaryKey"`
	ID         string `gorm:"column:id;size:64;primaryKey"`
	Text       string `gorm:"column:text;not null;default:''"`
	// Impact 是「选了它会怎样」。★ 没有它用户在盲选。
	Impact string `gorm:"column:impact;not null;default:''"`
	Ord    int    `gorm:"column:ord;not null;default:0"`
}

// TableName 显式指定表名。
func (DecisionOption) TableName() string { return "decision_options" }
