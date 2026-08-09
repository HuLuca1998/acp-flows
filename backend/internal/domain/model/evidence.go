package model

import (
	"errors"
	"fmt"
	"strings"
)

// EvidenceKind 是证据的类别。**封闭四类**，与设计稿一致。
type EvidenceKind string

const (
	// EvidenceDiff 是 git diff。
	EvidenceDiff EvidenceKind = "diff"
	// EvidenceTest 是测试输出。
	EvidenceTest EvidenceKind = "test"
	// EvidenceCommand 是命令记录。
	EvidenceCommand EvidenceKind = "command"
	// EvidenceReview 是审查意见。
	EvidenceReview EvidenceKind = "review"
)

var allEvidenceKinds = []EvidenceKind{
	EvidenceDiff, EvidenceTest, EvidenceCommand, EvidenceReview,
}

// EvidenceSource 说这条证据**是谁采集的**。
type EvidenceSource string

const (
	// EvidenceFromApp 是应用直接采集的：读 git、跑命令、收原始输出。
	//
	// ★★ 这是 M8 的全部意义。让 AI 报告自己干了什么，等于让被考核的人
	// 填自己的考勤表——它不需要撒谎，只需要「记错了」一次，
	// 用户就再也不知道该信哪一条。
	EvidenceFromApp EvidenceSource = "app"
	// EvidenceFromAgent 是 AI 转述的。
	//
	// ★ 留着这一类不是为了用它，是为了**把它标出来**：
	// 分不出来源的话，一条转述会和一份真 diff 长得一样。
	EvidenceFromAgent EvidenceSource = "agent"
)

var (
	// ErrUnknownEvidenceKind 表示类别不在封闭的四类里。
	ErrUnknownEvidenceKind = errors.New("model: 证据类别不在四类里")
	// ErrEvidenceSourceRequired 表示没写谁采集的。
	//
	// ★★ 留空的话，一条 AI 转述会和一份应用采集的 diff 长得一样——
	// 而用户判断「该不该信」全靠这一个字段。
	ErrEvidenceSourceRequired = errors.New("model: 证据必须写明是谁采集的")
)

// Evidence 是一条证据。
//
// ★★ **不可变**：字段全私有，只有值接收者的读方法。证据被改写过的话，
// 它就不再是证据了——「当时到底跑出了什么」没有第二个地方可查。
type Evidence struct {
	id     string
	unitID string
	kind   EvidenceKind
	source EvidenceSource
	// summary 是一行摘要（`3 个文件 +64 −12`），列表里显示它。
	summary string
	// body 是原始输出。★ **原样存**，不截断不美化——
	// 截断过的输出在排查时等于没有。
	body string
	// criteria 是这条证据支持哪几条验收标准。
	//
	// ★ 一条证据可以支持多条标准（一次测试跑通了三条），
	// 反过来一条标准也可能要几条证据。所以是多对多。
	criteria []string
}

// NewEvidence 造一条证据。
func NewEvidence(
	id, unitID string, kind EvidenceKind, source EvidenceSource,
	summary, body string, criteria []string,
) (Evidence, error) {
	if strings.TrimSpace(id) == "" {
		return Evidence{}, errors.New("model: 证据必须有标识")
	}
	if !kind.IsValid() {
		return Evidence{}, fmt.Errorf("%w: %q", ErrUnknownEvidenceKind, kind)
	}
	// ★★ 来源必填：留空的话一条 AI 转述会和一份应用采集的 diff 长得一样，
	// 而用户判断「该不该信」全靠这一个字段。
	if source != EvidenceFromApp && source != EvidenceFromAgent {
		return Evidence{}, fmt.Errorf("%w: %q", ErrEvidenceSourceRequired, source)
	}
	return Evidence{
		id: id, unitID: unitID, kind: kind, source: source,
		summary: strings.TrimSpace(summary), body: body,
		criteria: trimAll(criteria),
	}, nil
}

// IsValid 报告这个类别在不在封闭的四类里。
func (k EvidenceKind) IsValid() bool {
	for _, v := range allEvidenceKinds {
		if v == k {
			return true
		}
	}
	return false
}

// ID 返回证据标识（形如 ev-441）。
func (e Evidence) ID() string { return e.id }

// UnitID 返回它属于哪个单元。
func (e Evidence) UnitID() string { return e.unitID }

// Kind 返回类别。
func (e Evidence) Kind() EvidenceKind { return e.kind }

// Source 返回是谁采集的。
func (e Evidence) Source() EvidenceSource { return e.source }

// Summary 返回一行摘要。
func (e Evidence) Summary() string { return e.summary }

// Body 返回原始输出。★ 原样，不截断。
func (e Evidence) Body() string { return e.body }

// Criteria 返回它支持哪几条验收标准。★ 副本。
func (e Evidence) Criteria() []string { return append([]string(nil), e.criteria...) }

// Trustworthy 报告这条证据是不是应用直接采集的。
//
// ★★ 界面据此区分显示：AI 转述的那条要**标出来**，
// 而不是和真 diff 混在一起——用户判断「该不该信」全靠这一点。
func (e Evidence) Trustworthy() bool { return e.source == EvidenceFromApp }

// CriteriaCoverage 算「每条标准有没有证据」。
//
// ★★ 返回的是**标准 → 证据 id 列表**，没有证据的那条对应空切片——
// 而不是把它从结果里去掉。去掉的话，调用方会以为「所有标准都有证据」，
// 而实际上是它们根本没出现在这张表里。
func CriteriaCoverage(criteria []Criterion, evidence []Evidence) map[string][]string {
	out := make(map[string][]string, len(criteria))
	for _, c := range criteria {
		out[c.ID] = []string{}
	}
	for _, e := range evidence {
		for _, cID := range e.criteria {
			// ★ 只认契约里真有的那些标准：证据指向一条不存在的标准时
			// 静静收下的话，「已覆盖 5 条」里会有一条根本不在契约里
			if _, ok := out[cID]; ok {
				out[cID] = append(out[cID], e.id)
			}
		}
	}
	return out
}
