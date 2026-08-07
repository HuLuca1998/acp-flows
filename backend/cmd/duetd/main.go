// Command duetd 是 Duet 的后端进程。
//
// 它承载全部业务逻辑，对外只暴露 HTTP + SSE。Tauri 壳与浏览器走同一套接口——
// 壳不得通过 IPC 绕过 HTTP，否则 Web 版当天就废（见 docs/spec/architecture.md §1）。
//
// 本文件是全仓库**唯一做依赖装配**的地方：手工 new，一眼能看出谁依赖谁。
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/acp/agent"
	"github.com/HuLuca1998/acp-flows/backend/internal/acp/runtime"
	"github.com/HuLuca1998/acp-flows/backend/internal/api"
	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/internal/app/project"
	"github.com/HuLuca1998/acp-flows/backend/internal/app/system"
	"github.com/HuLuca1998/acp-flows/backend/internal/app/work"
	"github.com/HuLuca1998/acp-flows/backend/internal/eventbus"
	"github.com/HuLuca1998/acp-flows/backend/internal/gitx"
	"github.com/HuLuca1998/acp-flows/backend/internal/platform"
	"github.com/HuLuca1998/acp-flows/backend/internal/platform/logging"
	"github.com/HuLuca1998/acp-flows/backend/internal/release"
	"github.com/HuLuca1998/acp-flows/backend/internal/store"
)

// defaultReleaseManifest 是发布源上的 latest.json。
//
// ★ 必须与 shell/src-tauri/tauri.conf.json 的 plugins.updater.endpoints 一致：
// 分成两个真源的话，界面说「有更新」而 updater 说「没有」，
// 用户会点一个永远不动的按钮。
const defaultReleaseManifest = "https://github.com/HuLuca1998/acp-flows/releases/latest/download/latest.json"

// 构建时注入：go build -ldflags "-X main.version=1.5.0 -X main.commit=abc1234"
var (
	version = "0.0.0-dev"
	commit  = "unknown"
)

const (
	shutdownGrace = 10 * time.Second
	devPort       = 7777
)

