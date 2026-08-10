package work

import (
	"context"
	"errors"
	"fmt"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/internal/constant"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// ErrDecisionsUnavailable 表示没装配决策存储。
var ErrDecisionsUnavailable = errors.New("work: 没有配置决策存储")

// SetDecisions 装上决策存储。
func (s *Service) SetDecisions(d port.Decisions) { s.decisions = d }

// DecisionOptionView / DecisionView 是交给上层的决策视图。
type DecisionOptionView struct {
	ID   string
	Text string
	// Impact 是「选了它会怎样」。★ 界面上必须显示——没有它用户在盲选。
	Impact string
	// Recommended 说这个是不是 AI 推荐的那个。
	//
	// ★★ **只是标记，不是预选**：预选中的话，用户会顺手点确定——
	// 而那正好绕过了「让他自己决定」这件事。
	Recommended bool
}

// DecisionView 是一条待决策。
type DecisionView struct {
	ID       string
	UnitID   string
	Level    string
	Question string
	Options  []DecisionOptionView
	// AnsweredWith 空表示还没答。
	AnsweredWith string
}

// AskDecision 提一条要用户拿主意的决定。
//
// ★★ D2/D3 时工作进入 `waiting_user`——**AI 停在这里等他**。
// 不停的话，它会带着一个自己选的答案往下做几十个文件。
func (s *Service) AskDecision(ctx context.Context, d *model.Decision) error {
	if s.decisions == nil {
		return fmt.Errorf("%w: %s", ErrDecisionsUnavailable, d.WorkID())
	}
	if err := s.decisions.SaveDecision(ctx, d); err != nil {
		return err
	}

	// ★ 只有 D2/D3 才停下来等。D0/D1 记一笔就够了——
	// 全停的话，用户会被一堆「用哪个变量名」的问题烦死。
	if d.Level().NeedsUser() {
		if w, err := s.repo.FindWork(ctx, d.WorkID()); err == nil {
			if tErr := w.Transition(constant.WorkStateWaitingUser); tErr == nil {
				_ = s.repo.SaveWork(ctx, w)
				s.emit(ctx, d.WorkID(), "state_change",
					map[string]any{"to": string(w.State()), "decision": d.ID()})
			}
		}
	}

	options := make([]map[string]any, 0, len(d.Options()))
	for _, o := range d.Options() {
		options = append(options, map[string]any{
			"id": o.ID, "text": o.Text, "impact": o.Impact,
			"recommended": o.ID == d.Recommended(),
		})
	}
	s.emit(ctx, d.WorkID(), "decision", map[string]any{
		"id": d.ID(), "level": string(d.Level()), "unit": d.UnitID(),
		"question": d.Question(), "options": options,
	})
	return nil
}

// AnswerDecision 记下用户的选择。
//
// ★★ **只能答一次**（领域层守着）。答完工作从 `waiting_user` 回到执行态——
// 不回的话它会一直停在那儿，而用户以为自己已经放行了。
func (s *Service) AnswerDecision(ctx context.Context, workID, decisionID, optionID string) error {
	if s.decisions == nil {
		return fmt.Errorf("%w: %s", ErrDecisionsUnavailable, workID)
	}

	d, err := s.decisions.FindDecision(ctx, decisionID)
	if err != nil {
		return fmt.Errorf("查决策 %s: %w", decisionID, err)
	}
	if err := d.Answer(optionID); err != nil {
		return err
	}
	if err := s.decisions.SaveDecision(ctx, d); err != nil {
		return err
	}
	s.emit(ctx, workID, "decision", map[string]any{
		"id": d.ID(), "answered_with": optionID,
	})

	// ★ 还有别的待决策就**继续等**——一次答一条，剩下的还没答完。
	pending, err := s.decisions.PendingDecisions(ctx, workID)
	if err == nil && len(pending) == 0 {
		if w, findErr := s.repo.FindWork(ctx, workID); findErr == nil &&
			w.State() == constant.WorkStateWaitingUser {
			if tErr := w.Transition(constant.WorkStateExecuting); tErr == nil {
				_ = s.repo.SaveWork(ctx, w)
				s.emit(ctx, workID, "state_change", map[string]any{"to": string(w.State())})
			}
		}
	}
	return nil
}

// PendingDecisionsOf 列出一个工作还没答的决策。
func (s *Service) PendingDecisionsOf(ctx context.Context, workID string) ([]DecisionView, error) {
	if s.decisions == nil {
		return nil, fmt.Errorf("%w: %s", ErrDecisionsUnavailable, workID)
	}
	list, err := s.decisions.PendingDecisions(ctx, workID)
	if err != nil {
		return nil, fmt.Errorf("列出待决策 %s: %w", workID, err)
	}

	out := make([]DecisionView, 0, len(list))
	for _, d := range list {
		out = append(out, toDecisionView(d))
	}
	return out, nil
}

func toDecisionView(d *model.Decision) DecisionView {
	view := DecisionView{
		ID: d.ID(), UnitID: d.UnitID(), Level: string(d.Level()),
		Question: d.Question(), AnsweredWith: d.AnsweredWith(),
		Options: make([]DecisionOptionView, 0, len(d.Options())),
	}
	for _, o := range d.Options() {
		view.Options = append(view.Options, DecisionOptionView{
			ID: o.ID, Text: o.Text, Impact: o.Impact,
			Recommended: o.ID == d.Recommended(),
		})
	}
	return view
}
