package fake

import (
	"math/rand/v2"
	"time"
)

// Latency 控制事件的时序。
//
// ★ **所有随机都由 Seed 驱动。** 测试里禁止不可复现的随机——
// 一个复现不了的失败，下一轮 AI 只会把它当 flaky 测试重跑一遍
// （testing-strategy.md §9）。
type Latency struct {
	// Base 是每个 Step 的基础延迟，叠加在脚本自己的 Delay 之上。
	Base time.Duration
	// Jitter 是抖动上界，由 Seed 确定性生成，范围 [0, Jitter)。
	Jitter time.Duration
	// Reorder 打开后按 Seed 确定性交换相邻 Step。
	Reorder bool
	// Seed 为 0 时用固定值 1，保证可复现。
	Seed int64
}

func (l Latency) seed() int64 {
	if l.Seed == 0 {
		return 1
	}
	return l.Seed
}

// delayFor 返回第 i 步的实际等待时长。
//
// 抖动按 (seed, i) 确定性生成 —— 用全局随机源的话，
// 同一个 seed 在不同测试里会给出不同结果，取决于谁先跑。
func (l Latency) delayFor(i int, scripted time.Duration) time.Duration {
	d := scripted + l.Base
	if l.Jitter > 0 {
		rng := rand.New(rand.NewPCG(uint64(l.seed()), uint64(i)))
		d += time.Duration(rng.Int64N(int64(l.Jitter)))
	}
	return d
}

// reorderSteps 按 seed 确定性交换相邻步骤。
//
// 保证**一定**改变顺序（长度 ≥ 2 时）：若随机结果恰好是原序，
// 强制交换第一对。否则 Reorder 这个开关会在某些 seed 下静默失效，
// 而测试看起来是绿的。
func reorderSteps(steps []Step, seed int64) []Step {
	if len(steps) < 2 {
		return steps
	}
	out := make([]Step, len(steps))
	copy(out, steps)

	rng := rand.New(rand.NewPCG(uint64(seed), 0))
	swapped := false
	for i := 0; i+1 < len(out); i++ {
		if rng.IntN(2) == 1 {
			out[i], out[i+1] = out[i+1], out[i]
			swapped = true
			i++ // 换过的这一对不再参与下一次交换，避免整体退化成轮转
		}
	}
	if !swapped {
		out[0], out[1] = out[1], out[0]
	}
	return out
}

// ── 预设 ──────────────────────────────────────────────────────────
//
// 签名固定为 func(*Runtime)，这样能直接放进表驱动测试的 setup 字段
// （testing-strategy.md §3.5）。改签名会让那批测试全部改写。

// NeverStops 让 Runtime **永不响应 session/prompt**。
//
// 用于测 ErrCancelTimeout → M1 的 update/prepare 返回 blocked。
// ★ 注意它只掐 stopReason，事件照常流出 ——
// 连接整个挂掉的话，测出来的是「连接断了」而不是「Runtime 不收尾」。
func NeverStops(r *Runtime) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.neverStops = true
}

// SilentAfter 让 Runtime 在首个 session/prompt 之后 d 彻底断流。
//
// 用于测静默超时与断点续传：消费方必须**感知到断开**，
// 而不是永久阻塞——永久阻塞的症状是测试挂住，比失败更难查。
func SilentAfter(d time.Duration) func(*Runtime) {
	return func(r *Runtime) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.silentAfter = d
	}
}

// Slow 给每一步加基础延迟与确定性抖动。
func Slow(base, jitter time.Duration) func(*Runtime) {
	return func(r *Runtime) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.latency.Base = base
		r.latency.Jitter = jitter
	}
}

// Reorder 按 seed 确定性打乱相邻事件的顺序。
//
// 真实 runtime 的事件不保证按发出顺序到达，消费方必须自己按 seq 归位。
func Reorder(seed int64) func(*Runtime) {
	return func(r *Runtime) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.latency.Reorder = true
		r.latency.Seed = seed
	}
}
