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
}

// TableName 显式指定表名，不依赖 GORM 的自动推导。
func (Event) TableName() string { return "events" }
