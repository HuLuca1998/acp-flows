// Package logging 是日志基础设施：分层级、按域可调、双去处（stderr + 落库）。
//
// 存在的理由：**日志是 AI 调试时的唯一观测面**。人可以 attach 调试器、看界面、
// 凭经验猜；AI 只能看日志。所以标准比一般项目高——结构化、可查询、可按域调级别。
//
// 规范见 docs/logging.md。
package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// LevelTrace 比 Debug 更低，用于协议报文全文、SQL 语句、逐帧事件。
//
// 它**不进 stderr**（量太大会把生命周期日志淹掉），只落库。
const LevelTrace slog.Level = -8

// levelNames 是 DUET_LOG 里可写的级别名。
var levelNames = map[string]slog.Level{
	"trace": LevelTrace,
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}

// ParseLevels 解析 DUET_LOG 的格式：`[全局级别,][组件=级别,]...`
//
//	""                          → info，无组件覆盖
//	"debug"                     → 全局 debug
//	"acp=trace"                 → 全局 info，acp 到 trace
//	"info,acp=trace,store=debug"
//
// 一次调试通常只关心一个组件，全局调 debug 会淹没在噪音里——所以支持按域调。
func ParseLevels(spec string) (slog.Level, map[string]slog.Level, error) {
	global := slog.LevelInfo
	comps := map[string]slog.Level{}

	for part := range strings.SplitSeq(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, value, isComponent := strings.Cut(part, "=")
		name, value = strings.TrimSpace(name), strings.TrimSpace(value)

		if isComponent {
			lv, err := lookupLevel(value)
			if err != nil {
				return 0, nil, fmt.Errorf("组件 %s 的级别无效: %w", name, err)
			}
			comps[strings.ToLower(name)] = lv
			continue
		}
		lv, err := lookupLevel(name)
		if err != nil {
			return 0, nil, err
		}
		global = lv
	}
	return global, comps, nil
}

func lookupLevel(name string) (slog.Level, error) {
	if lv, ok := levelNames[strings.ToLower(name)]; ok {
		return lv, nil
	}
	// 错误信息要列出可用取值，否则用户不知道该写什么
	return 0, fmt.Errorf("未知日志级别 %q，可用：trace debug info warn error", name)
}

// ── context 关联字段 ────────────────────────────────────────
//
// 日志的价值在于能把一件事的全过程串起来。手动传字段必然会漏，
// 所以关联字段（work_id / unit_id / attempt_id / trace_id）走 context 自动继承。

type ctxKey struct{}

// With 往 context 里塞一组关联字段，下游的日志自动带上。
//
// 在入口处调用（HTTP 中间件、ACP turn 开始），业务代码不用管。
func With(ctx context.Context, args ...any) context.Context {
	prev, _ := ctx.Value(ctxKey{}).([]any)
	merged := make([]any, 0, len(prev)+len(args))
	merged = append(merged, prev...)
	merged = append(merged, args...)
	return context.WithValue(ctx, ctxKey{}, merged)
}

// FromContext 返回带上 context 里全部关联字段的 logger。
func FromContext(ctx context.Context) *slog.Logger {
	return FromContextWith(ctx, slog.Default())
}

// FromContextWith 同 FromContext，但用指定的 base logger（测试用）。
func FromContextWith(ctx context.Context, base *slog.Logger) *slog.Logger {
	fields, _ := ctx.Value(ctxKey{}).([]any)
	if len(fields) == 0 {
		return base
	}
	return base.With(fields...)
}

// ── Sink：落库去处 ──────────────────────────────────────────

// Entry 是一条要落库的日志。
type Entry struct {
	Time      time.Time
	Level     slog.Level
	Component string
	Message   string
	// Attrs 是扁平化后的键值对，落库时序列化成 JSON。
	Attrs map[string]any
	// 关联字段单独提出来，因为它们要建索引。
	WorkID    string
	UnitID    string
	AttemptID string
	TraceID   string
}

// Sink 是日志的落库去处。
//
// ★ Write **不返回 error**：这是刻意的设计。落库失败只能降级为「只写 stderr」，
// 绝不向上抛——日志系统挂掉不该让产品挂掉（docs/logging.md §6）。
// 签名上就不给调用方处理错误的机会，避免有人写出 `if err := sink.Write(); err != nil { return err }`。
type Sink interface {
	Write(Entry)
	Close() error
}

// ── Handler ─────────────────────────────────────────────────

