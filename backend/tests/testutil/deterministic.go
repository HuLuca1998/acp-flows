package testutil

import (
	"fmt"
	"sync"
	"time"
)

// T0 是测试里的固定基准时间。
//
// 用一个具体到秒的 UTC 时刻，golden 文件里出现它时一眼能认出是测试数据。
var T0 = time.Date(2026, 8, 7, 3, 12, 44, 0, time.UTC)

// FixedClockImpl 是永远吐同一个时间的 port.Clock。
type FixedClockImpl struct{ t time.Time }

// FixedClock 返回一个永远吐同一个时间的 port.Clock。
//
// 需要时间推进的场景用 StepClock。
func FixedClock(t time.Time) *FixedClockImpl { return &FixedClockImpl{t: t} }

// Now 返回固定时间。
func (c *FixedClockImpl) Now() time.Time { return c.t }

// Advance 把时钟往前拨，用于测试超时与时序。
func (c *FixedClockImpl) Advance(d time.Duration) { c.t = c.t.Add(d) }

// SeqIDGenImpl 是确定性的 ID 生成器。
type SeqIDGenImpl struct {
	mu   sync.Mutex
	seq  map[string]int
	ulid int
}

// SeqIDGen 返回确定性的 ID 生成器。
//
// 同一个前缀按 001、002 递增；ULID 保证按生成顺序单调递增
// （事件表依赖它排序，不递增会让断点续传的测试失去意义）。
func SeqIDGen() *SeqIDGenImpl { return &SeqIDGenImpl{seq: map[string]int{}} }

// NextID 按前缀返回递增的确定性标识符（work-001、work-002…）。
func (g *SeqIDGenImpl) NextID(prefix string) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.seq[prefix]++
	return fmt.Sprintf("%s-%03d", prefix, g.seq[prefix])
}

// NextULID 返回单调递增的标识符——事件表按它排序。
func (g *SeqIDGenImpl) NextULID() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.ulid++
	// 定宽零填充，保证字典序 == 生成顺序。
	return fmt.Sprintf("evt_%020d", g.ulid)
}
