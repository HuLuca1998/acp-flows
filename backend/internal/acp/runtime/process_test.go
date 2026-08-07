package runtime_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/HuLuca1998/acp-flows/backend/internal/acp/runtime"
)

// U2.2.1 · 子进程生命周期（验收点 V5 的前置）
//
// ★ 这里跑的是**真进程**：真 fork、真信号、真退出码。
// 把 os/exec 换成接口再塞个假实现的话，"SIGTERM 超时后会不会 SIGKILL"
// 这个问题就永远测不出来——而那正是这个单元存在的理由。

// script 在临时目录里造一个可执行的 shell 脚本，返回它的路径。
func script(t *testing.T, name, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatalf("造脚本失败: %v", err)
	}
	return path
}

// waitForFile 等一个文件出现。
//
// ★ 用它而不是 sleep 一个猜出来的时长。**进程刚 Start 时还没执行到脚本的
// 第一行**——立刻 Stop 的话，SIGTERM 打在一个还没设好 trap 的进程上，
// 测的就不是「忽略 TERM 之后会不会升级到 KILL」，而是「刚起来的进程能不能被杀」。
// 第一版就是这么写的，Stop 只用了 93µs 就返回，看起来像实现没等宽限期。
func waitForFile(t *testing.T, path string) {
	t.Helper()
	for range 100 {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("等不到 %s——子进程没跑到那一步，这个用例什么也没测到", path)
}

// alive 报告这个 pid 还在不在（且不是僵尸）。
//
// signal 0 只做权限与存在性检查，不真的发信号——**但僵尸进程也会返回 nil**，
// 所以还要看 ps 的状态位。R4 要的正是「回收掉了」而不只是「杀掉了」。
func alive(t *testing.T, pid int) bool {
	t.Helper()
	out, err := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return false // ps 找不到这个 pid
	}
	stat := strings.TrimSpace(string(out))
	if stat == "" {
		return false
	}
	// Z 开头是僵尸：进程没了但父进程还没 Wait，表项还占着
	return !strings.HasPrefix(stat, "Z")
}

// R1：spawn 前清除嵌套会话环境变量。
//
// ★ 本项目**必踩**：Duet 自己就在 Claude Code 里开发，不清这些变量
// claude-agent-acp 会误判自己跑在另一个 agent 内部而拒绝服务
// （docs/notes/acp-field-notes.md §5 坑 1）。
func TestProcess_ClearsNestedSessionEnv(t *testing.T) {
	t.Setenv("CLAUDECODE", "1")
	t.Setenv("CLAUDE_CODE_ENTRYPOINT", "cli")
	t.Setenv("KEEP_ME", "yes")

	out := filepath.Join(t.TempDir(), "env.txt")
	// 写完就退出，然后 Wait——用 Stop 的话会在它写完之前就把它杀了
	bin := script(t, "dump", `env > `+out)

	p, err := runtime.Start(context.Background(), runtime.StartSpec{
		Bin:       bin,
		EnvRemove: []string{"CLAUDECODE", "CLAUDE_CODE_ENTRYPOINT"},
	})
	if err != nil {
		t.Fatalf("启动失败: %v", err)
	}
	if err := p.Wait(); err != nil {
		t.Fatalf("子进程失败: %v", err)
	}

	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("子进程没写出环境: %v", err)
	}
	env := string(raw)

	for _, banned := range []string{"CLAUDECODE=", "CLAUDE_CODE_ENTRYPOINT="} {
		if strings.Contains(env, banned) {
			t.Errorf("子进程环境里还有 %s——claude-agent-acp 会误判嵌套并拒绝服务", banned)
		}
	}
	// 只删该删的：把整个环境清空会让子进程找不到 PATH、HOME
	if !strings.Contains(env, "KEEP_ME=yes") {
		t.Error("把不该删的环境变量也删了")
	}
}

// R2：stderr 被完整采集，报错时带出来。
//
// 子进程失败时只说「exit status 1」，等于把唯一的线索丢了——
// 真正有用的是它在 stderr 上喊的那句话。
func TestProcess_CapturesStderr(t *testing.T) {
	bin := script(t, "noisy", `echo "cannot connect to model provider" >&2
exit 3`)

	p, err := runtime.Start(context.Background(), runtime.StartSpec{Bin: bin})
	if err != nil {
		t.Fatalf("启动失败: %v", err)
	}

	err = p.Wait()
	if err == nil {
		t.Fatal("子进程 exit 3 却没报错")
	}
	if !strings.Contains(err.Error(), "cannot connect to model provider") {
		t.Errorf("错误信息里没有 stderr 的内容，排查时就没线索了：%v", err)
	}
	if !strings.Contains(p.Stderr(), "cannot connect") {
		t.Errorf("Stderr() = %q", p.Stderr())
	}
}

