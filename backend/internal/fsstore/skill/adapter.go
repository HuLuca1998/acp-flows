package skill

import (
	"path/filepath"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// Store 实现 port.SkillScanner。
//
// ★ 它只做两件事：定位目录、把 Entry 翻成 port 的形状。
// 扫描与校验的逻辑在 Scan / domain，别在这里再来一套。
type Store struct {
	// Home 是 `~/.acpflows`。
	Home string
}

// ScanGlobal 扫全局 Skill 库。
func (s Store) ScanGlobal() ([]port.SkillEntry, error) {
	root := filepath.Join(s.Home, "skills")
	entries, err := Scan(Options{
		Root:  root,
		Scope: model.SkillScopeGlobal,
		// ★ 来源报**真实路径**，不写死 `~/.acpflows/skills`。
		//
		// 写死的话，开发态（`~/.duet-dev/.acpflows`）与任何自定义数据目录下
		// 界面都会告诉用户一个不存在的路径——他照着去找，发现那儿什么都没有，
		// 然后以为是应用坏了。真机走查时就是这么发现的。
		Source: root,
	})
	if err != nil {
		return nil, err
	}

	return toPortEntries(entries), nil
}

// DiscoverInProject 找出项目里已有的 skill。
func (s Store) DiscoverInProject(projectPath string) ([]port.SkillEntry, error) {
	entries, err := Discover(projectPath)
	if err != nil {
		return nil, err
	}
	return toPortEntries(entries), nil
}

// toPortEntries 把扫描结果翻成 port 的形状。
//
// ★ 空集合返回**空切片而不是 nil**：nil 会序列化成 `null`，
// 而前端拿到 null 会崩在 `.map` 上——空态本身是最常见的状态。
func toPortEntries(entries []Entry) []port.SkillEntry {
	out := make([]port.SkillEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, port.SkillEntry{
			Name:             e.Name,
			Dir:              e.Dir,
			Version:          e.Version,
			Description:      e.Description,
			Compatibility:    e.Compatibility,
			Scope:            string(e.Scope),
			Source:           e.Source,
			Status:           string(e.Status),
			ValidationOK:     e.Validation.OK,
			ValidationReason: e.Validation.Reason,
		})
	}
	return out
}
