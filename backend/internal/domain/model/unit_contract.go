package model

import (
	"errors"
	"fmt"
)

// 契约相关的错误。
var (
	// ErrContractFrozen 表示契约已冻结，不能再改。
	//
	// ★ 冻结的意思是「这就是这次要做的事，说定了」。之后还能改的话，
	// AI 可以一边做一边把标准改成自己刚好达到的样子。
	ErrContractFrozen = errors.New("model: 契约已冻结")
	// ErrContractEmpty 表示契约一条验收标准都没有。
	ErrContractEmpty = errors.New("model: 契约至少要有一条验收标准")
	// ErrContractVersionNotNext 表示版本号不是「当前 + 1」。
	ErrContractVersionNotNext = errors.New("model: 契约版本号必须严格递增且不跳号")
)

// Criterion 是一条验收标准。
type Criterion struct {
	ID   string
	Text string
}

// UnitContract 是一个单元的契约：这次要做什么、凭什么说做完了。
//
// ★ 冻结之前可以加标准，冻结之后**什么都改不了**——要改就出新版本
// （`Revise`），旧的那份原样留着。
type UnitContract struct {
	unitID   string
	version  int
	criteria []Criterion
	frozen   bool
}

// NewUnitContract 造一份没冻结的契约。
func NewUnitContract(unitID string, version int) *UnitContract {
	return &UnitContract{unitID: unitID, version: version}
}

// UnitID 返回单元标识。
func (c *UnitContract) UnitID() string { return c.unitID }

// Version 返回版本号。
func (c *UnitContract) Version() int { return c.version }

// IsFrozen 报告是否已冻结。
func (c *UnitContract) IsFrozen() bool { return c.frozen }

// Criteria 返回验收标准的副本——返回内部切片的话调用方能改掉它。
func (c *UnitContract) Criteria() []Criterion {
	out := make([]Criterion, len(c.criteria))
	copy(out, c.criteria)
	return out
}

// AddCriterion 加一条验收标准。冻结之后返回 ErrContractFrozen。
func (c *UnitContract) AddCriterion(id, text string) error {
	if c.frozen {
		return fmt.Errorf("%w: %s v%d", ErrContractFrozen, c.unitID, c.version)
	}
	c.criteria = append(c.criteria, Criterion{ID: id, Text: text})
	return nil
}

// Freeze 冻结契约。**幂等**：用户点两下「冻结」是常态。
//
// ★ 一条验收标准都没有时拒绝冻结：空契约冻结之后，「做完了」这件事
// 没有任何判据——AI 说做完了就是做完了。
func (c *UnitContract) Freeze() error {
	if c.frozen {
		return nil
	}
	if len(c.criteria) == 0 {
		return fmt.Errorf("%w: %s v%d", ErrContractEmpty, c.unitID, c.version)
	}
	c.frozen = true
	return nil
}

// Revise 基于这份契约造下一版。
//
// ★ 新版本是**没冻结**的（可以继续加标准），而**旧的那份一个字都不变**——
// 那是「修订」与「改写」的全部区别。
func (c *UnitContract) Revise(version int) (*UnitContract, error) {
	if version != c.version+1 {
		return nil, fmt.Errorf("%w: 当前 v%d，下一版只能是 v%d，给的是 v%d",
			ErrContractVersionNotNext, c.version, c.version+1, version)
	}
	next := NewUnitContract(c.unitID, version)
	next.criteria = c.Criteria() // 副本，改新的不影响旧的
	return next, nil
}
