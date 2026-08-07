package store

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"

	"github.com/HuLuca1998/acp-flows/backend/internal/store/entity"
)

// Event 是一条事件。
//
// ★ 这里**重新定义**而不是 import eventbus 的同名类型：
// 基础设施之间不互相依赖（depguard 的 infra 规则）。两边字段一致，
// Go 的结构化类型让 ProjectRepo 这样的实现自动满足 eventbus.Store 接口——
// 但那要求字段名与顺序完全一致，改一边就要改另一边。
//
// 代价是这份重复；收益是 store 与 eventbus 谁都不知道对方存在，
// 将来换掉任何一边都不用动另一边。
type Event struct {
	ID      string          `json:"id"`
	Seq     int64           `json:"seq"`
	WorkID  string          `json:"work_id,omitempty"`
	Source  string          `json:"source"`
	Type    string          `json:"type"`
	TS      time.Time       `json:"ts"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// EventRepo 是事件的持久化实现。
type EventRepo struct {
	db *gorm.DB
}

const eventColumns = "seq, id, work_id, source, type, ts, payload"

// AppendEvent 落一条事件，并把数据库发放的序号写回 e.Seq。
//
// ★ 序号由**自增主键**发放，不由内存计数器发。内存计数器一重启就归零，
// 而前端按 seq 去重——归零之后新事件会被当成旧的丢掉，
// 表现是「重启后 AI 说的话不再显示了」。
func (r *EventRepo) AppendEvent(ctx context.Context, e *Event) error {
	payload := e.Payload
	if len(payload) == 0 {
		payload = json.RawMessage("{}")
	}

	row := &entity.Event{
		ID: e.ID, WorkID: e.WorkID, Source: e.Source,
		Type: e.Type, TS: e.TS, Payload: string(payload),
	}
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		return translate("append event "+e.Type, err)
	}

	e.Seq = row.Seq
	return nil
}

// MaxSeq 返回已落库的最大序号；空库返回 0。
func (r *EventRepo) MaxSeq(ctx context.Context) (int64, error) {
	var maxSeq *int64
	if err := r.db.WithContext(ctx).
		Model(&entity.Event{}).
		Select("MAX(seq)").
		Scan(&maxSeq).Error; err != nil {
		return 0, translate("max event seq", err)
	}
	if maxSeq == nil {
		return 0, nil // 空表：MAX() 返回 NULL，不是 0
	}
	return *maxSeq, nil
}

// EventsAfter 返回序号大于 after 的事件，最多 limit 条，按序号升序。
//
// ★ 断线重连**只补没收到的**。从头补的话，用户重连后会看到整条时间线又
// 重放一遍；补少了则中间有洞，而洞是看不出来的——用户不知道自己漏了什么。
//
// limit 是必需的：一个断了一整天的客户端重连时，不该把几万条一次性灌给它。
func (r *EventRepo) EventsAfter(ctx context.Context, after int64, limit int) ([]Event, error) {
	if limit <= 0 {
		limit = 500
	}

	var rows []entity.Event
	if err := r.db.WithContext(ctx).
		Select(eventColumns).
		Where("seq > ?", after).
		Order("seq").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, translate("events after", err)
	}

	// 空结果返回空切片而不是 nil：上层要序列化成 [] 而不是 null
	out := make([]Event, 0, len(rows))
	for _, row := range rows {
		out = append(out, Event{
			ID: row.ID, Seq: row.Seq, WorkID: row.WorkID,
			Source: row.Source, Type: row.Type, TS: row.TS,
			Payload: json.RawMessage(row.Payload),
		})
	}
	return out, nil
}
