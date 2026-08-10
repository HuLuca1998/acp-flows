package work

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// ErrNoStatusProbeForEvidence 表示没装配 git 探针，采不了 diff。
//
// ★★ 明确报错而不是造一条空证据：空证据会让「验收证据 1 条」这个数
// 变成假的——用户以为有东西可看，点开是空的。
var ErrNoStatusProbeForEvidence = errors.New("work: 没有配置 git 探针，采不了 diff 证据")

// SetEvidence 装上证据存储。
func (s *Service) SetEvidence(e port.Evidence) { s.evidence = e }

// CollectDiffEvidence 为一个单元采集 **git diff** 证据。
//
// ★★ **应用自己去读，不问 AI**（M8 第 4 条）。让 AI 报告自己改了什么，
// 等于让被考核的人填自己的考勤表——它不需要撒谎，只需要「记错了」一次，
// 用户就再也不知道该信哪一条。
//
// ★ 采集是**只读**的：`ProbeWorktree` 只跑 `git status` / `git diff`，
// 一个字节都不写。
func (s *Service) CollectDiffEvidence(
	ctx context.Context, workID, unitID string, criteria []string,
) (model.Evidence, error) {
	if s.status == nil {
		return model.Evidence{}, fmt.Errorf("%w: %s", ErrNoStatusProbeForEvidence, workID)
	}
	w, err := s.repo.FindWork(ctx, workID)
	if err != nil {
		return model.Evidence{}, fmt.Errorf("查工作 %s: %w", workID, err)
	}

	st, err := s.status.ProbeWorktreeState(ctx, w.WorktreePath(), w.BaseCommit())
	if err != nil {
		// ★★ 采不到就**说出来**，不留一条空证据：空证据会让
		// 「验收证据 1 条」这个数变成假的——用户以为有东西可看，
		// 点开是空的，而他不会再信这个数。
		s.emit(ctx, workID, "evidence", map[string]any{
			"unit": unitID, "kind": string(model.EvidenceDiff),
			"failed": true, "reason": err.Error(),
		})
		return model.Evidence{}, fmt.Errorf("采集 diff 证据 %s: %w", unitID, err)
	}

	ev, err := model.NewEvidence(
		s.ids.NextID("ev"), unitID, model.EvidenceDiff,
		// ★★ 这条是应用读出来的，所以是 app——**不是** agent
		model.EvidenceFromApp,
		diffSummary(st), diffBody(st), criteria,
	)
	if err != nil {
		return model.Evidence{}, fmt.Errorf("造 diff 证据: %w", err)
	}

	if s.evidence != nil {
		if saveErr := s.evidence.SaveEvidence(ctx, workID, ev); saveErr != nil {
			return model.Evidence{}, fmt.Errorf("保存证据 %s: %w", ev.ID(), saveErr)
		}
	}
	s.emit(ctx, workID, "evidence", map[string]any{
		"unit": unitID, "id": ev.ID(), "kind": string(ev.Kind()),
		"source": string(ev.Source()), "summary": ev.Summary(),
	})
	return ev, nil
}

// diffSummary 造一行摘要：`3 个文件 +64 −12`。
//
// ★ 增删**分开数**：合成一个「76 行改动」的话，
// 「删了 64 行」和「加了 64 行」长得一样，而那是两件很不同的事。
func diffSummary(st port.WorktreeState) string {
	var added, removed int
	for _, c := range st.Changes {
		added += c.Added
		removed += c.Removed
	}
	return fmt.Sprintf("%d 个文件 +%d −%d", len(st.Changes), added, removed)
}

// diffBody 把逐文件的改动摊成原始文本。
func diffBody(st port.WorktreeState) string {
	var b strings.Builder
	for _, c := range st.Changes {
		fmt.Fprintf(&b, "%s\t+%d\t−%d\n", c.Path, c.Added, c.Removed)
	}
	return b.String()
}

// EvidenceView 是交给上层的证据视图。
type EvidenceView struct {
	ID      string
	UnitID  string
	Kind    string
	Source  string
	Summary string
	Body    string
	// Trustworthy 说这条是不是应用直接采集的。
	//
	// ★★ 界面据此把 AI 转述的标出来——分不出来源的话，
	// 一条转述会和一份真 diff 长得一样。
	Trustworthy bool
	Criteria    []string
}

// AcceptanceView 是「N 条证据 / M 条标准」那一屏。
type AcceptanceView struct {
	UnitID string
	// Criteria 是契约里的验收标准，**顺序照契约**。
	Criteria []CriterionCoverage
	Evidence []EvidenceView
}

// CriterionCoverage 是一条标准与它的证据。
type CriterionCoverage struct {
	ID   string
	Text string
	// EvidenceIDs 为空表示**没有证据**——设计稿的 `○ 无证据`。
	//
	// ★★ 空**不等于通过**：把没证据当成通过的话，
	// 一个什么都没做的单元也能「全部通过」。
	EvidenceIDs []string
}

// AcceptanceOf 把契约的标准与采到的证据对上。
func (s *Service) AcceptanceOf(ctx context.Context, workID, unitID string) (AcceptanceView, error) {
	if s.contracts == nil {
		return AcceptanceView{}, fmt.Errorf("%w: %s", ErrContractsUnavailable, unitID)
	}
	c, err := s.contracts.LatestContract(ctx, unitID)
	if err != nil {
		return AcceptanceView{}, fmt.Errorf("查契约 %s: %w", unitID, err)
	}

	var evs []model.Evidence
	if s.evidence != nil {
		evs, err = s.evidence.EvidenceOf(ctx, workID, unitID)
		if err != nil {
			return AcceptanceView{}, fmt.Errorf("查证据 %s: %w", unitID, err)
		}
	}

	cov := model.CriteriaCoverage(c.Criteria(), evs)
	view := AcceptanceView{
		UnitID:   unitID,
		Criteria: make([]CriterionCoverage, 0, len(c.Criteria())),
		Evidence: make([]EvidenceView, 0, len(evs)),
	}
	// ★ 顺序照**契约**，不照 map：map 的遍历顺序每次都不同，
	// 而用户是照着契约那张表一条条核对的。
	for _, crit := range c.Criteria() {
		ids := cov[crit.ID]
		if ids == nil {
			ids = []string{}
		}
		view.Criteria = append(view.Criteria, CriterionCoverage{
			ID: crit.ID, Text: crit.Text, EvidenceIDs: ids,
		})
	}
	for _, e := range evs {
		view.Evidence = append(view.Evidence, EvidenceView{
			ID: e.ID(), UnitID: e.UnitID(), Kind: string(e.Kind()),
			Source: string(e.Source()), Summary: e.Summary(), Body: e.Body(),
			Trustworthy: e.Trustworthy(), Criteria: e.Criteria(),
		})
	}
	return view, nil
}
