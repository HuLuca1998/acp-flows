package agent

import (
	"context"
	"log/slog"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
)

// busSink 把事件转发到总线。
type busSink struct {
	bus port.WorkEventBus
	ctx context.Context
	log *slog.Logger
	// role / roleName / runtime 盖在**每一条**从这条会话发出去的事件上。
	//
	// ★★ 在这里盖而不是让每个 emit 点自己填：漏填一处的话，
	// 界面上那条消息就没有角色标签，而用户会以为它是「系统」说的。
	role     string
	roleName string
	runtime  string
	// reqVersion / reqFrozen 是**这一轮开始时**需求的样子，同样盖在每条事件上。
	//
	// ★ 一轮之内需求不会变（用户是在两轮之间改需求的），所以读一次就够。
	// 每条事件各查一次的话，一段流式文本能打出上百次查询。
	reqVersion int
	reqFrozen  bool
}

// Emit 实现 Sink。
//
// ★ 发不出去只记一条日志，**不让这一轮失败**。事件是给界面看的，
// 而 AI 那边的活已经干了——因为通知发不出去就报错的话，
// 用户看到「失败」而磁盘上躺着一堆已经改好的文件，比不通知更糟。
func (s busSink) Emit(e WorkEvent) {
	if s.bus == nil {
		return
	}
	// ★ 只在事件自己没带角色时盖——将来某个事件想说明「这是另一个角色
	// 产出的」时，不该被这里覆盖掉。
	if e.Role == "" {
		e.Role, e.RoleDisplayName, e.Runtime = s.role, s.roleName, s.runtime
	}
	// ★ 同理只在事件自己没带时盖。
	if e.RequirementVersion == 0 {
		e.RequirementVersion, e.RequirementFrozen = s.reqVersion, s.reqFrozen
	}
	if err := s.bus.PublishWorkEvent(s.ctx, e); err != nil {
		s.log.Warn("事件发不到总线", "type", e.Type, "work_id", e.WorkID, "err", err)
	}
}

// systemPromptOf 挑这一轮该用的开场白。
//
// ★ 每轮自带的优先于 runner 上那份全局的：一个 runner 服务所有角色，
// 而**开场白是按角色变的**——用全局那份的话，需求分析师和实现工程师
// 收到一模一样的一段话，角色库里那八张卡片就只是界面上的装饰。
func systemPromptOf(t port.AgentTurn, fallback string) string {
	if t.SystemPrompt != "" {
		return t.SystemPrompt
	}
	return fallback
}
