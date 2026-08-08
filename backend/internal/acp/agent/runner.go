package agent

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/acp/runtime"
	"github.com/HuLuca1998/acp-flows/backend/internal/acp/session"
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
	// Policy 是权限裁决策略。留空按 session.PolicyAsk 处理。
	Policy session.Policy
	// AskUser 把需要用户裁决的请求交出去，**阻塞到他做出选择**。
	//
	// ★ 比 session.AskUserFunc 多一个 workID：Broker 要知道这条请求属于
	// 哪个工作，否则事件发不到对的时间线上，用户会在 A 工作里看到
	// B 工作的权限卡片。
	//
	// 为 nil 时会话拿到的也是 nil（不是一个假装有人在接的空函数），
	// 于是一律回 cancelled。
	AskUser AskUserFunc
	// DetectTimeout 留空时用 runtime.DefaultTimeout。
	DetectTimeout time.Duration
	// Log 留空时用 slog.Default()。
	Log *slog.Logger

	// live 记着每个工作正在跑的那一轮，供取消用。
	//
	// ★ 跑完必须摘掉：留着的话，取消一个早就结束的工作会去动一条
	// 已经关掉的会话——轻则报一句看不懂的错，重则卡在那儿等一个
	// 永远不来的收尾。而 app 层拿到错误会把一个成功结束的工作推到 failed。
	mu   sync.Mutex
	live map[string]*liveTurn
}

// liveTurn 是正在跑的一轮：会话与它的进程。
//
// ★ session 可以是 nil——进程起来了但**握手还没完成**时就是这样。
// 那时用户照样能点「停」，而我们必须能杀掉它：一个连 initialize 都不回的
// Agent 正是最该被杀的那种。先 track 进程、会话后补。
type liveTurn struct {
	session *session.Session
	proc    *runtime.Process
}

// AskUserFunc 把权限请求交给用户，阻塞到他做出选择。
//
// ★ **没有超时**：用户可能去泡了杯咖啡；替他超时等于替他做决定。
type AskUserFunc func(ctx context.Context, workID string, ask session.PermissionAsk) (session.Answer, error)

// PermissionFor 组出某一轮的权限配置：把 workID 绑进 AskUser，
// 并把路径缩短成**项目内相对路径**。
//
// 导出是为了能直接验这两件事——拉一个真进程要一整套脚本，
// 而这里要验的只是闭包。
func (r *ProcessRunner) PermissionFor(turn port.AgentTurn) session.Permission {
	perm := session.Permission{Policy: r.Policy}
	if r.AskUser == nil {
		// ★ 留 nil，不塞空函数：空函数返回空 Answer，session 把它当成
		// 「用户没选」→ cancelled。绕一圈结果一样，但中间多了一层
		// 假装有人在接的代码。
		return perm
	}
	ask, cwd := r.AskUser, turn.Cwd
	perm.AskUser = func(ctx context.Context, a session.PermissionAsk) (session.Answer, error) {
		a.Path = shortenPath(cwd, a.Path)
		return ask(ctx, turn.WorkID, a)
	}
	return perm
}

