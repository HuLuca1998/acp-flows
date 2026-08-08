package store

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/platform/logging"
)

// 落库的三条硬约束（docs/rules/logging.md §6）在这里实现：
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

	// closed 在 Close 之后为真。
	//
	// ★ 没有它的话，关掉之后再写会 panic「send on closed channel」。
	// 那不是理论问题：进程启动失败时，main 要用 slog.Error 记下原因，
	// 而那时 sink 已经关了——**真正的失败原因被 panic 栈完全盖住**，
	// 用户看到的是一堆 goroutine 而不是「为什么起不来」。真机撞到过。
	//
	// 用读写锁而不是 atomic：Write 与 close(ch) 之间要真正互斥，
	// 只标记的话仍然有「查完标记、还没发、对方关了」这个窗口。
	mu     sync.RWMutex
	closed bool

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

// Write 把一条日志塞进缓冲。**永不阻塞、永不返回错误、永不 panic。**
//
// 关掉之后写日志是正常的：优雅退出的顺序永远排不完美，
// 而记一条日志不该有致命后果。
func (l *LogSink) Write(e logging.Entry) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.closed {
		// 已经关了：静静丢掉并计数。这时候进程正在退出，
		// 落不落库都不重要——重要的是别把退出路径炸掉。
		l.dropped.add(1)
		return
	}

	select {
	case l.ch <- e:
	default:
		// 缓冲满：丢弃并计数。阻塞业务路径比丢日志严重得多。
		l.dropped.add(1)
	}
}

// Close 冲刷剩余日志并停止后台写入。
func (l *LogSink) Close() error {
	l.closer.Do(func() {
		// ★ 拿写锁再关：与 Write 的读锁互斥，保证没有人正卡在
		// 「查完 closed、还没发」那个窗口里。
		l.mu.Lock()
		l.closed = true
		close(l.ch)
		l.mu.Unlock()
	})
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

// prune 执行保留策略。时间取自注入的 Clock ——
// **不要用 time.Now()**：那会让保留策略成为唯一测不了的一段
// （`internal/platform/AGENTS.md`：不确定性只从平台层进来）。
func (l *LogSink) prune() {
	l.s.PruneLogsWithCap(l.s.clk.Now().UTC(), logMaxRows)
}

// PruneLogs 按默认条数上限执行保留策略。
func (s *Store) PruneLogs(now time.Time) { s.PruneLogsWithCap(now, logMaxRows) }

// PruneLogsWithCap 执行保留策略：
//
//	普通日志保留 logRetainDays 天；ERROR 保留 logRetainErrDay 天；
//	总行数超过 maxRows 时从最旧的删。
//
// ★ 导出并把 now / maxRows 参数化，是为了让保留策略可被断言。
// 保留策略写错的症状是「几个月后磁盘满」或者「要查的老 ERROR 没了」——
// 两种都太晚才发现，必须在测试里钉死。
//
// 全部错误都吞掉：清理失败不该影响产品（`docs/rules/logging.md` §6）。
func (s *Store) PruneLogsWithCap(now time.Time, maxRows int64) {
	db := s.db

	// 普通日志先删；ERROR 留更久——排查线上问题时最需要老 ERROR
	_ = db.Exec(`DELETE FROM logs WHERE ts < ? AND level < ?`,
		now.AddDate(0, 0, -logRetainDays), int(slog.LevelError)).Error
	_ = db.Exec(`DELETE FROM logs WHERE ts < ?`,
		now.AddDate(0, 0, -logRetainErrDay)).Error

	// 条数上限：超了从**最旧**的删。
	// 只按时间清理挡不住「一天之内刷了几百万条」——那正是 TRACE 打开时会发生的事。
	var n int64
	if err := db.Table("logs").Count(&n).Error; err == nil && n > maxRows {
		_ = db.Exec(`DELETE FROM logs WHERE seq IN (
			SELECT seq FROM logs ORDER BY seq LIMIT ?)`, n-maxRows).Error
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
