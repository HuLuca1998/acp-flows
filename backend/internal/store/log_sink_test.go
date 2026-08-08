package store_test

// 日志落库。规格见 docs/rules/logging.md §6 的三条硬约束：
//
//	① 不阻塞业务路径   ② 失败不向上抛   ③ 不无限增长
//
// 这三条都是「出问题时没人会发现」的那一类：
// 阻塞了只表现为「应用有点卡」、丢了只表现为「查不到那条日志」、
// 无限增长要几个月才暴露。所以必须逐条断言，不能靠人工观察。

import (
	"log/slog"
	"testing"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/platform/logging"
	"github.com/HuLuca1998/acp-flows/backend/internal/store"
	"github.com/HuLuca1998/acp-flows/backend/tests/testutil"
)

// countLogs 数 logs 表的行数。
func countLogs(t *testing.T, s *store.Store) int64 {
	t.Helper()
	var n int64
	if err := s.DB().Table("logs").Count(&n).Error; err != nil {
		t.Fatalf("count logs: %v", err)
	}
	return n
}

type logRow struct {
	Seq       int64
	Level     int
	Component string
	Msg       string
	Attrs     string
	WorkID    string `gorm:"column:work_id"`
	TraceID   string `gorm:"column:trace_id"`
	Ts        time.Time
}

func readLogs(t *testing.T, s *store.Store) []logRow {
	t.Helper()
	var rows []logRow
	if err := s.DB().Table("logs").Order("seq").Find(&rows).Error; err != nil {
		t.Fatalf("read logs: %v", err)
	}
	return rows
}

// R1 ★ 落库的内容与写入的一致 —— 关联字段与 attrs 都不能丢。
//
// 关联字段是排查时的钥匙（debug skill §3）。丢了的话日志还在，
// 但「这个 Work 的全过程」这类查询就查不出来了，等于白记。
func TestLogSink_R1_PersistsAllFields(t *testing.T) {
	s := newStore(t)
	sink := s.NewLogSink()

	sink.Write(logging.Entry{
		Time:      testutil.T0,
		Level:     slog.LevelWarn,
		Component: "acp",
		Message:   "取消超时",
		Attrs:     map[string]any{"elapsed_seconds": 30, "runtime": "codex"},
		WorkID:    "work-08",
		UnitID:    "unit-012",
		AttemptID: "att-03",
		TraceID:   "tr-77",
	})
	if err := sink.Close(); err != nil { // Close 会冲刷
		t.Fatalf("close sink: %v", err)
	}

	rows := readLogs(t, s)
	if len(rows) != 1 {
		t.Fatalf("落库 %d 条, want 1", len(rows))
	}
	r := rows[0]
	if r.Msg != "取消超时" {
		t.Errorf("msg = %q, want 取消超时", r.Msg)
	}
	if r.Level != int(slog.LevelWarn) {
		t.Errorf("level = %d, want %d", r.Level, int(slog.LevelWarn))
	}
	if r.Component != "acp" {
		t.Errorf("component = %q, want acp", r.Component)
	}
	if r.WorkID != "work-08" || r.TraceID != "tr-77" {
		t.Errorf("关联字段丢了: work_id=%q trace_id=%q", r.WorkID, r.TraceID)
	}
	// attrs 存 JSON，两个键都要在
	for _, want := range []string{`"elapsed_seconds":30`, `"runtime":"codex"`} {
		if !contains(r.Attrs, want) {
			t.Errorf("attrs 里缺 %s\n实际: %s", want, r.Attrs)
		}
	}
}

// R2 ★ Close 必须冲刷缓冲里剩下的日志，一条都不能丢。
//
// 批量大小是 200，不满一批时靠定时器或 Close 冲刷。
// Close 不冲刷的话，进程退出前那几十条——**恰恰是崩溃前最有价值的那几十条**——全没了。
func TestLogSink_R2_CloseFlushesRemainder(t *testing.T) {
	s := newStore(t)
	sink := s.NewLogSink()

	const n = 37 // 刻意小于批量大小 200
	for i := 0; i < n; i++ {
		sink.Write(logging.Entry{Time: testutil.T0, Message: "x"})
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("close sink: %v", err)
	}

	if got := countLogs(t, s); got != n {
		t.Errorf("落库 %d 条, want %d —— Close 没冲刷干净", got, n)
	}
}

