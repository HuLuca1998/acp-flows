// Command duetd 是 Duet 的后端进程。
//
// 它承载全部业务逻辑，对外只暴露 HTTP + SSE。Tauri 壳与浏览器走同一套接口——
// 壳不得通过 IPC 绕过 HTTP，否则 Web 版当天就废（见 docs/architecture.md §1）。
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

	"github.com/HuLuca1998/acp-flows/backend/internal/api"
	"github.com/HuLuca1998/acp-flows/backend/internal/platform"
	"github.com/HuLuca1998/acp-flows/backend/internal/store"
)

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
	)
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: levelFromEnv(),
	})))

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
	defer func() {
		if cerr := db.Close(); cerr != nil {
			slog.Error("关闭数据库失败", "err", cerr)
		}
	}()

	token, err := resolveToken(*dev)
	if err != nil {
		return err
	}

	handler, err := api.NewRouter(api.Config{Token: token, Version: version, Commit: commit})
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

func levelFromEnv() slog.Level {
	if os.Getenv("DUET_LOG_DEBUG") == "1" {
		return slog.LevelDebug
	}
	return slog.LevelInfo
}
