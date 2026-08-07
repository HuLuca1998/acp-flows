package runtime

import (
	"context"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
)

// DefaultTimeout 是每条探测子命令的上限。
//
// ★ 别调小。这些命令基本都是 node 启动器，冷启动本身就要几百毫秒；
// macOS 第一次执行还要过代码签名检查。设得太紧的表现是「装好了却显示
// 检测失败」，而且只在别人的机器上出现。
const DefaultTimeout = 10 * time.Second

// Detector 实现 port.RuntimeDetector。
//
// 它是基础设施：app 层只认识接口，不认识这个类型（依赖方向 api → app → domain，
// 基础设施反过来实现 app 定义的 port）。
type Detector struct {
	// Specs 留空时用内置注册表。
	Specs []Spec
	// Timeout 留空时用 DefaultTimeout。
	Timeout time.Duration
}

// DetectAll 检测全部 Runtime。每个都有结论，一个失败不影响其余。
func (d Detector) DetectAll(ctx context.Context) []port.RuntimeStatus {
	specs := d.Specs
	if specs == nil {
		specs = Registered()
	}
	timeout := d.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	results := DetectAll(ctx, specs, timeout)
	out := make([]port.RuntimeStatus, 0, len(results))
	for _, r := range results {
		out = append(out, port.RuntimeStatus{
			Name:    r.Name,
			Status:  string(r.Status),
			Version: r.Version,
			Path:    r.Path,
			Remedy:  r.Remedy,
			Detail:  r.Detail,
		})
	}
	return out
}