// Options 配置一个 Handler。
type Options struct {
	// Stderr 是给人看的去处，只收 INFO 以上（除非组件级别调低）。
	Stderr io.Writer
	// Sink 是给 AI 查的去处，收**全部**级别。可以是 nil（只写 stderr）。
	Sink Sink
	// GlobalLevel 是默认级别。
	GlobalLevel slog.Level
	// ComponentLevels 按组件覆盖 GlobalLevel。
	ComponentLevels map[string]slog.Level
	// SyncForTests 让落库同步执行，测试里才能立刻断言。
	SyncForTests bool
}

// Handler 把一条记录分发到 stderr 与 Sink。
type Handler struct {
	opts   Options
	text   slog.Handler
	attrs  []slog.Attr
	groups []string

	// 共享指针而非值：派生的 handler 写同一个 stderr，共享一把锁才对；
	// 而且值拷贝 sync.Mutex 会被 go vet 的 copylocks 拦下。
	mu *sync.Mutex
}

// NewHandler 建一个双去处 handler。
func NewHandler(opts Options) *Handler {
	if opts.Stderr == nil {
		opts.Stderr = io.Discard
	}
	if opts.ComponentLevels == nil {
		opts.ComponentLevels = map[string]slog.Level{}
	}
	return &Handler{
		opts: opts,
		mu:   &sync.Mutex{},
		// stderr 的门槛单独判，这里放到最低，由 Handle 决定写不写
		text: slog.NewTextHandler(opts.Stderr, &slog.HandlerOptions{Level: LevelTrace}),
	}
}

// Enabled 用最宽松的级别判断——具体过滤在 Handle 里按组件做。
//
// 这里不能直接用 GlobalLevel：某个组件可能被调到了 trace，
// 在 Enabled 阶段就挡掉的话，那条记录根本到不了 Handle。
func (h *Handler) Enabled(_ context.Context, lv slog.Level) bool {
	lowest := h.opts.GlobalLevel
	for _, cl := range h.opts.ComponentLevels {
		if cl < lowest {
			lowest = cl
		}
	}
	return lv >= lowest
}

// Handle 分发一条记录。
func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	comp, fields := h.collect(r)

	// 按组件决定门槛
	threshold := h.opts.GlobalLevel
	if cl, ok := h.opts.ComponentLevels[comp]; ok {
		threshold = cl
	}
	if r.Level < threshold {
		return nil
	}

	// 落库：收全部级别。★ 失败在 Sink 内部吞掉，这里拿不到错误
	if h.opts.Sink != nil {
		h.opts.Sink.Write(Entry{
			Time:      r.Time,
			Level:     r.Level,
			Component: comp,
			Message:   r.Message,
			Attrs:     fields,
			WorkID:    str(fields["work_id"]),
			UnitID:    str(fields["unit_id"]),
			AttemptID: str(fields["attempt_id"]),
			TraceID:   str(fields["trace_id"]),
		})
	}

	// stderr 的门槛与落库不同（docs/logging.md §2 的表）：
	//   TRACE  永不进 —— 它是报文全文，量大到会把生命周期日志完全淹掉
	//   DEBUG  只在有效门槛调到 DEBUG 及以下时才进
	//   INFO+  总是进
	if !stderrEligible(r.Level, threshold) {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.text.Handle(ctx, r)
}

// stderrEligible 报告这条记录该不该写到 stderr。
//
// TRACE 是硬排除，不受门槛影响——把它放进 stderr 等于让 `make dev-logs` 不可用。
// 要看 TRACE 就去查库（docs/logging.md §9）。
func stderrEligible(lv, threshold slog.Level) bool {
	if lv <= LevelTrace {
		return false
	}
	return lv >= slog.LevelInfo || lv >= threshold
}

// collect 把 handler 上挂的 attrs 与记录自带的 attrs 合并成扁平 map，
// 并取出 component。
func (h *Handler) collect(r slog.Record) (component string, fields map[string]any) {
	fields = make(map[string]any, r.NumAttrs()+len(h.attrs))
	for _, a := range h.attrs {
		fields[a.Key] = a.Value.Any()
	}
	r.Attrs(func(a slog.Attr) bool {
		fields[a.Key] = a.Value.Any()
		return true
	})
	return str(fields["component"]), fields
}

// WithAttrs 实现 slog.Handler。
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := *h
	next.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	next.text = h.text.WithAttrs(attrs)
	return &next
}

// WithGroup 实现 slog.Handler。
func (h *Handler) WithGroup(name string) slog.Handler {
	next := *h
	next.groups = append(append([]string{}, h.groups...), name)
	next.text = h.text.WithGroup(name)
	return &next
}

func str(v any) string {
	s, _ := v.(string)
	return s
}
