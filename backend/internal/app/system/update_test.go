package system_test

// M1 · 一键更新：检查与准备
//
// 验收标准见 docs/plan/milestones/M1-install-and-update.md S1.1（更新界面与流程）。
//
// ★ 这一层的失败模式是**静默**的：判错了不报错，只是用户永远收不到更新，
// 或者更新时丢掉正在跑的工作。所以每条错误路径都要有断言。

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/internal/app/system"
	"github.com/HuLuca1998/acp-flows/backend/internal/constant"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// fakeSource 是手写的 ReleaseSource（backend/AGENTS.md：app 层用手写 fake，不用 mock 框架）。
type fakeSource struct {
	release port.Release
	err     error
	calls   int
}

func (f *fakeSource) Latest(context.Context) (port.Release, error) {
	f.calls++
	return f.release, f.err
}

// fakeWorks 是手写的 WorkLister。
type fakeWorks struct {
	works []*model.Work
	err   error
}

func (f *fakeWorks) ListWorks(context.Context) ([]*model.Work, error) {
	return f.works, f.err
}

const currentVersion = "1.4.2"

func newSvc(t *testing.T, src port.ReleaseSource, works port.WorkLister, updaterAvailable bool) *system.UpdateService {
	t.Helper()
	svc, err := system.NewUpdateService(system.UpdateConfig{
		CurrentVersion:   currentVersion,
		UpdaterAvailable: updaterAvailable,
		Source:           src,
		Works:            works,
	})
	if err != nil {
		t.Fatalf("构造用例失败: %v", err)
	}
	return svc
}

// 远端更新时报 available，并把用户做决定需要的信息都带上。
func TestCheck_ReportsAvailableWithDetails(t *testing.T) {
	published := time.Date(2026, 8, 7, 9, 0, 0, 0, time.UTC)
	src := &fakeSource{release: port.Release{
		Version:     "1.5.0",
		Notes:       "修复取消超时后 Runtime 仍在改文件",
		SizeBytes:   18_432_000,
		PublishedAt: published,
	}}

	got, err := newSvc(t, src, &fakeWorks{}, true).Check(context.Background())
	if err != nil {
		t.Fatalf("Check 失败: %v", err)
	}

	if got.State != system.UpdateStateAvailable {
		t.Errorf("state: want %q, got %q", system.UpdateStateAvailable, got.State)
	}
	if got.CurrentVersion != currentVersion {
		t.Errorf("current_version: want %q, got %q", currentVersion, got.CurrentVersion)
	}
	if got.LatestVersion != "1.5.0" {
		t.Errorf("latest_version: want %q, got %q", "1.5.0", got.LatestVersion)
	}
	// 更新说明与体积是用户「现在更不更」的判断依据，丢了等于让他盲点
	if got.Notes != "修复取消超时后 Runtime 仍在改文件" {
		t.Errorf("notes 丢了: %q", got.Notes)
	}
	if got.SizeBytes != 18_432_000 {
		t.Errorf("size_bytes: want %d, got %d", 18_432_000, got.SizeBytes)
	}
	if !got.PublishedAt.Equal(published) {
		t.Errorf("published_at: want %v, got %v", published, got.PublishedAt)
	}
}

// 相同或更旧一律 idle。
//
// ★ 「更旧也提示更新」是回滚发布后的真实故障：用户会被反复劝着装回旧版。
func TestCheck_SameOrOlderIsIdle(t *testing.T) {
	tests := []struct {
		name   string
		remote string
	}{
		{"版本相同", currentVersion},
		{"远端更旧", "1.4.1"},
		{"远端主版本更旧", "0.9.9"},
		{"远端是同号快照（低于正式版）", "1.4.2-snapshot.20260807.a1b2c3d"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := &fakeSource{release: port.Release{Version: tt.remote}}
			got, err := newSvc(t, src, &fakeWorks{}, true).Check(context.Background())
			if err != nil {
				t.Fatalf("Check 失败: %v", err)
			}
			if got.State != system.UpdateStateIdle {
				t.Errorf("远端 %s 对本地 %s：want idle, got %q", tt.remote, currentVersion, got.State)
			}
			if got.LatestVersion != "" {
				t.Errorf("没有可用更新时不该报 latest_version，got %q", got.LatestVersion)
			}
		})
	}
}

// Web 版报 unsupported，并且**根本不查发布源**。
//
// 浏览器里没有 updater，查了也没用；省掉的不只是一次网络请求——
// 更是「提示了更新却点不动」这种把用户卡死的界面。
func TestCheck_WebBuildIsUnsupportedAndSkipsNetwork(t *testing.T) {
	src := &fakeSource{release: port.Release{Version: "9.9.9"}}

	got, err := newSvc(t, src, &fakeWorks{}, false).Check(context.Background())
	if err != nil {
		t.Fatalf("Check 失败: %v", err)
	}

	if got.State != system.UpdateStateUnsupported {
		t.Errorf("state: want %q, got %q", system.UpdateStateUnsupported, got.State)
	}
	if src.calls != 0 {
		t.Errorf("Web 版不该查发布源，实际查了 %d 次", src.calls)
	}
	// 当前版本仍要报——设置页照样要显示「你在用哪个版本」
	if got.CurrentVersion != currentVersion {
		t.Errorf("current_version: want %q, got %q", currentVersion, got.CurrentVersion)
	}
}

