package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/HuLuca1998/acp-flows/backend/internal/acp/runtime"
	"github.com/HuLuca1998/acp-flows/backend/internal/acp/session"
)

// sessionKey 唯一确定一条常驻会话。
//
// ★★ **工作 + 角色**，两者缺一不可：
//
//   - 只按工作：需求分析师和实现工程师会共用一条会话，
//     而前者是只读的、后者能写——那等于把写权限漏给了只读角色
//   - 只按角色：两个工作会共用一条会话，A 项目的上下文会串到 B 项目
type sessionKey struct {
	workID string
	roleID string
}

// liveSession 是一条活着的会话及其进程。
type liveSession struct {
	proc *runtime.Process
	sess *session.Session
	// spec 记着它是哪个 Runtime 起的，收拾时要用。
	spec runtime.Spec
}

// sessionPool 按「工作 + 角色」维持长连接。
//
// ★★ 这个池存在的唯一理由是 **Q42**：
// 「和用户沟通的 AI 只能是同一个会话……不要每次用户发送消息就启动一个新的会话。」
//
// 每轮拉一个新进程的后果是实打实的：用户「补充一句」时 AI **不记得上一句**
// （上下文每轮清零），每轮重付一次启动与上下文成本，
// 而「一次对话」在协议上本该是**一条**会话。
type sessionPool struct {
	mu    sync.Mutex
	items map[sessionKey]*liveSession
}

func newSessionPool() *sessionPool {
	return &sessionPool{items: map[sessionKey]*liveSession{}}
}

// get 取一条已有的会话；没有则返回 nil。
func (p *sessionPool) get(k sessionKey) *liveSession {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.items[k]
}

// put 记下一条会话。
func (p *sessionPool) put(k sessionKey, ls *liveSession) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.items[k] = ls
}

// drop 摘掉并返回一条会话，供调用方收拾。
func (p *sessionPool) drop(k sessionKey) *liveSession {
	p.mu.Lock()
	defer p.mu.Unlock()
	ls := p.items[k]
	delete(p.items, k)
	return ls
}

// dropWork 摘掉一个工作的**全部**会话。
//
// ★ 工作结束或暂停时调用：留着的话，一个 paused 的工作会一直占着
// 两三个 Agent 进程，而用户以为它已经停了。
func (p *sessionPool) dropWork(workID string) []*liveSession {
	p.mu.Lock()
	defer p.mu.Unlock()

	var out []*liveSession
	for k, ls := range p.items {
		if k.workID == workID {
			out = append(out, ls)
			delete(p.items, k)
		}
	}
	return out
}

// sessionsOf 返回一个工作现有的会话数，供测试与诊断用。
func (p *sessionPool) sessionsOf(workID string) int {
	p.mu.Lock()
	defer p.mu.Unlock()

	var n int
	for k := range p.items {
		if k.workID == workID {
			n++
		}
	}
	return n
}

// closeAll 收拾一组会话。**不返回错误**：这是清理路径，
// 报错也没人能做什么，记一条日志就够了。
func closeAll(ctx context.Context, log *slog.Logger, sessions []*liveSession) {
	if log == nil {
		log = slog.Default()
	}
	for _, ls := range sessions {
		closeOne(ctx, log, ls)
	}
}

func closeOne(ctx context.Context, log *slog.Logger, ls *liveSession) {
	if ls == nil {
		return
	}
	if ls.sess != nil {
		if err := ls.sess.Close(); err != nil {
			log.Warn("关会话失败", "runtime", ls.spec.Name, "err", err)
		}
	}
	if ls.proc == nil {
		return
	}
	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), stopTimeout)
	defer cancel()
	if err := ls.proc.Stop(stopCtx); err != nil {
		log.Warn("收拾 Agent 进程失败", "runtime", ls.spec.Name, "err", err)
	}
}

// openSession 拉起进程并开一条**已收权**的会话。
//
// ★★ 收权在这里一次做完，且在任何 prompt 之前——中间不留窗口。
//
// ★★ `onProcess` 在进程起来之后、**握手之前**立刻调用。
//
// 这个回调不是为了好看：一个连 `initialize` 都不回的 Agent 会让
// `session.Open` 一直挂着，而那时如果调用方还没记下这个进程，
// `KillAgent` 就找不到它——用户点了停，界面转圈，进程一直跑着。
// 「先记下进程再握手」是硬顺序，有测试守着。
func openSession(
	ctx context.Context, spec runtime.Spec, cwd, roleID string,
	perm session.Permission, onProcess func(*runtime.Process),
) (*liveSession, error) {
	modeID, err := modeIDFor(roleID, spec.Name)
	if err != nil {
		// ★ 算不出档名就**根本不拉进程**：拉起来再失败的话，
		// 那个进程要靠 defer 收，而收拾路径上的失败没人看得到。
		return nil, err
	}

	proc, err := runtime.Start(ctx, runtime.StartSpec{
		Bin: spec.Bin,
		// ★ Agent 就在这个工作自己的 worktree 里干活。
		Dir: cwd,
		// acp-field-notes §5 坑 1：带着 CLAUDECODE 这些标记，
		// claude-agent-acp 会误判自己跑在另一个 agent 内部而拒绝服务。
		EnvRemove: spec.EnvRemove,
	})
	if err != nil {
		return nil, fmt.Errorf("拉起 %s: %w", spec.Name, err)
	}

	// ★★ 握手之前先把进程交出去——见函数注释。
	if onProcess != nil {
		onProcess(proc)
	}

	sess, err := session.Open(ctx, session.Options{
		Transport:      stdio{r: proc.Stdout(), w: proc.Stdin()},
		Cwd:            cwd,
		Permission:     perm,
		RequiredModeID: modeID,
	})
	if err != nil {
		// ★★ **把 Agent 的 stderr 带进错误**，在收拾进程之前读。
		//
		// 不带的话，用户看到的是「对方已断开」，而真正的原因
		// （「请先登录」「版本不兼容」）躺在一个没人读的管道里。
		// 开会话失败恰恰是最需要这条信息的时刻——那时还没有任何事件流。
		wrapped := fmt.Errorf("%s: agent: open session: %w", spec.Name, err)
		if msg := strings.TrimSpace(proc.Stderr()); msg != "" {
			wrapped = fmt.Errorf("%s: agent: open session: %w（它说：%s）",
				spec.Name, err, lastLines(msg, 5))
		}

		// 开不出会话就把进程收掉——留着的话它会一直挂在那儿。
		stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), stopTimeout)
		defer cancel()
		_ = proc.Stop(stopCtx)
		return nil, wrapped
	}

	return &liveSession{proc: proc, sess: sess, spec: spec}, nil
}
