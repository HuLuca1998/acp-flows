package reply

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// ErrNoMemoryInReply 表示 AI 的回复里没有可解析的记忆候选。
//
// ★ 这**不是**一个需要打断用户的错误：绝大多数轮次本来就不该产出记忆。
// 调用方拿到它应该安静地跳过，而不是弹一句「解析失败」。
var ErrNoMemoryInReply = errors.New("work: AI 的回复里没有记忆候选")

// MemoryFence 是记忆候选专属的围栏标记。
//
// ★ 与 `duet-plan` `duet-contract` 分开：一轮回复里可能同时讲到计划与
// 「这个仓库的迁移必须手写 SQL」这条经验，共用标记的话会挑错那一段。
const MemoryFence = "duet-memory"

// memoryPayload 是 AI 要输出的形状。
//
// ★★ **这里故意没有 status 字段。**
//
// AI 写不出 `active`——不是因为我们把它降级了，而是因为压根没有那个入口：
// 解析出来的东西一律交给 `model.ProposeCandidate`，它只造 candidate。
// 留一个 status 字段再去过滤的话，总有一天有人会「顺手支持一下」。
type memoryPayload struct {
	// Kind 是 constraint / experience / fact 之一。
	Kind string `json:"kind"`
	// Scope 只收 `project` / `cross_project` 两个语义值。
	//
	// ★ **AI 不填项目名**：`model.MemoryScope` 存的是项目名本身，
	// 而 AI 不该也不必知道我们内部管这个项目叫什么。它只回答
	// 「这条经验只在这里成立，还是到哪都成立」。
	Scope string `json:"scope"`
	// Title 是一句话摘要，进 md 的 frontmatter。
	Title string `json:"title"`
	// Text 是正文——用户在审核时读的就是这段。
	Text string `json:"text"`
	// SourceRefs 指向 Evidence 或 Unit（INV-MEM：记忆必须有出处）。
	SourceRefs []string `json:"source_refs"`
}

// MemoryInstructions 是拼在 prompt 末尾、告诉 AI 怎么提记忆的那段话。
//
// ★★ 明写「**没有就不要输出**」：不写这句的话，它每轮都会硬凑一条，
// 而用户的记忆库很快会被「这个项目用 Go 写」这种废话塞满，
// 真正有用的那几条就淹掉了。
func MemoryInstructions() string {
	return "\n\n如果这一轮里出现了**值得记住的经验**（下次遇到同类问题能少走弯路的），" +
		"在回复最后用 ```" + MemoryFence + " 围栏输出一条，形如：\n\n```" + MemoryFence + `
{
  "kind": "constraint",
  "scope": "project",
  "title": "这个仓库的迁移必须手写 SQL",
  "text": "gorm AutoMigrate 会把 events 表的 role 列改成 NOT NULL，而旧数据里那列是空的——启动直接失败。",
  "source_refs": ["unit-013"]
}
` + "```\n\n**没有就不要输出这段。** 硬凑一条的话，用户的记忆库会被废话塞满，" +
		"真正有用的那几条反而淹掉了。它是不是 active 由用户决定，你不用管。"
}

// MemoryDraft 是从回复里解析出来的一条候选，正文另外落到 md 文件。
type MemoryDraft struct {
	Kind model.MemoryKind
	// CrossProject 报告这条经验是不是到哪个项目都成立。
	//
	// ★ 这里**不给 MemoryScope**：项目内的 scope 是项目名，
	// 而解析层不知道当前项目叫什么。编一个字面量填进去的话，
	// 记忆库里会出现一个叫「project」的假项目。由调用方补。
	CrossProject bool
	Title        string
	Text         string
	SourceRefs   []string
}

// ParseMemoryReply 从 AI 的回复里抠出记忆候选。
//
// ★ 没有围栏时返回 `ErrNoMemoryInReply`——那是**常态**，不是故障。
func ParseMemoryReply(reply string) (MemoryDraft, error) {
	raw, ok := extractFenced(reply, MemoryFence)
	if !ok {
		return MemoryDraft{}, ErrNoMemoryInReply
	}

	var p memoryPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		// ★★ 带上原话：静默失败的话，用户永远不知道 AI 其实想记点什么。
		return MemoryDraft{}, fmt.Errorf("%w: JSON 解析失败: %v: 原话: %s",
			ErrNoMemoryInReply, err, truncate(raw))
	}

	kind := model.MemoryKind(strings.TrimSpace(p.Kind))
	if !kind.IsValid() {
		return MemoryDraft{}, fmt.Errorf("%w: %w: %q", ErrNoMemoryInReply,
			model.ErrInvalidMemoryKind, p.Kind)
	}
	if strings.TrimSpace(p.Text) == "" {
		// 没有正文的候选，用户在审核界面上看到的是一个空白卡片——无从判断收不收
		return MemoryDraft{}, fmt.Errorf("%w: 正文是空的", ErrNoMemoryInReply)
	}

	// ★★ 缺省是**项目内**，不是跨项目：认不出的取值也算项目内。
	// 一条只在这个仓库成立的经验跑到别的项目去，会让那边的 AI
	// 照着一条错误的前提干活——而他很难想到问题出在一条老记忆上。
	crossProject := strings.TrimSpace(p.Scope) == "cross_project"

	return MemoryDraft{
		Kind:         kind,
		CrossProject: crossProject,
		Title:        strings.TrimSpace(p.Title),
		Text:         strings.TrimSpace(p.Text),
		SourceRefs:   p.SourceRefs,
	}, nil
}

// truncate 截断原话，免得一条错误日志刷屏。
func truncate(s string) string {
	const limit = 200
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "…"
}