// 发布源出错必须往上报，不能静默当成 idle。
//
// 静默的后果：网络断了 / GitHub 挂了 / URL 写错了，界面全都显示「已是最新版本」。
// 用户不会去查，只会以为没更新——这是最难发现的一类故障。
func TestCheck_SourceFailurePropagates(t *testing.T) {
	wantErr := errors.New("dial tcp: lookup api.github.com: no such host")
	src := &fakeSource{err: wantErr}

	_, err := newSvc(t, src, &fakeWorks{}, true).Check(context.Background())
	if err == nil {
		t.Fatal("发布源出错时必须返回错误，绝不能静默报「已是最新版本」")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("错误应可追溯到根因: got %v", err)
	}
}

// 远端版本号非法也必须报错。
//
// latest.json 里写错一个字符时，静默当成 0.0.0 会让所有客户端永远收不到更新。
func TestCheck_MalformedRemoteVersionIsAnError(t *testing.T) {
	src := &fakeSource{release: port.Release{Version: "latest"}}

	_, err := newSvc(t, src, &fakeWorks{}, true).Check(context.Background())
	if err == nil {
		t.Fatal("远端版本号非法必须报错，不能静默当成 0.0.0")
	}
	if !errors.Is(err, model.ErrInvalidVersion) {
		t.Errorf("应能辨识出是版本号非法: got %v", err)
	}
}

// 没有进行中的工作时放行。
func TestPrepare_NoActiveWorkIsReady(t *testing.T) {
	got, err := newSvc(t, &fakeSource{}, &fakeWorks{}, true).Prepare(context.Background())
	if err != nil {
		t.Fatalf("Prepare 失败: %v", err)
	}
	if got.Status != system.PrepareReady {
		t.Errorf("status: want %q, got %q", system.PrepareReady, got.Status)
	}
	if len(got.Blocked) != 0 {
		t.Errorf("没有工作时不该有 blocked 项: %+v", got.Blocked)
	}
}

// ★ 失败安全：只要有一个非终态工作就 blocked，绝不放行。
//
// M1 里程碑写死了这条：「先放行、以后再补暂停逻辑」会在中间那段时间里
// 真实地丢掉用户几十分钟的工作，而「更新不丢工作」正是这个产品能被信任的前提。
// 完整的「暂停 + 落检查点」要等 M3 的两段式取消与 M4 的 U4.1.2。
func TestPrepare_AnyActiveWorkBlocks(t *testing.T) {
	active := []constant.WorkState{
		constant.WorkStateExecuting,
		constant.WorkStateClarifying,
		constant.WorkStatePlanning,
		constant.WorkStateReviewingUnit,
		constant.WorkStateWaitingUser,
		constant.WorkStatePaused,
	}

	for _, state := range active {
		t.Run(string(state), func(t *testing.T) {
			works := &fakeWorks{works: []*model.Work{workInState(t, "work-08", state)}}

			got, err := newSvc(t, &fakeSource{}, works, true).Prepare(context.Background())
			if err != nil {
				t.Fatalf("Prepare 失败: %v", err)
			}
			if got.Status != system.PrepareBlocked {
				t.Fatalf("状态 %s 的工作必须挡住更新: want blocked, got %q", state, got.Status)
			}
			if len(got.Blocked) != 1 {
				t.Fatalf("want 1 个 blocked 项, got %d", len(got.Blocked))
			}
			if got.Blocked[0].WorkID != "work-08" {
				t.Errorf("blocked work_id: want %q, got %q", "work-08", got.Blocked[0].WorkID)
			}
			// reason 是机器可读码，前端据此查 i18n 词条（docs/rules/i18n.md §3）
			if got.Blocked[0].Reason != system.ReasonWorkInProgress {
				t.Errorf("reason: want %q, got %q", system.ReasonWorkInProgress, got.Blocked[0].Reason)
			}
		})
	}
}

// 终态工作不挡更新。
func TestPrepare_TerminalWorkDoesNotBlock(t *testing.T) {
	works := &fakeWorks{works: []*model.Work{
		workInState(t, "work-01", constant.WorkStateCompleted),
		workInState(t, "work-02", constant.WorkStateFailed),
		workInState(t, "work-03", constant.WorkStateInitializingFailed),
	}}

	got, err := newSvc(t, &fakeSource{}, works, true).Prepare(context.Background())
	if err != nil {
		t.Fatalf("Prepare 失败: %v", err)
	}
	if got.Status != system.PrepareReady {
		t.Errorf("已结束的工作不该挡住更新: got %q, blocked=%+v", got.Status, got.Blocked)
	}
}

// 查不到工作列表时**按 blocked 处理**，不是按 ready。
//
// 数据库读失败时放行，等于在「我不知道有没有工作在跑」的情况下重启应用。
// 失败安全的方向是拒绝，不是放行。
func TestPrepare_ListFailureBlocksInsteadOfAllowing(t *testing.T) {
	works := &fakeWorks{err: errors.New("database is locked")}

	got, err := newSvc(t, &fakeSource{}, works, true).Prepare(context.Background())
	if err == nil && got.Status == system.PrepareReady {
		t.Fatal("查不到工作列表时绝不能放行——那是在不知情的情况下重启用户的应用")
	}
}

// 构造用例时缺依赖要立刻失败，而不是运行时 panic。
func TestNewUpdateService_RejectsMissingDeps(t *testing.T) {
	_, err := system.NewUpdateService(system.UpdateConfig{
		CurrentVersion: "", UpdaterAvailable: true,
		Source: &fakeSource{}, Works: &fakeWorks{},
	})
	if err == nil {
		t.Error("当前版本号为空时必须拒绝构造——那会让所有比较都失去意义")
	}
}

func workInState(t *testing.T, id string, state constant.WorkState) *model.Work {
	t.Helper()
	return model.NewWorkAt(id, state)
}
