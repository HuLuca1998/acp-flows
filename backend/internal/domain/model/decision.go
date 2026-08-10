package model

import (
	"errors"
	"fmt"
	"strings"
)

// DecisionLevel 是决定的等级，**封闭四级**（D0–D3）。
//
// ★ 等级决定「要不要停下来问」：D0/D1 是 AI 自己就能定的，
// D2/D3 必须问用户。分不出等级的话，要么什么都问（用户被烦死），
// 要么什么都不问（他在几十个文件之后才发现）。
type DecisionLevel string

const (
	// DecisionD0 是不影响外部行为的实现细节，AI 自己定。
	DecisionD0 DecisionLevel = "D0"
	// DecisionD1 是有取舍但可逆的，AI 定了**记一笔**。
	DecisionD1 DecisionLevel = "D1"
	// DecisionD2 是改变外部行为或验收标准的，**必须问用户**。
	DecisionD2 DecisionLevel = "D2"
	// DecisionD3 是要回滚已验收工作、或改需求的，**必须问用户**。
	DecisionD3 DecisionLevel = "D3"
)

var allDecisionLevels = []DecisionLevel{
	DecisionD0, DecisionD1, DecisionD2, DecisionD3,
}

var (
	// ErrUnknownDecisionLevel 表示等级不在 D0–D3 里。
	ErrUnknownDecisionLevel = errors.New("model: 决定等级不在 D0–D3 里")

	// ErrNotEnoughOptions 表示选项少于两个。
	//
	// ★★ 一个选项的「决策」不是在问，是在**通知**——而通知不该占用
	// 用户「停下来做个决定」的注意力。
	ErrNotEnoughOptions = errors.New("model: 决策至少要有两个选项")

	// ErrOptionNeedsImpact 表示某个选项没写影响。
	//
	// ★★ 没有影响说明的选项，用户是在**盲选**：他看到三个名字，
	// 而不知道选哪个会发生什么。
	ErrOptionNeedsImpact = errors.New("model: 每个选项都要写明影响")

	// ErrUnknownRecommendation 表示「推荐」指向了不存在的选项。
	ErrUnknownRecommendation = errors.New("model: 推荐指向了不存在的选项")

	// ErrDecisionAnswered 表示这条决策已经答过了。
	//
	// ★★ 答过还能改的话，「他当时选了什么」就没有答案——
	// 而后面几十个文件的改动都是照着那个选择做的。
	ErrDecisionAnswered = errors.New("model: 这条决策已经答过了")
)

// DecisionOption 是一个可选项。
type DecisionOption struct {
	ID   string
	Text string
	// Impact 是**选了它会怎样**。
	//
	// ★★ 必填：没有影响说明的话用户在盲选——他看到三个名字，
	// 而不知道选哪个会发生什么。
	Impact string
}

// Decision 是一次要用户拿主意的决定。
//
// ★★ **不可变**（除了作答一次）：答过还能改的话，「他当时选了什么」
// 就没有答案，而后面几十个文件的改动都是照着那个选择做的。
type Decision struct {
	id       string
	workID   string
	unitID   string
	level    DecisionLevel
	question string
	options  []DecisionOption
	// recommended 是 AI 推荐的那个选项 id。
	//
	// ★ 可以为空（它也拿不准）。**但不为空时必须指向存在的选项**——
	// 指向不存在的话，界面上那个「推荐」标记会落在谁身上说不清。
	recommended string
	// answeredWith 是用户选的那个；空表示**还没答**（「稍后决定」）。
	answeredWith string
}

