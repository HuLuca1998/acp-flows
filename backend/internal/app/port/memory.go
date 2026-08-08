package port

import (
	"context"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// MemoryFilter 是记忆列表的筛选条件。零值表示不筛。
type MemoryFilter struct {
	// Scope 为空时不按归属筛；传 `*` 只取跨项目记忆。
	Scope string
	// Status 为空时不按状态筛。
	Status string
}

// MemoryRepo 读写记忆的**索引与状态**。
//
// ★★ **没有 Delete**（INV-MEM-6）：失效不等于删除。
// 历史运行仍要能追溯当时用过哪条记忆。
//
// ★★ **不碰正文**（INV-MEM-8）：正文只在 md 文件里。
type MemoryRepo interface {
	ListMemories(ctx context.Context, f MemoryFilter) ([]*model.Memory, error)
	FindMemory(ctx context.Context, id string) (*model.Memory, error)
	SaveMemory(ctx context.Context, m *model.Memory) error
}
