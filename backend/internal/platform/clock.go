package platform

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"sync"
	"time"
)

// systemClock 是生产环境的时间源。
type systemClock struct{}

// NewClock 返回系统时间源。
//
// 一律返回 UTC：数据库里存 UTC，事件流里也是 UTC，
// 时区转换只在前端展示时做一次。
func NewClock() *systemClock { return &systemClock{} }

func (systemClock) Now() time.Time { return time.Now().UTC() }

// idGen 生成带类型前缀的顺序 ID 与按时间可排序的 ULID。
type idGen struct {
	mu   sync.Mutex
	seq  map[string]int
	clk  interface{ Now() time.Time }
	rand func() uint64
}

// NewIDGen 返回生产环境的 ID 生成器。
//
// 前缀序号由调用方在启动时用数据库里的最大值预热（PrimeSeq），
// 避免重启后 ID 回退撞主键。
func NewIDGen(clk interface{ Now() time.Time }) *idGen {
	return &idGen{
		seq:  map[string]int{},
		clk:  clk,
		rand: cryptoRandUint64,
	}
}

// PrimeSeq 把某个前缀的序号预热到 n，供启动时从数据库回填。
func (g *idGen) PrimeSeq(prefix string, n int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if n > g.seq[prefix] {
		g.seq[prefix] = n
	}
}

// NextID 返回形如 "work-08" 的标识符。
func (g *idGen) NextID(prefix string) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.seq[prefix]++
	return fmt.Sprintf("%s-%02d", prefix, g.seq[prefix])
}

// NextULID 返回按时间可排序的标识符，用于高频写入的事件表。
//
// 形状是 evt_<48位毫秒时间戳><随机>，定宽十六进制保证字典序 == 时间序。
func (g *idGen) NextULID() string {
	ms := uint64(g.clk.Now().UnixMilli())
	return fmt.Sprintf("evt_%012x%016x", ms&0xFFFFFFFFFFFF, g.rand())
}

func cryptoRandUint64() uint64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand 失败在实践中意味着系统级故障；ID 唯一性是硬需求，
		// 退化成可预测值会造成主键冲突，所以这里不静默兜底。
		panic(fmt.Sprintf("platform: crypto/rand failed: %v", err))
	}
	return binary.BigEndian.Uint64(b[:])
}
