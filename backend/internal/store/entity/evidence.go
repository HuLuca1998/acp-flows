package entity

import "time"

// Evidence 是 evidence 表的行结构。
//
// ★★ 这张表**没有 UPDATE 路径**：证据被改写过就不再是证据了。
type Evidence struct {
	ID     string `gorm:"column:id;size:64;primaryKey"`
	WorkID string `gorm:"column:work_id;size:64;not null"`
	UnitID string `gorm:"column:unit_id;size:64;not null"`
	Kind   string `gorm:"column:kind;size:16;not null"`
	// Source 分 app / agent，**必填**——分不出来源的话，
	// 一条 AI 转述会和一份应用采集的 diff 长得一样。
	Source  string `gorm:"column:source;size:16;not null"`
	Summary string `gorm:"column:summary;not null;default:''"`
	// Body 是原始输出，原样存不截断。
	Body      string    `gorm:"column:body;not null;default:''"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
}

// TableName 显式指定表名。
func (Evidence) TableName() string { return "evidence" }

// EvidenceCriterion 是证据与验收标准的多对多关系行。
type EvidenceCriterion struct {
	EvidenceID  string `gorm:"column:evidence_id;size:64;primaryKey"`
	CriterionID string `gorm:"column:criterion_id;size:64;primaryKey"`
	Ord         int    `gorm:"column:ord;not null;default:0"`
}

// TableName 显式指定表名。
func (EvidenceCriterion) TableName() string { return "evidence_criteria" }
