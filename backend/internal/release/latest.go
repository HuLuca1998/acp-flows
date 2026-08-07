// Package release 读发布源上的 latest.json。
//
// ★ **只读一个几百字节的 manifest，绝不下载安装包。**
// 安装包由 Tauri updater 处理——duetd 会在更新时被替换掉，
// 让它管自己的替换过程是错的（docs/adr/0002）。
//
// 查的是**和 Tauri updater 同一个 URL**（tauri.conf.json 的 plugins.updater.endpoints）。
// 分成两个真源的话，界面说「有更新」而 updater 说「没有」，
// 用户会点一个永远不动的按钮。
package release

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
)

// maxManifestBytes 是 latest.json 的读取上限。
//
// 正常只有几百字节。不设上限的话，一个被劫持或写错的 endpoint
// 返回几 GB 就能把 duetd 拖死。
const maxManifestBytes = 256 << 10

// defaultTimeout 是单次检查的超时。
//
// 检查更新是用户点了「检查更新」之后的前台操作，卡住比失败更糟——
// 失败至少会告诉他重试。
const defaultTimeout = 10 * time.Second

// HTTPSource 从 HTTP 端点读 Tauri updater 的 latest.json。
type HTTPSource struct {
	url    string
	client *http.Client
}

// NewHTTPSource 构造发布源。client 为 nil 时用带超时的默认客户端。
func NewHTTPSource(url string, client *http.Client) *HTTPSource {
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
	}
	return &HTTPSource{url: url, client: client}
}

// manifest 是 Tauri v2 updater 的 latest.json 结构。
//
// 只解析 duetd 要展示的字段：platforms 里的下载地址与签名归 updater 管，
// 这里解析它们没有意义，还会在格式变化时无谓地失败。
type manifest struct {
	Version string `json:"version"`
	Notes   string `json:"notes"`
	PubDate string `json:"pub_date"`
}

// Latest 读取发布源上的最新版本。
func (s *HTTPSource) Latest(ctx context.Context) (port.Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return port.Release{}, fmt.Errorf("release: 构造请求失败: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return port.Release{}, fmt.Errorf("release: 请求 %s 失败: %w", s.url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		// 状态码要带出来：404 是最常见的一种——还没发过任何 release 时
		// releases/latest/download/latest.json 就是 404。
		return port.Release{}, fmt.Errorf("release: %s 返回 HTTP %d", s.url, resp.StatusCode)
	}

	// 多读 1 字节用来判断是否超限：正好读满上限说明后面还有。
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxManifestBytes+1))
	if err != nil {
		return port.Release{}, fmt.Errorf("release: 读取响应失败: %w", err)
	}
	if len(body) > maxManifestBytes {
		return port.Release{}, fmt.Errorf(
			"release: %s 的响应超过 %d 字节——latest.json 正常只有几百字节，这个端点不对",
			s.url, maxManifestBytes)
	}

	var m manifest
	if err := json.Unmarshal(body, &m); err != nil {
		return port.Release{}, fmt.Errorf("release: 解析 latest.json 失败: %w", err)
	}
	if m.Version == "" {
		return port.Release{}, errors.New("release: latest.json 缺少 version 字段")
	}

	out := port.Release{Version: m.Version, Notes: m.Notes}
	if m.PubDate != "" {
		// 解析失败不致命：发布时间只是展示用，没有它照样能更新。
		if t, err := time.Parse(time.RFC3339, m.PubDate); err == nil {
			out.PublishedAt = t
		}
	}
	return out, nil
}
