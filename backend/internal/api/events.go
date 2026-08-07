package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/eventbus"
)

// resumeLimit 是一次补发的上限。
//
// 断了一整天的客户端重连时不该被灌几万条——那会让界面卡住好几秒，
// 而它真正关心的是最近发生了什么。超出的部分靠下一次 Last-Event-ID 继续补。
const resumeLimit = 500

// heartbeatInterval 是心跳间隔。
//
// ★ 没有心跳的话，一条长时间无事件的连接会被中间层（代理、系统休眠）
// 静默掐断，而**客户端不知道**——它以为还连着，只是 AI 恰好没说话。
// SSE 的注释行（以 : 开头）不会触发 onmessage，正是为这个用的。
const heartbeatInterval = 25 * time.Second

// eventHistory 是补发用的历史事件源。接口定义在使用方。
type eventHistory interface {
	EventsAfter(ctx context.Context, after int64, limit int) ([]eventbus.Event, error)
}

// handleStreamEvents 处理 GET /v1/events —— SSE 长连接。
//
// ★ **先订阅，再补历史。** 反过来的话，补发与订阅之间发生的事件会掉进缝里：
// 谁都不会送到，而用户永远不知道自己漏了什么。
// 先订阅的代价是那段时间的事件可能重复——重复是看得见的（前端按 seq 去重），
// 缺失不是。
func handleStreamEvents(bus *eventbus.Bus, hist eventHistory) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if bus == nil {
			writeProblem(w, http.StatusServiceUnavailable,
				"event_stream_unavailable", "Event bus is not configured")
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			// 不能 flush 就没法流式——与其半死不活，不如明确报错
			writeProblem(w, http.StatusInternalServerError,
				"streaming_unsupported", "ResponseWriter does not support flushing")
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		// 没有 no-cache 的话中间层可能缓存整条流，用户看到的是几分钟前的进度
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		// nginx 之类的反代默认会缓冲响应体，那会把「边发边到」变成「一次性吐出」
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		ctx := r.Context()
		sub := bus.Subscribe(ctx, 0)
		defer sub.Close()

		// 补历史。Last-Event-ID 是标准 SSE 头，浏览器的 EventSource
		// 会自动带上，前端不用自己维护游标。
		if after, ok := parseLastEventID(r); ok && hist != nil {
			past, err := hist.EventsAfter(ctx, after, resumeLimit)
			if err != nil {
				// 补发失败就断开，让客户端重连再试——
				// 装作没事继续推的话，中间那段会永久缺失
				return
			}
			for _, e := range past {
				if !writeSSE(w, flusher, e) {
					return
				}
			}
		}

		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case e, open := <-sub.Events():
				if !open {
					return
				}
				if !writeSSE(w, flusher, e) {
					return
				}
			case <-ticker.C:
				// 注释行：保活但不触发客户端的 onmessage
				if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
					return
				}
				flusher.Flush()
			}
		}
	}
}

// parseLastEventID 读 Last-Event-ID 头。
//
// 解析不了时按**全新连接**处理而不是报错：中间层或手工重放请求很容易带上
// 乱七八糟的值，为此回 500 会让用户彻底连不上，而全新连接至少还能用。
func parseLastEventID(r *http.Request) (int64, bool) {
	raw := r.Header.Get("Last-Event-ID")
	if raw == "" {
		return 0, false
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

// writeSSE 写一条 SSE 消息，返回是否还能继续写。
//
// ★ id 用 seq：浏览器的 EventSource 会把它记进 Last-Event-ID 并在重连时带回，
// 前端因此不需要自己维护游标。
func writeSSE(w http.ResponseWriter, flusher http.Flusher, e eventbus.Event) bool {
	data, err := json.Marshal(e)
	if err != nil {
		// 单条编不出来不该断掉整条流——跳过它，其余照常
		return true
	}

	if _, err := fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", e.Seq, e.Type, data); err != nil {
		return false // 客户端断了
	}
	flusher.Flush()
	return true
}
