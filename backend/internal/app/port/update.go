package port

import (
	"context"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// Release 是发布源上的一个版本。
//
// 字段对应 latest.json 里 duetd 需要展示给用户的部分。
// **安装包本身不经过 duetd** —— 下载与安装由 Tauri updater 做（docs/adr/0002）。
type Release struct {
	// Version 是发布源报的版本号，可能非法（上游拼错），由用例负责校验。
	Version string
	// Notes 是更新说明，界面原样展示。
	Notes string
	// SizeBytes 是安装包体积，用于让用户判断要不要现在更新。
	SizeBytes int64
	// PublishedAt 是发布时间。
	PublishedAt time.Time
}

// ReleaseSource 查询发布源上的最新版本。
//
// 实现必须**只读**：绝不下载安装包、绝不写磁盘。
// 检查更新的全部网络开销就是拉一个几百字节的 latest.json。
type ReleaseSource interface {
	Latest(ctx context.Context) (Release, error)
}

// WorkLister 只读列出全部工作。
//
// `update/prepare` 用它判断「现在更新会不会打断用户」。
// 接口刻意只有一个方法：巨型 fake 是假测试的温床。
type WorkLister interface {
	ListWorks(ctx context.Context) ([]*model.Work, error)
}
