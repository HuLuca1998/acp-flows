package model_test

// M1 · 一键更新的地基：版本比较
//
// 这段逻辑决定「有没有新版本」。判错的后果不是报错，是**用户永远收不到更新**
// 或者**被反复提示更新到旧版**——两者都不会有任何日志告诉你出事了。
//
// 版本形态由 docs/adr/0007-release-revision-from-prior-art.md 修订 1、2 决定：
//   正式版   1.4.2
//   快照版   0.0.0-snapshot.20260807.a1b2c3d   （prerelease，只给手动下载的人）
// 仓库里的 version 字段**始终是 0.0.0**，发布时用 jq 注入。

import (
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// ★ 最容易错的一条：按字符串比，"1.10.0" < "1.9.0"，用户从此收不到更新。
func TestParseVersion_ComparesNumericallyNotLexically(t *testing.T) {
	older := mustParse(t, "1.9.0")
	newer := mustParse(t, "1.10.0")

	if !newer.After(older) {
		t.Errorf("1.10.0 必须比 1.9.0 新——按字符串比会判反，用户从此收不到更新")
	}
	if older.After(newer) {
		t.Error("1.9.0 不该比 1.10.0 新")
	}
}

func TestParseVersion_Ordering(t *testing.T) {
	tests := []struct {
		name  string
		left  string
		right string
		// want 为 true 表示 left 比 right 新。
		want bool
	}{
		{"主版本优先", "2.0.0", "1.99.99", true},
		{"次版本次之", "1.5.0", "1.4.99", true},
		{"修订号最后", "1.4.3", "1.4.2", true},
		{"相等不算新", "1.4.2", "1.4.2", false},
		{"两位数次版本", "1.10.0", "1.9.0", true},
		{"两位数修订号", "1.4.10", "1.4.9", true},
		// 预发布版**低于**同号正式版：0.0.0-snapshot 是开发快照，
		// 判反的话正式版用户会被劝降级到快照。
		{"快照低于正式版", "0.0.0", "0.0.0-snapshot.20260807.a1b2c3d", true},
		{"正式版高于快照", "1.4.2-rc.1", "1.4.2", false},
		{"两个快照按标识符比", "0.0.0-snapshot.20260808.aaa", "0.0.0-snapshot.20260807.zzz", true},
		{"v 前缀等价", "v1.4.2", "1.4.1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			left := mustParse(t, tt.left)
			right := mustParse(t, tt.right)
			if got := left.After(right); got != tt.want {
				t.Errorf("%s After %s: want %v, got %v", tt.left, tt.right, tt.want, got)
			}
		})
	}
}

// 非法版本必须报错，不能静默当成 0.0.0。
//
// 静默降级的后果：latest.json 里的版本号写错一个字符，
// 所有客户端都会认为「远端是 0.0.0」，于是**永远提示不更新**——
// 而这个故障没有任何症状，直到有人发现半年没收到更新。
func TestParseVersion_RejectsMalformed(t *testing.T) {
	bad := []string{
		"",
		"1.4",     // 缺修订号
		"1.4.2.3", // 多一段
		"1.4.x",   // 非数字
		"latest",  // 语义化版本之外的标签
		"1.-4.2",  // 负数
		"01.4.2",  // 前导零（语义化版本明确禁止）
		" 1.4.2",  // 带空白
		"1.4.2-",  // 空的预发布标识
	}

	for _, s := range bad {
		t.Run("拒绝_"+s, func(t *testing.T) {
			if _, err := model.ParseVersion(s); err == nil {
				t.Errorf("%q 是非法版本，必须返回错误而不是静默当成零值", s)
			}
		})
	}
}

// String 要能还原，latest.json 与界面展示都靠它。
func TestVersion_StringRoundTrip(t *testing.T) {
	for _, s := range []string{"1.4.2", "0.0.0", "0.0.0-snapshot.20260807.a1b2c3d"} {
		v := mustParse(t, s)
		if got := v.String(); got != s {
			t.Errorf("round-trip: want %q, got %q", s, got)
		}
	}
	// v 前缀是 tag 的写法，解析时接受，输出时**不带** ——
	// 界面上「v1.4.2」的 v 由前端加，后端只给纯版本号。
	v := mustParse(t, "v1.4.2")
	if got := v.String(); got != "1.4.2" {
		t.Errorf("v 前缀应被规整掉: want %q, got %q", "1.4.2", got)
	}
}

// IsSnapshot 用来在界面上把开发快照和正式版区分开。
func TestVersion_IsSnapshot(t *testing.T) {
	if !mustParse(t, "0.0.0-snapshot.20260807.a1b2c3d").IsSnapshot() {
		t.Error("带 snapshot 预发布标识的应判为快照")
	}
	if mustParse(t, "1.4.2").IsSnapshot() {
		t.Error("正式版不是快照")
	}
	// 仓库里的占位版本：还没发布过任何版本时 duetd 报的就是它。
	if mustParse(t, "0.0.0").IsSnapshot() {
		t.Error("0.0.0 是占位版本，不是快照——两者在界面上的文案不同")
	}
}

func mustParse(t *testing.T, s string) model.Version {
	t.Helper()
	v, err := model.ParseVersion(s)
	if err != nil {
		t.Fatalf("解析 %q 失败: %v", s, err)
	}
	return v
}
