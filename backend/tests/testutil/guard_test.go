package testutil_test

// U0.1.2 R3 · 隔离守卫（铁律 6）
//
// 守卫本身必须有测试，否则它可能一直是失效的——而失效的守卫比没有守卫更糟，
// 因为它制造了「测试碰不到真实数据」的错觉。
//
// 验收标准见 docs/milestones/M0-acp-foundation.md § S0.1 U0.1.2。

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/tests/testutil"
)

// R3 · 访问用户真实数据目录时必须失败，且失败信息要指向铁律 6。
func TestGuard_R3_RejectsRealDataDir(t *testing.T) {
	home := testutil.UserHomeForTest(t)

	tests := []struct {
		name string
		path string
	}{
		{"数据目录本身", filepath.Join(home, ".acpflows")},
		{"数据库文件", filepath.Join(home, ".acpflows", "duet.db")},
		{"凭据文件", filepath.Join(home, ".acpflows", "credentials")},
		{"worktree 根目录", filepath.Join(home, ".duet", "worktrees")},
		{"带多余分隔符也要拦住", filepath.Join(home, ".acpflows") + "/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := testutil.CheckPathAllowed(tt.path)
			if err == nil {
				t.Fatalf("守卫放行了真实数据目录: %s", tt.path)
			}
			// 失败信息必须能让人立刻知道违反了哪条规则
			if !strings.Contains(err.Error(), "铁律 6") {
				t.Errorf("守卫的错误信息未指向铁律 6: %q", err)
			}
			if !strings.Contains(err.Error(), tt.path) {
				t.Errorf("守卫的错误信息未包含被拦的路径: %q", err)
			}
		})
	}
}

// 守卫不能误伤：临时目录必须放行，否则所有测试都跑不了。
func TestGuard_AllowsTempDir(t *testing.T) {
	for _, p := range []string{
		t.TempDir(),
		filepath.Join(t.TempDir(), ".acpflows", "duet.db"), // 临时目录下同名也要放行
		filepath.Join(t.TempDir(), "repo", ".git"),
	} {
		if err := testutil.CheckPathAllowed(p); err != nil {
			t.Errorf("守卫误伤了临时路径 %s: %v", p, err)
		}
	}
}

// TempPaths 必须落在临时目录里，且各条路径互不重叠。
func TestTempPaths_IsolatedAndDistinct(t *testing.T) {
	p := testutil.TempPaths(t)

	got := map[string]string{
		"DataDir":         p.DataDir(),
		"DBPath":          p.DBPath(),
		"RuntimeSession":  p.RuntimeSession(),
		"RuntimesDir":     p.RuntimesDir(),
		"CredentialsPath": p.CredentialsPath(),
		"WorktreeRoot":    p.WorktreeRoot(),
	}

	home := testutil.UserHomeForTest(t)
	seen := map[string]string{}
	for name, path := range got {
		if path == "" {
			t.Errorf("%s 返回空路径", name)
			continue
		}
		if !filepath.IsAbs(path) {
			t.Errorf("%s 不是绝对路径: %s", name, path)
		}
		if strings.HasPrefix(path, filepath.Join(home, ".acpflows")) {
			t.Errorf("%s 指向了用户真实数据目录: %s", name, path)
		}
		if err := testutil.CheckPathAllowed(path); err != nil {
			t.Errorf("%s 没通过守卫: %v", name, err)
		}
		if prev, dup := seen[path]; dup {
			t.Errorf("%s 与 %s 路径重复: %s", name, prev, path)
		}
		seen[path] = name
	}

	// 两次调用必须是不同的临时目录，测试之间不能共享状态
	if testutil.TempPaths(t).DataDir() == p.DataDir() {
		t.Error("两次 TempPaths 返回了同一个目录，测试间会互相污染")
	}
}

// 确定性：注入的 Clock 与 IDGen 必须产出可预测的序列。
func TestDeterministicClockAndIDGen(t *testing.T) {
	clk := testutil.FixedClock(testutil.T0)
	if !clk.Now().Equal(testutil.T0) {
		t.Errorf("FixedClock.Now() = %v, want %v", clk.Now(), testutil.T0)
	}
	if !clk.Now().Equal(clk.Now()) {
		t.Error("FixedClock 两次调用返回了不同的时间")
	}

	gen := testutil.SeqIDGen()
	want := []string{"work-001", "work-002", "unit-001"}
	got := []string{gen.NextID("work"), gen.NextID("work"), gen.NextID("unit")}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("第 %d 个 ID: got %q, want %q", i, got[i], want[i])
		}
	}

	// ULID 必须按生成顺序单调递增（事件表依赖它排序）
	a, b := gen.NextULID(), gen.NextULID()
	if a >= b {
		t.Errorf("ULID 未单调递增: %q >= %q", a, b)
	}
}
