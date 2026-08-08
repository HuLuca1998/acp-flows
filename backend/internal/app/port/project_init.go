package port

import "context"

// InitAction 是初始化计划里的一步。
type InitAction struct {
	Kind         string
	Path         string
	Reason       string
	Lines        []string
	AlreadyThere bool
}

// InitPlan 是一次项目初始化的完整计划。
type InitPlan struct {
	Root      string
	IsGitRepo bool
	Actions   []InitAction
}

// ProjectInitializer 算出并执行「把一个目录变成 Duet 项目」要做的事。
//
// ★★ **Plan 只读，Apply 照单执行，两者用同一份计划。**
// 分开算两遍的话它们必然漂移，而漂移的方向永远是
// 「预演里没说的那件事被做了」。
type ProjectInitializer interface {
	// PlanInit 算出计划。**一个字节都不写。**
	PlanInit(path string) (InitPlan, error)
	// ApplyInit 照计划执行。中途失败要把**自己建的东西**清掉。
	ApplyInit(plan InitPlan) error
}

// GitRemoteInfo 是 origin 的识别结果。
type GitRemoteInfo struct {
	URL      string
	Host     string
	Slug     string
	IsGitHub bool
}

// RemoteProbe 读出项目的 git remote。
//
// ★ **不碰凭据、不发网络请求**（Q41）：`git remote get-url` 就够了，
// 而用户只是想确认「Duet 认出的是不是我这个仓库」。
type RemoteProbe interface {
	ProbeRemote(ctx context.Context, path string) (GitRemoteInfo, error)
}

// GhInfo 是本机 GitHub CLI 的状态。
type GhInfo struct {
	Status  string
	Version string
	Account string
	Remedy  string
}

// GhDetector 检测本机的 gh。
//
// ★★ Duet **不保管令牌**（Q41）——只报「装了吗、登录了吗」。
type GhDetector interface {
	DetectGh(ctx context.Context) GhInfo
}
