package model_test

import (
	"errors"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// M7 U7.1.1 · 契约的写入边界
//
// ★★ 这是契约真正的产物：没有它，「AI 要动文件」只能靠用户逐条判断；
// 有了它，越界的那一次会被标出来——而用户要的正是
// 「它有没有动不该动的东西」。

// ★★ R1 · 边界按**路径段**比，不是字符串前缀。
//
// `internal/acp/` 不该匹配 `internal/acpx/foo.go`——按字符串比的话它会
// 匹配上，而那是一个用户从没同意过的目录。
func TestWriteBoundary_R1_MatchesByPathSegment(t *testing.T) {
	b := model.WriteBoundary{Allowed: []string{"internal/acp/"}}

	for _, tc := range []struct {
		path string
		want model.BoundaryVerdict
	}{
		{"internal/acp/session.go", model.BoundaryInside},
		{"internal/acp/agent/runner.go", model.BoundaryInside},
		{"internal/acp", model.BoundaryInside}, // 目录本身
		// ★★ 这一条是重点：只差一个字母的**另一个目录**
		{"internal/acpx/foo.go", model.BoundaryOutside},
		{"internal/api/works.go", model.BoundaryOutside},
	} {
		if got := b.Judge(tc.path); got != tc.want {
			t.Errorf("Judge(%q) = %q，想要 %q", tc.path, got, tc.want)
		}
	}
}

// ★★ R2 · 禁止项**压过**允许项。
//
// 反过来的话，「允许 internal/、禁止 internal/api/schema.go」
// 会让那个文件被放行——而它正是被单独拎出来禁止的那一个。
func TestWriteBoundary_R2_ForbiddenWins(t *testing.T) {
	b := model.WriteBoundary{
		Allowed:   []string{"internal/"},
		Forbidden: []string{"internal/api/gen/"},
	}

	if got := b.Judge("internal/api/works.go"); got != model.BoundaryInside {
		t.Errorf("允许范围内的文件被判成了 %q", got)
	}
	if got := b.Judge("internal/api/gen/api.gen.go"); got != model.BoundaryOutside {
		t.Errorf("被单独禁止的文件却放行了：%q——"+
			"「允许整个包但别碰生成物」是最常见的边界形状", got)
	}
}

// ★★ R3 · 边界为空 → `unknown`，**不是** `in_boundary`。
//
// 把「不知道」当成「没问题」，等于在最该提醒的时候保持沉默：
// 契约没冻结时用户正好最需要看清楚 AI 要动什么。
func TestWriteBoundary_R3_EmptyIsUnknownNotAllowed(t *testing.T) {
	var empty model.WriteBoundary

	if got := empty.Judge("anything.go"); got != model.BoundaryUnknown {
		t.Errorf("空边界判成了 %q——把「不知道」当成「没问题」，"+
			"等于在最该提醒的时候保持沉默", got)
	}
}

// ★ 有边界但没命中允许项 = 越界。「没说可以」就是不可以。
func TestWriteBoundary_UnlistedIsOutside(t *testing.T) {
	b := model.WriteBoundary{Allowed: []string{"internal/acp/"}}

	if got := b.Judge("README.md"); got != model.BoundaryOutside {
		t.Errorf("没说可以的路径判成了 %q——安全默认是拒绝，不是放行", got)
	}
}

// ★★ `..` 要折叠：不折叠的话 `internal/acp/../../etc/passwd`
// 会被判成「在 internal/acp/ 里」。
func TestWriteBoundary_FoldsDotDot(t *testing.T) {
	b := model.WriteBoundary{Allowed: []string{"internal/acp/"}}

	if got := b.Judge("internal/acp/../../etc/passwd"); got != model.BoundaryOutside {
		t.Errorf("穿出去的路径判成了 %q——那是一次逃逸，不是边界内的写入", got)
	}
}

// 首尾的 `/` 与 `./` 不影响判定——AI 给的路径形态我们说了不算。
func TestWriteBoundary_NormalizesPaths(t *testing.T) {
	b := model.WriteBoundary{Allowed: []string{"/internal/acp/"}}

	for _, p := range []string{"internal/acp/x.go", "./internal/acp/x.go", "/internal/acp/x.go"} {
		if got := b.Judge(p); got != model.BoundaryInside {
			t.Errorf("Judge(%q) = %q，路径形态不该影响判定", p, got)
		}
	}
}

// ★ R5 · 不看文件存不存在：AI 要**新建**文件时那个路径当然还不存在，
// 而那正是最需要判边界的时刻。
func TestWriteBoundary_R5_DoesNotNeedTheFileToExist(t *testing.T) {
	b := model.WriteBoundary{Allowed: []string{"internal/acp/"}}

	if got := b.Judge("internal/acp/a-file-that-does-not-exist.go"); got != model.BoundaryInside {
		t.Errorf("还不存在的文件判成了 %q——新建文件正是最需要判边界的时刻", got)
	}
}

// ★★ R4 · 冻结后边界不可变。
func TestUnitContract_R4_BoundaryIsFrozenToo(t *testing.T) {
	c := model.NewUnitContract("unit-012", 1)
	if err := c.SetBoundary(model.WriteBoundary{Allowed: []string{"internal/acp/"}}); err != nil {
		t.Fatal(err)
	}
	if err := c.AddCriterion("ac-1", "取消必须幂等"); err != nil {
		t.Fatal(err)
	}
	if err := c.Freeze(); err != nil {
		t.Fatal(err)
	}

	err := c.SetBoundary(model.WriteBoundary{Allowed: []string{"/"}})
	if !errors.Is(err, model.ErrContractFrozen) {
		t.Fatalf("冻结后还能改边界：%v——那等于边界随时可以被放宽到全放行", err)
	}
	// ★ 判据：边界一个字没变
	if got := c.Judge("README.md"); got != model.BoundaryOutside {
		t.Errorf("边界被改了：README.md 判成了 %q", got)
	}
}

// 边界返回的是副本，改它不影响契约。
func TestUnitContract_BoundaryReturnsCopy(t *testing.T) {
	c := model.NewUnitContract("unit-012", 1)
	if err := c.SetBoundary(model.WriteBoundary{Allowed: []string{"internal/acp/"}}); err != nil {
		t.Fatal(err)
	}

	b := c.Boundary()
	b.Allowed[0] = "/"
	if c.Judge("README.md") == model.BoundaryInside {
		t.Error("Boundary() 返回了内部切片——调用方能把边界改成全放行")
	}
}

// ★ 修订出的新版本**带着边界**，且是副本。
//
// 不带的话，v2 一出来就是「什么都不许改」——而用户以为只是改了一条标准。
func TestUnitContract_ReviseCarriesTheBoundary(t *testing.T) {
	c := model.NewUnitContract("unit-012", 1)
	if err := c.SetBoundary(model.WriteBoundary{Allowed: []string{"internal/acp/"}}); err != nil {
		t.Fatal(err)
	}

	next, err := c.Revise(2)
	if err != nil {
		t.Fatal(err)
	}
	if got := next.Judge("internal/acp/x.go"); got != model.BoundaryInside {
		t.Errorf("新版本的边界 = %q——v2 一出来就什么都不许改，"+
			"而用户以为只是改了一条标准", got)
	}

	// 改新版本的边界不影响旧的
	if err := next.SetBoundary(model.WriteBoundary{Allowed: []string{"docs/"}}); err != nil {
		t.Fatal(err)
	}
	if got := c.Judge("internal/acp/x.go"); got != model.BoundaryInside {
		t.Errorf("改新版本动到了旧版本：%q", got)
	}
}
