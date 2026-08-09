package entity

import "time"

// Event 是 events 表的行结构。
//
// ★ seq 用自增主键：序号因此由数据库发放，而不是内存计数器。
// 这是全库第二处用自增的地方（另一处是 logs），理由相同——
// 高频写入，且序号本身对外有意义（前端按它去重与续传）。
type Event struct {
	Seq     int64     `gorm:"column:seq;primaryKey;autoIncrement"`
	ID      string    `gorm:"column:id;size:64;not null"`
	WorkID  string    `gorm:"column:work_id;size:64;not null;default:''"`
	Source  string    `gorm:"column:source;size:16;not null"`
	Type    string    `gorm:"column:type;size:64;not null"`
	TS      time.Time `gorm:"column:ts;not null"`
	Payload string    `gorm:"column:payload;not null;default:'{}'"`

	// Role / RoleDisplayName / Runtime 说明**这一条是谁说的**。
	//
	// ★★ 不存的话，用户重开应用（或前端首次连接带 `Last-Event-ID: 0`
	// 要历史）看到的第一屏就没有角色标签——而他正是靠这个标签判断
	// 「现在是谁在说话、他能不能动我的文件」。
	Role            string `gorm:"column:role;size:64;not null;default:''"`
	RoleDisplayName string `gorm:"column:role_display_name;size:64;not null;default:''"`
	Runtime         string `gorm:"column:runtime;size:32;not null;default:''"`
	// RequirementVersion / RequirementFrozen 是说这句话时需求的样子。
	// 0 表示那时还没有需求快照。
	RequirementVersion int  `gorm:"column:requirement_version;not null;default:0"`
	RequirementFrozen  bool `gorm:"column:requirement_frozen;not null;default:false"`
}

// TableName 显式指定表名，不依赖 GORM 的自动推导。
func (Event) TableName() string { return "events" }
