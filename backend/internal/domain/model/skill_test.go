package model_test

import (
	"errors"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// 契约来源：docs/spec/domain-model.md §15（INV-SKL-1..10）
//           design/INVENTORY.md §十（Skill 页：版本号 · 命中计数 · 校验态）

// R2 · 校验不过要说清**为什么**（INV-SKL-2）。
//
// ★★ 静默拒绝的后果：用户看到一个 draft 的 skill 却不知道该改什么。
// 他唯一能做的事是删了重建——而重建出来还是 draft。
func TestValidateSkill_R2_ExplainsWhyItFailed(t *testing.T) {
	cases := []struct {
		name       string
		hasSkillMD bool
		fm         model.SkillFrontmatter
		wantOK     bool
		wantIn     string
	}{
		{
			name:       "缺 description",
			hasSkillMD: true,
			fm:         model.SkillFrontmatter{Name: "rust-test-first"},
			wantIn:     "description",
		},
		{
			name:       "缺 name",
			hasSkillMD: true,
			fm:         model.SkillFrontmatter{Description: "先写测试再写实现"},
			wantIn:     "name",
		},
		{
			name:       "两个都缺时一次说全",
			hasSkillMD: true,
			fm:         model.SkillFrontmatter{},
			wantIn:     "name、description",
		},
		{
			name:       "缺 SKILL.md 本身",
			hasSkillMD: false,
			fm:         model.SkillFrontmatter{Name: "x", Description: "y"},
			wantIn:     "SKILL.md",
		},
		{
			name:       "版本号形态不对",
			hasSkillMD: true,
			fm:         model.SkillFrontmatter{Name: "acp-fake-runtime", Description: "造假 Agent", Version: "1.4.2"},
			wantIn:     "主.次",
		},
		{
			name:       "齐全",
			hasSkillMD: true,
			fm:         model.SkillFrontmatter{Name: "rust-test-first", Description: "先写测试再写实现", Version: "2.1"},
			wantOK:     true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := model.ValidateSkill(c.hasSkillMD, c.fm)
			if got.OK != c.wantOK {
				t.Fatalf("OK = %v，想要 %v（原因：%s）", got.OK, c.wantOK, got.Reason)
			}
			if c.wantOK {
				if got.Reason != "" {
					t.Errorf("通过了却带着原因 %q", got.Reason)
				}
				return
			}
			if got.Reason == "" {
				t.Fatal("没通过却不说为什么——用户不知道该改什么（INV-SKL-2）")
			}
			if !contains(got.Reason, c.wantIn) {
				t.Errorf("原因 = %q，应含 %q", got.Reason, c.wantIn)
			}
		})
	}
}

// ★ 缺 SKILL.md 时**不能**报成「缺 description」。
//
// 顺序错了的话，一个根本没有 SKILL.md 的目录会把人引向改 frontmatter，
// 而那个文件压根不存在。
func TestValidateSkill_R2b_MissingFileTakesPrecedence(t *testing.T) {
	got := model.ValidateSkill(false, model.SkillFrontmatter{})
	if contains(got.Reason, "description") {
		t.Errorf("没有 SKILL.md 却报成 %q——会把人引向改一个不存在的文件", got.Reason)
	}
	if !contains(got.Reason, "SKILL.md") {
		t.Errorf("原因 = %q，应该指出缺的是 SKILL.md", got.Reason)
	}
}

// R4 · 版本号可比较。
func TestSkillVersion_R4_Compare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"2.1", "1.4", 1},
		{"1.4", "2.1", -1},
		{"2.1", "2.1", 0},
		{"0.4", "0.9", -1},
		// ★ 按段比较，不是按字符串比：字符串比法下 "2.10" < "2.9"
		{"2.10", "2.9", 1},
		{"v2.1", "2.1", 0}, // v 前缀可选
	}

	for _, c := range cases {
		a, err := model.ParseSkillVersion(c.a)
		if err != nil {
			t.Fatalf("解析 %q: %v", c.a, err)
		}
		b, err := model.ParseSkillVersion(c.b)
		if err != nil {
			t.Fatalf("解析 %q: %v", c.b, err)
		}
		if got := a.Compare(b); got != c.want {
			t.Errorf("%s 比 %s = %d，想要 %d", c.a, c.b, got, c.want)
		}
	}
}

func TestParseSkillVersion_RejectsMalformed(t *testing.T) {
	bad := []string{
		"",      // 空
		"2",     // 只有一段
		"1.4.2", // 三段——那是应用版本的形态，不是 Skill 的
		"a.b",   // 不是数字
		"2.",    // 次版本为空
		".1",    // 主版本为空
		"01.4",  // ★ 前导零：01 和 1 会被排成两个版本，而界面上长得几乎一样
		"-1.0",  // 负数
	}
	for _, s := range bad {
		if v, err := model.ParseSkillVersion(s); !errors.Is(err, model.ErrInvalidSkillVersion) {
			t.Errorf("%q 解出了 %v，想要 ErrInvalidSkillVersion（错误 = %v）", s, v, err)
		}
	}
}

func TestSkillVersion_StringRoundTrip(t *testing.T) {
	for _, s := range []string{"2.1", "0.4", "12.30"} {
		v, err := model.ParseSkillVersion(s)
		if err != nil {
			t.Fatalf("解析 %q: %v", s, err)
		}
		if got := v.String(); got != s {
			t.Errorf("%q 转回来是 %q", s, got)
		}
	}
}

// 状态是封闭枚举，三态（INV-SKL-1/5）。
func TestSkillStatus_ClosedEnum(t *testing.T) {
	all := model.AllSkillStatuses()
	if len(all) != 3 {
		t.Fatalf("状态 %d 个，§15.3 是 draft / active / deprecated 三个", len(all))
	}
	for _, s := range all {
		if !s.IsValid() {
			t.Errorf("%q 在全集里却判定为非法", s)
		}
	}
	if model.SkillStatus("published").IsValid() {
		t.Error("「published」被判成合法——发布后的状态叫 active")
	}
	if model.SkillStatus("").IsValid() {
		t.Error("空串被判成合法")
	}
}

func TestAllSkillStatuses_ReturnsCopy(t *testing.T) {
	first := model.AllSkillStatuses()
	first[0] = "篡改"
	if second := model.AllSkillStatuses(); second[0] != model.SkillDraft {
		t.Errorf("改了返回值之后再取 = %q", second[0])
	}
}
