// Package work 是「一次工作」的用例：新建、列出、恢复。
//
// 一个工作 = 一个独立 worktree + 一条 ACP 会话 + 一条时间线。
// 这三样必须一起活、一起死——只建了 worktree 而会话没起来的话，
// 用户会看到一个永远停在「正在初始化」的条目，而磁盘上躺着一个没人用的目录。
package work

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

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
	// Branch 与 BaseCommit 是这个工作的 git 现场。
	//
	// ★ 右栏「领先几个 commit」与验收时的 diff 都要 BaseCommit 当起点，
	// 不记的话「这个工作到底改了什么」没有答案。
	Branch     string
	BaseCommit string
}

// Service 是工作用例。
type Service struct {
	repo      port.WorkRepo
	worktrees port.Worktrees
	// status 探测开工前的仓库状态。为 nil 时 Prepare 报错——
	// **不返回一个空状态**：那会让弹层显示「仓库很干净」而实际没查过。
	status    port.RepoStatusProbe
	bus       port.WorkEventBus
	ids       port.IDGen
	runner    port.AgentRunner
	canceller port.AgentCanceller
	// requirements 存需求快照。可以为 nil（只跑 API 冒烟时），
	// 那时工作照建，只是没有需求版本——**不是让整轮对话失败**。
	requirements port.Requirements
	// plans 存计划版本。可以为 nil，那时产不出计划但工作照建。
	plans port.Plans
	// contracts 存单元契约。边界判定要靠它——为 nil 时一律判「说不清」，
	// **不是**「没问题」。
	contracts port.Contracts

	// cancelling 记着「哪些工作正在被用户主动停」。
	// 后台那一轮据此区分「用户停的」与「AI 跑挂了」。
	cancelMu   sync.Mutex
	cancelling map[string]bool
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
// Start 开一个工作。
//
// ★ baseRef 是用户选的基线（分支名或 commit），留空时用仓库当前 HEAD。
func (s *Service) Start(ctx context.Context, project, prompt, baseRef string) (View, error) {
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
	// ★★ **先把用户自己说的那句话记下来**，排在一切之前。
	//
	// 不发的话，对话页上只有 AI 的回复——用户看不到自己说了什么。
	// 而「它有没有听懂我」正是靠两句话对照着看出来的：
	// 他说「先别写代码」，AI 上来就改文件，这个对照是他唯一的判据。
	//
	// ★ 排在 state_change 之前：那句话是**最先发生的事**。
	s.emit(ctx, id, "user_message", map[string]any{"text": prompt})
	s.emit(ctx, id, "state_change", map[string]any{"to": string(w.State())})

	// ★ 用户那句话就是需求快照 v1 的第一条。
	// 记在跑之前：这一轮产出的事件要盖上「说这句话时需求是 v1」。
	s.recordSaid(ctx, id, prompt)

	wt, err := s.worktrees.CreateWorktree(ctx, project, id, baseRef)
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

	// ★★ **切好之后立刻记下现场**，且在状态迁移之前。
	//
	// 记晚了的话，中间任何一次失败都会留下一个「有 worktree 但不知道
	// 基线在哪」的工作——那时右栏算不出「AI 干了什么」，
	// 而用户看到的是一个空面板。
	w.SetWorktree(wt.Path, wt.Branch, wt.BaseCommit)

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
		Project: project, Worktree: wt.Path, Prompt: prompt,
		Branch: wt.Branch, BaseCommit: wt.BaseCommit,
	}

	s.runTurn(ctx, id, wt.Path, prompt, w.State())

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
func (s *Service) runTurn(
	ctx context.Context, workID, worktree, prompt string, state constant.WorkState,
) {
	s.runTurnWith(ctx, workID, worktree, prompt, state, nil)
}

// runTurnWith 跑一轮，并在结束时把 Agent 说过的话交给 onReply。
//
// ★ onReply 在**后台那个 goroutine 里**被调用：调用方不许在里面做慢操作，
// 也不许假设自己还在原来的请求上下文里。
func (s *Service) runTurnWith(
	ctx context.Context, workID, worktree, prompt string,
	state constant.WorkState, onReply func(string),
) {
	if s.runner == nil {
		return
	}
	turnCtx := context.WithoutCancel(ctx)

	go func() {
		// 这一轮结束了，取消标记就没用了。放在 goroutine 里而不是
		// Cancel 里——只有它知道自己什么时候真的跑完。
		defer s.clearCancelling(workID)

		// ★ 需求版本在**这一轮开始时**读一次，盖在它产出的每条事件上。
		// 放在 goroutine 里而不是请求线程上：这是一次 IO，
		// 而用户点完「发送」该立刻看到界面动起来。
		version, frozen := s.requirementOf(turnCtx, workID)

		err := s.runner.RunTurn(turnCtx, port.AgentTurn{
			WorkID: workID,
			// ★★ **角色决定这条会话有多大权限。** 澄清需求阶段是
			// 需求分析师——只读，它读得到代码与记忆但一个字节都写不了。
			// 不传的话 acp 层退到实现工程师（受控写），
			// 那意味着用户以为自己只是在聊天，而对面能改他的文件。
			RoleID:       roleForState(state),
			OnReply:      onReply,
			SystemPrompt: systemPromptFor(roleForState(state)),
			// ★ 传的是工作自己的 worktree，不是用户的项目目录——
			// 后者等于让 AI 直接在他的分支上改文件。
			Cwd:                worktree,
			Prompt:             prompt,
			RequirementVersion: version,
			RequirementFrozen:  frozen,
		})
		if err == nil {
			return
		}

		// ★ 用户主动停的**不算失败**。两者都表现为 RunTurn 返回错误，
		// 但对用户是完全不同的两件事——他明明是自己点的停，
		// 界面却说「失败」。真机走查撞到过。
		if s.isCancelling(workID) {
			return
		}

		// ★★ 排队满 / 排队时被放弃**同样不算失败**。
		//
		// 推到 failed 的话，用户只是手快点了几下就得重开一个工作——
		// 而他那几句话其实一句都没丢，只是没排上。
		if errors.Is(err, port.ErrTurnQueueFull) || errors.Is(err, port.ErrTurnAbandoned) {
			s.emit(turnCtx, workID, "turn_end", map[string]any{
				"reason": "queue_full", "detail": err.Error(),
			})
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