// R3：关闭先 SIGTERM，超时再 SIGKILL。
//
// 用一个**忽略 SIGTERM** 的假进程——真实的 Agent 在处理请求时也可能不理会
// 第一次礼貌请求。只发 SIGTERM 就撒手的话，用户点了退出而进程还在跑，
// 下次启动会撞上「端口被占」或者两个进程同时改同一个仓库。
func TestProcess_EscalatesToKill(t *testing.T) {
	ready := filepath.Join(t.TempDir(), "ready")
	bin := script(t, "stubborn", `trap '' TERM
touch `+ready+`
while true; do sleep 0.05; done`)

	p, err := runtime.Start(context.Background(), runtime.StartSpec{
		Bin:       bin,
		TermGrace: 200 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("启动失败: %v", err)
	}
	pid := p.PID()
	waitForFile(t, ready) // trap 设好之前 SIGTERM 照样能杀死它

	start := time.Now()
	if err := p.Stop(context.Background()); err != nil {
		t.Fatalf("停止失败: %v", err)
	}
	elapsed := time.Since(start)

	if alive(t, pid) {
		t.Error("进程还活着——SIGTERM 被忽略后没有升级到 SIGKILL")
	}
	// 至少等过一次宽限期才升级：立刻 KILL 的话，肯干净退出的进程
	// 就没机会保存现场了
	if elapsed < 150*time.Millisecond {
		t.Errorf("只用了 %v 就杀掉了，没给 SIGTERM 留够时间", elapsed)
	}
}

// ★ 孙进程也要一起走。
//
// ACP Runtime 的真实形态是 node 启动器再 fork 出实际进程。只杀直接子进程的话，
// 孙进程会变成孤儿继续跑——继续占着 worktree、继续改用户的文件。
// 这和 WaitDelay 那个坑同源：kill 只作用于直接子进程。
func TestProcess_KillsGrandchildren(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "grandchild.pid")
	bin := script(t, "forker", `
sh -c 'while true; do sleep 0.05; done' &
echo $! > `+pidFile+`
trap '' TERM
while true; do sleep 0.05; done`)

	p, err := runtime.Start(context.Background(), runtime.StartSpec{
		Bin:       bin,
		TermGrace: 200 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("启动失败: %v", err)
	}

	// 等孙进程把自己的 pid 写出来
	var grandchild int
	for range 40 {
		if raw, err := os.ReadFile(pidFile); err == nil {
			if n, convErr := strconv.Atoi(strings.TrimSpace(string(raw))); convErr == nil && n > 0 {
				grandchild = n
				break
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	if grandchild == 0 {
		t.Fatal("孙进程没起来，这个用例什么也没测到")
	}

	if err := p.Stop(context.Background()); err != nil {
		t.Fatalf("停止失败: %v", err)
	}

	// 给内核一点时间收尸
	for range 20 {
		if !alive(t, grandchild) {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Errorf("孙进程 %d 还活着——它会继续占着 worktree、继续改用户的文件", grandchild)
}

// R4：停掉之后不留僵尸。
//
// Go 里子进程退出后必须 Wait 才会被回收，否则进程表里一直挂着一条 Z。
// 一次会话留一条，用户开一天就是几十条。
func TestProcess_LeavesNoZombie(t *testing.T) {
	bin := script(t, "quick", `exit 0`)

	p, err := runtime.Start(context.Background(), runtime.StartSpec{Bin: bin})
	if err != nil {
		t.Fatalf("启动失败: %v", err)
	}
	pid := p.PID()

	if err := p.Wait(); err != nil {
		t.Fatalf("等待失败: %v", err)
	}

	if alive(t, pid) {
		t.Errorf("pid %d 还在（或是僵尸）——Wait 没有回收它", pid)
	}
}

// Stop 可以重复调用。界面上「停止」按钮被连点两下是很常见的，
// 第二次报错会让用户以为出了问题。
func TestProcess_StopIsIdempotent(t *testing.T) {
	ready := filepath.Join(t.TempDir(), "ready")
	bin := script(t, "idle", `touch `+ready+`
while true; do sleep 0.05; done`)

	p, err := runtime.Start(context.Background(), runtime.StartSpec{
		Bin:       bin,
		TermGrace: 200 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("启动失败: %v", err)
	}
	waitForFile(t, ready)

	if err := p.Stop(context.Background()); err != nil {
		t.Fatalf("第一次停止失败: %v", err)
	}
	if err := p.Stop(context.Background()); err != nil {
		t.Errorf("第二次停止报错了：%v", err)
	}
}

// 可执行文件不存在时，错误信息要指出是哪个文件。
func TestProcess_StartFailureNamesTheBinary(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope")

	_, err := runtime.Start(context.Background(), runtime.StartSpec{Bin: missing})
	if err == nil {
		t.Fatal("可执行文件不存在却启动成功了")
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("错误信息没说是哪个文件：%v", err)
	}
}
