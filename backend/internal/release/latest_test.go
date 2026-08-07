package release_test

// M1 · 发布源：读 Tauri updater 的 latest.json
//
// ★ 后端与 Tauri updater **查同一个 latest.json**。分成两个真源的话，
// 界面说「有更新」而 updater 说「没有」，用户会点一个永远不动的按钮。

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/release"
)

// latest.json 的真实形态（Tauri v2 updater）。
const latestJSON = `{
  "version": "1.5.0",
  "notes": "修复取消超时后 Runtime 仍在改文件",
  "pub_date": "2026-08-07T09:00:00Z",
  "platforms": {
    "darwin-aarch64": {
      "signature": "dW50cnVzdGVkIGNvbW1lbnQ6...",
      "url": "https://github.com/HuLuca1998/acp-flows/releases/download/v1.5.0/Duet_1.5.0_universal.app.tar.gz"
    }
  }
}`

func TestLatest_ParsesTauriUpdaterManifest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(latestJSON))
	}))
	t.Cleanup(srv.Close)

	got, err := release.NewHTTPSource(srv.URL, srv.Client()).Latest(context.Background())
	if err != nil {
		t.Fatalf("Latest 失败: %v", err)
	}

	if got.Version != "1.5.0" {
		t.Errorf("version: want %q, got %q", "1.5.0", got.Version)
	}
	if got.Notes != "修复取消超时后 Runtime 仍在改文件" {
		t.Errorf("notes: got %q", got.Notes)
	}
	want := time.Date(2026, 8, 7, 9, 0, 0, 0, time.UTC)
	if !got.PublishedAt.Equal(want) {
		t.Errorf("pub_date: want %v, got %v", want, got.PublishedAt)
	}
}

// 非 200 必须报错，且错误里带状态码——排查时第一个要看的就是它。
//
// ★ 404 是最可能出现的一种：还没发过任何 release 时，
// `releases/latest/download/latest.json` 就是 404。
// 静默当成「已是最新」的话，第一个版本发出去之前没人会发现这条链路是断的。
func TestLatest_NonOKIsAnError(t *testing.T) {
	for _, code := range []int{http.StatusNotFound, http.StatusInternalServerError, http.StatusForbidden} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(code)
			}))
			t.Cleanup(srv.Close)

			_, err := release.NewHTTPSource(srv.URL, srv.Client()).Latest(context.Background())
			if err == nil {
				t.Fatalf("HTTP %d 必须返回错误", code)
			}
			if !containsCode(err.Error(), code) {
				t.Errorf("错误信息里要带状态码，got %v", err)
			}
		})
	}
}

// 坏 JSON 报错，不静默返回零值。
func TestLatest_MalformedJSONIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"version": `))
	}))
	t.Cleanup(srv.Close)

	if _, err := release.NewHTTPSource(srv.URL, srv.Client()).Latest(context.Background()); err == nil {
		t.Fatal("坏 JSON 必须报错")
	}
}

// 缺 version 字段要报错：那样的 manifest 对更新检查毫无意义。
func TestLatest_MissingVersionIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"notes":"忘了写版本号"}`))
	}))
	t.Cleanup(srv.Close)

	if _, err := release.NewHTTPSource(srv.URL, srv.Client()).Latest(context.Background()); err == nil {
		t.Fatal("缺 version 必须报错")
	}
}

// ctx 取消要被尊重：设置页关掉时这次检查就该停，不该继续占着连接。
func TestLatest_RespectsContextCancellation(t *testing.T) {
	blocked := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		close(blocked)
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := release.NewHTTPSource(srv.URL, srv.Client()).Latest(ctx); err == nil {
		t.Fatal("ctx 已取消时必须返回错误")
	}
}

// 响应体过大要截断，不能把内存吃光。
//
// latest.json 正常只有几百字节。一个被劫持或写错的 endpoint 返回几 GB 时，
// 不设上限等于把 duetd 拖死。
func TestLatest_RejectsOversizedBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		huge := make([]byte, 2<<20) // 2MB，远超正常的几百字节
		for i := range huge {
			huge[i] = 'x'
		}
		_, _ = w.Write(huge)
	}))
	t.Cleanup(srv.Close)

	if _, err := release.NewHTTPSource(srv.URL, srv.Client()).Latest(context.Background()); err == nil {
		t.Fatal("超大响应必须被拒绝，不能无限读进内存")
	}
}

func containsCode(msg string, code int) bool {
	for _, c := range []string{"404", "500", "403"} {
		if c == itoa(code) {
			return contains(msg, c)
		}
	}
	return contains(msg, itoa(code))
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
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
