package model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrInvalidVersion 表示版本号不符合语义化版本。
//
// ★ 解析失败必须报错，**绝不静默当成 0.0.0**：
// latest.json 里的版本号写错一个字符时，静默降级会让所有客户端认为
// 「远端是 0.0.0」，于是永远提示不更新——而这个故障没有任何症状。
var ErrInvalidVersion = errors.New("model: 非法的版本号")

// Version 是语义化版本。
//
// 本项目只出现两种形态（docs/adr/0007 修订 1、2）：
//
//	正式版   1.4.2
//	快照版   0.0.0-snapshot.20260807.a1b2c3d   （prerelease，只给手动下载的人）
//
// 仓库里的 version 字段**始终是 0.0.0**，发布时用 jq 注入。
type Version struct {
	Major int
	Minor int
	Patch int
	// PreRelease 是 `-` 之后的部分，空串表示正式版。
	PreRelease string
}

// ParseVersion 解析语义化版本。接受可选的 v 前缀（tag 的写法）。
func ParseVersion(s string) (Version, error) {
	// 不做 TrimSpace：带空白的版本号说明上游拼错了，
	// 容忍它只会让错误延后到更难查的地方。
	raw := strings.TrimPrefix(s, "v")

	core, pre, hasPre := strings.Cut(raw, "-")
	if hasPre && pre == "" {
		return Version{}, fmt.Errorf("%w: %q 的预发布标识为空", ErrInvalidVersion, s)
	}

	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("%w: %q 应为 主.次.修订 三段", ErrInvalidVersion, s)
	}

	nums := make([]int, 3)
	for i, p := range parts {
		n, err := parseVersionNumber(p)
		if err != nil {
			return Version{}, fmt.Errorf("%w: %q 的第 %d 段: %w", ErrInvalidVersion, s, i+1, err)
		}
		nums[i] = n
	}

	return Version{Major: nums[0], Minor: nums[1], Patch: nums[2], PreRelease: pre}, nil
}

// parseVersionNumber 解析一段版本号。
//
// 拒绝前导零（语义化版本明确禁止）与负数：`01.4.2` 与 `1.-4.2` 都是上游拼错的信号。
func parseVersionNumber(p string) (int, error) {
	if p == "" {
		return 0, errors.New("为空")
	}
	if len(p) > 1 && p[0] == '0' {
		return 0, fmt.Errorf("%q 有前导零", p)
	}
	n, err := strconv.Atoi(p)
	if err != nil {
		return 0, fmt.Errorf("%q 不是数字", p)
	}
	if n < 0 {
		return 0, fmt.Errorf("%q 是负数", p)
	}
	return n, nil
}

// After 报告 v 是否比 other 新。
//
// ★ 三段**按数值比**，不是按字符串比：字符串比会得出 "1.10.0" < "1.9.0"，
// 用户从此收不到更新，且没有任何报错。
//
// 预发布版**低于**同号正式版（语义化版本规定）：
// 判反的话正式版用户会被劝降级到开发快照。
func (v Version) After(other Version) bool {
	if v.Major != other.Major {
		return v.Major > other.Major
	}
	if v.Minor != other.Minor {
		return v.Minor > other.Minor
	}
	if v.Patch != other.Patch {
		return v.Patch > other.Patch
	}

	switch {
	case v.PreRelease == "" && other.PreRelease == "":
		return false // 完全相同
	case v.PreRelease == "":
		return true // 正式版 > 预发布版
	case other.PreRelease == "":
		return false // 预发布版 < 正式版
	default:
		// 两个预发布版按标识符字典序。本项目的快照标识是
		// snapshot.<日期>.<短 sha>，日期在前，字典序即时间序。
		return v.PreRelease > other.PreRelease
	}
}

// IsSnapshot 报告这是不是一个开发快照。
//
// 界面上快照与正式版的文案不同；`0.0.0`（仓库里的占位版本，还没发布过任何版本时
// duetd 报的就是它）**不算**快照。
func (v Version) IsSnapshot() bool {
	return strings.HasPrefix(v.PreRelease, "snapshot")
}

// String 还原成字符串。**不带 v 前缀** —— 界面上的 v 由前端加。
func (v Version) String() string {
	core := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.PreRelease == "" {
		return core
	}
	return core + "-" + v.PreRelease
}