// shortenPath 把 worktree 里的绝对路径缩成项目内相对路径。
//
// ★ 用户关心的是「README.md」，不是
// `/Users/luca/.acpflows/worktrees/work-01/README.md`——后者撑成两行，
// 而且把「worktree 放在哪」这个内部实现摊给了他。真机走查撞到的。
//
// worktree 之外的路径**保持原样**：那时候完整路径才是有信息量的
// （AI 要动 /etc/hosts 的话，用户必须看见这一点）。
func shortenPath(cwd, path string) string {
	if cwd == "" || path == "" || !filepath.IsAbs(path) {
		return path
	}
	rel, err := filepath.Rel(cwd, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return path
	}
	return rel
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

	// ★ **先记下进程再握手。** 反过来的话，一个连 initialize 都不回的
	// Agent 会让 session.Open 挂住，而那时 KillAgent 找不到它——
	// 用户点了停，界面转圈，那个进程一直跑着。
	lt := &liveTurn{proc: proc}
	r.track(turn.WorkID, lt)
	// 跑完就摘。留着的话，取消一个早就结束的工作会去动一条已经关掉的会话。
	defer r.untrack(turn.WorkID)

	s, openErr := session.Open(ctx, session.Options{
		Transport:  stdio{r: proc.Stdout(), w: proc.Stdin()},
		Cwd:        turn.Cwd,
		Permission: r.PermissionFor(turn),
	})
	if openErr != nil {
		return r.wrapAgentError(spec, proc, fmt.Errorf("agent: open session: %w", openErr))
	}
	r.attachSession(turn.WorkID, s)
	defer func() { _ = s.Close() }()

	runErr := RunOn(ctx, s, Spec{
		Permission:   r.PermissionFor(turn),
		Transport:    stdio{r: proc.Stdout(), w: proc.Stdin()},
		Cwd:          turn.Cwd,
		WorkID:       turn.WorkID,
		Prompt:       turn.Prompt,
		SystemPrompt: r.SystemPrompt,
		Sink:         busSink{bus: r.Bus, ctx: ctx, log: log},
		Log:          log,
	})
	return r.wrapAgentError(spec, proc, runErr)
}

// wrapAgentError 把 Agent 的 stderr 带进错误。
//
// ★ 不带的话，用户看到的是「连接断开」，而真正的原因
// （「请先登录」之类）躺在一个没人读的管道里。
func (r *ProcessRunner) wrapAgentError(
	spec runtime.Spec, proc *runtime.Process, err error,
) error {
	if err == nil {
		return nil
	}
	if msg := strings.TrimSpace(proc.Stderr()); msg != "" {
		return fmt.Errorf("%s: %w（它说：%s）", spec.Name, err, lastLines(msg, 5))
	}
	return fmt.Errorf("%s: %w", spec.Name, err)
}

// CancelTurn 实现 port.AgentCanceller：停掉这个工作正在跑的那一轮。
//
// ★ 没在跑时**不报错**：用户点一个本来就该没反应的按钮不该看到吓人的错误，
// 而 app 层拿到错误会把一个已经成功结束的工作推到 failed。
func (r *ProcessRunner) CancelTurn(ctx context.Context, workID string) (bool, error) {
	lt := r.lookup(workID)
	if lt == nil {
		return false, nil
	}
	if lt.session == nil {
		// 进程起来了但握手还没完成。**这种最该杀**：
		// 一个连 initialize 都不回的 Agent 不会响应任何取消。
		return true, fmt.Errorf("agent: 工作 %s 的会话尚未建立", workID)
	}
	err := lt.session.Cancel(ctx)
	// ★ 把「必须杀」翻成一个布尔值交出去：app 层不许 import acp
	// （depguard 挡着），拿不到那边的哨兵错误。
	return session.MustKill(err), err
}

// KillAgent 实现 port.AgentCanceller：杀掉这个工作的 Agent 进程组。
//
// ★ 这是「界面说已取消、后台还在烧钱改文件」的唯一防线。
func (r *ProcessRunner) KillAgent(workID string) {
	lt := r.lookup(workID)
	if lt == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), stopTimeout)
	defer cancel()
	if err := lt.proc.Stop(ctx); err != nil {
		log := r.Log
		if log == nil {
			log = slog.Default()
		}
		log.Warn("强制结束 Agent 进程失败", "work_id", workID, "err", err)
	}
}

func (r *ProcessRunner) track(workID string, lt *liveTurn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.live == nil {
		r.live = make(map[string]*liveTurn)
	}
	r.live[workID] = lt
}

// attachSession 在握手完成后把会话补进去。
func (r *ProcessRunner) attachSession(workID string, s *session.Session) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if lt := r.live[workID]; lt != nil {
		lt.session = s
	}
}

func (r *ProcessRunner) untrack(workID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.live, workID)
}

func (r *ProcessRunner) lookup(workID string) *liveTurn {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.live[workID]
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
