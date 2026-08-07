package entity

import "time"

// Project 是 projects 表的行结构。
//
// ★ 这里**没有**任何「Duet 在用户项目里放了什么」的字段——
// 因为一个字节都没放。worktree 在 `~/.acpflows/worktrees`（Q30），与项目目录无关。
type Project struct {
	ID   string `gorm:"column:id;primaryKey;size:64"`
	Name string `gorm:"column:name;size:255;not null"`
	// Path 有唯一索引：用户从 Finder 拖两次是很常见的，
	// 落成两条一模一样的记录会让他以为自己点错了。
	Path          string    `gorm:"column:path;size:1024;not null;uniqueIndex"`
	IsGitRepo     bool      `gorm:"column:is_git_repo;not null;default:false"`
	DefaultBranch string    `gorm:"column:default_branch;size:255;not null;default:''"`
	CreatedAt     time.Time `gorm:"column:created_at;not null"`
	UpdatedAt     time.Time `gorm:"column:updated_at;not null"`
}

// TableName 显式指定表名，不依赖 GORM 的自动推导——
// 推导规则变化或字段改名时，表结构会静默漂移。
func (Project) TableName() string { return "projects" }
