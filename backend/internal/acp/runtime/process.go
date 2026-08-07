package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// DefaultTermGrace 是发出 SIGTERM 后等它自己退出的时间。
//
// 给得太短，肯干净退出的 Agent 就没机会保存现场（写完当前文件、关掉会话）；
// 给得太长，用户点了停止要干等。3 秒是「正常退出绰绰有余、卡住时人还没不耐烦」。
const DefaultTermGrace = 3 * time.Second

// stderrLimit 是 stderr 的保留上限。
//
// 不设限的话，一个疯狂刷日志的 Agent 能把内存吃光——而我们真正需要的
// 只是失败时最后那几行。
const stderrLimit = 64 * 1024

// StartSpec 描述怎么拉起一个子进程。
type StartSpec struct {
	// Bin 是可执行文件路径。
	Bin string
	// Args 是命令行参数。
	Args []string
	// Dir 是工作目录；留空则继承当前进程的。
	Dir string
	// EnvRemove 是必须从子进程环境里删掉的变量。
	//
	// ★ 本项目必踩：Duet 自己就在 Claude Code 里开发，带着 CLAUDECODE
	// 等标记的话 claude-agent-acp 会误判自己跑在另一个 agent 内部而**拒绝服务**
	// （docs/notes/acp-field-notes.md §5 坑 1）。
	EnvRemove []string
	// TermGrace 是 SIGTERM 到 SIGKILL 之间的等待时间；留空用 DefaultTermGrace。
	TermGrace time.Duration
}

// Process 是一个受管的 Agent 子进程。
//
// ★ 它跑在**自己的进程组**里。ACP Runtime 的真实形态是 node 启动器再 fork
// 出实际进程；只杀直接子进程的话，孙进程会变成孤儿继续跑——继续占着 worktree、
// 继续改用户的文件。这与 exec 的 WaitDelay 坑同源：信号只作用于你手上的那个 pid。
type Process struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	grace  time.Duration

	// stderr 由一个后台 goroutine 持续读走。不读的话管道写满，
	// 子进程会阻塞在写 stderr 上——表现成「Agent 卡住了」，而原因毫无提示。
	stderrMu   sync.Mutex
	stderrBuf  bytes.Buffer
	stderrDone chan struct{}

	waitOnce sync.Once
	waitErr  error
	stopOnce sync.Once
}

// Start 拉起子进程。
func Start(ctx context.Context, spec StartSpec) (*Process, error) {
	grace := spec.TermGrace
	if grace == 0 {
		grace = DefaultTermGrace
	}

	cmd := exec.CommandContext(ctx, spec.Bin, spec.Args...)
	cmd.Dir = spec.Dir
	if len(spec.EnvRemove) > 0 {
		cmd.Env = cleanEnv(spec.EnvRemove)
	}
	// ctx 到期时 CommandContext 只 kill 直接子进程，孙进程握着管道能把
	// 超时架空。见 scripts/check/check-naming.sh 第 8 节。
	cmd.WaitDelay = time.Second

	// ★ 自成进程组：Stop 才能用 kill(-pgid) 把整棵进程树一起带走。
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe for %s: %w", spec.Bin, err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe for %s: %w", spec.Bin, err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe for %s: %w", spec.Bin, err)
	}

	if err := cmd.Start(); err != nil {
		// 带上 Bin：只说「no such file」的话，排查时不知道是哪个文件
		return nil, fmt.Errorf("start %s: %w", spec.Bin, err)
	}

	p := &Process{
		cmd: cmd, stdin: stdin, stdout: stdout, grace: grace,
		stderrDone: make(chan struct{}),
	}
	go p.drainStderr(stderr)
	return p, nil
}

// PID 返回子进程的进程号。
func (p *Process) PID() int { return p.cmd.Process.Pid }

// Stdin 是写给 Agent 的一端（ACP 的 JSON-RPC 走这里）。
func (p *Process) Stdin() io.WriteCloser { return p.stdin }

// Stdout 是 Agent 的输出。
func (p *Process) Stdout() io.Reader { return p.stdout }

// Stderr 返回已采集到的 stderr 内容。
func (p *Process) Stderr() string {
	p.stderrMu.Lock()
	defer p.stderrMu.Unlock()
	return p.stderrBuf.String()
}

// Wait 等子进程退出并回收它。
//
// ★ 必须调用，否则进程表里留一条僵尸。一次会话留一条，用户开一天就是几十条。
// 可以重复调用，返回同一个结果。
//
// 失败时把 stderr 附在错误里：只说「exit status 1」等于把唯一的线索丢了，
// 真正有用的是 Agent 在 stderr 上喊的那句话。
func (p *Process) Wait() error {
	p.waitOnce.Do(func() {
		err := p.cmd.Wait()
		<-p.stderrDone // 等 stderr 读完，否则错误里可能少最后几行

		if err != nil {
			if tail := p.Stderr(); tail != "" {
				p.waitErr = fmt.Errorf("%w: %s", err, tail)
				return
			}
			p.waitErr = err
		}
	})
	return p.waitErr
}

// Stop 停掉子进程：先 SIGTERM，超时再 SIGKILL，**都作用于整个进程组**。
//
// 可以重复调用——界面上「停止」按钮被连点两下很常见，第二次报错会让用户
// 以为出了问题。
func (p *Process) Stop(ctx context.Context) error {
	p.stopOnce.Do(func() {
		// 负数 pid = 整个进程组。只发给 p.PID() 的话，node 启动器 fork 出的
		// 实际进程会活下来继续改用户的文件。
		pgid := -p.PID()

		_ = syscall.Kill(pgid, syscall.SIGTERM)

		done := make(chan struct{})
		go func() {
			_ = p.Wait()
			close(done)
		}()

		select {
		case <-done:
			return
		case <-time.After(p.grace):
			// 宽限期到了还没退出——它可能 trap 掉了 TERM，也可能真的卡死。
			// 不升级的话，用户点了退出而进程还在跑，下次启动会撞上
			// 「端口被占」或者两个进程同时改同一个仓库。
			_ = syscall.Kill(pgid, syscall.SIGKILL)
			<-done
		case <-ctx.Done():
			_ = syscall.Kill(pgid, syscall.SIGKILL)
			<-done
		}
	})

	// 正常停止不算错误：SIGTERM/SIGKILL 导致的退出码不该冒泡成故障
	err := p.Wait()
	if isSignalExit(err) {
		return nil
	}
	return err
}

// drainStderr 持续读 stderr，超过上限后丢弃新内容但继续读。
//
// ★ **必须一直读到 EOF**。中途 return 的话管道会写满，子进程阻塞在
// write(2) 上——表现是「Agent 突然不动了」，而没有任何错误信息。
func (p *Process) drainStderr(r io.ReadCloser) {
	defer close(p.stderrDone)

	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			p.stderrMu.Lock()
			if p.stderrBuf.Len() < stderrLimit {
				p.stderrBuf.Write(buf[:n])
			}
			p.stderrMu.Unlock()
		}
		if err != nil {
			return
		}
	}
}

// isSignalExit 判断是不是被信号杀掉的——那是 Stop 的预期结果，不是故障。
func isSignalExit(err error) bool {
	if err == nil {
		return false
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	status, ok := exitErr.Sys().(syscall.WaitStatus)
	return ok && status.Signaled()
}
