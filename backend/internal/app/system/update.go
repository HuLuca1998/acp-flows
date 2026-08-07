// Package system 是系统级用例：版本、更新检查、更新前准备。
//
// **只检查，绝不下载、绝不安装**（docs/adr/0002）。安装包由 Tauri updater 处理，
// duetd 从头到尾不碰它——duetd 会在更新时被替换掉，让它管自己的替换过程是错的。
package system

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// UpdateState 是检查更新的结果状态，取值与 api/openapi.yaml 的 UpdateStatus.state 一致。
type UpdateState string

// 三种状态。
const (
	// UpdateStateIdle 已是最新（或远端更旧）。
	UpdateStateIdle UpdateState = "idle"
	// UpdateStateAvailable 有可用更新。
	UpdateStateAvailable UpdateState = "available"
	// UpdateStateUnsupported Web 版，没有 updater，无法自动更新。
	UpdateStateUnsupported UpdateState = "unsupported"
)

// PrepareStatus 是更新前准备的结果。
type PrepareStatus string

// 两种结果。
const (
	// PrepareReady 可以继续安装。
	PrepareReady PrepareStatus = "ready"
	// PrepareBlocked 有工作挡着，前端**不得继续安装**。
	PrepareBlocked PrepareStatus = "blocked"
)

// 机器可读的阻塞原因码。前端据此查 i18n 词条，不直接展示（docs/rules/i18n.md §3）。
const (
	// ReasonWorkInProgress 有未结束的工作。
	ReasonWorkInProgress = "work_in_progress"
	// ReasonWorkStateUnknown 查不到工作状态——失败安全，按挡住处理。
	ReasonWorkStateUnknown = "work_state_unknown"
)

// UpdateStatus 是检查更新的结果。
type UpdateStatus struct {
	State          UpdateState
	CurrentVersion string
	LatestVersion  string
	Notes          string
	SizeBytes      int64
	PublishedAt    time.Time
}

// BlockedWork 是一条挡住更新的工作。
type BlockedWork struct {
	WorkID string
	Reason string
}

// PrepareResult 是更新前准备的结果。
type PrepareResult struct {
	Status  PrepareStatus
	Blocked []BlockedWork
}

// UpdateConfig 构造 UpdateService 需要的东西。
type UpdateConfig struct {
	// CurrentVersion 是当前运行的版本，构建时注入。
	CurrentVersion string
	// UpdaterAvailable 表示这个进程是被 Tauri 壳拉起来的（有 updater）。
	// 纯 Web 形态下为 false。
	UpdaterAvailable bool
	Source           port.ReleaseSource
	Works            port.WorkLister
}

// UpdateService 实现检查更新与更新前准备。
type UpdateService struct {
	current          model.Version
	updaterAvailable bool
	source           port.ReleaseSource
	works            port.WorkLister
}

// NewUpdateService 构造用例。缺依赖时立刻返回错误，不留到运行时 panic。
func NewUpdateService(cfg UpdateConfig) (*UpdateService, error) {
	current, err := model.ParseVersion(cfg.CurrentVersion)
	if err != nil {
		return nil, fmt.Errorf("system: 当前版本号不合法: %w", err)
	}
	if cfg.Works == nil {
		return nil, errors.New("system: UpdateConfig.Works 必填")
	}
	// Source 允许为 nil：Web 形态下根本不会用到它。
	return &UpdateService{
		current:          current,
		updaterAvailable: cfg.UpdaterAvailable,
		source:           cfg.Source,
		works:            cfg.Works,
	}, nil
}

// Check 检查有没有可用更新。**只查一个几百字节的 latest.json，不下载任何安装包。**
func (s *UpdateService) Check(ctx context.Context) (UpdateStatus, error) {
	status := UpdateStatus{
		State:          UpdateStateIdle,
		CurrentVersion: s.current.String(),
	}

	// Web 形态直接返回，**不查发布源**：查了也没法安装，
	// 而「提示了更新却点不动」会把用户卡在一个没有出路的界面上。
	if !s.updaterAvailable || s.source == nil {
		status.State = UpdateStateUnsupported
		return status, nil
	}

	release, err := s.source.Latest(ctx)
	if err != nil {
		// ★ 绝不降级成「已是最新版本」：网络断了、URL 写错了都会走到这里，
		// 静默的话用户永远不知道自己在用半年前的版本。
		return UpdateStatus{}, fmt.Errorf("system: 查询发布源失败: %w", err)
	}

	latest, err := model.ParseVersion(release.Version)
	if err != nil {
		return UpdateStatus{}, fmt.Errorf("system: 发布源报的版本号不合法 %q: %w", release.Version, err)
	}

	// 远端不比本地新就是 idle —— 包括**远端更旧**的情况。
	// 回滚发布之后如果还提示更新，用户会被反复劝着装回旧版。
	if !latest.After(s.current) {
		return status, nil
	}

	status.State = UpdateStateAvailable
	status.LatestVersion = latest.String()
	status.Notes = release.Notes
	status.SizeBytes = release.SizeBytes
	status.PublishedAt = release.PublishedAt
	return status, nil
}

// Prepare 判断现在更新会不会打断用户。
//
// ★ **失败安全**：只要有一个非终态工作就 blocked，查询失败也 blocked。
// 这是简化版——完整语义是「两段式取消 → 落检查点 → paused」，
// 要等 M3 的 U3.2.1 与 M4 的 U4.1.2。在那之前**宁可拦住更新，也不能丢用户的工作**：
// 「更新不丢工作」是这个产品能被信任的前提。
func (s *UpdateService) Prepare(ctx context.Context) (PrepareResult, error) {
	works, err := s.works.ListWorks(ctx)
	if err != nil {
		// 查不到就是不知道有没有工作在跑。不知道的时候重启应用是最坏的选择。
		return PrepareResult{
			Status:  PrepareBlocked,
			Blocked: []BlockedWork{{Reason: ReasonWorkStateUnknown}},
		}, fmt.Errorf("system: 查询工作列表失败: %w", err)
	}

	var blocked []BlockedWork
	for _, w := range works {
		if w == nil || model.IsTerminal(w.State()) {
			continue
		}
		blocked = append(blocked, BlockedWork{WorkID: w.ID(), Reason: ReasonWorkInProgress})
	}

	if len(blocked) > 0 {
		return PrepareResult{Status: PrepareBlocked, Blocked: blocked}, nil
	}
	return PrepareResult{Status: PrepareReady}, nil
}