// R3 ★ Write 永不阻塞：缓冲满时丢弃并计数。
//
// 阻塞业务路径比丢几条日志严重得多。这条如果破了，
// 症状是「应用偶尔卡住」，而没有人会想到是日志系统。
func TestLogSink_R3_WriteNeverBlocks(t *testing.T) {
	s := newStore(t)
	sink := s.NewLogSink()
	t.Cleanup(func() { _ = sink.Close() })

	// 远超缓冲容量（4096）。如果 Write 会阻塞，这个循环就跑不完，
	// 测试表现为**挂住**而不是失败——所以套一层超时。
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 50_000; i++ {
			sink.Write(logging.Entry{Time: testutil.T0, Message: "flood"})
		}
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Write 阻塞了 —— 缓冲满时必须丢弃而不是等待")
	}
}

// R4 ★ 保留策略：普通日志 7 天，ERROR 30 天。
//
// ERROR 留更久是因为排查线上问题时最需要老的 ERROR——
// 而普通日志的量是它的几百倍，留久了会把用户磁盘吃光。
func TestLogSink_R4_RetentionKeepsErrorsLonger(t *testing.T) {
	s := newStore(t)
	now := testutil.T0

	// 直接插历史数据，绕过 sink 的异步路径——这里测的是 prune 的判据
	insert := func(ageDays int, level slog.Level, msg string) {
		t.Helper()
		err := s.DB().Exec(
			`INSERT INTO logs (ts, level, component, msg, attrs) VALUES (?,?,?,?,'{}')`,
			now.AddDate(0, 0, -ageDays), int(level), "test", msg).Error
		if err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	insert(1, slog.LevelInfo, "新的 INFO")     // 保留
	insert(10, slog.LevelInfo, "旧的 INFO")    // 超过 7 天 → 删
	insert(10, slog.LevelError, "旧的 ERROR")  // 超过 7 天但 < 30 天 → 保留
	insert(40, slog.LevelError, "更旧的 ERROR") // 超过 30 天 → 删

	s.PruneLogs(now)

	rows := readLogs(t, s)
	got := map[string]bool{}
	for _, r := range rows {
		got[r.Msg] = true
	}
	for _, keep := range []string{"新的 INFO", "旧的 ERROR"} {
		if !got[keep] {
			t.Errorf("%q 被误删了", keep)
		}
	}
	for _, drop := range []string{"旧的 INFO", "更旧的 ERROR"} {
		if got[drop] {
			t.Errorf("%q 应该被清掉但还在", drop)
		}
	}
}

// R5 ★ 条数上限：超了从最旧的删，留下的是最新的。
//
// 只按时间清理挡不住「一天之内刷了几百万条」的情况——
// 那正是 TRACE 打开时会发生的事。
func TestLogSink_R5_RowCapDropsOldest(t *testing.T) {
	s := newStore(t)
	now := testutil.T0

	const total = 50
	for i := 0; i < total; i++ {
		err := s.DB().Exec(
			`INSERT INTO logs (ts, level, component, msg, attrs) VALUES (?,0,'test',?,'{}')`,
			now, itoa(i)).Error
		if err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	const rowCap = 20
	s.PruneLogsWithCap(now, rowCap)

	rows := readLogs(t, s)
	if len(rows) != rowCap {
		t.Fatalf("剩 %d 条, want %d", len(rows), rowCap)
	}
	// 留下的必须是**最新**的那 20 条（seq 最大的），不是最旧的
	if rows[0].Msg != itoa(total-rowCap) {
		t.Errorf("最旧的一条是 %q, want %q —— 删错了方向，把新日志删了",
			rows[0].Msg, itoa(total-rowCap))
	}
}

// R6 · attrs 序列化失败时不丢整条日志。
//
// 一个不可序列化的字段不该让整条日志消失——那条日志的 message
// 往往正是排查需要的。
func TestLogSink_R6_UnmarshalableAttrsDoNotDropEntry(t *testing.T) {
	s := newStore(t)
	sink := s.NewLogSink()

	sink.Write(logging.Entry{
		Time:    testutil.T0,
		Message: "带了个序列化不了的字段",
		// channel 不能被 json.Marshal
		Attrs: map[string]any{"ch": make(chan int)},
	})
	if err := sink.Close(); err != nil {
		t.Fatalf("close sink: %v", err)
	}

	rows := readLogs(t, s)
	if len(rows) != 1 {
		t.Fatalf("落库 %d 条, want 1 —— 序列化失败不该丢整条", len(rows))
	}
	if rows[0].Msg != "带了个序列化不了的字段" {
		t.Errorf("message 丢了: %q", rows[0].Msg)
	}
	if !contains(rows[0].Attrs, "__marshal_error") {
		t.Errorf("attrs 没有标记序列化失败: %s", rows[0].Attrs)
	}
}

// R7 ★ 落库失败不能 panic、不能阻塞。
//
// 表被删、磁盘满、库被锁——都只该降级为「这几条日志没了」。
func TestLogSink_R7_InsertFailureIsSwallowed(t *testing.T) {
	s := newStore(t)
	sink := s.NewLogSink()

	// 把表拿掉，制造必然失败的写入
	if err := s.DB().Exec(`DROP TABLE logs`).Error; err != nil {
		t.Fatalf("drop table: %v", err)
	}

	for i := 0; i < 10; i++ {
		sink.Write(logging.Entry{Time: testutil.T0, Message: "写不进去"})
	}
	// 这一行如果 panic 或挂住，测试就失败了——那正是要防的
	if err := sink.Close(); err != nil {
		t.Errorf("Close 返回了错误: %v —— 日志系统挂掉不该让产品挂掉", err)
	}
}

// R8 · Close 幂等，重复调用不 panic。
//
// 关闭路径上会有多个地方兜底调 Close，重复关闭 panic 会把优雅退出变成崩溃退出。
func TestLogSink_R8_CloseIsIdempotent(t *testing.T) {
	s := newStore(t)
	sink := s.NewLogSink()
	for i := 0; i < 3; i++ {
		if err := sink.Close(); err != nil {
			t.Fatalf("第 %d 次 Close 出错: %v", i+1, err)
		}
	}
}

// ── 小工具（避免为两处引入 strings / strconv 的心智负担）──────

func contains(haystack, needle string) bool {
	return len(needle) == 0 || indexOf(haystack, needle) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// ★★ Close 之后再写**不许 panic**。
//
// 真机撞到的：进程启动失败 → main 想用 slog.Error 记下原因 → 那时
// LogSink 已经关了 → 「send on closed channel」panic。
// 用户看到的是一个 goroutine 栈，而**真正的启动失败原因被完全盖住**。
//
// 关掉之后写日志是正常的：优雅退出的顺序永远排不完美，而记一条日志
// 不该有致命后果。
func TestLogSink_WriteAfterCloseDoesNotPanic(t *testing.T) {
	s := newStore(t)
	sink := s.NewLogSink()

	if err := sink.Close(); err != nil {
		t.Fatalf("关闭失败: %v", err)
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Close 之后写日志 panic 了：%v\n"+
				"真正的后果是启动失败的原因被这个 panic 完全盖住——"+
				"用户看到一个 goroutine 栈，而不是「为什么起不来」", r)
		}
	}()

	sink.Write(logging.Entry{Level: slog.LevelError, Message: "退出路径上的一条日志"})
	// 再写几条，确认不是碰巧
	for i := 0; i < 10; i++ {
		sink.Write(logging.Entry{Level: slog.LevelInfo, Message: "又一条"})
	}
}

// 重复 Close 也不该炸——优雅退出路径上很容易调两次。
func TestLogSink_DoubleCloseIsSafe(t *testing.T) {
	s := newStore(t)
	sink := s.NewLogSink()

	if err := sink.Close(); err != nil {
		t.Fatal(err)
	}
	if err := sink.Close(); err != nil {
		t.Errorf("第二次 Close 报错: %v", err)
	}
}