func main() {
	if err := run(); err != nil {
		slog.Error("duetd 退出", "err", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		port = flag.Int("port", 0, "监听端口，0 表示随机分配")
		dev  = flag.Bool("dev", false, "开发模式：固定端口 7777，token 取自 DUET_DEV_TOKEN")
		// updater 由 Tauri 壳在拉起 sidecar 时带上。纯 Web 形态下不带，
		// 检查更新会直接返回 unsupported —— 浏览器里没有 updater，
		// 提示了更新却点不动会把用户卡在没有出路的界面上。
		updaterAvailable   = flag.Bool("updater", false, "本进程由 Tauri 壳拉起，具备自动更新能力")
		releaseManifestURL = flag.String("release-manifest", defaultReleaseManifest, "发布源 latest.json 的地址")
	)
	flag.Parse()

	// 日志级别按域可调：DUET_LOG="info,acp=trace,store=debug"
	// 一次调试通常只关心一个组件，全局调 debug 会淹没在噪音里（docs/rules/logging.md §3）。
	globalLevel, compLevels, err := logging.ParseLevels(os.Getenv("DUET_LOG"))
	if err != nil {
		return fmt.Errorf("DUET_LOG: %w", err)
	}

	// ── 装配顺序即依赖方向的证明 ─────────────────────────────
	// platform → store → (app) → api。反向依赖一律拒绝。

	paths, err := resolvePaths(*dev)
	if err != nil {
		return err
	}
	if err := paths.EnsureDirs(); err != nil {
		return err
	}

	clk := platform.NewClock()

	db, err := store.Open(paths.DBPath(), clk)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}

	// 日志双去处：stderr 给人看，SQLite 给 AI 查（docs/rules/logging.md §1）。
	// 落库必须在 store 之后装配 —— 它要往 logs 表写。
	sink := db.NewLogSink()
	slog.SetDefault(slog.New(logging.NewHandler(logging.Options{
		Stderr:          os.Stderr,
		Sink:            sink,
		GlobalLevel:     globalLevel,
		ComponentLevels: compLevels,
	})))
	defer func() {
		// 先冲刷日志再关数据库 —— 顺序反了会丢掉关闭过程的日志
		_ = sink.Close()
	}()
	defer func() {
		if cerr := db.Close(); cerr != nil {
			slog.Error("关闭数据库失败", "err", cerr)
		}
	}()

	token, err := resolveToken(*dev)
	if err != nil {
		return err
	}

	// 更新链路。发布源与 Tauri updater 查**同一个 latest.json**——
	// 两个真源会让界面说「有更新」而 updater 说「没有」。
	//
	// UpdaterAvailable 由 --updater 决定：Tauri 壳拉起 sidecar 时带上它。
	// 纯 Web 形态下为 false，检查更新直接返回 unsupported 且不发网络请求。
	updateSvc, err := system.NewUpdateService(system.UpdateConfig{
		CurrentVersion:   version,
		UpdaterAvailable: *updaterAvailable,
		Source:           release.NewHTTPSource(*releaseManifestURL, nil),
		Works:            db.Works(),
	})
	if err != nil {
		return fmt.Errorf("build update service: %w", err)
	}

	// ★ 预热 ID 序号。IDGen 的计数在内存里，进程一重启就归零——
	// 不回填的话，一个已经有 proj-01 的库重启后会再发一次 proj-01，
	// 用户重启应用后第一次添加项目就撞主键。
	// 这个坑在开发机上撞不到（数据库总是空的），只会在用户那儿炸。
	ids := platform.NewIDGen(clk)
	maxSeq, err := db.Projects().MaxProjectSeq(context.Background())
	if err != nil {
		return fmt.Errorf("prime project id seq: %w", err)
	}
	ids.PrimeSeq("proj", maxSeq)

	projectSvc := project.New(db.Projects(), gitProbe{}, ids)
	bus := eventbus.New(eventStore{db.Events()})
	// ★ Agent 真的会被拉起来。这里传的是**内置注册表**（claude / codex）：
	// 用哪一个由检测结果决定，上层不认识任何品牌名。
	agentRunner := &agent.ProcessRunner{Bus: workBus{bus}}
	workSvc := work.New(
		db.Works(), worktrees{root: paths.WorktreeRoot()}, workBus{bus}, ids, agentRunner)

	handler, err := api.NewRouter(api.Config{
		Token:   token,
		Version: version,
		Commit:  commit,
		Update:  updateSvc,
		// 环境检测用内置注册表。**不缓存**：用户装完 codex 回来刷新一下
		// 就该看到变化，而缓存的表现是「照着提示装好了，界面还是说没装」。
		Runtimes: runtime.Detector{},
		Projects: projectSvc,
		// 事件流：bus 负责扇出，EventHistory 负责断线重连补发。
		// 两者接的是同一张 events 表——bus 写、history 读。
		Bus:          bus,
		EventHistory: eventStore{db.Events()},
		Works:        workSvc,
	})
	if err != nil {
		return fmt.Errorf("build router: %w", err)
	}

	// ── 只监听回环 ───────────────────────────────────────────
	// 端口 0 表示让内核分配；开发模式固定 7777 便于 vite 代理。
	listenPort := *port
	if listenPort == 0 && *dev {
		listenPort = devPort
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", listenPort))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	actual := ln.Addr().(*net.TCPAddr).Port

	if err := writeSession(paths.RuntimeSession(), actual, token); err != nil {
		return err
	}
	defer func() { _ = os.Remove(paths.RuntimeSession()) }()

	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	slog.Info("duetd 已启动",
		"addr", fmt.Sprintf("http://127.0.0.1:%d", actual),
		"version", version,
		"data_dir", paths.DataDir(),
		"dev", *dev)

	// ── 优雅关闭 ─────────────────────────────────────────────
	// SIGTERM 是 Tauri 关闭 sidecar 时发的，不能漏。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		if serveErr := srv.Serve(ln); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errCh <- serveErr
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
		slog.Info("收到退出信号，正在优雅关闭")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	// TODO(M0 U0.2.2): 这里还要优雅关闭全部 ACP Runtime 子进程，
	// 漏了会留下僵尸进程。见 backend/internal/acp/AGENTS.md。
	slog.Info("duetd 已退出")
	return nil
}

// resolvePaths 决定数据目录。
//
// 开发模式落在 ~/.duet-dev，与用户真实数据隔离——
// AI 自测时不能碰 ~/.acpflows（铁律 6）。
func resolvePaths(dev bool) (*platform.OSPaths, error) {
	if root := os.Getenv("DUET_DATA_DIR"); root != "" {
		return platform.NewPathsAt(root), nil
	}
	if dev {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve home dir: %w", err)
		}
		return platform.NewPathsAt(filepath.Join(home, ".duet-dev")), nil
	}
	return platform.NewPaths()
}

