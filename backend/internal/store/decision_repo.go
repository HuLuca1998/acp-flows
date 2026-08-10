package store

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
	"github.com/HuLuca1998/acp-flows/backend/internal/store/entity"
)

// DecisionRepo 是决策的持久化实现。
//
// ★★ **没有 Delete**，而 Update 只有一条路：作答。答过的决策一个字都不能改。
type DecisionRepo struct {
	db  *gorm.DB
	clk Clock
}

const (
	decisionColumns = "id, work_id, unit_id, level, question, " +
		"recommended, answered_with, created_at"
	decisionOptionColumns = "decision_id, id, text, impact, ord"
)

// SaveDecision 存一条决策，连同它的选项。
//
// ★★ 已经答过的**不许改写**：改了的话「他当时选了什么」就没有答案。
func (r *DecisionRepo) SaveDecision(ctx context.Context, d *model.Decision) error {
	var existing entity.Decision
	err := r.db.WithContext(ctx).Select(decisionColumns).
		First(&existing, "id = ?", d.ID()).Error

	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		// 新的一条，往下走
	case err != nil:
		return fmt.Errorf("store: 查决策 %s: %w", d.ID(), err)
	case existing.AnsweredWith != "":
		// ★ 已答：只有原样重存才放行（重试与手快点两下都是常态）
		if existing.AnsweredWith == d.AnsweredWith() {
			return nil
		}
		return fmt.Errorf("%w: %s（当时选的是 %s）",
			model.ErrDecisionAnswered, d.ID(), existing.AnsweredWith)
	}

	now := r.clk.Now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row := &entity.Decision{
			ID: d.ID(), WorkID: d.WorkID(), UnitID: d.UnitID(),
			Level: string(d.Level()), Question: d.Question(),
			Recommended: d.Recommended(), AnsweredWith: d.AnsweredWith(),
			CreatedAt: now,
		}
		if saveErr := tx.Save(row).Error; saveErr != nil {
			return fmt.Errorf("store: 存决策 %s: %w", d.ID(), saveErr)
		}
		// 选项整组重写（未答时才走到这里，那时它还是草稿）
		if delErr := tx.Where("decision_id = ?", d.ID()).
			Delete(&entity.DecisionOption{}).Error; delErr != nil {
			return fmt.Errorf("store: 清选项 %s: %w", d.ID(), delErr)
		}
		for i, o := range d.Options() {
			if createErr := tx.Create(&entity.DecisionOption{
				DecisionID: d.ID(), ID: o.ID, Text: o.Text, Impact: o.Impact, Ord: i,
			}).Error; createErr != nil {
				return fmt.Errorf("store: 存选项 %s: %w", o.ID, createErr)
			}
		}
		return nil
	})
}

// FindDecision 取一条决策。查不到返回 model.ErrNotFound。
func (r *DecisionRepo) FindDecision(ctx context.Context, id string) (*model.Decision, error) {
	var row entity.Decision
	err := r.db.WithContext(ctx).Select(decisionColumns).First(&row, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("%w: 决策 %s", model.ErrNotFound, id)
	}
	if err != nil {
		return nil, fmt.Errorf("store: 查决策 %s: %w", id, err)
	}
	return r.hydrate(ctx, row)
}

// PendingDecisions 列出一个工作**还没答**的决策。
//
// ★★ 左栏那个亮蓝点靠它：不列的话，用户不知道有件事在等他，
// 而工作停在 `waiting_user` 永远不动。
func (r *DecisionRepo) PendingDecisions(
	ctx context.Context, workID string,
) ([]*model.Decision, error) {
	var rows []entity.Decision
	if err := r.db.WithContext(ctx).Select(decisionColumns).
		Where("work_id = ? AND answered_with = ''", workID).
		Order("created_at, id").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("store: 列出待决策 %s: %w", workID, err)
	}

	// ★ 空集合返回空切片而不是 nil：「没有待决策」是常态
	out := make([]*model.Decision, 0, len(rows))
	for _, row := range rows {
		d, err := r.hydrate(ctx, row)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

func (r *DecisionRepo) hydrate(
	ctx context.Context, row entity.Decision,
) (*model.Decision, error) {
	var optRows []entity.DecisionOption
	if err := r.db.WithContext(ctx).Select(decisionOptionColumns).
		Where("decision_id = ?", row.ID).Order("ord").Find(&optRows).Error; err != nil {
		return nil, fmt.Errorf("store: 查选项 %s: %w", row.ID, err)
	}
	options := make([]model.DecisionOption, 0, len(optRows))
	for _, o := range optRows {
		options = append(options, model.DecisionOption{ID: o.ID, Text: o.Text, Impact: o.Impact})
	}

	// ★ 走 RestoreDecision 而不是 NewDecision：后者校验「至少两个选项」
	// 这类规则，而这里读的是已经存过的东西——校验变严时不该让老数据读不出来。
	return model.RestoreDecision(
		row.ID, row.WorkID, row.UnitID, model.DecisionLevel(row.Level),
		row.Question, options, row.Recommended, row.AnsweredWith,
	), nil
}
