package runtime_test

import (
	"context"
	"testing"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/acp/runtime"
)

// 适配层把 Result 翻译成 port.RuntimeStatus。这层唯一会出的错就是**丢字段**——
// 丢了 Remedy，用户就看不到该敲什么命令；丢了 Version，界面显示空白。
// 编译器不会管这个：少写一行赋值，零值照样合法。
func TestDetectorKeepsEveryField(t *testing.T) {
	dir, _ := binDir(t)
	fakeBin(t, dir, "codex", "case \"$1\" in\n"+
		"  --version) echo 'codex-cli 1.1.7' ;;\n"+
		"  login)     echo 'Not logged in' >&2; exit 1 ;;\n"+
		"esac\n")

	got := runtime.Detector{Specs: []runtime.Spec{codexSpec()}, Timeout: 5 * time.Second}.
		DetectAll(context.Background())

	if len(got) != 1 {
		t.Fatalf("返回 %d 条，想要 1 条", len(got))
	}
	r := got[0]
	if r.Name != "codex" {
		t.Errorf("Name = %q", r.Name)
	}
	if r.Status != string(runtime.StatusNotAuthenticated) {
		t.Errorf("Status = %q, 想要 not_authenticated", r.Status)
	}
	if r.Version != "1.1.7" {
		t.Errorf("Version = %q, 想要 1.1.7", r.Version)
	}
	if r.Remedy != "codex login" {
		t.Errorf("Remedy = %q, 想要 codex login——丢了它用户就不知道该敲什么", r.Remedy)
	}
	if r.Path == "" {
		t.Error("Path 是空的，排查时没有线索")
	}
	if r.Detail == "" {
		t.Error("Detail 是空的，失败原因就此丢失")
	}
}

// 零值 Detector 用内置注册表和默认超时——duetd 里就是这么构造的。
//
// PATH 被清空，所以结果必然是「都没装」：这样断言不依赖跑测试的机器上
// 到底装了什么，同时又真的走了「Specs 为 nil 时取 Registered()」这条分支。
func TestDetectorZeroValueUsesRegistry(t *testing.T) {
	binDir(t)

	got := runtime.Detector{}.DetectAll(context.Background())

	if len(got) != len(runtime.Registered()) {
		t.Fatalf("返回 %d 条，注册表里有 %d 条", len(got), len(runtime.Registered()))
	}
	for _, r := range got {
		if r.Status != string(runtime.StatusNotInstalled) {
			t.Errorf("%s: Status = %q, 空 PATH 下想要 not_installed", r.Name, r.Status)
		}
		if r.Remedy == "" {
			t.Errorf("%s: 没给安装命令——「未安装」却不说怎么装，等于没说", r.Name)
		}
	}
}
