package entity

import "time"

// SkillHit 是一个 Skill 被注入过几次。
//
// ★ Ref 是 `<scope>:<dir>` 而不是 name：name 来自 frontmatter，
// 用户改一次名计数就断了，而目录才是那个 Skill 的身份。
type SkillHit struct {
	Ref       string    `gorm:"column:ref;primaryKey;size:512"`
	HitCount  int       `gorm:"column:hit_count;not null;default:0"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null"`
}

// TableName 固定表名。
func (SkillHit) TableName() string { return "skill_hits" }
