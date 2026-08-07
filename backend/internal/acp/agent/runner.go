package agent

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/acp/runtime"
	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
)

// stopTimeout 是这一轮结束后收拾 Agent 进程的上限。
//
// ★ 到点就 SIGKILL 整个进程组。留着不管的话，用户每提一个需求就多一个
// 常驻进程，各自握着一个 worktree——关掉应用之后它们还在。
const stopTimeout = 5 * time.Second

// ProcessRunner 拉起一个真的 Agent 进程跑一轮，把事件发到总线。
//
// 它实现 port.AgentRunner。
type ProcessRunner struct {
	// Specs 是候选 Runtime，按优先级排；留空时用内置注册表。
	Specs []runtime.Spec
	// Bus 收事件。
	Bus port.WorkEventBus
	// SystemPrompt 只拼在每条会话的第一轮前面。
	SystemPrompt string
	// DetectTimeout 留空时用 runtime.DefaultTimeout。
	DetectTimeout time.Duration
	// Log 留空时用 slog.Default()。
	Log *slog.Logger
}

// RunTurn 实现 port.AgentRunner：挑一个就绪的 Runtime，拉起来跑一轮。
//
// ★ **阻塞到这一轮结束**（可能好几分钟）。调用方负责另起 goroutine，
// 并且别把它挂在 HTTP 请求的 ctx 上。
func (r *ProcessRunner) RunTurn(ctx context.Context, turn port.AgentTurn) error {
	log := r.Log
	if log == nil {
		log = slog.Default()
	}

	spec, err := r.pick(ctx)
	if err != nil {
		return err
	}

	proc, err := runtime.Start(ctx, runtime.StartSpec{
		// 两个适配器都是专职的 ACP 服务端，不吃额外参数
		Bin: spec.Bin,
		// ★ Agent 就在这个工作自己的 worktree 里干活。
		Dir: turn.Cwd,
		// acp-field-notes §5 坑 1：带着 CLAUDECODE 这些标记，
		// claude-agent-acp 会误判自己跑在另一个 agent 内部而拒绝服务。
		EnvRemove: spec.EnvRemove,
	})
	if err != nil {
		return fmt.Errorf("拉起 %s: %w", spec.Name, err)
	}

	// ★ 无论这一轮怎么收场，进程都要收掉。
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), stopTimeout)
		defer cancel()
		if err := proc.Stop(stopCtx); err != nil {
			log.Warn("收拾 Agent 进程失败", "runtime", spec.Name, "err", err)
		}
	}()

	runErr := Run(ctx, Spec{
		Transport:    stdio{r: proc.Stdout(), w: proc.Stdin()},
		Cwd:          turn.Cwd,
		WorkID:       turn.WorkID,
		Prompt:       turn.Prompt,
		SystemPrompt: r.SystemPrompt,
		Sink:         busSink{bus: r.Bus, ctx: ctx, log: log},
		Log:          log,
	})
	if runErr == nil {
		return nil
	}

	// ★ 把 Agent 的 stderr 带回来。不带的话，用户看到的是「连接断开」，
	// 而真正的原因（「请先登录」之类）躺在一个没人读的管道里。
	if msg := strings.TrimSpace(proc.Stderr()); msg != "" {
		return fmt.Errorf("%s: %w（它说：%s）", spec.Name, runErr, lastLines(msg, 5))
	}
	return fmt.Errorf("%s: %w", spec.Name, runErr)
}

// pick 挑第一个就绪的 Runtime。
//
// ★ 一个都没就绪时，错误里要**带上补救办法**。「connection refused」这种话
// 对用户毫无意义——他需要知道的是「装一下 claude-agent-acp」或者「先登录」。
// 这条错误最终会出现在时间线的失败事件里，是他唯一能看到的线索。
func (r *ProcessRunner) pick(ctx context.Context) (runtime.Spec, error) {
	specs := r.Specs
	if specs == nil {
		specs = runtime.Registered()
	}
	timeout := r.DetectTimeout
	if timeout == 0 {
		timeout = runtime.DefaultTimeout
	}

	var remedies []string
	for _, spec := range specs {
		res := runtime.Detect(ctx, spec, timeout)
		if res.Status == runtime.StatusReady {
			return spec, nil
		}
		remedies = append(remedies, fmt.Sprintf("%s：%s", spec.Name, res.Remedy))
	}
	return runtime.Spec{}, fmt.Errorf(
		"没有可用的 AI Runtime，请先按下面任一条处理——%s", strings.Join(remedies, "；"))
}

// busSink 把事件转发到总线。
type busSink struct {
	bus port.WorkEventBus
	ctx context.Context
	log *slog.Logger
}

// Emit 实现 Sink。
//
// ★ 发不出去只记一条日志，**不让这一轮失败**。事件是给界面看的，
// 而 AI 那边的活已经干了——因为通知发不出去就报错的话，
// 用户看到「失败」而磁盘上躺着一堆已经改好的文件，比不通知更糟。
func (s busSink) Emit(e WorkEvent) {
	if s.bus == nil {
		return
	}
	if err := s.bus.PublishWorkEvent(s.ctx, e); err != nil {
		s.log.Warn("事件发不到总线", "type", e.Type, "work_id", e.WorkID, "err", err)
	}
}

// stdio 把进程的 stdout/stdin 拼成一条双工通道。
//
// Close 只关**写端**：读端由进程退出时自己关掉，
// 这边抢着关会让还没读完的最后几条通知丢掉。
type stdio struct {
	r io.Reader
	w io.WriteCloser
}

func (s stdio) Read(p []byte) (int, error)  { return s.r.Read(p) }
func (s stdio) Write(p []byte) (int, error) { return s.w.Write(p) }
func (s stdio) Close() error                { return s.w.Close() }

// lastLines 取最后 n 行——stderr 可能很长，而有用的信息通常在末尾。
func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, " / ")
}
