package work

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// ErrNotAcceptingMessages 表示这个工作已经不听了（终态）。
//
// ★★ 明确报错而不是静默收下：静默的话用户对着一个永远不动的时间线干等，
// 以为 AI 在想事情——而实际上那句话根本没人接。
var ErrNotAcceptingMessages = errors.New("work: 这个工作已经结束，不再接收消息")

// Say 在一个**已有**的工作里接着说一句。
//
// ★★ 这是「连着说三句，AI 记得前两句」的那条路（Q42）。
// 会话按「工作 + 角色」常驻，所以第二句进的是上一句的上下文。
//
// ★ 不走这里而是再 `Start` 一次的话，用户说第二句时开的是一个**新工作**：
// 新 worktree、新会话、新时间线——前一句彻底不在上下文里，
// 而他以为自己只是补充了一句。
func (s *Service) Say(ctx context.Context, workID, text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		// 空话不发。发出去的话 Agent 会为一句空白跑一整轮
		return fmt.Errorf("work: 消息不能为空")
	}

	w, err := s.repo.FindWork(ctx, workID)
	if err != nil {
		return fmt.Errorf("查工作 %s: %w", workID, err)
	}

	// ★ 先问规则再动手：终态的工作连事件都不该发。
	// 发了的话，时间线上会留下一句「用户说了什么」而永远没有下文。
	if model.IsTerminal(w.State()) {
		return fmt.Errorf("%w: 工作 %s 处于 %s", ErrNotAcceptingMessages, workID, w.State())
	}

	// worktree 还没切好就没有可执行的现场。
	//
	// ★ 这不是终态（工作可能正在初始化），所以单独判——
	// 用 Cwd 为空去跑一轮的话，Agent 会在**进程的当前目录**里干活，
	// 而那是 duetd 自己的目录。
	if w.WorktreePath() == "" {
		return fmt.Errorf("%w: 工作 %s 的工作区还没准备好", ErrNotAcceptingMessages, workID)
	}

	// ★ 用户那句话先进时间线，再跑。反过来的话，AI 的回复可能
	// 排在他自己的问题前面——时间线是按发出顺序排的。
	s.emit(ctx, workID, "user_message", map[string]any{"text": text})
	// ★ 记进需求快照再跑：这一轮产出的事件要盖上更新后的版本号。
	// 冻结过的版本会因此出一个 v(n+1)——冻结之后又提新要求，那就是改需求。
	s.recordSaid(ctx, workID, text)
	s.runTurn(ctx, workID, w.WorktreePath(), text, w.State())
	return nil
}