// resolveToken 生成或读取一次性 bearer token。
func resolveToken(dev bool) (string, error) {
	if dev {
		if t := os.Getenv("DUET_DEV_TOKEN"); t != "" {
			return t, nil
		}
		return "dev-local-token", nil
	}
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

// writeSession 把端口与 token 写给 Tauri 壳读。
//
// 权限 0600：同机其他用户不该读到它——拿到 token 就能驱动 Agent 写用户的代码。
func writeSession(path string, port int, token string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create runtime dir: %w", err)
	}
	body, err := json.Marshal(map[string]any{
		"port":  port,
		"token": token,
		"pid":   os.Getpid(),
	})
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		return fmt.Errorf("write session file: %w", err)
	}
	return nil
}

// gitProbe 把 gitx 接到 app/port 上。
//
// cmd 是唯一做依赖装配的地方，所以这层薄适配器放这里——
// 让 gitx 直接返回 port 的类型会让基础设施依赖 app 的数据结构。
type gitProbe struct{}

func (gitProbe) ProbeGit(ctx context.Context, path string) (port.GitInfo, error) {
	info, err := gitx.Probe(ctx, path)
	if errors.Is(err, gitx.ErrNotADirectory) {
		// 基础设施的错误类型不许穿到 app/api——翻成契约里的哨兵，
		// 让界面能说出「这个文件夹找不到」而不是一句通用错误
		return port.GitInfo{}, fmt.Errorf("%w: %s", port.ErrPathNotFound, path)
	}
	return port.GitInfo{IsRepo: info.IsRepo, DefaultBranch: info.DefaultBranch}, err
}

// eventStore 把 store 的事件仓储接到 eventbus 与 api 上。
//
// ★ 为什么需要这层翻译：store.Event 与 eventbus.Event 字段完全一致，
// 但**具名结构体之间不能互相赋值**——Go 的结构化类型只对 interface 生效。
// 而 depguard 的 infra 规则不许基础设施之间互相 import（store 不能认识
// eventbus，反之亦然），所以接缝只能落在 cmd —— 唯一做装配的地方。
type eventStore struct{ repo *store.EventRepo }

func (s eventStore) AppendEvent(ctx context.Context, e *eventbus.Event) error {
	row := &store.Event{
		ID: e.ID, WorkID: e.WorkID, Source: e.Source,
		Type: e.Type, TS: e.TS, Payload: e.Payload,
	}
	if err := s.repo.AppendEvent(ctx, row); err != nil {
		return err
	}
	e.Seq = row.Seq // 序号由数据库发放，写回给调用方
	return nil
}

func (s eventStore) MaxSeq(ctx context.Context) (int64, error) {
	return s.repo.MaxSeq(ctx)
}

func (s eventStore) EventsAfter(ctx context.Context, after int64, limit int) ([]eventbus.Event, error) {
	rows, err := s.repo.EventsAfter(ctx, after, limit)
	if err != nil {
		return nil, err
	}
	out := make([]eventbus.Event, 0, len(rows))
	for _, r := range rows {
		out = append(out, eventbus.Event{
			ID: r.ID, Seq: r.Seq, WorkID: r.WorkID,
			Source: r.Source, Type: r.Type, TS: r.TS, Payload: r.Payload,
		})
	}
	return out, nil
}

// worktrees 把 gitx 的 worktree 操作接到 app/port 上。
//
// ★ root 是 `~/.acpflows/worktrees`——**用户项目之外**（open-questions Q30）。
// 这一层存在的意义就是把那个路径决定钉死在装配处，
// 不让 app 层自己去拼路径（拼错了就写进用户仓库了）。
type worktrees struct{ root string }

func (w worktrees) CreateWorktree(ctx context.Context, repo, workID string) (string, error) {
	wt, err := gitx.AddWorktree(ctx, gitx.WorktreeSpec{
		Repo: repo, Root: w.root, WorkID: workID, Branch: "duet/" + workID,
	})
	return wt.Path, err
}

func (w worktrees) RemoveWorktree(ctx context.Context, repo, path string) error {
	return gitx.RemoveWorktree(ctx, repo, path)
}

// workBus 把 app 层的工作事件接到事件总线上。
//
// 两个 Event 类型字段一致但不能互相赋值（结构化类型只对 interface 生效），
// 而 depguard 不许 app 与 eventbus 互相依赖——接缝落在 cmd。
type workBus struct{ bus *eventbus.Bus }

func (b workBus) PublishWorkEvent(ctx context.Context, e port.WorkEvent) error {
	payload, err := json.Marshal(e.Payload)
	if err != nil {
		// 载荷编不出来不该让工作本身失败——事件是给界面看的
		payload = []byte("{}")
	}
	return b.bus.Publish(ctx, eventbus.Event{
		ID: "evt_" + e.WorkID, WorkID: e.WorkID,
		Source: e.Source, Type: e.Type, Payload: payload,
	})
}
