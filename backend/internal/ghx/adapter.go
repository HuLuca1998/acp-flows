package ghx

import (
	"context"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
)

// Detector 实现 port.GhDetector。
type Detector struct{}

// DetectGh 检测本机的 gh。★★ 结果里不含任何令牌（Q41）。
func (Detector) DetectGh(ctx context.Context) port.GhInfo {
	r := Detect(ctx, nil)
	return port.GhInfo{
		Status:  string(r.Status),
		Version: r.Version,
		Account: r.Account,
		Remedy:  r.Remedy,
	}
}
