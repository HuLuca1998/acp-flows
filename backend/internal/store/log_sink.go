package store

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/platform/logging"
)

// 落库的三条硬约束（docs/logging.md §6）在这里实现：
//
//	① 不阻塞业务路径  —— 异步缓冲 + 批量提交
//	② 失败不向上抛    —— Write 不返回 error，内部吞掉
//	③ 不无限增长      —— 保留 7 天 / 20 万条，ERROR 保留 30 天
const (
	logBufferSize   = 4096
	logBatchSize    = 200
	logFlushEvery   = 500 * time.Millisecond
	logRetainDays   = 7
	logRetainErrDay = 30
	logMaxRows      = 200_000
)

// LogSink 把日志异步批量写进 SQLite。
type LogSink struct {
	s      *Store
	ch     chan logging.Entry
	done   chan struct{}
	closer sync.Once

	// dropped 记录因缓冲满而丢弃的条数。
	// 满了就丢是刻意的——阻塞业务路径比丢几条日志严重得多。
	dropped atomic64
}

// NewLogSink 建一个落库 sink 并启动后台写入。
func (s *Store) NewLogSink() *LogSink {
	sink := &LogSink{
		s:    s,
		ch:   make(chan logging.Entry, logBufferSize),
		done: make(chan struct{}),
	}
	go sink.loop()
	return sink
}

// Write 把一条日志塞进缓冲。**永不阻塞、永不返回错误。**
func (l *LogSink) Write(e logging.Entry) {
	select {
	case l.ch <- e:
	default:
		// 缓冲满：丢弃并计数。阻塞业务路径比丢日志严重得多。
		l.dropped.add(1)
	}
}

// Close 冲刷剩余日志并停止后台写入。
func (l *LogSink) Close() error {
	l.closer.Do(func() { close(l.ch) })
	<-l.done
	return nil
}

func (l *LogSink) loop() {
	defer close(l.done)
	ticker := time.NewTicker(logFlushEvery)
	defer ticker.Stop()

	// 启动时清一次，之后每小时清一次
	l.prune()
	prune := time.NewTicker(time.Hour)
	defer prune.Stop()

	batch := make([]logging.Entry, 0, logBatchSize)
	flush := func() {
		if len(batch) > 0 {
			l.insert(batch)
			batch = batch[:0]
		}
	}

	for {
		select {
		case e, ok := <-l.ch:
			if !ok {
				flush()
				return
			}
			batch = append(batch, e)
			if len(batch) >= logBatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-prune.C:
			l.prune()
		}
	}
}

// insert 批量写入。**失败只能吞掉**——日志系统挂掉不该让产品挂掉。
func (l *LogSink) insert(batch []logging.Entry) {
	rows := make([]map[string]any, 0, len(batch))
	for _, e := range batch {
		attrs, err := json.Marshal(e.Attrs)
		if err != nil {
			attrs = []byte(`{"__marshal_error":true}`)
		}
		rows = append(rows, map[string]any{
			"ts": e.Time, "level": int(e.Level), "component": e.Component,
			"msg": e.Message, "attrs": string(attrs),
			"work_id": e.WorkID, "unit_id": e.UnitID,
			"attempt_id": e.AttemptID, "trace_id": e.TraceID,
		})
	}
	// 独立的短事务，不与业务事务混——SQLite 是单写者（database.md §6）
	_ = l.s.db.Table("logs").Create(rows).Error
}

// prune 执行保留策略。
func (l *LogSink) prune() {
	now := time.Now().UTC()
	db := l.s.db

	// 普通日志保留 7 天；ERROR 保留 30 天——排查线上问题时最需要老 ERROR
	_ = db.Exec(`DELETE FROM logs WHERE ts < ? AND level < ?`,
		now.AddDate(0, 0, -logRetainDays), int(slog.LevelError)).Error
	_ = db.Exec(`DELETE FROM logs WHERE ts < ?`,
		now.AddDate(0, 0, -logRetainErrDay)).Error

	// 条数上限：超了从最旧的删
	var n int64
	if err := db.Table("logs").Count(&n).Error; err == nil && n > logMaxRows {
		_ = db.Exec(`DELETE FROM logs WHERE seq IN (
			SELECT seq FROM logs ORDER BY seq LIMIT ?)`, n-logMaxRows).Error
	}
}

// atomic64 是个极小的原子计数器，避免为一个计数引入依赖。
type atomic64 struct {
	mu sync.Mutex
	n  int64
}

func (a *atomic64) add(d int64) { a.mu.Lock(); a.n += d; a.mu.Unlock() }

// DroppedLogs 返回因缓冲满而丢弃的日志条数。
func (l *LogSink) DroppedLogs() int64 {
	l.dropped.mu.Lock()
	defer l.dropped.mu.Unlock()
	return l.dropped.n
}
