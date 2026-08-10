package work

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// SetRequirements 装上需求快照仓储。
//
// ★ 单独一个 setter 而不是塞进 New：`work.New` 已经有五个参数，
// 再加就到了「调用方数不清第几个是什么」的程度。而这一条是可选的——
// 只跑 API 冒烟时没有它，那时工作照建，只是没有需求版本。
func (s *Service) SetRequirements(r port.Requirements) { s.requirements = r }

// requirementOf 读出一个工作当前的需求版本，供事件盖章用。
//
// ★ **读不到不是错**：没装配、还没有快照、库挂了——三种情况下
// 用户都应该照常看到消息，只是消息头上没有 `requirement vN` 那枚标签。
// 因为读不出版本号就让整轮对话失败，是拿一个装饰品换掉了主体。
func (s *Service) requirementOf(ctx context.Context, workID string) (version int, frozen bool) {
	if s.requirements == nil {
		return 0, false
	}
	req, err := s.requirements.LatestRequirement(ctx, workID)
	if err != nil {
		if !errors.Is(err, model.ErrNotFound) {
			slog.Warn("读需求版本失败", "work", workID, "err", err)
		}
		return 0, false
	}
	return req.Version(), req.Frozen()
}

// recordSaid 把用户说的一句话记进需求快照。
//
// ★★ 当前阶段**需求条目就是用户说过的话**。
//
// 需求分析师把它精炼成可验证条目要等 AI 的结构化产出（M6+）——
// 在那之前，「用户说过什么」是我们能给出的最诚实的一版需求：
// 它不编造、不猜测，用户回头核对时看到的就是自己的原话。
//
// 三种走法：
//
//   - 还没有快照 → 造 v1 草稿
//   - 当前版**未冻结** → 追加进这一版（追问一轮就改一次，不该升版本号）
//   - 当前版**已冻结** → 修订出 v(n+1)：冻结之后又提新要求，那就是改需求
//
// ★ 出错只记日志不往上报：需求快照是给用户看的一层记录，
// 而他那句话已经发给 AI 了。因为记录失败就让整轮对话失败，
// 用户会看到「发送失败」而实际上 AI 已经在干活了——那是更糟的谎。
func (s *Service) recordSaid(ctx context.Context, workID, said string) {
	if s.requirements == nil {
		return
	}

	next, err := s.nextRequirement(ctx, workID, said)
	if err != nil {
		slog.Warn("记录需求快照失败", "work", workID, "err", err)
		return
	}
	if err := s.requirements.SaveRequirement(ctx, next); err != nil {
		slog.Warn("保存需求快照失败", "work", workID, "err", err)
	}
}

func (s *Service) nextRequirement(
	ctx context.Context, workID, said string,
) (*model.RequirementSnapshot, error) {
	cur, err := s.requirements.LatestRequirement(ctx, workID)
	if errors.Is(err, model.ErrNotFound) {
		return model.NewRequirement(workID, []string{said}, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("查最新需求: %w", err)
	}

	items := append(cur.Items(), said)
	if cur.Frozen() {
		// 冻结之后又提新要求 —— 那就是改需求，出新版本（旧版原样留着）
		return cur.Revise(items, cur.OpenFacts())
	}
	// 还在追问的过程中：原地改，不升版本号
	if reviseErr := cur.ReviseDraft(items, cur.OpenFacts()); reviseErr != nil {
		return nil, fmt.Errorf("改需求草稿: %w", reviseErr)
	}
	return cur, nil
}

// FreezeRequirement 冻结一个工作当前的需求版本。
//
// ★★ **由用户点，不由 AI 判断**。AI 说「我觉得问清楚了」和用户说
// 「就这样」是两件事——而冻结之后这一版就进了计划与契约，改不动了。
//
// ★ 还有待确认的事实时会被领域层拒（INV-REQ-1）：带着没问清的问题往下走，
// AI 会自己替用户做决定，而那些决定会一路固化进计划与契约。
func (s *Service) FreezeRequirement(ctx context.Context, workID string) error {
	if s.requirements == nil {
		return fmt.Errorf("%w: 没有配置需求存储", ErrRequirementsUnavailable)
	}

	req, err := s.requirements.LatestRequirement(ctx, workID)
	if err != nil {
		return fmt.Errorf("查最新需求 %s: %w", workID, err)
	}
	if err := req.Freeze(); err != nil {
		return fmt.Errorf("冻结需求 %s v%d: %w", workID, req.Version(), err)
	}
	if err := s.requirements.SaveRequirement(ctx, req); err != nil {
		return fmt.Errorf("保存需求 %s v%d: %w", workID, req.Version(), err)
	}
	// ★ **不发事件。** 设计稿里「已冻结」是消息头上的一枚标签
	// （「说这句话时需求是 v2 且已冻结」），不是时间线上的一条独立记录。
	// 发一条的话，时间线上会多出一条设计稿里没有的东西（铁律 3）。
	// 界面自己知道用户点了冻结，回读一次即可。
	return nil
}

// ErrRequirementsUnavailable 表示没装配需求存储。
//
// ★ 明确报错而不是静静成功：静静成功的话用户点了「冻结」，界面显示已冻结，
// 而库里什么都没有——下次打开又变回未冻结。
var ErrRequirementsUnavailable = errors.New("work: 没有配置需求存储")

// RequirementView 是交给上层的需求视图。
type RequirementView struct {
	Version   int
	Items     []string
	OpenFacts []string
	Frozen    bool
}

// RequirementOf 读出一个工作的当前需求。查不到返回 model.ErrNotFound。
func (s *Service) RequirementOf(ctx context.Context, workID string) (RequirementView, error) {
	if s.requirements == nil {
		return RequirementView{}, fmt.Errorf("%w: %s", ErrRequirementsUnavailable, workID)
	}
	req, err := s.requirements.LatestRequirement(ctx, workID)
	if err != nil {
		return RequirementView{}, fmt.Errorf("查需求 %s: %w", workID, err)
	}
	return RequirementView{
		Version:   req.Version(),
		Items:     req.Items(),
		OpenFacts: req.OpenFacts(),
		Frozen:    req.Frozen(),
	}, nil
}
