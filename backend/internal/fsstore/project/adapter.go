package project

import "github.com/HuLuca1998/acp-flows/backend/internal/app/port"

// Store 实现 port.ProjectInitializer。
//
// ★ 它只做形状翻译。计划与执行的逻辑在 MakePlan / Apply，别在这里再来一套。
type Store struct{}

// PlanInit 算出计划。**一个字节都不写。**
func (Store) PlanInit(path string) (port.InitPlan, error) {
	p, err := MakePlan(path)
	if err != nil {
		return port.InitPlan{}, err
	}
	out := port.InitPlan{
		Root:      p.Root,
		IsGitRepo: p.IsGitRepo,
		Actions:   make([]port.InitAction, 0, len(p.Actions)),
	}
	for _, a := range p.Actions {
		out.Actions = append(out.Actions, port.InitAction{
			Kind:         string(a.Kind),
			Path:         a.Path,
			Reason:       a.Reason,
			Lines:        a.Lines,
			AlreadyThere: a.AlreadyThere,
		})
	}
	return out, nil
}

// ApplyInit 照计划执行。
func (Store) ApplyInit(plan port.InitPlan) error {
	p := &Plan{
		Root:      plan.Root,
		IsGitRepo: plan.IsGitRepo,
		Actions:   make([]Action, 0, len(plan.Actions)),
	}
	for _, a := range plan.Actions {
		p.Actions = append(p.Actions, Action{
			Kind:         ActionKind(a.Kind),
			Path:         a.Path,
			Reason:       a.Reason,
			Lines:        a.Lines,
			AlreadyThere: a.AlreadyThere,
		})
	}
	return Apply(p)
}