// NewDecision 造一条待决策。
func NewDecision(
	id, workID, unitID string, level DecisionLevel,
	question string, options []DecisionOption, recommended string,
) (*Decision, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("model: 决策必须有标识")
	}
	if !level.IsValid() {
		return nil, fmt.Errorf("%w: %q", ErrUnknownDecisionLevel, level)
	}
	if strings.TrimSpace(question) == "" {
		return nil, errors.New("model: 决策必须说清在问什么")
	}
	// ★★ 至少两个选项：一个选项的「决策」是在通知，不是在问。
	if len(options) < 2 {
		return nil, fmt.Errorf("%w: %s 只有 %d 个", ErrNotEnoughOptions, id, len(options))
	}

	known := make(map[string]bool, len(options))
	for _, o := range options {
		if strings.TrimSpace(o.ID) == "" {
			return nil, errors.New("model: 选项必须有标识")
		}
		// ★★ 影响必填——没有它用户在盲选
		if strings.TrimSpace(o.Impact) == "" {
			return nil, fmt.Errorf("%w: 选项 %s", ErrOptionNeedsImpact, o.ID)
		}
		known[o.ID] = true
	}
	if recommended != "" && !known[recommended] {
		return nil, fmt.Errorf("%w: %q", ErrUnknownRecommendation, recommended)
	}

	return &Decision{
		id: id, workID: workID, unitID: unitID, level: level,
		question:    strings.TrimSpace(question),
		options:     append([]DecisionOption(nil), options...),
		recommended: recommended,
	}, nil
}

// RestoreDecision 从持久化状态重建，供 store 层用。
//
// ★ 与 NewDecision 分开：后者校验「至少两个选项」这类规则，
// 而这里读的是已经存过的东西——校验变严时不该让老数据读不出来。
func RestoreDecision(
	id, workID, unitID string, level DecisionLevel,
	question string, options []DecisionOption, recommended, answeredWith string,
) *Decision {
	return &Decision{
		id: id, workID: workID, unitID: unitID, level: level,
		question:    question,
		options:     append([]DecisionOption(nil), options...),
		recommended: recommended, answeredWith: answeredWith,
	}
}

// IsValid 报告这个等级在不在 D0–D3 里。
func (l DecisionLevel) IsValid() bool {
	for _, v := range allDecisionLevels {
		if v == l {
			return true
		}
	}
	return false
}

// NeedsUser 报告这个等级**必须问用户**。
//
// ★★ D2/D3 是「改变外部行为」与「回滚已验收的东西」——
// AI 自己定的话，用户是在几十个文件之后才发现。
func (l DecisionLevel) NeedsUser() bool {
	return l == DecisionD2 || l == DecisionD3
}

// ID 返回决策标识（形如 dec-01）。
func (d *Decision) ID() string { return d.id }

// WorkID 返回所属工作。
func (d *Decision) WorkID() string { return d.workID }

// UnitID 返回所属单元；可以为空（计划层面的决定）。
func (d *Decision) UnitID() string { return d.unitID }

// Level 返回等级。
func (d *Decision) Level() DecisionLevel { return d.level }

// Question 返回在问什么。
func (d *Decision) Question() string { return d.question }

// Options 返回选项。★ 副本。
func (d *Decision) Options() []DecisionOption {
	return append([]DecisionOption(nil), d.options...)
}

// Recommended 返回 AI 推荐的选项 id；空表示它也拿不准。
func (d *Decision) Recommended() string { return d.recommended }

// AnsweredWith 返回用户选的选项 id；空表示**还没答**。
func (d *Decision) AnsweredWith() string { return d.answeredWith }

// Answered 报告答过没有。
func (d *Decision) Answered() bool { return d.answeredWith != "" }

// Answer 记下用户的选择。
//
// ★★ **只能答一次**：答过还能改的话，「他当时选了什么」就没有答案，
// 而后面几十个文件的改动都是照着那个选择做的。
func (d *Decision) Answer(optionID string) error {
	if d.Answered() {
		return fmt.Errorf("%w: %s（当时选的是 %s）",
			ErrDecisionAnswered, d.id, d.answeredWith)
	}
	for _, o := range d.options {
		if o.ID == optionID {
			d.answeredWith = optionID
			return nil
		}
	}
	// ★ 选一个不存在的选项要报错，**不静默收下**：
	// 收下的话，「他选了什么」会变成一个谁都不认识的字符串。
	return fmt.Errorf("model: 决策 %s 里没有选项 %q", d.id, optionID)
}
