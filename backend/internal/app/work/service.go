// Package work 是「一次工作」的用例：新建、列出、恢复。
//
// 一个工作 = 一个独立 worktree + 一条 ACP 会话 + 一条时间线。
// 这三样必须一起活、一起死——只建了 worktree 而会话没起来的话，
// 用户会看到一个永远停在「正在初始化」的条目，而磁盘上躺着一个没人用的目录。
package work

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/internal/constant"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

const idPrefix = "work"

// View 是交给上层的工作视图。
type View struct {
	ID       string
	State    constant.WorkState
	Project  string
	Worktree string
	Prompt   string
}

// Service 是工作用例。
type Service struct {
	repo      port.WorkRepo
	worktrees port.Worktrees
	bus       port.WorkEventBus
	ids       port.IDGen
	runner    port.AgentRunner
}

// New 组装用例。runner 可以为 nil（只跑 API 冒烟时），那时工作建出来但没人干活。
func New(
	repo port.WorkRepo,
	wt port.Worktrees,
	bus port.WorkEventBus,
	ids port.IDGen,
	runner port.AgentRunner,
) *Service {
	return &Service{repo: repo, worktrees: wt, bus: bus, ids: ids, runner: runner}
}

// Start 新建一个工作：切 worktree → 落库 → 发事件。
//
// ★ 状态**不能一步跳到 clarifying**。worktree 可能切失败，那时用户要看到
// 「初始化失败」而不是「正在澄清需求」——后者会让他对着一个永远等不到回应的
// 界面干等，而真正的原因（不是 git 仓库、磁盘满了）没人告诉他。
//
// ★ 失败的工作**也要落库**。不落的话，用户点了「开始」之后什么都没发生，
// 他不知道是没点上还是失败了。
func (s *Service) Start(ctx context.Context, project, prompt string) (View, error) {
	if !filepath.IsAbs(project) {
		return View{}, fmt.Errorf("%w: %q", model.ErrProjectPathNotAbsolute, project)
	}
	if strings.TrimSpace(prompt) == "" {
		// 没有需求的工作没有意义，而它会占着一个 worktree
		return View{}, fmt.Errorf("work: 需求不能为空")
	}

	id := s.ids.NextID(idPrefix)
	w := model.NewWork(id)
	if err := s.repo.SaveWork(ctx, w); err != nil {
		return View{}, fmt.Errorf("保存工作 %s: %w", id, err)
	}
	s.emit(ctx, id, "state_change", map[string]any{"to": string(w.State())})

	worktree, err := s.worktrees.CreateWorktree(ctx, project, id)
	if err != nil {
		// 切失败进终态。**不可恢复**：worktree 没切成就没有可执行的现场
		// （ADR 0006 Q1），假装能重试只会让用户反复点一个注定失败的按钮。
		if tErr := w.Transition(constant.WorkStateInitializingFailed); tErr == nil {
			_ = s.repo.SaveWork(ctx, w)
			s.emit(ctx, id, "state_change", map[string]any{
				"to": string(constant.WorkStateInitializingFailed), "reason": err.Error(),
			})
		}
		return View{}, fmt.Errorf("为工作 %s 切工作区: %w", id, err)
	}

	if err := w.Transition(constant.WorkStateClarifying); err != nil {
		return View{}, fmt.Errorf("工作 %s 状态迁移: %w", id, err)
	}
	if err := s.repo.SaveWork(ctx, w); err != nil {
		return View{}, fmt.Errorf("保存工作 %s: %w", id, err)
	}
	s.emit(ctx, id, "state_change", map[string]any{"to": string(w.State())})

	// ★ **先把视图取出来，再开后台那一轮。** 反过来的话，
	// 后台 goroutine 可能已经把 w 推到 failed，而这边还在读 w.State()——
	// 两个 goroutine 同时碰一个领域对象，race detector 会红，
	// 而在用户那儿的表现是返回的状态时对时不对。
	view := View{
		ID: id, State: w.State(),
		Project: project, Worktree: worktree, Prompt: prompt,
	}

	s.runTurn(ctx, id, worktree, prompt)

	return view, nil
}

// runTurn 在后台把需求送给 AI，跑完一轮。
//
// ★ **另起 goroutine，且脱开请求的 ctx。** 一轮对话要好几分钟，而
// HTTP 处理函数一返回请求的 ctx 就被取消——挂在上面的话，AI 刚说两句就被砍掉，
// 用户看到的是时间线停在半截、没有任何报错。
//
// 用 WithoutCancel 而不是 context.Background()：它保留了链路上的值
// （日志的 trace id 之类），只是不跟着取消。
func (s *Service) runTurn(ctx context.Context, workID, worktree, prompt string) {
	if s.runner == nil {
		return
	}
	turnCtx := context.WithoutCancel(ctx)

	go func() {
		err := s.runner.RunTurn(turnCtx, port.AgentTurn{
			WorkID: workID,
			// ★ 传的是工作自己的 worktree，不是用户的项目目录——
			// 后者等于让 AI 直接在他的分支上改文件。
			Cwd:    worktree,
			Prompt: prompt,
		})
		if err == nil {
			return
		}

		// ★ 跑挂了要**说出来**。静默的话，用户看到工作停在「正在澄清需求」，
		// 永远等不到下一句，而真正的原因（claude 没装、没登录）没人告诉他。
		s.failWork(turnCtx, workID, err)
	}()
}

// failWork 把工作推到 failed 并把原因发出去。
func (s *Service) failWork(ctx context.Context, workID string, cause error) {
	w, findErr := s.repo.FindWork(ctx, workID)
	if findErr == nil {
		if tErr := w.Transition(constant.WorkStateFailed); tErr == nil {
			_ = s.repo.SaveWork(ctx, w)
		}
	}
	// 落库失败也照样发事件：用户至少要知道出事了，而不是对着「正在澄清需求」干等
	s.emit(ctx, workID, "state_change", map[string]any{
		"to": string(constant.WorkStateFailed), "reason": cause.Error(),
	})
}

// List 列出全部工作。
func (s *Service) List(ctx context.Context) ([]View, error) {
	works, err := s.repo.ListWorks(ctx)
	if err != nil {
		return nil, fmt.Errorf("列出工作: %w", err)
	}

	// 空结果返回空切片而不是 nil：api 层要序列化成 [] 而不是 null
	out := make([]View, 0, len(works))
	for _, w := range works {
		out = append(out, View{ID: w.ID(), State: w.State()})
	}
	return out, nil
}

// emit 发一条事件。
//
// ★ 发失败**不让整个操作失败**：事件是给界面看的，
// 而工作本身已经建好了。因为发不出通知就回滚一个已经成功的工作，
// 用户会更困惑——他的 worktree 明明在那儿。
func (s *Service) emit(ctx context.Context, workID, typ string, payload map[string]any) {
	if s.bus == nil {
		return
	}
	_ = s.bus.PublishWorkEvent(ctx, port.WorkEvent{
		WorkID: workID, Source: "app", Type: typ, Payload: payload,
	})
}
